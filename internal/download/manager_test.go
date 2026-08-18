package download

// ====================== BEGIN NAV INDEX ======================
// NAV INDEX — auto-generated symbol map (refresh via the navindex skill)
//   L46    TestSanitizeFileName
//   L53    TestRetryPendingSkipsDone
//   L64    TestMergePartsOrder
//   L87    TestMergePartsPreservesExistingOutput
//   L113   TestMergePartsDoesNotPublishPartialOutput
//   L147   TestPrepareUsesUniqueOutputPath
//   L182   TestListAndGetCloneChannels
//   L229   TestListNewestFirst
//   L241   TestNewManagerRestoresCompletedAndInterruptedDownloads
//   L280   TestNewManagerRepairsMissingCompletedPart
//   L300   TestNewManagerIgnoresManifestTargetOutsideReceiveRoot
//   L315   TestPruneTerminalJobsKeepsNewestAndActive
//   L339   historyManifest
//   L368   writeHistoryManifest
//   L377   TestCleanupPreviewAndCleanup
//   L446   TestForgetRemovesResumeStateButPreservesFinalOutput
//   L481   TestForgetRejectsActiveAndUnsafeDownload
//   L499   TestPrepareRejectsOversizedChunkBeforeNetwork
//   L506   TestProbeDownload_HEAD_OK
//   L533   TestProbeDownload_HEAD_Fail_GET_Range_206_OK
//   L570   TestProbeDownload_HEAD_Fail_GET_Range_200_Fail
//   L592   TestProbeDownload_HEAD_Fail_GET_Range_206_No_ContentRange
//   L614   TestProbeDownload_HEAD_Fail_GET_Fail
//   L635   TestChunkTimeoutByMode
//   L646   TestTargetChunkCountPolicy
// ======================= END NAV INDEX =======================

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Codyte/MultiSend/internal/config"
	"github.com/Codyte/MultiSend/internal/manifest"
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

