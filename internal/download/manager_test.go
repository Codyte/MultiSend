package download

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"lab/multinet/internal/config"
	"lab/multinet/internal/manifest"
)

func TestSanitizeFileName(t *testing.T) {
	got := sanitizeFileName(`..\..\evil.exe`)
	if got != "____evil.exe" {
		t.Fatalf("unexpected sanitized name: %s", got)
	}
}

func TestRetryPendingSkipsDone(t *testing.T) {
	mf := &manifest.Manifest{Chunks: []manifest.ChunkState{
		{Index: 0, Status: manifest.StatusDone},
		{Index: 1, Status: manifest.StatusPending},
	}}
	p := retryPending(mf)
	if len(p) != 1 || p[0].Index != 1 {
		t.Fatalf("unexpected pending set: %+v", p)
	}
}

func TestMergePartsOrder(t *testing.T) {
	dir := t.TempDir()
	final := filepath.Join(dir, "out.bin")
	mf := &manifest.Manifest{
		TransferID: "t1",
		TotalBytes: 6,
		Chunks: []manifest.ChunkState{
			{Index: 0, Size: 3, Status: manifest.StatusDone},
			{Index: 1, Size: 3, Status: manifest.StatusDone},
		},
	}
	_ = os.MkdirAll(sessionDirFor(mf.TransferID, final), 0o755)
	_ = os.WriteFile(partPathFor(mf.TransferID, final, 0), []byte("abc"), 0o644)
	_ = os.WriteFile(partPathFor(mf.TransferID, final, 1), []byte("def"), 0o644)
	if err := mergeParts(final, mf); err != nil {
		t.Fatalf("merge failed: %v", err)
	}
	b, _ := os.ReadFile(final)
	if string(b) != "abcdef" {
		t.Fatalf("unexpected merged content: %q", string(b))
	}
}

func TestListAndGetCloneChannels(t *testing.T) {
	m := &Manager{jobs: map[string]*state{}}
	channels := map[string]ChannelStatus{
		"cable": {State: "active", BytesSent: 10},
	}
	m.jobs["id"] = &state{job: Job{
		ID:           "id",
		Channels:     channels,
		InFlight:     map[string]int{"cable": 1},
		QueueDepth:   map[string]int{"cable": 2},
		IdleGapMS:    map[string]int64{"cable": 100},
		FailedChunks: []ChunkFailureRef{{Index: 1}},
	}}

	list := m.List()
	if len(list) != 1 {
		t.Fatalf("expected 1 job, got %d", len(list))
	}
	list[0].Channels["cable"] = ChannelStatus{State: "offline"}
	list[0].InFlight["cable"] = 99
	list[0].QueueDepth["cable"] = 99
	list[0].IdleGapMS["cable"] = 999
	list[0].FailedChunks[0].Index = 99
	if got := m.jobs["id"].job.Channels["cable"].State; got != "active" {
		t.Fatalf("expected original channel state to remain active, got %s", got)
	}
	if got := m.jobs["id"].job.FailedChunks[0].Index; got != 1 {
		t.Fatalf("expected original failed chunk index to remain 1, got %d", got)
	}
	if got := m.jobs["id"].job.InFlight["cable"]; got != 1 {
		t.Fatalf("expected original in_flight to remain 1, got %d", got)
	}

	job, ok := m.Get("id")
	if !ok {
		t.Fatal("expected job to exist")
	}
	job.Channels["cable"] = ChannelStatus{State: "offline"}
	job.InFlight["cable"] = 10
	if got := m.jobs["id"].job.Channels["cable"].State; got != "active" {
		t.Fatalf("expected cloned get result, got %s", got)
	}
	if got := m.jobs["id"].job.InFlight["cable"]; got != 1 {
		t.Fatalf("expected cloned in_flight result, got %d", got)
	}
}

