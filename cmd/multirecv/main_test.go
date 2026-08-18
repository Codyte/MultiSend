package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/Codyte/MultiSend/internal/manifest"
	"github.com/Codyte/MultiSend/internal/proto"
)

func sendToHandle(outDir string, h proto.Header, payload []byte) (proto.ChunkAck, error) {
	client, server := net.Pipe()
	_ = client.SetDeadline(time.Now().Add(5 * time.Second))
	_ = server.SetDeadline(time.Now().Add(5 * time.Second))
	done := make(chan struct{})
	go func() {
		handle(server, outDir)
		close(done)
	}()
	go func() {
		if err := proto.WriteHeader(client, h); err == nil {
			_, _ = client.Write(payload)
		}
	}()
	ack, err := proto.ReadChunkAck(client)
	if err != nil {
		_ = client.Close()
		<-done
		return proto.ChunkAck{}, err
	}
	_ = client.Close()
	<-done
	return ack, nil
}

func chunkHeader(transferID string, index int64, payload []byte) proto.Header {
	sum := sha256.Sum256(payload)
	return proto.Header{
		TransferID:  transferID,
		FileName:    "test.bin",
		PartIndex:   int(index) + 1,
		TotalParts:  2,
		PartSize:    int64(len(payload)),
		ChunkIndex:  index,
		Offset:      index * int64(len(payload)),
		TotalBytes:  int64(len(payload) * 2),
		ChunkSHA256: hex.EncodeToString(sum[:]),
	}
}

func TestHandlePersistsConcurrentChunksSafely(t *testing.T) {
	outDir := t.TempDir()
	payloads := [][]byte{[]byte("left"), []byte("rght")}
	acks := make([]proto.ChunkAck, len(payloads))
	errs := make([]error, len(payloads))
	var wg sync.WaitGroup
	for i := range payloads {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			acks[i], errs[i] = sendToHandle(outDir, chunkHeader("tx-safe", int64(i), payloads[i]), payloads[i])
		}(i)
	}
	wg.Wait()
	for i, ack := range acks {
		if errs[i] != nil {
			t.Fatalf("chunk %d: %v", i, errs[i])
		}
		if ack.Status != "done" || ack.ChunkIndex != int64(i) || ack.BytesReceived != int64(len(payloads[i])) {
			t.Fatalf("ack %d = %+v", i, ack)
		}
	}
	sessionDir := filepath.Join(outDir, "test.bin_tx-safe")
	mf, err := manifest.Load(filepath.Join(sessionDir, "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	if len(mf.Chunks) != 2 {
		t.Fatalf("manifest chunks = %d", len(mf.Chunks))
	}
	for i, payload := range payloads {
		got, err := os.ReadFile(filepath.Join(sessionDir, fmt.Sprintf("test.bin.chunk%06d", i+1)))
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(got, payload) {
			t.Fatalf("chunk %d = %q", i, got)
		}
	}
}

func TestHandleRejectsTraversalAndHashMismatch(t *testing.T) {
	outDir := t.TempDir()
	payload := []byte("left")
	h := chunkHeader("..\\escape", 0, payload)
	ack, err := sendToHandle(outDir, h, payload)
	if err != nil || ack.Status != "failed" {
		t.Fatalf("traversal ack = %+v", ack)
	}
	h = chunkHeader("tx-hash", 0, payload)
	h.ChunkSHA256 = hex.EncodeToString(make([]byte, sha256.Size))
	ack, err = sendToHandle(outDir, h, payload)
	if err != nil || ack.Status != "failed" {
		t.Fatalf("hash ack = %+v", ack)
	}
	t.Setenv("MULTISEND_NODE_SECRET", "expected-secret")
	h = chunkHeader("tx-auth", 0, payload)
	proto.SignHeader(&h, []byte("wrong-secret"))
	ack, err = sendToHandle(outDir, h, payload)
	if err != nil || ack.Status != "failed" {
		t.Fatalf("authentication ack = %+v", ack)
	}
	entries, err := os.ReadDir(outDir)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			t.Fatalf("unexpected output file: %s", entry.Name())
		}
	}
}

func TestValidateChunkHeaderBounds(t *testing.T) {
	t.Parallel()
	payload := []byte("left")
	valid := chunkHeader("tx-valid", 0, payload)
	if err := validateChunkHeader(valid); err != nil {
		t.Fatalf("valid header: %v", err)
	}
	for _, mutate := range []func(*proto.Header){
		func(h *proto.Header) { h.TransferID = "" },
		func(h *proto.Header) { h.ChunkIndex = 2 },
		func(h *proto.Header) { h.Offset = h.TotalBytes },
		func(h *proto.Header) { h.ChunkSHA256 = "bad" },
	} {
		h := valid
		mutate(&h)
		if err := validateChunkHeader(h); err == nil {
			t.Fatalf("invalid header accepted: %+v", h)
		}
	}
}

func TestPublishChunkPreservesCorruptManifest(t *testing.T) {
	sessionDir := t.TempDir()
	manifestPath := filepath.Join(sessionDir, "manifest.json")
	if err := os.WriteFile(manifestPath, []byte("{"), 0o600); err != nil {
		t.Fatalf("write corrupt manifest: %v", err)
	}
	tmp, err := os.CreateTemp(sessionDir, ".incoming-*.tmp")
	if err != nil {
		t.Fatalf("create temporary chunk: %v", err)
	}
	payload := []byte("left")
	if _, err := tmp.Write(payload); err != nil {
		t.Fatalf("write temporary chunk: %v", err)
	}
	if err := tmp.Close(); err != nil {
		t.Fatalf("close temporary chunk: %v", err)
	}
	h := chunkHeader("tx-corrupt", 0, payload)
	if err := publishChunk(sessionDir, h.FileName, tmp.Name(), h, int64(len(payload))); err == nil {
		t.Fatal("expected corrupt manifest error")
	}
	if raw, err := os.ReadFile(manifestPath); err != nil || string(raw) != "{" {
		t.Fatalf("corrupt manifest was overwritten: data=%q err=%v", raw, err)
	}
	if _, err := os.Stat(filepath.Join(sessionDir, "test.bin.chunk000001")); !os.IsNotExist(err) {
		t.Fatalf("chunk should not be published, err=%v", err)
	}
}
