package main

// ====================== BEGIN NAV INDEX ======================
// NAV INDEX — auto-generated symbol map (refresh via the navindex skill)
//   L23    TestOperationHistoryRestoresSafeJobAndPullContext
//   L91    TestOperationHistoryMarksUnsafeManifestNonResumable
//   L140   TestOperationHistoryRestoresCompletedJobWithoutSourceFile
//   L169   writeP2PHistoryManifest
// ======================= END NAV INDEX =======================

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Codyte/MultiSend/internal/config"
	"github.com/Codyte/MultiSend/internal/manifest"
	"github.com/Codyte/MultiSend/internal/transfer"
)

func TestOperationHistoryRestoresSafeJobAndPullContext(t *testing.T) {
	root := t.TempDir()
	userProfile := filepath.Join(root, "User")
	t.Setenv("USERPROFILE", userProfile)
	receivePath := filepath.Join(root, "Receive")
	configPath := filepath.Join(root, "Config", "MultiSend", "config.json")
	sourcePath := filepath.Join(root, "source.bin")
	if err := os.WriteFile(sourcePath, []byte("abcdef"), 0o644); err != nil {
		t.Fatalf("write source: %v", err)
	}
	info, err := os.Stat(sourcePath)
	if err != nil {
		t.Fatalf("stat source: %v", err)
	}
	manifestPath := writeP2PHistoryManifest(t, userProfile, sourcePath, info.ModTime(), "127.0.0.1:56200", []string{manifest.StatusDone, manifest.StatusPending})

	started := time.Now().Add(-time.Minute)
	a := &app{configPath: configPath, cfg: config.Config{ReceivePath: receivePath}, jobsByID: map[string]*jobState{}, pullsByID: map[string]*pullJobState{}}
	job := &jobState{
		details: jobDetails{
			ID:              "job-1001",
			Status:          "running",
			ManifestPath:    manifestPath,
			ResumeSupported: true,
			StartedAt:       started.Format(time.RFC3339),
			UpdatedAt:       started.Format(time.RFC3339),
		},
		filePath:       sourcePath,
		targetAddr:     "127.0.0.1:56200",
		receiveRelPath: "Incoming",
		publishName:    "published.bin",
		folderResult:   "zip",
		chunkSize:      3,
		startedAt:      started,
	}
	a.persistJobLocked(job)
	pull := &pullJobState{
		ID:             "pull-1002",
		SourceURL:      `file://peer/share/source.bin`,
		OutputDir:      receivePath,
		OutputPath:     filepath.Join(receivePath, "source.bin"),
		RemoteNodeID:   "peer-node",
		RemotePeerAddr: "127.0.0.1",
		RemoteJobID:    "job-1003",
		Status:         "running",
		StartedAt:      started,
		UpdatedAt:      started,
	}
	a.persistPullLocked(pull)

	restored := &app{configPath: configPath, cfg: config.Config{ReceivePath: receivePath}, jobsByID: map[string]*jobState{}, pullsByID: map[string]*pullJobState{}}
	restored.restoreOperationHistory()
	gotJob := restored.jobsByID[job.details.ID]
	if gotJob == nil || gotJob.details.Status != "canceled" || !gotJob.details.ResumeSupported {
		t.Fatalf("unexpected restored job: %+v", gotJob)
	}
	if gotJob.details.ChunksDone != 1 || gotJob.details.ChunksPending != 1 || gotJob.details.BytesSent != 3 {
		t.Fatalf("manifest progress not restored: %+v", gotJob.details)
	}
	if gotJob.receiveRelPath != "Incoming" || gotJob.publishName != "published.bin" || gotJob.chunkSize != 3 {
		t.Fatalf("send options not restored: %+v", gotJob)
	}
	gotPull := restored.pullsByID[pull.ID]
	if gotPull == nil || gotPull.Status != "canceled" || !strings.Contains(gotPull.Message, "reconnect") {
		t.Fatalf("unexpected restored pull: %+v", gotPull)
	}
}