func TestCleanupPreviewAndCleanup(t *testing.T) {
	root := t.TempDir()
	final := filepath.Join(root, "Downloads", "file.bin")
	jobID := "id"
	sessionDir := sessionDirFor(jobID, final)
	if err := os.MkdirAll(sessionDir, 0o755); err != nil {
		t.Fatalf("mkdir session: %v", err)
	}
	if err := os.WriteFile(final, []byte("abcdef"), 0o644); err != nil {
		t.Fatalf("write final: %v", err)
	}
	if err := os.WriteFile(partPathFor(jobID, final, 0), []byte("abc"), 0o644); err != nil {
		t.Fatalf("write part0: %v", err)
	}
	if err := os.WriteFile(partPathFor(jobID, final, 1), []byte("def"), 0o644); err != nil {
		t.Fatalf("write part1: %v", err)
	}
	manifestPath := filepath.Join(sessionDir, "manifest.json")
	if err := os.WriteFile(manifestPath, []byte("{}"), 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
	m := &Manager{
		jobs: map[string]*state{
			"id": {
				job: Job{
					ID:            "id",
					Status:        "done",
					OutputPath:    final,
					ManifestPath:  manifestPath,
					TotalBytes:    6,
					ChunksTotal:   2,
					ChunksDone:    2,
					ChunksFailed:  0,
					ChunksPending: 0,
					ChunksSending: 0,
				},
				startedAt: time.Now(),
				lastAt:    time.Now(),
			},
		},
		baseDir: root,
		cfg: config.Config{
			KeepManifests:            false,
			CleanupCompletedChunks:   true,
			CleanupEmptyDownloadDirs: true,
		},
	}
	prev, err := m.PreviewCleanup("id")
	if err != nil {
		t.Fatalf("preview failed: %v", err)
	}
	if !prev.Safe || prev.ChunkFiles != 2 || prev.ChunkBytes != 6 {
		t.Fatalf("unexpected preview: %+v", prev)
	}
	if len(prev.Actions) != 2 {
		t.Fatalf("expected two actions, got %+v", prev.Actions)
	}
	res, err := m.Cleanup("id", CleanupRequest{})
	if err != nil {
		t.Fatalf("cleanup failed: %v", err)
	}
	if res.DeletedChunks != 2 || !res.ManifestDeleted || !res.DirDeleted {
		t.Fatalf("unexpected cleanup result: %+v", res)
	}
	if _, err := os.Stat(sessionDir); !os.IsNotExist(err) {
		t.Fatalf("expected session dir deleted, stat err=%v", err)
	}
}

func TestProbeDownload_HEAD_OK(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodHead {
			w.Header().Set("Content-Length", "1024")
			w.Header().Set("Accept-Ranges", "bytes")
			w.WriteHeader(http.StatusOK)
			return
		}
		t.Fatalf("unexpected method: %s", r.Method)
	}))
	defer ts.Close()

	fileName, total, acceptRanges, _, _, err := ProbeDownload(ts.URL, "test.bin")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if total != 1024 {
		t.Errorf("expected total 1024, got %d", total)
	}
	if !acceptRanges {
		t.Error("expected acceptRanges to be true")
	}
	if fileName != "test.bin" {
		t.Errorf("expected test.bin, got %s", fileName)
	}
}

func TestProbeDownload_HEAD_Fail_GET_Range_206_OK(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodHead {
			// Simulate abrupt failure (e.g. 500 or just error)
			// Actually, headDownload will error if we close connection
			hj, ok := w.(http.Hijacker)
			if ok {
				conn, _, _ := hj.Hijack()
				conn.Close()
				return
			}
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		if r.Method == http.MethodGet {
			if r.Header.Get("Range") == "bytes=0-0" {
				w.Header().Set("Content-Range", "bytes 0-0/2048")
				w.WriteHeader(http.StatusPartialContent)
				return
			}
		}
		t.Fatalf("unexpected method: %s", r.Method)
	}))
	defer ts.Close()

	_, total, acceptRanges, _, _, err := ProbeDownload(ts.URL, "")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if total != 2048 {
		t.Errorf("expected total 2048, got %d", total)
	}
	if !acceptRanges {
		t.Error("expected acceptRanges to be true")
	}
}

func TestProbeDownload_HEAD_Fail_GET_Range_200_Fail(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodHead {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		if r.Method == http.MethodGet {
			w.WriteHeader(http.StatusOK)
			return
		}
	}))
	defer ts.Close()

	_, _, _, _, _, err := ProbeDownload(ts.URL, "")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if err.Error() != "server ignored Range header; chunked download requires HTTP 206" {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestProbeDownload_HEAD_Fail_GET_Range_206_No_ContentRange(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodHead {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		if r.Method == http.MethodGet {
			w.WriteHeader(http.StatusPartialContent)
			return
		}
	}))
	defer ts.Close()

	_, _, _, _, _, err := ProbeDownload(ts.URL, "")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if err.Error() != "server returned 206 but missing valid Content-Range" {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestProbeDownload_HEAD_Fail_GET_Fail(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hj, ok := w.(http.Hijacker)
		if ok {
			conn, _, _ := hj.Hijack()
			conn.Close()
			return
		}
	}))
	defer ts.Close()

	_, _, _, _, _, err := ProbeDownload(ts.URL, "")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	// Error message should contain both HEAD and GET failures
	if !strings.Contains(err.Error(), "download_probe_failed") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestChunkTimeoutByMode(t *testing.T) {
	m := &Manager{}
	if got := m.chunkTimeout(32*1024*1024, "legacy"); got != 45*time.Second {
		t.Fatalf("legacy timeout mismatch: %v", got)
	}
	piped := m.chunkTimeout(32*1024*1024, "pipelined")
	if piped < 30*time.Second || piped > 3*time.Minute {
		t.Fatalf("pipelined timeout out of bounds: %v", piped)
	}
}

func TestTargetChunkCountPolicy(t *testing.T) {
	m := &Manager{}
	if got := m.targetChunkCount(1024*1024*1024, 1); got != 4 {
		t.Fatalf("single-iface large file should cap at 4 chunks, got %d", got)
	}
	if got := m.targetChunkCount(64*1024*1024, 1); got != 1 {
		t.Fatalf("single-iface small file should use 1 chunk, got %d", got)
	}
	if got := m.targetChunkCount(1024*1024*1024, 2); got != 6 {
		t.Fatalf("dual-iface 1GB should cap at 6 chunks, got %d", got)
	}
}
