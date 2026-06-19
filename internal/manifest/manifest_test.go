package manifest

import (
	"path/filepath"
	"testing"
	"time"
)

func sampleManifest() *Manifest {
	now := time.Now().UTC()
	return &Manifest{
		SchemaVersion: 1,
		TransferID:    "t1",
		Type:          "p2p",
		FileName:      "f.bin",
		TotalBytes:    100,
		ChunkSize:     50,
		CreatedAt:     now,
		UpdatedAt:     now,
		Chunks: []ChunkState{
			{Index: 0, Offset: 0, Size: 50, Status: StatusPending},
			{Index: 1, Offset: 50, Size: 50, Status: StatusPending},
		},
	}
}

func TestSaveLoadAtomic(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "a.manifest.json")
	m := sampleManifest()
	if err := SaveAtomic(path, m); err != nil {
		t.Fatalf("save: %v", err)
	}
	got, err := Load(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if got.TransferID != m.TransferID {
		t.Fatalf("got %s", got.TransferID)
	}
}

func TestStatusTransitions(t *testing.T) {
	m := sampleManifest()
	m.MarkSending(0, "cable")
	if m.Chunks[0].Status != StatusSending {
		t.Fatal("want sending")
	}
	m.MarkDone(0, 50, "hash")
	if m.Chunks[0].Status != StatusDone || m.Chunks[0].BytesDone != 50 {
		t.Fatal("want done")
	}
	m.MarkFailed(1, "err")
	if m.Chunks[1].Status != StatusFailed || m.Chunks[1].Attempts != 1 {
		t.Fatal("want failed")
	}
}

func TestDoneNotRegressAfterLoad(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "a.manifest.json")
	m := sampleManifest()
	m.MarkDone(0, 50, "hash")
	m.MarkSending(1, "wifi")
	m.MarkFailed(1, "x")
	if err := SaveAtomic(path, m); err != nil {
		t.Fatalf("save: %v", err)
	}
	got, err := Load(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if got.Chunks[0].Status != StatusDone {
		t.Fatalf("done regressed: %s", got.Chunks[0].Status)
	}
	if got.Chunks[1].Status != StatusPending {
		t.Fatalf("failed should become pending for retry: %s", got.Chunks[1].Status)
	}
}