func TestOperationHistoryMarksUnsafeManifestNonResumable(t *testing.T) {
	root := t.TempDir()
	userProfile := filepath.Join(root, "User")
	t.Setenv("USERPROFILE", userProfile)
	configPath := filepath.Join(root, "Config", "MultiSend", "config.json")
	sourcePath := filepath.Join(root, "source.bin")
	if err := os.WriteFile(sourcePath, []byte("abc"), 0o644); err != nil {
		t.Fatalf("write source: %v", err)
	}
	outsideManifest := filepath.Join(root, "outside.manifest.json")
	if err := manifest.SaveAtomic(outsideManifest, &manifest.Manifest{
		SchemaVersion: 1,
		TransferID:    "tx-2001",
		Type:          "p2p",
		FileName:      "source.bin",
		TotalBytes:    3,
		ChunkSize:     3,
		Source:        manifest.SourceInfo{Identity: "outside", AbsPath: sourcePath},
		Target:        manifest.TargetInfo{Address: "127.0.0.1:56200"},
		Chunks:        []manifest.ChunkState{{Index: 0, Offset: 0, Size: 3, Status: manifest.StatusPending}},
	}); err != nil {
		t.Fatalf("write outside manifest: %v", err)
	}
	a := &app{configPath: configPath, jobsByID: map[string]*jobState{}, pullsByID: map[string]*pullJobState{}}
	a.persistJobLocked(&jobState{
		details:    jobDetails{ID: "job-2002", Status: "running", ManifestPath: outsideManifest, StartedAt: time.Now().Format(time.RFC3339)},
		filePath:   sourcePath,
		targetAddr: "127.0.0.1:56200",
	})
	info, _ := os.Stat(sourcePath)
	validManifest := writeP2PHistoryManifest(t, userProfile, sourcePath, info.ModTime(), "127.0.0.1:56200", []string{manifest.StatusPending})
	a.persistJobLocked(&jobState{
		details:     jobDetails{ID: "job-2003", Status: "running", ManifestPath: validManifest, StartedAt: time.Now().Format(time.RFC3339)},
		filePath:    sourcePath,
		targetAddr:  "127.0.0.1:56200",
		cleanupPath: os.TempDir(),
	})

	restored := &app{configPath: configPath, jobsByID: map[string]*jobState{}, pullsByID: map[string]*pullJobState{}}
	restored.restoreOperationHistory()
	job := restored.jobsByID["job-2002"]
	if job == nil || job.details.Status != "failed" || job.details.ResumeSupported {
		t.Fatalf("unsafe manifest must remain visible but non-resumable: %+v", job)
	}
	if _, ok := restored.jobsByID["job-2003"]; ok {
		t.Fatal("job with broad cleanup path must be ignored")
	}
}

func TestOperationHistoryRestoresCompletedJobWithoutSourceFile(t *testing.T) {
	root := t.TempDir()
	userProfile := filepath.Join(root, "User")
	t.Setenv("USERPROFILE", userProfile)
	configPath := filepath.Join(root, "Config", "MultiSend", "config.json")
	sourcePath := filepath.Join(root, "completed.bin")
	if err := os.WriteFile(sourcePath, []byte("abc"), 0o644); err != nil {
		t.Fatalf("write source: %v", err)
	}
	info, _ := os.Stat(sourcePath)
	manifestPath := writeP2PHistoryManifest(t, userProfile, sourcePath, info.ModTime(), "127.0.0.1:56200", []string{manifest.StatusDone})
	a := &app{configPath: configPath, jobsByID: map[string]*jobState{}, pullsByID: map[string]*pullJobState{}}
	a.persistJobLocked(&jobState{
		details:    jobDetails{ID: "job-3001", Status: "done", ManifestPath: manifestPath, StartedAt: time.Now().Format(time.RFC3339)},
		filePath:   sourcePath,
		targetAddr: "127.0.0.1:56200",
	})
	if err := os.Remove(sourcePath); err != nil {
		t.Fatalf("remove completed source: %v", err)
	}

	restored := &app{configPath: configPath, jobsByID: map[string]*jobState{}, pullsByID: map[string]*pullJobState{}}
	restored.restoreOperationHistory()
	job := restored.jobsByID["job-3001"]
	if job == nil || job.details.Status != "done" || job.details.ResumeSupported || job.details.Percent != 100 {
		t.Fatalf("completed job should not require source file: %+v", job)
	}
}

func TestForgetOperationHistoryOnlyRemovesTerminalRecord(t *testing.T) {
	root := t.TempDir()
	configPath := filepath.Join(root, "Config", "MultiSend", "config.json")
	a := &app{configPath: configPath, jobsByID: map[string]*jobState{}, pullsByID: map[string]*pullJobState{}}
	job := &jobState{details: jobDetails{ID: "job-5001", Status: "done", StartedAt: time.Now().Format(time.RFC3339)}}
	pull := &pullJobState{ID: "pull-5002", Status: "failed", StartedAt: time.Now()}
	a.jobsByID[job.details.ID] = job
	a.pullsByID[pull.ID] = pull
	a.persistJobLocked(job)
	a.persistPullLocked(pull)

	jobPath := filepath.Join(a.operationHistoryDir("jobs"), job.details.ID+".json")
	pullPath := filepath.Join(a.operationHistoryDir("pulls"), pull.ID+".json")
	if err := a.forgetJob(job.details.ID); err != nil {
		t.Fatalf("forget job: %v", err)
	}
	if err := a.forgetPull(pull.ID); err != nil {
		t.Fatalf("forget pull: %v", err)
	}
	if _, ok := a.jobsByID[job.details.ID]; ok {
		t.Fatal("job remained in memory")
	}
	if _, ok := a.pullsByID[pull.ID]; ok {
		t.Fatal("pull remained in memory")
	}
	for _, path := range []string{jobPath, pullPath} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("history sidecar still exists: %s err=%v", path, err)
		}
	}
}

