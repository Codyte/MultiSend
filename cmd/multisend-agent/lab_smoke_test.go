package main

import (
	"path/filepath"
	"testing"
)

func TestValidateLabReceivePath(t *testing.T) {
	base := filepath.Join(`C:\Users\Public`, "Downloads", "MultiSend")
	okPath := filepath.Join(base, "_lab", "receive", "session-a")
	got, err := validateLabReceivePath(base, okPath)
	if err != nil {
		t.Fatalf("expected valid path, got err: %v", err)
	}
	if got != okPath {
		t.Fatalf("expected %s got %s", okPath, got)
	}
}

func TestValidateLabReceivePathRejectOutside(t *testing.T) {
	base := filepath.Join(`C:\Users\Public`, "Downloads", "MultiSend")
	bad := filepath.Join(`C:\Users\Public`, "Downloads", "Other", "x")
	if _, err := validateLabReceivePath(base, bad); err == nil {
		t.Fatal("expected rejection for outside path")
	}
}

func TestValidateLabReceivePathRejectEmpty(t *testing.T) {
	base := filepath.Join(`C:\Users\Public`, "Downloads", "MultiSend")
	if _, err := validateLabReceivePath(base, ""); err == nil {
		t.Fatal("expected rejection for empty lab path")
	}
}

func TestValidateLabReceivePathRejectTraversal(t *testing.T) {
	base := filepath.Join(`C:\Users\Public`, "Downloads", "MultiSend")
	bad := filepath.Join(base, "_lab", "receive", "..", "..", "evil")
	if _, err := validateLabReceivePath(base, bad); err == nil {
		t.Fatal("expected rejection for traversal path")
	}
}
