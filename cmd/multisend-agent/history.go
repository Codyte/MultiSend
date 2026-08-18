package main

// ====================== BEGIN NAV INDEX ======================
// NAV INDEX — auto-generated symbol map (refresh via the navindex skill)
//   L55    type persistedJobState
//   L69    type persistedPullState
//   L74    type operationHistoryFile
//   L79    app.restoreOperationHistory
//   L84    app.operationHistoryDir
//   L91    app.persistJobLocked
//   L118   app.persistPullLocked
//   L132   saveOperationHistory
//   L164   app.restoreJobHistory
//   L185   app.jobStateFromHistory
//   L256   app.validateRestoredJobOptions
//   L280   validP2PHistoryManifest
//   L321   sourceMatchesManifest
//   L330   manifestAllDone
//   L339   app.restorePullHistory
//   L377   recentOperationHistory
//   L399   validOperationID
//   L413   app.pruneJobsLocked
//   L429   app.prunePullsLocked
//   L445   app.forgetJob
//   L475   validateManagedCleanupPath
//   L512   removeManagedCleanupPath
//   L523   app.forgetPull
//   L548   samePath
//   L554   pathWithin
//   L565   jobState.persistedSendOptions
//   L577   logHistoryError
// ======================= END NAV INDEX =======================

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/Codyte/MultiSend/internal/manifest"
	"github.com/Codyte/MultiSend/internal/transfer"
)

const (
	operationHistoryVersion = 1
	maxOperationHistory     = 500
)

type persistedJobState struct {
	Version        int        `json:"version"`
	Details        jobDetails `json:"details"`
	FilePath       string     `json:"file_path"`
	TargetAddr     string     `json:"target_addr"`
	Lab            bool       `json:"lab"`
	LabPath        string     `json:"lab_path,omitempty"`
	ReceiveRelPath string     `json:"receive_rel_path,omitempty"`
	PublishName    string     `json:"publish_name,omitempty"`
	FolderResult   string     `json:"folder_result,omitempty"`
	CleanupPath    string     `json:"cleanup_path,omitempty"`
	ChunkSize      int64      `json:"chunk_size,omitempty"`
}

type persistedPullState struct {
	Version int          `json:"version"`
	State   pullJobState `json:"state"`
}

type operationHistoryFile struct {
	path    string
	modTime time.Time
}

func (a *app) restoreOperationHistory() {
	a.restoreJobHistory()
	a.restorePullHistory()
}

func (a *app) operationHistoryDir(kind string) string {
	if strings.TrimSpace(a.configPath) == "" || !filepath.IsAbs(a.configPath) {
		return ""
	}
	return filepath.Join(filepath.Dir(a.configPath), "history", kind)
}

func (a *app) persistJobLocked(st *jobState) {
	if st == nil || !validOperationID(st.details.ID, "job-") {
		return
	}
	dir := a.operationHistoryDir("jobs")
	if dir == "" {
		return
	}
	record := persistedJobState{
		Version:        operationHistoryVersion,
		Details:        st.details,
		FilePath:       st.filePath,
		TargetAddr:     st.targetAddr,
		Lab:            st.lab,
		LabPath:        st.labPath,
		ReceiveRelPath: st.receiveRelPath,
		PublishName:    st.publishName,
		FolderResult:   st.folderResult,
		CleanupPath:    st.cleanupPath,
		ChunkSize:      st.chunkSize,
	}
	path := filepath.Join(dir, st.details.ID+".json")
	if err := saveOperationHistory(path, record); err != nil {
		logHistoryError("job_history_save_failed", st.details.ID, err)
	}
}

func (a *app) persistPullLocked(st *pullJobState) {
	if st == nil || !validOperationID(st.ID, "pull-") {
		return
	}
	dir := a.operationHistoryDir("pulls")
	if dir == "" {
		return
	}
	path := filepath.Join(dir, st.ID+".json")
	if err := saveOperationHistory(path, persistedPullState{Version: operationHistoryVersion, State: *st}); err != nil {
		logHistoryError("pull_history_save_failed", st.ID, err)
	}
}

