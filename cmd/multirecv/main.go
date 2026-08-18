package main

import (
	"crypto/sha256"
	"encoding/hex"
	"flag"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/Codyte/MultiSend/internal/manifest"
	"github.com/Codyte/MultiSend/internal/proto"
)

var manifestMu sync.Mutex

func main() {
	listen := flag.String("listen", "0.0.0.0", "listen host")
	port := flag.Int("port", 9000, "listen port")
	outDir := flag.String("output", ".", "output directory")
	flag.Parse()

	if err := os.MkdirAll(*outDir, 0o755); err != nil {
		exitf("mkdir output: %v", err)
	}

	ln, err := net.Listen("tcp", fmt.Sprintf("%s:%d", *listen, *port))
	if err != nil {
		exitf("listen: %v", err)
	}
	defer ln.Close()
	fmt.Printf("listening on %s:%d\n", *listen, *port)

	for {
		conn, err := ln.Accept()
		if err != nil {
			fmt.Fprintf(os.Stderr, "accept: %v\n", err)
			continue
		}
		go handle(conn, *outDir)
	}
}

func handle(conn net.Conn, outDir string) {
	defer conn.Close()
	h, err := proto.ReadHeader(conn)
	if err != nil {
		fmt.Fprintf(os.Stderr, "read header: %v\n", err)
		return
	}
	if err := validateChunkHeader(h); err != nil {
		writeChunkError(conn, h, err)
		return
	}
	if !proto.VerifyHeader(h, []byte(os.Getenv("MULTISEND_NODE_SECRET"))) {
		writeChunkError(conn, h, fmt.Errorf("authentication failed"))
		return
	}

	name := filepath.Base(h.FileName)
	sessionDir := filepath.Join(outDir, fmt.Sprintf("%s_%s", name, h.TransferID))
	if err := os.MkdirAll(sessionDir, 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "mkdir session: %v\n", err)
		return
	}
	tmp, err := os.CreateTemp(sessionDir, ".incoming-*.tmp")
	if err != nil {
		writeChunkError(conn, h, fmt.Errorf("create temporary chunk: %w", err))
		return
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)

	hasher := sha256.New()
	n, err := io.CopyN(io.MultiWriter(tmp, hasher), conn, h.PartSize)
	if err != nil {
		_ = tmp.Close()
		fmt.Fprintf(os.Stderr, "copy data: %v\n", err)
		return
	}
	if err := tmp.Close(); err != nil {
		writeChunkError(conn, h, fmt.Errorf("close temporary chunk: %w", err))
		return
	}
	if got := hex.EncodeToString(hasher.Sum(nil)); !strings.EqualFold(got, h.ChunkSHA256) {
		writeChunkError(conn, h, fmt.Errorf("chunk hash mismatch"))
		return
	}

	manifestMu.Lock()
	err = publishChunk(sessionDir, name, tmpPath, h, n)
	manifestMu.Unlock()
	if err != nil {
		writeChunkError(conn, h, err)
		return
	}
	if err := proto.WriteChunkAck(conn, proto.ChunkAck{Type: "chunk_ack", TransferID: h.TransferID, ChunkIndex: h.ChunkIndex, Status: "done", BytesReceived: n}); err != nil {
		fmt.Fprintf(os.Stderr, "write ack: %v\n", err)
		return
	}

	fmt.Printf("received %s chunk %d/%d (%d bytes)\n", name, h.PartIndex, h.TotalParts, h.PartSize)
}

func publishChunk(sessionDir, name, tmpPath string, h proto.Header, n int64) error {
	mfPath := filepath.Join(sessionDir, "manifest.json")
	m := &manifest.Manifest{SchemaVersion: 1, TransferID: h.TransferID, Type: "p2p", FileName: name, TotalBytes: h.TotalBytes, ChunkSize: h.PartSize, Chunks: []manifest.ChunkState{{Index: h.ChunkIndex, Offset: h.Offset, Size: h.PartSize, Status: manifest.StatusDone, BytesDone: n}}}
	if existing, lerr := manifest.Load(mfPath); lerr == nil {
		if existing.TransferID != h.TransferID || existing.FileName != name || existing.TotalBytes != h.TotalBytes {
			return fmt.Errorf("manifest identity mismatch")
		}
		m = existing
		found := false
		for i := range m.Chunks {
			if m.Chunks[i].Index == h.ChunkIndex {
				m.Chunks[i].Status = manifest.StatusDone
				m.Chunks[i].BytesDone = n
				found = true
				break
			}
		}
		if !found {
			m.Chunks = append(m.Chunks, manifest.ChunkState{Index: h.ChunkIndex, Offset: h.Offset, Size: h.PartSize, Status: manifest.StatusDone, BytesDone: n})
		}
	}
	outPath := filepath.Join(sessionDir, fmt.Sprintf("%s.chunk%06d", name, h.ChunkIndex+1))
	if err := os.Remove(outPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove existing chunk: %w", err)
	}
	if err := os.Rename(tmpPath, outPath); err != nil {
		return fmt.Errorf("publish chunk: %w", err)
	}
	if err := manifest.SaveAtomic(mfPath, m); err != nil {
		return fmt.Errorf("save manifest: %w", err)
	}
	return nil
}

func validateChunkHeader(h proto.Header) error {
	if len(h.TransferID) == 0 || len(h.TransferID) > 128 || strings.IndexFunc(h.TransferID, func(r rune) bool {
		return !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_')
	}) >= 0 {
		return fmt.Errorf("invalid transfer_id")
	}
	if h.TotalParts > 1_000_000 || h.ChunkIndex < 0 || h.ChunkIndex >= int64(h.TotalParts) || h.PartIndex != int(h.ChunkIndex)+1 {
		return fmt.Errorf("invalid chunk position")
	}
	if h.TotalBytes < 0 || h.Offset < 0 || h.PartSize > h.TotalBytes || h.Offset > h.TotalBytes-h.PartSize {
		return fmt.Errorf("invalid chunk bounds")
	}
	if len(h.ChunkSHA256) != sha256.Size*2 {
		return fmt.Errorf("chunk_sha256 is required")
	}
	if _, err := hex.DecodeString(h.ChunkSHA256); err != nil {
		return fmt.Errorf("invalid chunk_sha256")
	}
	return nil
}

func writeChunkError(conn net.Conn, h proto.Header, err error) {
	_ = proto.WriteChunkAck(conn, proto.ChunkAck{Type: "chunk_ack", TransferID: h.TransferID, ChunkIndex: h.ChunkIndex, Status: "failed", Error: err.Error()})
	fmt.Fprintf(os.Stderr, "reject chunk: %v\n", err)
}

func exitf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