func TestMergePartsPreservesExistingOutput(t *testing.T) {
	dir := t.TempDir()
	final := filepath.Join(dir, "out.bin")
	if err := os.WriteFile(final, []byte("original"), 0o600); err != nil {
		t.Fatal(err)
	}
	mf := &manifest.Manifest{
		TransferID: "t-existing",
		TotalBytes: 3,
		Chunks:     []manifest.ChunkState{{Index: 0, Size: 3, Status: manifest.StatusDone}},
	}
	if err := os.MkdirAll(sessionDirFor(mf.TransferID, final), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(partPathFor(mf.TransferID, final, 0), []byte("new"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := mergeParts(final, mf); err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("expected collision error, got %v", err)
	}
	got, err := os.ReadFile(final)
	if err != nil || string(got) != "original" {
		t.Fatalf("existing output changed: data=%q err=%v", got, err)
	}
}

func TestMergePartsDoesNotPublishPartialOutput(t *testing.T) {
	dir := t.TempDir()
	final := filepath.Join(dir, "out.bin")
	mf := &manifest.Manifest{
		TransferID: "t-partial",
		TotalBytes: 6,
		Chunks: []manifest.ChunkState{
			{Index: 0, Size: 3, Status: manifest.StatusDone},
			{Index: 1, Size: 3, Status: manifest.StatusDone},
		},
	}
	if err := os.MkdirAll(sessionDirFor(mf.TransferID, final), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(partPathFor(mf.TransferID, final, 0), []byte("abc"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := mergeParts(final, mf); err == nil {
		t.Fatal("missing chunk must fail the merge")
	}
	if _, err := os.Stat(final); !os.IsNotExist(err) {
		t.Fatalf("partial final output was published: %v", err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".multisend-merge-") {
			t.Fatalf("temporary merge file leaked: %s", entry.Name())
		}
	}
}

func TestPrepareUsesUniqueOutputPath(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodHead {
			t.Fatalf("unexpected method: %s", r.Method)
		}
		w.Header().Set("Content-Length", "1024")
		w.Header().Set("Accept-Ranges", "bytes")
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	root := t.TempDir()
	m := &Manager{
		jobs:          map[string]*state{},
		cfg:           config.Config{ReceivePath: root},
		rootRecv:      root,
		lastCfgReload: time.Now(),
	}
	first, err := m.prepare("dl-unique-1", StartRequest{URL: server.URL, FileName: "same.bin"})
	if err != nil {
		t.Fatal(err)
	}
	m.jobs[first.job.ID] = first
	second, err := m.prepare("dl-unique-2", StartRequest{URL: server.URL, FileName: "same.bin"})
	if err != nil {
		t.Fatal(err)
	}
	if got := filepath.Base(first.job.OutputPath); got != "same.bin" {
		t.Fatalf("first output = %q", got)
	}
	if got := filepath.Base(second.job.OutputPath); got != "same (2).bin" {
		t.Fatalf("second output = %q", got)
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

func TestListNewestFirst(t *testing.T) {
	m := &Manager{jobs: map[string]*state{
		"dl-10": {job: Job{ID: "dl-10"}},
		"dl-30": {job: Job{ID: "dl-30"}},
		"dl-20": {job: Job{ID: "dl-20"}},
	}}
	list := m.List()
	if len(list) != 3 || list[0].ID != "dl-30" || list[1].ID != "dl-20" || list[2].ID != "dl-10" {
		t.Fatalf("unexpected list order: %+v", list)
	}
}

func TestNewManagerRestoresCompletedAndInterruptedDownloads(t *testing.T) {
	root := t.TempDir()
	completedFinal := filepath.Join(root, "Custom", "complete.bin")
	completed := historyManifest("dl-1001", completedFinal, []string{manifest.StatusDone, manifest.StatusDone})
	writeHistoryManifest(t, completed)
	if err := os.WriteFile(completedFinal, []byte("abcdef"), 0o644); err != nil {
		t.Fatalf("write completed file: %v", err)
	}
	for i, value := range []string{"abc", "def"} {
		if err := os.WriteFile(partPathFor(completed.TransferID, completedFinal, int64(i)), []byte(value), 0o644); err != nil {
			t.Fatalf("write completed part: %v", err)
		}
	}

	partialFinal := filepath.Join(root, "Downloads", "partial.bin")
	partial := historyManifest("dl-1002", partialFinal, []string{manifest.StatusDone, manifest.StatusSending})
	writeHistoryManifest(t, partial)
	if err := os.WriteFile(partPathFor(partial.TransferID, partialFinal, 0), []byte("abc"), 0o644); err != nil {
		t.Fatalf("write partial part: %v", err)
	}

	m := NewManager(config.Config{ReceivePath: root})
	completedJob, ok := m.Get(completed.TransferID)
	if !ok || completedJob.Status != "done" || completedJob.BytesDone != 6 || completedJob.ResumeSupported {
		t.Fatalf("unexpected restored completed job: found=%v job=%+v", ok, completedJob)
	}
	partialJob, ok := m.Get(partial.TransferID)
	if !ok || partialJob.Status != "canceled" || partialJob.ChunksDone != 1 || partialJob.ChunksPending != 1 || !partialJob.ResumeSupported {
		t.Fatalf("unexpected restored interrupted job: found=%v job=%+v", ok, partialJob)
	}
	if !strings.Contains(partialJob.Message, "ready to resume") {
		t.Fatalf("expected resumable restart message, got %q", partialJob.Message)
	}
	preview, err := m.PreviewCleanup(completed.TransferID)
	if err != nil || !preview.Safe {
		t.Fatalf("custom receive subfolder should be safe to clean: preview=%+v err=%v", preview, err)
	}
}

func TestNewManagerRepairsMissingCompletedPart(t *testing.T) {
	root := t.TempDir()
	final := filepath.Join(root, "Downloads", "missing.bin")
	mf := historyManifest("dl-1003", final, []string{manifest.StatusDone})
	manifestPath := writeHistoryManifest(t, mf)

	m := NewManager(config.Config{ReceivePath: root})
	job, ok := m.Get(mf.TransferID)
	if !ok || job.Status != "canceled" || job.ChunksDone != 0 || job.ChunksPending != 1 || job.BytesDone != 0 {
		t.Fatalf("missing part was not repaired for resume: found=%v job=%+v", ok, job)
	}
	reloaded, err := manifest.Load(manifestPath)
	if err != nil {
		t.Fatalf("reload repaired manifest: %v", err)
	}
	if reloaded.Chunks[0].Status != manifest.StatusPending {
		t.Fatalf("expected repaired pending chunk, got %+v", reloaded.Chunks[0])
	}
}

func TestNewManagerIgnoresManifestTargetOutsideReceiveRoot(t *testing.T) {
	root := t.TempDir()
	outside := filepath.Join(t.TempDir(), "outside.bin")
	mf := historyManifest("dl-1004", outside, []string{manifest.StatusPending})
	insideSession := filepath.Join(root, "outside.download.dl-1004")
	if err := manifest.SaveAtomic(filepath.Join(insideSession, "manifest.json"), mf); err != nil {
		t.Fatalf("write invalid manifest: %v", err)
	}

	m := NewManager(config.Config{ReceivePath: root})
	if _, ok := m.Get(mf.TransferID); ok {
		t.Fatal("manifest with target outside receive root must be ignored")
	}
}

func TestPruneTerminalJobsKeepsNewestAndActive(t *testing.T) {
	m := &Manager{jobs: map[string]*state{}}
	base := time.Now().Add(-time.Hour)
	for i := 0; i < maxRememberedTerminalJobs+2; i++ {
		id := fmt.Sprintf("terminal-%03d", i)
		m.jobs[id] = &state{job: Job{ID: id, Status: "done"}, startedAt: base.Add(time.Duration(i) * time.Second)}
	}
	m.jobs["active"] = &state{job: Job{ID: "active", Status: "running"}, startedAt: base}

	m.pruneTerminalLocked()
	if len(m.jobs) != maxRememberedTerminalJobs+1 {
		t.Fatalf("unexpected retained job count: %d", len(m.jobs))
	}
	if _, ok := m.jobs["active"]; !ok {
		t.Fatal("active job must never be pruned")
	}
	if _, ok := m.jobs["terminal-000"]; ok {
		t.Fatal("oldest terminal job should be pruned")
	}
	if _, ok := m.jobs[fmt.Sprintf("terminal-%03d", maxRememberedTerminalJobs+1)]; !ok {
		t.Fatal("newest terminal job should be retained")
	}
}

func historyManifest(id, finalPath string, statuses []string) *manifest.Manifest {
	chunks := make([]manifest.ChunkState, 0, len(statuses))
	for i, status := range statuses {
		bytesDone := int64(0)
		if status == manifest.StatusDone {
			bytesDone = 3
		}
		chunks = append(chunks, manifest.ChunkState{
			Index:     int64(i),
			Offset:    int64(i * 3),
			Size:      3,
			Status:    status,
			BytesDone: bytesDone,
		})
	}
	return &manifest.Manifest{
		SchemaVersion: 1,
		TransferID:    id,
		Type:          "internet_download",
		FileName:      filepath.Base(finalPath),
		TotalBytes:    int64(len(statuses) * 3),
		ChunkSize:     3,
		CreatedAt:     time.Now().Add(-time.Minute).UTC(),
		Source:        manifest.SourceInfo{Type: "http", URL: "https://example.com/file.bin"},
		Target:        manifest.TargetInfo{Type: "file", Path: finalPath},
		Chunks:        chunks,
	}
}

func writeHistoryManifest(t *testing.T, mf *manifest.Manifest) string {
	t.Helper()
	path := filepath.Join(sessionDirFor(mf.TransferID, mf.Target.Path), "manifest.json")
	if err := manifest.SaveAtomic(path, mf); err != nil {
		t.Fatalf("write history manifest: %v", err)
	}
	return path
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

func TestForgetRemovesResumeStateButPreservesFinalOutput(t *testing.T) {
	root := t.TempDir()
	final := filepath.Join(root, "Downloads", "file.bin")
	id := "dl-7001"
	mf := historyManifest(id, final, []string{manifest.StatusDone, manifest.StatusPending})
	manifestPath := writeHistoryManifest(t, mf)
	if err := os.WriteFile(partPathFor(id, final, 0), []byte("abc"), 0o644); err != nil {
		t.Fatalf("write part: %v", err)
	}
	if err := os.WriteFile(final, []byte("published output"), 0o644); err != nil {
		t.Fatalf("write final: %v", err)
	}
	m := &Manager{jobs: map[string]*state{id: {
		job:     Job{ID: id, Status: "canceled", OutputPath: final, ManifestPath: manifestPath},
		rootDir: root,
	}}}

	res, err := m.Forget(id)
	if err != nil {
		t.Fatalf("forget: %v", err)
	}
	if res.DeletedChunks != 1 || !res.ManifestDeleted {
		t.Fatalf("unexpected result: %+v", res)
	}
	if _, ok := m.Get(id); ok {
		t.Fatal("download remained in memory")
	}
	if _, err := os.Stat(final); err != nil {
		t.Fatalf("final output was removed: %v", err)
	}
	if _, err := os.Stat(manifestPath); !os.IsNotExist(err) {
		t.Fatalf("manifest still exists: %v", err)
	}
}

func TestForgetRejectsActiveAndUnsafeDownload(t *testing.T) {
	root := t.TempDir()
	final := filepath.Join(root, "Downloads", "file.bin")
	id := "dl-7002"
	m := &Manager{jobs: map[string]*state{id: {
		job:     Job{ID: id, Status: "running", OutputPath: final, ManifestPath: filepath.Join(sessionDirFor(id, final), "manifest.json")},
		rootDir: root,
	}}}
	if _, err := m.Forget(id); err == nil || !strings.Contains(err.Error(), "active") {
		t.Fatalf("expected active rejection, got %v", err)
	}
	m.jobs[id].job.Status = "failed"
	m.jobs[id].job.OutputPath = filepath.Join(t.TempDir(), "outside.bin")
	if _, err := m.Forget(id); err == nil || !strings.Contains(err.Error(), "outside") {
		t.Fatalf("expected unsafe path rejection, got %v", err)
	}
}

func TestPrepareRejectsOversizedChunkBeforeNetwork(t *testing.T) {
	m := &Manager{jobs: map[string]*state{}, cfg: config.Config{}, rootRecv: t.TempDir()}
	if _, err := m.prepare("dl-7003", StartRequest{URL: "https://example.com/file.bin", ChunkSizeMB: 1025}); err == nil || !strings.Contains(err.Error(), "between 0 and 1024") {
		t.Fatalf("expected chunk validation error, got %v", err)
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