func saveOperationHistory(path string, value any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".history-*.tmp")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	if err := tmp.Chmod(0o600); err != nil {
		_ = tmp.Close()
		return err
	}
	if _, err := tmp.Write(b); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpPath, path)
}

func (a *app) restoreJobHistory() {
	for _, candidate := range recentOperationHistory(a.operationHistoryDir("jobs")) {
		b, err := os.ReadFile(candidate.path)
		if err != nil {
			continue
		}
		var record persistedJobState
		if json.Unmarshal(b, &record) != nil || record.Version != operationHistoryVersion {
			continue
		}
		st, err := a.jobStateFromHistory(record, candidate.path)
		if err != nil {
			continue
		}
		if _, exists := a.jobsByID[st.details.ID]; !exists {
			a.jobsByID[st.details.ID] = st
		}
	}
	a.pruneJobsLocked()
}

func (a *app) jobStateFromHistory(record persistedJobState, recordPath string) (*jobState, error) {
	id := record.Details.ID
	if !validOperationID(id, "job-") || filepath.Base(recordPath) != id+".json" {
		return nil, fmt.Errorf("invalid job history id")
	}
	filePath, err := filepath.Abs(strings.TrimSpace(record.FilePath))
	if err != nil || !filepath.IsAbs(strings.TrimSpace(record.FilePath)) || strings.TrimSpace(record.TargetAddr) == "" {
		return nil, fmt.Errorf("invalid job request context")
	}
	if host, portText, splitErr := net.SplitHostPort(record.TargetAddr); splitErr != nil || strings.TrimSpace(host) == "" {
		return nil, fmt.Errorf("invalid job target")
	} else if port, parseErr := strconv.Atoi(portText); parseErr != nil || port < 1 || port > 65535 {
		return nil, fmt.Errorf("invalid job target port")
	}
	if err := a.validateRestoredJobOptions(record, filePath); err != nil {
		return nil, err
	}
	startedAt, _ := time.Parse(time.RFC3339, record.Details.StartedAt)
	if startedAt.IsZero() {
		startedAt = time.Now()
	}
	record.Details.FilePath = filePath
	record.Details.MbpsNow = 0
	record.Details.MbpsAvg = 0
	record.Details.Mbps30s = 0
	record.Details.Channels.Cable.MbpsNow = 0
	record.Details.Channels.Wifi.MbpsNow = 0
	record.Details.EtaSeconds = -1
	st := &jobState{
		details:        record.Details,
		filePath:       filePath,
		targetAddr:     record.TargetAddr,
		lab:            record.Lab,
		labPath:        record.LabPath,
		receiveRelPath: record.ReceiveRelPath,
		publishName:    record.PublishName,
		folderResult:   record.FolderResult,
		cleanupPath:    record.CleanupPath,
		chunkSize:      record.ChunkSize,
		startedAt:      startedAt,
		lastAt:         time.Now(),
		samples:        make([]progressSample, 0, 40),
	}
	mf, err := validP2PHistoryManifest(st.details.ManifestPath, filePath, record.TargetAddr)
	if err != nil {
		st.details.Status = "failed"
		st.details.Message = "history manifest unavailable; operation cannot be resumed"
		st.details.ResumeSupported = false
		return st, nil
	}
	a.refreshJobFromManifestLocked(&st.details)
	if manifestAllDone(mf) {
		st.details.Status = "done"
		st.details.Message = ""
		st.details.ResumeSupported = false
		st.details.Percent = 100
		st.details.BytesSent = st.details.TotalBytes
		st.details.EtaSeconds = 0
		return st, nil
	}
	st.details.Status = "canceled"
	st.details.Message = "interrupted by agent restart; ready to resume"
	st.details.CompletedAt = ""
	st.details.ResumeSupported = sourceMatchesManifest(filePath, record.TargetAddr, mf)
	if !st.details.ResumeSupported {
		st.details.Status = "failed"
		st.details.Message = "source file changed or is unavailable; operation cannot be resumed"
	}
	return st, nil
}

