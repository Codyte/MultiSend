package proto

import (
	"bytes"
	"testing"
)

func TestWriteReadHeader(t *testing.T) {
	h := Header{MessageType: "chunk_start", TransferID: "t1", FileName: "backup.zip", PartIndex: 1, TotalParts: 2, PartSize: 123, ChunkIndex: 0, Offset: 0, TotalBytes: 456}
	var b bytes.Buffer
	if err := WriteHeader(&b, h); err != nil {
		t.Fatalf("write: %v", err)
	}
	got, err := ReadHeader(&b)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if got.FileName != h.FileName || got.PartSize != h.PartSize || got.TransferID != h.TransferID || got.ChunkIndex != h.ChunkIndex {
		t.Fatalf("got %+v want %+v", got, h)
	}
}

func TestWriteReadChunkAck(t *testing.T) {
	ack := ChunkAck{Type: "chunk_ack", TransferID: "t1", ChunkIndex: 2, Status: "done", BytesReceived: 100}
	var b bytes.Buffer
	if err := WriteChunkAck(&b, ack); err != nil {
		t.Fatalf("write ack: %v", err)
	}
	got, err := ReadChunkAck(&b)
	if err != nil {
		t.Fatalf("read ack: %v", err)
	}
	if got.TransferID != ack.TransferID || got.ChunkIndex != ack.ChunkIndex || got.Status != ack.Status {
		t.Fatalf("got %+v want %+v", got, ack)
	}
}