func TestForgetOperationHistoryRejectsActiveAndUnavailableStorage(t *testing.T) {
	a := &app{jobsByID: map[string]*jobState{
		"job-6001": {details: jobDetails{ID: "job-6001", Status: "running"}},
		"job-6002": {details: jobDetails{ID: "job-6002", Status: "done"}},
	}}
	if err := a.forgetJob("job-6001"); err == nil || !strings.Contains(err.Error(), "active") {
		t.Fatalf("expected active rejection, got %v", err)
	}
	if err := a.forgetJob("job-6002"); err == nil || !strings.Contains(err.Error(), "unavailable") {
		t.Fatalf("expected unavailable history rejection, got %v", err)
	}
}

func TestManagedLauncherCleanupPathAndForget(t *testing.T) {
	stagingRoot := filepath.Join(os.TempDir(), "MultiSend", "staging")
	if err := os.MkdirAll(stagingRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	stagingDir, err := os.MkdirTemp(stagingRoot, "batch-test-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(stagingDir) })
	sourcePath := filepath.Join(stagingDir, "batch.zip")
	if err := os.WriteFile(sourcePath, []byte("zip"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := validateManagedCleanupPath(sourcePath, stagingDir)
	if err != nil || !samePath(got, stagingDir) {
		t.Fatalf("managed cleanup = %q, %v", got, err)
	}
	if _, err := validateManagedCleanupPath(sourcePath, os.TempDir()); err == nil {
		t.Fatal("broad temporary root accepted")
	}
	if err := removeManagedCleanupPath(sourcePath, os.TempDir()); err == nil {
		t.Fatal("broad temporary root removed")
	}

	root := t.TempDir()
	a := &app{configPath: filepath.Join(root, "Config", "MultiSend", "config.json"), jobsByID: map[string]*jobState{}}
	job := &jobState{
		details:     jobDetails{ID: "job-7001", Status: "canceled", StartedAt: time.Now().Format(time.RFC3339)},
		filePath:    sourcePath,
		cleanupPath: stagingDir,
	}
	a.jobsByID[job.details.ID] = job
	a.persistJobLocked(job)
	if err := a.forgetJob(job.details.ID); err != nil {
		t.Fatalf("forget managed job: %v", err)
	}
	if _, err := os.Stat(stagingDir); !os.IsNotExist(err) {
		t.Fatalf("managed staging still exists: %v", err)
	}
}

func writeP2PHistoryManifest(t *testing.T, userProfile, sourcePath string, modTime time.Time, target string, statuses []string) string {
	t.Helper()
	identity, err := transfer.SenderIdentity(sourcePath, target, 3)
	if err != nil {
		t.Fatalf("build sender identity: %v", err)
	}
	chunks := make([]manifest.ChunkState, 0, len(statuses))
	for i, status := range statuses {
		bytesDone := int64(0)
		if status == manifest.StatusDone {
			bytesDone = 3
		}
		chunks = append(chunks, manifest.ChunkState{Index: int64(i), Offset: int64(i * 3), Size: 3, Status: status, BytesDone: bytesDone})
	}
	path := filepath.Join(userProfile, "Downloads", "MultiSend", "manifests", identity+".manifest.json")
	if err := manifest.SaveAtomic(path, &manifest.Manifest{
		SchemaVersion: 1,
		TransferID:    "tx-4001",
		Type:          "p2p",
		FileName:      filepath.Base(sourcePath),
		TotalBytes:    int64(len(statuses) * 3),
		ChunkSize:     3,
		CreatedAt:     time.Now().Add(-time.Minute),
		Source:        manifest.SourceInfo{Type: "p2p", Identity: identity, AbsPath: sourcePath, ModTime: modTime.Unix()},
		Target:        manifest.TargetInfo{Type: "p2p", Address: target},
		Chunks:        chunks,
	}); err != nil {
		t.Fatalf("write p2p manifest: %v", err)
	}
	return path
}
