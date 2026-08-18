package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net"
	"os"
	"testing"

	"github.com/Codyte/MultiSend/internal/proto"
)

func TestSendPartRequiresMatchingAck(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	payload := []byte("payload")
	h := proto.Header{TransferID: "tx-test", FileName: "test.bin", PartIndex: 1, TotalParts: 1, PartSize: int64(len(payload)), TotalBytes: int64(len(payload))}
	done := make(chan error, 1)
	go func() {
		conn, err := listener.Accept()
		if err != nil {
			done <- err
			return
		}
		defer conn.Close()
		got, err := proto.ReadHeader(conn)
		if err == nil {
			_, err = io.CopyN(io.Discard, conn, got.PartSize)
		}
		if err == nil {
			err = proto.WriteChunkAck(conn, proto.ChunkAck{TransferID: got.TransferID, ChunkIndex: got.ChunkIndex, Status: "done", BytesReceived: got.PartSize})
		}
		done <- err
	}()
	if err := sendPart(context.Background(), listener.Addr().String(), "127.0.0.1", h, bytes.NewReader(payload)); err != nil {
		t.Fatalf("sendPart: %v", err)
	}
	if err := <-done; err != nil {
		t.Fatalf("receiver: %v", err)
	}

	listener2, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener2.Close()
	go func() {
		conn, err := listener2.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		got, err := proto.ReadHeader(conn)
		if err == nil {
			_, _ = io.CopyN(io.Discard, conn, got.PartSize)
			_ = proto.WriteChunkAck(conn, proto.ChunkAck{TransferID: got.TransferID, ChunkIndex: got.ChunkIndex, Status: "failed", Error: "rejected"})
		}
	}()
	if err := sendPart(context.Background(), listener2.Addr().String(), "127.0.0.1", h, bytes.NewReader(payload)); err == nil {
		t.Fatal("sendPart accepted failed ack")
	}
}

func TestHashSectionAndFormattingHelpers(t *testing.T) {
	t.Parallel()
	file, err := os.CreateTemp(t.TempDir(), "hash-*.bin")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	if _, err := file.Write([]byte("abcdef")); err != nil {
		t.Fatal(err)
	}
	got, err := hashSection(file, 1, 3)
	if err != nil {
		t.Fatal(err)
	}
	wantSum := sha256.Sum256([]byte("bcd"))
	if want := hex.EncodeToString(wantSum[:]); got != want {
		t.Fatalf("hash = %s, want %s", got, want)
	}
	if a, b, err := parseRatio("2:1"); err != nil || a != 2 || b != 1 {
		t.Fatalf("parseRatio = %d, %d, %v", a, b, err)
	}
	if _, _, err := parseRatio("0:1"); err == nil {
		t.Fatal("invalid ratio accepted")
	}
	if got := bar(5, 10, 4); got != "[##--]" {
		t.Fatalf("bar = %q", got)
	}
}