func (a *app) validateRestoredJobOptions(record persistedJobState, filePath string) error {
	if record.ReceiveRelPath != "" {
		if _, err := validateReceiveRelPath(record.ReceiveRelPath); err != nil {
			return fmt.Errorf("invalid restored receive path")
		}
	}
	if record.Lab {
		if _, err := validateLabReceivePath(a.cfg.ReceivePath, record.LabPath); err != nil {
			return fmt.Errorf("invalid restored lab path")
		}
	}
	if record.PublishName != "" && sanitizePublishName(record.PublishName) != record.PublishName {
		return fmt.Errorf("invalid restored publish name")
	}
	if record.FolderResult != "" && record.FolderResult != "zip" && record.FolderResult != "extract" {
		return fmt.Errorf("invalid restored folder result")
	}
	if strings.TrimSpace(record.CleanupPath) == "" {
		return nil
	}
	_, err := validateManagedCleanupPath(filePath, record.CleanupPath)
	return err
}

func validP2PHistoryManifest(path, filePath, targetAddr string) (*manifest.Manifest, error) {
	manifestPath, err := filepath.Abs(strings.TrimSpace(path))
	if err != nil || !filepath.IsAbs(strings.TrimSpace(path)) {
		return nil, fmt.Errorf("invalid manifest path")
	}
	if info, err := os.Lstat(manifestPath); err != nil || info.Mode()&os.ModeSymlink != 0 || info.IsDir() {
		return nil, fmt.Errorf("invalid manifest file")
	}
	root := filepath.Join(os.Getenv("USERPROFILE"), "Downloads", "MultiSend", "manifests")
	if strings.TrimSpace(os.Getenv("USERPROFILE")) == "" {
		root = filepath.Join(`C:\Users\Public`, "Downloads", "MultiSend", "manifests")
	}
	if !pathWithin(manifestPath, root) {
		return nil, fmt.Errorf("manifest outside sender history")
	}
	mf, err := manifest.Load(manifestPath)
	if err != nil || mf.Type != "p2p" || !validOperationID(mf.TransferID, "tx-") {
		return nil, fmt.Errorf("invalid p2p manifest")
	}
	identity := strings.TrimSuffix(filepath.Base(manifestPath), ".manifest.json")
	if identity == filepath.Base(manifestPath) || identity != mf.Source.Identity || !samePath(mf.Source.AbsPath, filePath) || mf.Target.Address != targetAddr || mf.TotalBytes <= 0 || len(mf.Chunks) == 0 {
		return nil, fmt.Errorf("manifest does not match job")
	}
	var offset int64
	for i, chunk := range mf.Chunks {
		if chunk.Index != int64(i) || chunk.Offset != offset || chunk.Size <= 0 || chunk.Size > mf.TotalBytes-offset || chunk.BytesDone < 0 || chunk.BytesDone > chunk.Size || chunk.Attempts < 0 {
			return nil, fmt.Errorf("invalid p2p chunk plan")
		}
		switch chunk.Status {
		case manifest.StatusPending, manifest.StatusSending, manifest.StatusDone, manifest.StatusFailed, manifest.StatusCanceled:
		default:
			return nil, fmt.Errorf("invalid p2p chunk status")
		}
		offset += chunk.Size
	}
	if offset != mf.TotalBytes {
		return nil, fmt.Errorf("p2p chunk plan size mismatch")
	}
	return mf, nil
}

func sourceMatchesManifest(path, targetAddr string, mf *manifest.Manifest) bool {
	info, err := os.Stat(path)
	if err != nil || info.IsDir() || info.Size() != mf.TotalBytes || info.ModTime().Unix() != mf.Source.ModTime {
		return false
	}
	identity, err := transfer.SenderIdentity(path, targetAddr, mf.ChunkSize)
	return err == nil && identity == mf.Source.Identity
}

func manifestAllDone(mf *manifest.Manifest) bool {
	for _, chunk := range mf.Chunks {
		if chunk.Status != manifest.StatusDone {
			return false
		}
	}
	return true
}

