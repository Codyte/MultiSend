package main

import (
	"flag"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"

	"lab/multinet/internal/manifest"
	"lab/multinet/internal/proto"
)

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

	name := filepath.Base(h.FileName)
	sessionDir := filepath.Join(outDir, fmt.Sprintf("%s_%s", name, h.TransferID))
	if err := os.MkdirAll(sessionDir, 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "mkdir session: %v\n", err)
		return
	}
	outPath := filepath.Join(sessionDir, fmt.Sprintf("%s.chunk%06d", name, h.ChunkIndex+1))
	_ = os.Remove(outPath)
	f, err := os.Create(outPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "create file: %v\n", err)
		return
	}
	defer f.Close()

	n, err := io.CopyN(f, conn, h.PartSize)
	if err != nil {
		fmt.Fprintf(os.Stderr, "copy data: %v\n", err)
		return
	}

	_ = proto.WriteChunkAck(conn, proto.ChunkAck{Type: "chunk_ack", TransferID: h.TransferID, ChunkIndex: h.ChunkIndex, Status: "done", BytesReceived: n})
	mfPath := filepath.Join(sessionDir, "manifest.json")
	m := &manifest.Manifest{SchemaVersion: 1, TransferID: h.TransferID, Type: "p2p", FileName: name, TotalBytes: h.TotalBytes, ChunkSize: h.PartSize, Chunks: []manifest.ChunkState{{Index: h.ChunkIndex, Offset: h.Offset, Size: h.PartSize, Status: manifest.StatusDone, BytesDone: n}}}
	if existing, lerr := manifest.Load(mfPath); lerr == nil {
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
	_ = manifest.SaveAtomic(mfPath, m)

	fmt.Printf("received %s (%d bytes)\n", filepath.Base(outPath), h.PartSize)
}

func exitf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
