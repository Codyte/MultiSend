package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
)

func TestRemoveSmokeRunTreeOnlyAcceptsDirectRunChild(t *testing.T) {
	root := filepath.Join(t.TempDir(), "runs")
	run := filepath.Join(root, "run-123")
	if err := os.MkdirAll(run, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(run, "artifact.bin"), []byte("test"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := removeSmokeRunTree(run, root); err != nil {
		t.Fatalf("remove valid run: %v", err)
	}
	if _, err := os.Stat(run); !os.IsNotExist(err) {
		t.Fatalf("run directory still exists: %v", err)
	}
	for _, unsafe := range []string{root, filepath.Dir(root), filepath.Join(root, "not-a-run"), filepath.Join(root, "nested", "run-123")} {
		if err := removeSmokeRunTree(unsafe, root); err == nil {
			t.Fatalf("expected rejection for %s", unsafe)
		}
	}
}

func TestTerminalSmokeStatus(t *testing.T) {
	for _, status := range []string{"done", "completed", "failed", "canceled"} {
		if !terminalSmokeStatus(status) {
			t.Fatalf("expected terminal status: %s", status)
		}
	}
	for _, status := range []string{"", "running", "canceling"} {
		if terminalSmokeStatus(status) {
			t.Fatalf("expected active status: %s", status)
		}
	}
}

func TestCleanupSmokeOperationCancelsThenDeletes(t *testing.T) {
	var canceled atomic.Bool
	var deleted atomic.Bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/jobs/job-1" && r.URL.Path != "/jobs/job-1/cancel" {
			http.NotFound(w, r)
			return
		}
		switch {
		case r.Method == http.MethodGet:
			status := "running"
			if canceled.Load() {
				status = "canceled"
			}
			_ = json.NewEncoder(w).Encode(map[string]string{"status": status})
		case r.Method == http.MethodPost && r.URL.Path == "/jobs/job-1/cancel":
			canceled.Store(true)
			_ = json.NewEncoder(w).Encode(map[string]string{"status": "canceling"})
		case r.Method == http.MethodDelete:
			deleted.Store(true)
			_ = json.NewEncoder(w).Encode(map[string]string{"status": "removed"})
		default:
			http.Error(w, "unexpected request", http.StatusMethodNotAllowed)
		}
	}))
	defer server.Close()

	if err := cleanupSmokeOperation(server.URL, "jobs", "job-1"); err != nil {
		t.Fatalf("cleanup operation: %v", err)
	}
	if !canceled.Load() || !deleted.Load() {
		t.Fatalf("expected cancel and delete: canceled=%t deleted=%t", canceled.Load(), deleted.Load())
	}
}