func (a *app) restorePullHistory() {
	for _, candidate := range recentOperationHistory(a.operationHistoryDir("pulls")) {
		b, err := os.ReadFile(candidate.path)
		if err != nil {
			continue
		}
		var record persistedPullState
		if json.Unmarshal(b, &record) != nil || record.Version != operationHistoryVersion {
			continue
		}
		st := record.State
		if !validOperationID(st.ID, "pull-") || !validOperationID(st.RemoteJobID, "job-") || filepath.Base(candidate.path) != st.ID+".json" {
			continue
		}
		if _, _, err := parseFileSource(st.SourceURL); err != nil {
			continue
		}
		outputDir, err := filepath.Abs(strings.TrimSpace(st.OutputDir))
		if err != nil || !filepath.IsAbs(strings.TrimSpace(st.OutputDir)) || !pathWithin(outputDir, a.cfg.ReceivePath) || strings.TrimSpace(st.RemotePeerAddr) == "" {
			continue
		}
		st.OutputDir = outputDir
		if strings.TrimSpace(st.OutputPath) != "" && (!filepath.IsAbs(st.OutputPath) || !pathWithin(st.OutputPath, a.cfg.ReceivePath)) {
			continue
		}
		if st.Status == "running" || st.Status == "canceling" || st.Status == "resuming" {
			st.Status = "canceled"
			st.Message = "interrupted by agent restart; reconnect to resume"
			st.UpdatedAt = time.Now()
		}
		copyState := st
		if _, exists := a.pullsByID[st.ID]; !exists {
			a.pullsByID[st.ID] = &copyState
		}
	}
	a.prunePullsLocked()
}

func recentOperationHistory(dir string) []operationHistoryFile {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	files := make([]operationHistoryFile, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || entry.Type()&os.ModeSymlink != 0 || !strings.HasSuffix(strings.ToLower(entry.Name()), ".json") {
			continue
		}
		info, err := entry.Info()
		if err == nil {
			files = append(files, operationHistoryFile{path: filepath.Join(dir, entry.Name()), modTime: info.ModTime()})
		}
	}
	sort.Slice(files, func(i, j int) bool { return files[i].modTime.After(files[j].modTime) })
	if len(files) > maxOperationHistory {
		files = files[:maxOperationHistory]
	}
	return files
}

func validOperationID(id, prefix string) bool {
	digits := strings.TrimPrefix(id, prefix)
	if digits == id || digits == "" {
		return false
	}
	for _, r := range digits {
		if r < '0' || r > '9' {
			return false
		}
	}
	_, err := strconv.ParseInt(digits, 10, 64)
	return err == nil
}

func (a *app) pruneJobsLocked() {
	terminal := make([]*jobState, 0, len(a.jobsByID))
	for _, st := range a.jobsByID {
		if st.details.Status == "done" || st.details.Status == "failed" || st.details.Status == "canceled" {
			terminal = append(terminal, st)
		}
	}
	if len(terminal) <= maxOperationHistory {
		return
	}
	sort.Slice(terminal, func(i, j int) bool { return terminal[i].startedAt.After(terminal[j].startedAt) })
	for _, st := range terminal[maxOperationHistory:] {
		delete(a.jobsByID, st.details.ID)
	}
}

func (a *app) prunePullsLocked() {
	terminal := make([]*pullJobState, 0, len(a.pullsByID))
	for _, st := range a.pullsByID {
		if st.Status == "done" || st.Status == "failed" || st.Status == "canceled" {
			terminal = append(terminal, st)
		}
	}
	if len(terminal) <= maxOperationHistory {
		return
	}
	sort.Slice(terminal, func(i, j int) bool { return terminal[i].StartedAt.After(terminal[j].StartedAt) })
	for _, st := range terminal[maxOperationHistory:] {
		delete(a.pullsByID, st.ID)
	}
}

func (a *app) forgetJob(id string) error {
	if !validOperationID(id, "job-") {
		return fmt.Errorf("invalid job id")
	}
	a.jobsMu.Lock()
	defer a.jobsMu.Unlock()
	st := a.jobsByID[id]
	if st == nil {
		return fmt.Errorf("job not found")
	}
	if st.details.Status != "done" && st.details.Status != "failed" && st.details.Status != "canceled" {
		return fmt.Errorf("active job cannot be removed")
	}
	dir := a.operationHistoryDir("jobs")
	if strings.TrimSpace(dir) == "" {
		return fmt.Errorf("operation history is unavailable")
	}
	path := filepath.Join(dir, id+".json")
	if st.cleanupPath != "" {
		if err := removeManagedCleanupPath(st.filePath, st.cleanupPath); err != nil {
			return fmt.Errorf("remove managed staging: %w", err)
		}
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	delete(a.jobsByID, id)
	return nil
}

func validateManagedCleanupPath(filePath, cleanupPath string) (string, error) {
	if strings.TrimSpace(cleanupPath) == "" {
		return "", nil
	}
	if !filepath.IsAbs(filePath) || !filepath.IsAbs(cleanupPath) {
		return "", fmt.Errorf("managed cleanup paths must be absolute")
	}
	fileAbs, err := filepath.Abs(filePath)
	if err != nil {
		return "", err
	}
	cleanupAbs, err := filepath.Abs(cleanupPath)
	if err != nil {
		return "", err
	}
	tempRoot, err := filepath.Abs(os.TempDir())
	if err != nil {
		return "", err
	}
	base := strings.ToLower(filepath.Base(cleanupAbs))
	parent := filepath.Dir(cleanupAbs)
	launcherRoot := filepath.Join(tempRoot, "MultiSend", "staging")
	managed := (samePath(parent, tempRoot) && strings.HasPrefix(base, "multisend-pull-")) ||
		(samePath(parent, launcherRoot) && strings.HasPrefix(base, "batch-"))
	if !managed || !pathWithin(fileAbs, cleanupAbs) {
		return "", fmt.Errorf("cleanup_path is outside a managed MultiSend staging directory")
	}
	if info, err := os.Lstat(cleanupAbs); err == nil {
		if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
			return "", fmt.Errorf("cleanup_path must be a real directory")
		}
	} else if !os.IsNotExist(err) {
		return "", err
	}
	return cleanupAbs, nil
}

func removeManagedCleanupPath(filePath, cleanupPath string) error {
	cleanupAbs, err := validateManagedCleanupPath(filePath, cleanupPath)
	if err != nil {
		return err
	}
	if cleanupAbs == "" {
		return nil
	}
	return os.RemoveAll(cleanupAbs)
}

func (a *app) forgetPull(id string) error {
	if !validOperationID(id, "pull-") {
		return fmt.Errorf("invalid pull id")
	}
	a.pullsMu.Lock()
	defer a.pullsMu.Unlock()
	st := a.pullsByID[id]
	if st == nil {
		return fmt.Errorf("pull job not found")
	}
	if st.Status != "done" && st.Status != "failed" && st.Status != "canceled" {
		return fmt.Errorf("active pull cannot be removed")
	}
	dir := a.operationHistoryDir("pulls")
	if strings.TrimSpace(dir) == "" {
		return fmt.Errorf("operation history is unavailable")
	}
	path := filepath.Join(dir, id+".json")
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	delete(a.pullsByID, id)
	return nil
}

func samePath(a, b string) bool {
	aAbs, aErr := filepath.Abs(a)
	bAbs, bErr := filepath.Abs(b)
	return aErr == nil && bErr == nil && strings.EqualFold(filepath.Clean(aAbs), filepath.Clean(bAbs))
}

func pathWithin(path, root string) bool {
	pathAbs, pathErr := filepath.Abs(path)
	rootAbs, rootErr := filepath.Abs(root)
	if pathErr != nil || rootErr != nil {
		return false
	}
	p := strings.ToLower(filepath.Clean(pathAbs))
	r := strings.ToLower(filepath.Clean(rootAbs))
	return p == r || strings.HasPrefix(p, r+string(os.PathSeparator))
}

func (st *jobState) persistedSendOptions() transfer.SendOptions {
	return transfer.SendOptions{
		ExplicitResume: true,
		Lab:            st.lab,
		LabReceivePath: st.labPath,
		ReceiveRelPath: st.receiveRelPath,
		PublishName:    st.publishName,
		FolderResult:   st.folderResult,
		ChunkSize:      st.chunkSize,
	}
}

func logHistoryError(event, id string, err error) {
	if err != nil {
		log.Printf("%s id=%s err=%v", event, id, err)
	}
}
