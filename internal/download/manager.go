package download

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	urlpkg "net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"lab/multinet/internal/chunk"
	"lab/multinet/internal/config"
	"lab/multinet/internal/ifmonitor"
	"lab/multinet/internal/manifest"
	"lab/multinet/internal/scheduler"
)

type ChannelStatus struct {
	State         string  `json:"state"`
	InterfaceName string  `json:"interface_name,omitempty"`
	LocalIP       string  `json:"local_ip,omitempty"`
	BytesSent     int64   `json:"bytes_sent"`
	MbpsNow       float64 `json:"mbps_now"`
	MbpsAvg       float64 `json:"mbps_avg"`
	Failures      int64   `json:"failures"`
	LastError     string  `json:"last_error,omitempty"`
}

type Job struct {
	ID              string                   `json:"id"`
	URL             string                   `json:"url"`
	Status          string                   `json:"status"`
	Message         string                   `json:"message,omitempty"`
	OutputPath      string                   `json:"output_path,omitempty"`
	ManifestPath    string                   `json:"manifest_path,omitempty"`
	BytesDone       int64                    `json:"bytes_done"`
	TotalBytes      int64                    `json:"total_bytes"`
	Percent         float64                  `json:"percent"`
	MbpsNow         float64                  `json:"mbps_now"`
	MbpsAvg         float64                  `json:"mbps_avg"`
	ChunksTotal     int64                    `json:"chunks_total"`
	ChunksDone      int64                    `json:"chunks_done"`
	ChunksFailed    int64                    `json:"chunks_failed"`
	ChunksPending   int64                    `json:"chunks_pending"`
	ChunksSending   int64                    `json:"chunks_sending"`
	ResumeSupported bool                     `json:"resume_supported"`
	PipelineMode    string                   `json:"pipeline_mode,omitempty"`
	InFlight        map[string]int           `json:"in_flight,omitempty"`
	QueueDepth      map[string]int           `json:"queue_depth,omitempty"`
	IdleGapMS       map[string]int64         `json:"idle_gap_ms,omitempty"`
	Channels        map[string]ChannelStatus `json:"channels,omitempty"`
	FailedChunks    []ChunkFailureRef        `json:"failed_chunks,omitempty"`
}

type ChunkFailureRef struct {
	Index     int64  `json:"index"`
	Attempts  int    `json:"attempts"`
	LastError string `json:"last_error,omitempty"`
	Channel   string `json:"channel,omitempty"`
}

type StartRequest struct {
	URL         string `json:"url"`
	OutputDir   string `json:"output_dir"`
	FileName    string `json:"file_name"`
	ChunkSizeMB int64  `json:"chunk_size_mb"`
}

type CleanupPreview struct {
	DownloadID   string   `json:"download_id"`
	Safe         bool     `json:"safe"`
	ChunkFiles   int      `json:"chunk_files"`
	ChunkBytes   int64    `json:"chunk_bytes"`
	ManifestPath string   `json:"manifest_path"`
	FinalFile    string   `json:"final_file"`
	Actions      []string `json:"actions"`
	Reason       string   `json:"reason,omitempty"`
}

type CleanupRequest struct {
	DeleteChunks   *bool `json:"delete_chunks,omitempty"`
	DeleteManifest *bool `json:"delete_manifest,omitempty"`
	DeleteEmptyDir *bool `json:"delete_empty_dir,omitempty"`
}

type CleanupResult struct {
	DownloadID      string   `json:"download_id"`
	DeletedChunks   int      `json:"deleted_chunks"`
	FreedBytes      int64    `json:"freed_bytes"`
	ManifestDeleted bool     `json:"manifest_deleted"`
	DirDeleted      bool     `json:"dir_deleted"`
	RemainingChunks int      `json:"remaining_chunks"`
	Warnings        []string `json:"warnings,omitempty"`
}

type Manager struct {
	mu       sync.RWMutex
	cfgMu    sync.RWMutex
	jobs     map[string]*state
	baseDir  string
	cfg      config.Config
	rootRecv string
	lastCfgReload time.Time
}

type state struct {
	job        Job
	mf         *manifest.Manifest
	cancel     context.CancelFunc
	startedAt  time.Time
	lastAt     time.Time
	lastBytes  int64
	samples    []sample
	chMu       sync.RWMutex
	channels   map[string]ChannelStatus
	allowRange bool
	chSamples  map[string][]sample
	idleGapMS  map[string]int64
	// rootDir is the download root captured at job creation. Using it (instead of
	// the live Manager.baseDir) keeps path validation stable if the config's
	// ReceivePath changes mid-job (B-9).
	rootDir string
}

// maxChunkAttempts is intentionally higher than the LAN sender's
// maxAttemptsPerChunk: HTTP(S) downloads cross the public internet where
// transient failures (proxies, rate limits, DNS) are common and worth retrying
// more aggressively than a local peer transfer. See transfer.maxAttemptsPerChunk.
const maxChunkAttempts = 8

type sample struct {
	at    time.Time
	bytes int64
}

type localChannel struct {
	name string
	ip   string
}

func NewManager(cfg config.Config) *Manager {
	base := filepath.Join(cfg.ReceivePath, "Downloads")
	return &Manager{
		jobs:          map[string]*state{},
		baseDir:       base,
		cfg:           cfg,
		rootRecv:      cfg.ReceivePath,
		lastCfgReload: time.Now(),
	}
}

func (m *Manager) getConfigSnapshot() config.Config {
	m.cfgMu.RLock()
	defer m.cfgMu.RUnlock()
	return m.cfg
}

func (m *Manager) maybeRefreshConfig() {
	now := time.Now()
	m.cfgMu.RLock()
	if now.Sub(m.lastCfgReload) < 2*time.Second {
		m.cfgMu.RUnlock()
		return
	}
	m.cfgMu.RUnlock()

	m.cfgMu.Lock()
	defer m.cfgMu.Unlock()
	if now.Sub(m.lastCfgReload) < 2*time.Second {
		return
	}
	cfg, _, exists, err := config.LoadIfExists()
	m.lastCfgReload = now
	if err != nil || !exists {
		return
	}
	m.cfg = cfg
	m.baseDir = filepath.Join(cfg.ReceivePath, "Downloads")
	m.rootRecv = cfg.ReceivePath
}

func (m *Manager) Start(req StartRequest) (Job, error) {
	id := fmt.Sprintf("dl-%d", time.Now().UnixNano())
	st, err := m.prepare(id, req)
	if err != nil {
		return Job{}, err
	}
	ctx, cancel := context.WithCancel(context.Background())
	st.cancel = cancel
	m.mu.Lock()
	m.jobs[id] = st
	m.mu.Unlock()
	go m.run(ctx, id, false)
	return st.job, nil
}

func (m *Manager) Resume(id string) (Job, error) {
	m.mu.RLock()
	st := m.jobs[id]
	m.mu.RUnlock()
	if st == nil {
		return Job{}, fmt.Errorf("job not found")
	}
	if st.job.Status != "canceled" && st.job.Status != "failed" {
		return Job{}, fmt.Errorf("job is not resumable")
	}
	ctx, cancel := context.WithCancel(context.Background())
	st.cancel = cancel
	st.job.Status = "running"
	st.job.Message = ""
	st.startedAt = time.Now()
	st.lastAt = st.startedAt
	st.lastBytes = st.job.BytesDone
	m.mu.Lock()
	m.jobs[id] = st
	m.mu.Unlock()
	go m.run(ctx, id, true)
	return st.job, nil
}

func (m *Manager) Cancel(id string) bool {
	m.mu.RLock()
	st := m.jobs[id]
	m.mu.RUnlock()
	if st == nil {
		return false
	}
	if st.cancel != nil {
		st.cancel()
	}
	m.mu.Lock()
	if st.job.Status == "running" {
		st.job.Status = "canceled"
		st.job.Message = "canceled"
	}
	m.jobs[id] = st
	m.mu.Unlock()
	return true
}

func (m *Manager) List() []Job {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]Job, 0, len(m.jobs))
	for _, st := range m.jobs {
		out = append(out, cloneJob(st.job))
	}
	return out
}

func (m *Manager) Get(id string) (Job, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	st := m.jobs[id]
	if st == nil {
		return Job{}, false
	}
	return cloneJob(st.job), true
}

func (m *Manager) prepare(id string, req StartRequest) (*state, error) {
	m.maybeRefreshConfig()
	cfg := m.getConfigSnapshot()
	rawURL := normalizeDownloadURL(req.URL)
	if rawURL == "" {
		return nil, fmt.Errorf("url is required")
	}
	baseDir := filepath.Join(cfg.ReceivePath, "Downloads")
	if err := os.MkdirAll(baseDir, 0o755); err != nil {
		return nil, err
	}
	fileName, total, acceptRanges, etag, lastMod, err := ProbeDownload(rawURL, req.FileName)
	if err != nil {
		return nil, err
	}
	if total <= 0 {
		return nil, fmt.Errorf("server did not provide valid content-length")
	}
	if !acceptRanges {
		return nil, fmt.Errorf("server does not support Accept-Ranges: bytes")
	}
	chunkSize := req.ChunkSizeMB * 1024 * 1024
	if chunkSize <= 0 {
		chunkSize = chunk.AutoChunkSize(total)
	}
	initialChCount := m.initialChannelCount(rawURL)
	targetChunks := m.targetChunkCount(total, initialChCount)
	if targetChunks > 0 {
		cappedChunkSize := (total + int64(targetChunks) - 1) / int64(targetChunks)
		if cappedChunkSize > chunkSize || req.ChunkSizeMB <= 0 {
			chunkSize = cappedChunkSize
		}
	}
	plan, err := chunk.BuildPlan(total, chunkSize)
	if err != nil {
		return nil, err
	}
	outputDir := strings.TrimSpace(req.OutputDir)
	if outputDir == "" {
		outputDir = baseDir
	}
	outRootAbs, err := filepath.Abs(outputDir)
	if err != nil {
		return nil, err
	}
	recvAbs, _ := filepath.Abs(cfg.ReceivePath)
	if !isUnder(outRootAbs, recvAbs) {
		return nil, fmt.Errorf("output_dir must stay inside %s", recvAbs)
	}
	safeName := sanitizeFileName(fileName)
	finalPath := filepath.Join(outRootAbs, safeName)
	sessionDir := sessionDirFor(id, finalPath)
	if err := os.MkdirAll(sessionDir, 0o755); err != nil {
		return nil, err
	}
	mf := &manifest.Manifest{
		SchemaVersion: 1,
		TransferID:    id,
		Type:          "internet_download",
		FileName:      safeName,
		TotalBytes:    total,
		ChunkSize:     chunkSize,
		CreatedAt:     time.Now().UTC(),
		UpdatedAt:     time.Now().UTC(),
		Source: manifest.SourceInfo{
			Type:    "http",
			URL:     rawURL,
			Address: etag,
		},
		Target: manifest.TargetInfo{
			Type:    "file",
			Path:    finalPath,
			Address: lastMod,
		},
		Chunks: make([]manifest.ChunkState, 0, len(plan.Chunks)),
	}
	for _, c := range plan.Chunks {
		mf.Chunks = append(mf.Chunks, manifest.ChunkState{
			Index:  c.Index,
			Offset: c.Offset,
			Size:   c.Size,
			Status: manifest.StatusPending,
		})
	}
	mfPath := filepath.Join(sessionDir, "manifest.json")
	if err := manifest.SaveAtomic(mfPath, mf); err != nil {
		return nil, err
	}
	channels := map[string]ChannelStatus{
		"cable": {State: "offline"},
		"wifi":  {State: "offline"},
	}
	now := time.Now()
	job := Job{
		ID:              id,
		URL:             rawURL,
		Status:          "running",
		OutputPath:      finalPath,
		ManifestPath:    mfPath,
		TotalBytes:      total,
		ChunksTotal:     int64(len(mf.Chunks)),
		ChunksPending:   int64(len(mf.Chunks)),
		ResumeSupported: true,
		PipelineMode:    config.NormalizeDownloadPipelineMode(cfg.DownloadPipelineMode),
		InFlight:        map[string]int{"cable": 0, "wifi": 0},
		QueueDepth:      map[string]int{"cable": 0, "wifi": 0},
		IdleGapMS:       map[string]int64{"cable": 0, "wifi": 0},
		Channels:        channels,
	}
	return &state{
		job:        job,
		mf:         mf,
		startedAt:  now,
		lastAt:     now,
		channels:   channels,
		allowRange: true,
		chSamples:  map[string][]sample{"cable": {}, "wifi": {}},
		idleGapMS:  map[string]int64{"cable": 0, "wifi": 0},
		rootDir:    baseDir,
	}, nil
}

func (m *Manager) run(ctx context.Context, id string, isResume bool) {
	m.mu.RLock()
	st := m.jobs[id]
	m.mu.RUnlock()
	if st == nil {
		return
	}
	if isResume {
		st.mf.ActivateResume(3)
		_ = manifest.SaveAtomic(st.job.ManifestPath, st.mf)
	}

	saveCtx, saveCancel := context.WithCancel(ctx)
	defer saveCancel()
	saveChan := make(chan struct{}, 1)
	triggerSave := func() {
		select {
		case saveChan <- struct{}{}:
		default:
		}
	}
	go func() {
		ticker := time.NewTicker(500 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-saveCtx.Done():
				return
			case <-ticker.C:
				m.mu.Lock()
				_ = manifest.SaveAtomic(st.job.ManifestPath, st.mf)
				m.mu.Unlock()
			case <-saveChan:
				m.mu.Lock()
				_ = manifest.SaveAtomic(st.job.ManifestPath, st.mf)
				m.mu.Unlock()
				time.Sleep(50 * time.Millisecond)
			}
		}
	}()
	sched := scheduler.NewWeightedScheduler()
	pipelineMode := config.NormalizeDownloadPipelineMode(m.cfg.DownloadPipelineMode)
	strategy := config.NormalizeDownloadChannelStrategy(m.cfg.DownloadChannelStrategy)
	initialChannels := m.currentChannels(st)
	strictSplit := strategy == "strict_split" && hasChannel(initialChannels, "cable") && hasChannel(initialChannels, "wifi")
	strictPlan := map[int64]string{}
	if strictSplit {
		strictPlan = buildStrictPlan(st.mf.Chunks, m.cfg.DownloadSplitCablePct)
	}
	perChannelLimit := 1
	if pipelineMode == "pipelined" {
		perChannelLimit = m.cfg.DownloadInFlightPerChan
		if perChannelLimit <= 0 {
			perChannelLimit = 2
		}
	}
	inFlight := map[string]int{"cable": 0, "wifi": 0}
	lastDispatchAt := map[string]time.Time{}
	var doneBytes atomic.Int64
	doneBytes.Store(st.mf.DoneBytes())
	lastProgressBytes := doneBytes.Load()
	lastProgressAt := time.Now()
	stallLimit := 75 * time.Second
	m.mu.Lock()
	st.job.PipelineMode = pipelineMode
	st.job.InFlight = cloneIntMap(inFlight)
	st.job.QueueDepth = map[string]int{"cable": 0, "wifi": 0}
	st.job.IdleGapMS = map[string]int64{"cable": 0, "wifi": 0}
	m.mu.Unlock()

	dispatchOnce := func() bool {
		select {
		case <-ctx.Done():
			return false
		default:
		}
		channels := m.currentChannels(st)
		m.refreshChannelStates(st, channels)

		m.mu.Lock()
		pending := retryPending(st.mf)
		currentDone := m.estimateProgressBytesLocked(st)
		if currentDone > lastProgressBytes {
			lastProgressBytes = currentDone
			lastProgressAt = time.Now()
		}
		if len(pending) == 0 {
			m.updatePipelineTelemetryLocked(st, channels, inFlight, pending, perChannelLimit)
			m.mu.Unlock()
			return false
		}
		if time.Since(lastProgressAt) > stallLimit {
			resetCount := m.resetStalledChunksLocked(st, stallLimit)
			if resetCount > 0 {
				triggerSave()
				pending = retryPending(st.mf)
				lastProgressAt = time.Now()
			}
		}
		snapshot := cloneIntMap(inFlight)
		m.updatePipelineTelemetryLocked(st, channels, snapshot, pending, perChannelLimit)
		m.mu.Unlock()

		availableChannels := make([]localChannel, 0, len(channels))
		for _, c := range channels {
			if snapshot[c.name] < perChannelLimit {
				availableChannels = append(availableChannels, c)
			}
		}
		idx, chName, ok := sched.NextChunk(pending, channelStates(availableChannels, snapshot, st))
		if !ok {
			m.mu.Lock()
			m.bumpIdleGapsLocked(st, channels, snapshot, perChannelLimit)
			m.updatePipelineTelemetryLocked(st, channels, snapshot, pending, perChannelLimit)
			m.mu.Unlock()
			time.Sleep(100 * time.Millisecond)
			return true
		}
		if strictSplit {
			pick, okPick := pickStrictPendingForChannel(pending, chName, strictPlan)
			if okPick {
				idx = pick
			} else {
				other := "wifi"
				if chName == "wifi" {
					other = "cable"
				}
				if snapshot[other] < perChannelLimit {
					if alt, okAlt := pickStrictPendingForChannel(pending, other, strictPlan); okAlt {
						chName = other
						idx = alt
					} else {
						time.Sleep(25 * time.Millisecond)
						return true
					}
				} else {
					time.Sleep(25 * time.Millisecond)
					return true
				}
			}
		}
		ch, ok := findChannel(channels, chName)
		if !ok {
			time.Sleep(50 * time.Millisecond)
			return true
		}

		m.mu.Lock()
		cst := st.mf.MarkAndGetSending(idx, ch.name)
		if cst == nil {
			m.mu.Unlock()
			return true
		}
		inFlight[ch.name]++
		lastDispatchAt[ch.name] = time.Now()
		triggerSave()
		m.updatePipelineTelemetryLocked(st, channels, inFlight, pending, perChannelLimit)
		m.mu.Unlock()

		partPath := partPathFor(st.job.ID, st.job.OutputPath, cst.Index)
		start := time.Now()
		chunkTimeout := m.chunkTimeout(cst.Size, pipelineMode)
		chunkCtx, cancel := context.WithTimeout(ctx, chunkTimeout)
		err := downloadChunkVia(chunkCtx, st.job.URL, cst.Offset, cst.Size, partPath, ch.ip)
		cancel()
		duration := time.Since(start)

		m.mu.Lock()
		if inFlight[ch.name] > 0 {
			inFlight[ch.name]--
		}
		if err != nil {
			st.mf.MarkFailed(cst.Index, err.Error())
			if c := st.mf.GetChunk(cst.Index); c != nil && c.Attempts < maxChunkAttempts {
				c.Status = manifest.StatusPending
			}
			triggerSave()
			m.bumpChannelFailure(st, ch.name, err)
			m.refreshStatsLocked(st, doneBytes.Load())
			m.updatePipelineTelemetryLocked(st, channels, inFlight, retryPending(st.mf), perChannelLimit)
			m.mu.Unlock()
			sched.ReportFailure(ch.name, err)
			return true
		}
		st.mf.MarkDone(cst.Index, cst.Size, "")
		triggerSave()
		doneBytes.Add(cst.Size)
		m.bumpChannelSuccess(st, ch.name, cst.Size)
		m.refreshStatsLocked(st, doneBytes.Load())
		m.updatePipelineTelemetryLocked(st, channels, inFlight, retryPending(st.mf), perChannelLimit)
		m.mu.Unlock()
		sched.ReportSuccess(ch.name, cst.Size, duration)
		return true
	}

	worker := func() {
		for {
			if !dispatchOnce() {
				return
			}
		}
	}

	wg := sync.WaitGroup{}
	statsStop := make(chan struct{})
	go func() {
		t := time.NewTicker(1 * time.Second)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-statsStop:
				return
			case <-t.C:
				m.mu.Lock()
				m.refreshStatsLocked(st, doneBytes.Load())
				m.mu.Unlock()
			}
		}
	}()
	workers := 2
	if pipelineMode == "pipelined" {
		workers = perChannelLimit * 2
		if workers < 2 {
			workers = 2
		}
	}
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); worker() }()
	}
	wg.Wait()
	close(statsStop)
	saveCancel()
	m.mu.Lock()
	_ = manifest.SaveAtomic(st.job.ManifestPath, st.mf)
	m.mu.Unlock()

	if ctx.Err() != nil {
		m.mu.Lock()
		st.mf.MarkCanceledPending()
		_ = manifest.SaveAtomic(st.job.ManifestPath, st.mf)
		st.job.Status = "canceled"
		st.job.Message = "canceled"
		m.refreshStatsLocked(st, doneBytes.Load())
		m.mu.Unlock()
		return
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	if !allDone(st.mf) {
		st.job.Status = "failed"
		st.job.Message = "download incomplete"
		m.refreshStatsLocked(st, doneBytes.Load())
		return
	}
	if err := mergeParts(st.job.OutputPath, st.mf); err != nil {
		st.job.Status = "failed"
		st.job.Message = err.Error()
		m.refreshStatsLocked(st, doneBytes.Load())
		return
	}
	st.job.Status = "done"
	st.job.Message = ""
	m.refreshStatsLocked(st, doneBytes.Load())
}

func (m *Manager) currentChannels(st *state) []localChannel {
	m.maybeRefreshConfig()
	cfg := m.getConfigSnapshot()
	if isLoopbackURL(st.job.URL) {
		return []localChannel{{name: "cable", ip: ""}}
	}
	infos := ifmonitor.DetectUsableInterfaces(cfg)
	var cable, wifi string
	for _, it := range infos {
		if !it.Usable || strings.TrimSpace(it.IPv4) == "" {
			continue
		}
		if it.Type == "ethernet" || it.Type == "usb_ethernet" || it.Type == "unknown" {
			if cable == "" {
				cable = it.IPv4
			}
		} else if it.Type == "wifi" {
			if wifi == "" {
				wifi = it.IPv4
			}
		}
	}
	out := make([]localChannel, 0, 2)
	if cable != "" {
		out = append(out, localChannel{name: "cable", ip: cable})
	}
	if wifi != "" {
		out = append(out, localChannel{name: "wifi", ip: wifi})
	}
	return out
}

func (m *Manager) initialChannelCount(rawURL string) int {
	m.maybeRefreshConfig()
	cfg := m.getConfigSnapshot()
	if isLoopbackURL(rawURL) {
		return 1
	}
	infos := ifmonitor.DetectUsableInterfaces(cfg)
	n := 0
	for _, it := range infos {
		if !it.Usable || strings.TrimSpace(it.IPv4) == "" {
			continue
		}
		if it.Type == "ethernet" || it.Type == "usb_ethernet" || it.Type == "unknown" || it.Type == "wifi" {
			n++
		}
	}
	if n <= 0 {
		return 1
	}
	return n
}

func (m *Manager) targetChunkCount(total int64, initialChannels int) int {
	// Single usable interface at start: keep at least 4 chunks on larger files so a
	// new interface can assist mid-download without exploding chunk count.
	if initialChannels <= 1 {
		if total <= 128*1024*1024 {
			return 1
		}
		return 4
	}
	// With 2+ interfaces: cap at 6 and avoid over-splitting small files.
	if total <= 128*1024*1024 {
		return 1
	}
	if total <= 512*1024*1024 {
		return 3
	}
	if total <= 1024*1024*1024 {
		return 6
	}
	return 6
}

func isLoopbackURL(raw string) bool {
	u, err := urlpkg.Parse(raw)
	if err != nil {
		return false
	}
	host := u.Host
	if h, _, err := net.SplitHostPort(u.Host); err == nil {
		host = h
	}
	host = strings.Trim(host, "[]")
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func (m *Manager) refreshChannelStates(st *state, channels []localChannel) {
	st.chMu.Lock()
	defer st.chMu.Unlock()
	active := map[string]bool{}
	for _, c := range channels {
		active[c.name] = true
		cs := st.channels[c.name]
		cs.State = "active"
		cs.LocalIP = c.ip
		cs.InterfaceName = c.name
		st.channels[c.name] = cs
	}
	for _, n := range []string{"cable", "wifi"} {
		if !active[n] {
			cs := st.channels[n]
			cs.State = "offline"
			st.channels[n] = cs
		}
	}
	st.job.Channels = cloneChannels(st.channels)
}

func (m *Manager) bumpChannelFailure(st *state, name string, err error) {
	st.chMu.Lock()
	defer st.chMu.Unlock()
	cs := st.channels[name]
	cs.Failures++
	cs.LastError = err.Error()
	if cs.Failures >= 3 {
		cs.State = "cooldown"
	} else {
		cs.State = "offline"
	}
	st.channels[name] = cs
	st.job.Channels = cloneChannels(st.channels)
}

func (m *Manager) bumpChannelSuccess(st *state, name string, bytes int64) {
	st.chMu.Lock()
	defer st.chMu.Unlock()
	cs := st.channels[name]
	cs.State = "active"
	cs.BytesSent += bytes
	cs.LastError = ""
	st.channels[name] = cs
	now := time.Now()
	st.chSamples[name] = append(st.chSamples[name], sample{at: now, bytes: cs.BytesSent})
	cut := now.Add(-30 * time.Second)
	k := 0
	for k < len(st.chSamples[name]) && st.chSamples[name][k].at.Before(cut) {
		k++
	}
	if k > 0 {
		st.chSamples[name] = append([]sample(nil), st.chSamples[name][k:]...)
	}
	st.job.Channels = cloneChannels(st.channels)
}

func (m *Manager) refreshStatsLocked(st *state, doneBytes int64) {
	progressBytes := m.estimateProgressBytesLocked(st)
	if doneBytes > progressBytes {
		progressBytes = doneBytes
	}
	st.job.BytesDone = progressBytes
	if st.job.TotalBytes > 0 {
		st.job.Percent = (float64(st.job.BytesDone) * 100) / float64(st.job.TotalBytes)
	}
	var done, failed, pending, sending int64
	failedList := make([]ChunkFailureRef, 0, 5)
	for _, c := range st.mf.Chunks {
		switch c.Status {
		case manifest.StatusDone:
			done++
		case manifest.StatusFailed:
			failed++
		case manifest.StatusSending:
			sending++
		default:
			pending++
		}
		if c.Status == manifest.StatusFailed && len(failedList) < 5 {
			failedList = append(failedList, ChunkFailureRef{
				Index:     c.Index,
				Attempts:  c.Attempts,
				LastError: c.LastError,
				Channel:   c.Channel,
			})
		}
	}
	st.job.ChunksDone = done
	st.job.ChunksFailed = failed
	st.job.ChunksPending = pending
	st.job.ChunksSending = sending
	st.job.FailedChunks = failedList

	now := time.Now()
	delta := now.Sub(st.lastAt).Seconds()
	if delta > 0 {
		diff := st.job.BytesDone - st.lastBytes
		if diff >= 0 {
			st.job.MbpsNow = (float64(diff) * 8) / (delta * 1_000_000)
		}
	}
	elapsed := now.Sub(st.startedAt).Seconds()
	if elapsed > 0 {
		st.job.MbpsAvg = (float64(st.job.BytesDone) * 8) / (elapsed * 1_000_000)
	}
	st.samples = append(st.samples, sample{at: now, bytes: st.job.BytesDone})
	cut := now.Add(-30 * time.Second)
	k := 0
	for k < len(st.samples) && st.samples[k].at.Before(cut) {
		k++
	}
	if k > 0 {
		st.samples = append([]sample(nil), st.samples[k:]...)
	}
	m.refreshChannelRates(st)
	st.lastAt = now
	st.lastBytes = st.job.BytesDone
}

func (m *Manager) estimateProgressBytesLocked(st *state) int64 {
	if st == nil || st.mf == nil {
		return 0
	}
	done := st.mf.DoneBytes()
	for _, c := range st.mf.Chunks {
		if c.Status != manifest.StatusSending {
			continue
		}
		p := partPathFor(st.job.ID, st.job.OutputPath, c.Index)
		fi, err := os.Stat(p)
		if err != nil {
			continue
		}
		n := fi.Size()
		if n < 0 {
			n = 0
		}
		if n > c.Size {
			n = c.Size
		}
		done += n
	}
	if done > st.job.TotalBytes {
		return st.job.TotalBytes
	}
	if done < 0 {
		return 0
	}
	return done
}

func (m *Manager) refreshChannelRates(st *state) {
	st.chMu.Lock()
	defer st.chMu.Unlock()
	perChannelBytes := m.channelProgressBytesLocked(st)
	now := time.Now()
	for _, name := range []string{"cable", "wifi"} {
		cs, ok := st.channels[name]
		if !ok {
			continue
		}
		cs.BytesSent = perChannelBytes[name]
		st.chSamples[name] = append(st.chSamples[name], sample{at: now, bytes: cs.BytesSent})
		cut := now.Add(-30 * time.Second)
		k := 0
		for k < len(st.chSamples[name]) && st.chSamples[name][k].at.Before(cut) {
			k++
		}
		if k > 0 {
			st.chSamples[name] = append([]sample(nil), st.chSamples[name][k:]...)
		}

		samples := st.chSamples[name]
		if len(samples) >= 2 {
			last := samples[len(samples)-1]
			prev := samples[len(samples)-2]
			d := last.at.Sub(prev.at).Seconds()
			if d > 0 && last.bytes >= prev.bytes {
				cs.MbpsNow = (float64(last.bytes-prev.bytes) * 8) / (d * 1_000_000)
			}
			elapsed := time.Since(st.startedAt).Seconds()
			if elapsed > 0 {
				cs.MbpsAvg = (float64(cs.BytesSent) * 8) / (elapsed * 1_000_000)
			}
		} else {
			cs.MbpsNow = 0
			elapsed := time.Since(st.startedAt).Seconds()
			if elapsed > 0 {
				cs.MbpsAvg = (float64(cs.BytesSent) * 8) / (elapsed * 1_000_000)
			} else {
				cs.MbpsAvg = 0
			}
		}
		st.channels[name] = cs
	}
	st.job.Channels = cloneChannels(st.channels)
}

func (m *Manager) channelProgressBytesLocked(st *state) map[string]int64 {
	out := map[string]int64{"cable": 0, "wifi": 0}
	if st == nil || st.mf == nil {
		return out
	}
	for _, c := range st.mf.Chunks {
		ch := c.Channel
		if ch != "cable" && ch != "wifi" {
			continue
		}
		switch c.Status {
		case manifest.StatusDone:
			out[ch] += c.BytesDone
		case manifest.StatusSending:
			p := partPathFor(st.job.ID, st.job.OutputPath, c.Index)
			fi, err := os.Stat(p)
			if err != nil {
				continue
			}
			n := fi.Size()
			if n < 0 {
				n = 0
			}
			if n > c.Size {
				n = c.Size
			}
			out[ch] += n
		}
	}
	return out
}

func ProbeDownload(rawURL, reqName string) (fileName string, total int64, acceptRanges bool, etag string, lastMod string, err error) {
	_, fileName, total, acceptRanges, etag, lastMod, err = headDownload(rawURL, reqName)
	if err == nil && total > 0 && acceptRanges {
		return fileName, total, acceptRanges, etag, lastMod, nil
	}

	headErr := err
	
	req, reqErr := http.NewRequest(http.MethodGet, rawURL, nil)
	if reqErr != nil {
		return "", 0, false, "", "", fmt.Errorf("download_probe_failed: HEAD failed (%v); server does not support HEAD or Range GET required for chunked download: %v", headErr, reqErr)
	}
	req.Header.Set("Range", "bytes=0-0")
	client := &http.Client{Timeout: 10 * time.Second}
	res, resErr := client.Do(req)
	if resErr != nil {
		return "", 0, false, "", "", fmt.Errorf("download_probe_failed: HEAD failed (%v); server does not support HEAD or Range GET required for chunked download: %v", headErr, resErr)
	}
	defer func() { _ = res.Body.Close() }()

	if res.StatusCode == http.StatusOK {
		return "", 0, false, "", "", fmt.Errorf("server ignored Range header; chunked download requires HTTP 206")
	}

	if res.StatusCode != http.StatusPartialContent {
		return "", 0, false, "", "", fmt.Errorf("server does not support HEAD or Range GET required for chunked download")
	}

	cr := res.Header.Get("Content-Range")
	if cr == "" {
		return "", 0, false, "", "", fmt.Errorf("server returned 206 but missing valid Content-Range")
	}

	parts := strings.Split(cr, "/")
	if len(parts) != 2 {
		return "", 0, false, "", "", fmt.Errorf("server returned 206 but missing valid Content-Range")
	}

	totalRange, parseErr := strconv.ParseInt(parts[1], 10, 64)
	if parseErr != nil || totalRange <= 0 {
		return "", 0, false, "", "", fmt.Errorf("server returned 206 but missing valid Content-Range")
	}

	fileName = strings.TrimSpace(reqName)
	if fileName == "" {
		fileName = fileNameFromHeaderOrURL(res.Header.Get("Content-Disposition"), rawURL)
	}

	return fileName, totalRange, true, res.Header.Get("ETag"), res.Header.Get("Last-Modified"), nil
}

func headDownload(rawURL, reqName string) (*http.Response, string, int64, bool, string, string, error) {
	req, err := http.NewRequest(http.MethodHead, rawURL, nil)
	if err != nil {
		return nil, "", 0, false, "", "", err
	}
	client := &http.Client{Timeout: 10 * time.Second}
	res, err := client.Do(req)
	if err != nil {
		return nil, "", 0, false, "", "", err
	}
	defer func() { _ = res.Body.Close() }()
	total, _ := strconv.ParseInt(res.Header.Get("Content-Length"), 10, 64)
	accept := strings.Contains(strings.ToLower(res.Header.Get("Accept-Ranges")), "bytes")
	fileName := strings.TrimSpace(reqName)
	if fileName == "" {
		fileName = fileNameFromHeaderOrURL(res.Header.Get("Content-Disposition"), rawURL)
	}
	return res, fileName, total, accept, res.Header.Get("ETag"), res.Header.Get("Last-Modified"), nil
}

func fileNameFromHeaderOrURL(cd, rawURL string) string {
	if strings.Contains(strings.ToLower(cd), "filename=") {
		parts := strings.Split(cd, "filename=")
		if len(parts) > 1 {
			n := strings.Trim(parts[1], `"; `)
			if n != "" {
				return n
			}
		}
	}
	u, err := urlpkg.Parse(rawURL)
	if err == nil {
		b := filepath.Base(u.Path)
		if b != "" && b != "." && b != "/" {
			return b
		}
	}
	return "download.bin"
}

func sanitizeFileName(name string) string {
	n := strings.TrimSpace(name)
	n = strings.ReplaceAll(n, "..", "_")
	n = strings.ReplaceAll(n, "/", "_")
	n = strings.ReplaceAll(n, "\\", "_")
	if n == "" {
		n = "download.bin"
	}
	return n
}

func sessionDirFor(jobID, finalPath string) string {
	if strings.TrimSpace(jobID) == "" {
		return finalPath + ".download"
	}
	return finalPath + ".download." + jobID
}

func partPathFor(jobID, finalPath string, index int64) string {
	session := sessionDirFor(jobID, finalPath)
	return filepath.Join(session, fmt.Sprintf("chunk%06d.part", index+1))
}

func retryPending(mf *manifest.Manifest) []manifest.ChunkState {
	out := make([]manifest.ChunkState, 0)
	for _, c := range mf.Chunks {
		if c.Status == manifest.StatusPending || (c.Status == manifest.StatusFailed && c.Attempts < maxChunkAttempts) {
			out = append(out, c)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Index < out[j].Index })
	return out
}

func (m *Manager) resetStalledChunksLocked(st *state, stallLimit time.Duration) int {
	if st == nil || st.mf == nil {
		return 0
	}
	reset := 0
	now := time.Now().UTC()
	for i := range st.mf.Chunks {
		c := &st.mf.Chunks[i]
		if c.Status == manifest.StatusSending {
			if c.StartedAt != nil && now.Sub(*c.StartedAt) < stallLimit {
				continue
			}
			c.Status = manifest.StatusPending
			c.Channel = ""
			reset++
		}
	}
	return reset
}

func normalizeDownloadURL(raw string) string {
	u := strings.TrimSpace(raw)
	if u == "" {
		return ""
	}
	pu, err := urlpkg.Parse(u)
	if err != nil {
		return u
	}
	if pu.Path == "/" || pu.Path == "" {
		return u
	}
	if strings.HasSuffix(pu.Path, "/") {
		trimmed := strings.TrimRight(pu.Path, "/")
		if trimmed != "" {
			pu.Path = trimmed
			return pu.String()
		}
	}
	return u
}

func allDone(mf *manifest.Manifest) bool {
	for _, c := range mf.Chunks {
		if c.Status != manifest.StatusDone {
			return false
		}
	}
	return true
}

func mergeParts(finalPath string, mf *manifest.Manifest) error {
	if err := os.MkdirAll(filepath.Dir(finalPath), 0o755); err != nil {
		return err
	}
	out, err := os.Create(finalPath)
	if err != nil {
		return err
	}
	defer out.Close()
	for _, c := range mf.Chunks {
		p := partPathFor(mf.TransferID, finalPath, c.Index)
		f, err := os.Open(p)
		if err != nil {
			return err
		}
		n, err := io.Copy(out, f)
		_ = f.Close()
		if err != nil {
			return err
		}
		if n != c.Size {
			return fmt.Errorf("chunk size mismatch at index %d", c.Index)
		}
	}
	st, err := os.Stat(finalPath)
	if err != nil {
		return err
	}
	if st.Size() != mf.TotalBytes {
		return fmt.Errorf("final size mismatch got=%d want=%d", st.Size(), mf.TotalBytes)
	}
	return nil
}

func downloadChunkVia(ctx context.Context, rawURL string, offset, size int64, outPath, localIP string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Range", fmt.Sprintf("bytes=%d-%d", offset, offset+size-1))
	tr := &http.Transport{}
	if strings.TrimSpace(localIP) != "" {
		ip := net.ParseIP(localIP)
		if ip != nil {
			d := &net.Dialer{LocalAddr: &net.TCPAddr{IP: ip}}
			tr.DialContext = d.DialContext
		}
	}
	client := &http.Client{Transport: tr, Timeout: 60 * time.Second}
	res, err := client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusPartialContent {
		return fmt.Errorf("range unsupported status=%d", res.StatusCode)
	}
	if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil {
		return err
	}
	f, err := os.Create(outPath)
	if err != nil {
		return err
	}
	defer f.Close()
	n, err := io.CopyN(f, res.Body, size)
	if err != nil {
		return err
	}
	if n != size {
		return fmt.Errorf("chunk bytes mismatch got=%d want=%d", n, size)
	}
	return nil
}

func isUnder(path, root string) bool {
	p, _ := filepath.Abs(path)
	r, _ := filepath.Abs(root)
	lp := strings.ToLower(p)
	lr := strings.ToLower(r)
	return lp == lr || strings.HasPrefix(lp, lr+string(os.PathSeparator))
}

func channelStates(channels []localChannel, inFlight map[string]int, st *state) []scheduler.Channel {
	out := make([]scheduler.Channel, 0, len(channels))
	st.chMu.RLock()
	defer st.chMu.RUnlock()
	for _, c := range channels {
		row := scheduler.Channel{Name: c.name, Active: true}
		row.InFlight = inFlight[c.name]
		if cs, ok := st.channels[c.name]; ok {
			row.Mbps30s = cs.MbpsAvg
			row.Failures = int(cs.Failures)
			row.LastError = cs.LastError
		}
		out = append(out, row)
	}
	return out
}

func hasChannel(channels []localChannel, name string) bool {
	for _, c := range channels {
		if c.name == name {
			return true
		}
	}
	return false
}

func buildStrictPlan(chunks []manifest.ChunkState, cablePct int) map[int64]string {
	if cablePct <= 0 || cablePct >= 100 {
		cablePct = 50
	}
	ordered := append([]manifest.ChunkState(nil), chunks...)
	sort.Slice(ordered, func(i, j int) bool { return ordered[i].Index < ordered[j].Index })
	plan := make(map[int64]string, len(ordered))
	var cableScore, wifiScore int
	for _, c := range ordered {
		cableScore += cablePct
		wifiScore += (100 - cablePct)
		if cableScore >= wifiScore {
			plan[c.Index] = "cable"
			cableScore -= 100
		} else {
			plan[c.Index] = "wifi"
			wifiScore -= 100
		}
	}
	return plan
}

func pickStrictPendingForChannel(pending []manifest.ChunkState, channel string, plan map[int64]string) (int64, bool) {
	for _, p := range pending {
		if plan[p.Index] == channel {
			return p.Index, true
		}
	}
	return 0, false
}

func cloneIntMap(in map[string]int) map[string]int {
	out := make(map[string]int, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func cloneInt64Map(in map[string]int64) map[string]int64 {
	out := make(map[string]int64, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func (m *Manager) bumpIdleGapsLocked(st *state, channels []localChannel, inFlight map[string]int, perChannelLimit int) {
	for _, ch := range channels {
		if inFlight[ch.name] >= perChannelLimit {
			continue
		}
		cs := st.channels[ch.name]
		if cs.State == "active" || cs.State == "offline" {
			st.idleGapMS[ch.name] += 100
		}
	}
}

func (m *Manager) updatePipelineTelemetryLocked(st *state, channels []localChannel, inFlight map[string]int, pending []manifest.ChunkState, perChannelLimit int) {
	_ = perChannelLimit
	st.job.InFlight = cloneIntMap(inFlight)
	active := 0
	for _, ch := range channels {
		if ch.name == "cable" || ch.name == "wifi" {
			active++
		}
	}
	queue := map[string]int{"cable": 0, "wifi": 0}
	if active > 0 && len(pending) > 0 {
		base := len(pending) / active
		rem := len(pending) % active
		idx := 0
		for _, ch := range channels {
			q := base
			if idx < rem {
				q++
			}
			queue[ch.name] = q
			idx++
		}
	}
	st.job.QueueDepth = queue
	if st.idleGapMS == nil {
		st.idleGapMS = map[string]int64{"cable": 0, "wifi": 0}
	}
	st.job.IdleGapMS = cloneInt64Map(st.idleGapMS)
}

func (m *Manager) chunkTimeout(size int64, mode string) time.Duration {
	if size <= 0 {
		size = 1 << 20
	}
	if mode == "pipelined" {
		min := 30 * time.Second
		max := 3 * time.Minute
		perMB := 1500 * time.Millisecond
		d := time.Duration((size/(1024*1024))+1) * perMB
		if d < min {
			return min
		}
		if d > max {
			return max
		}
		return d
	}
	return 45 * time.Second
}

func findChannel(channels []localChannel, name string) (localChannel, bool) {
	for _, c := range channels {
		if c.name == name {
			return c, true
		}
	}
	return localChannel{}, false
}

func cloneChannels(in map[string]ChannelStatus) map[string]ChannelStatus {
	out := make(map[string]ChannelStatus, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func cloneJob(in Job) Job {
	out := in
	if in.Channels != nil {
		out.Channels = cloneChannels(in.Channels)
	}
	if in.InFlight != nil {
		out.InFlight = cloneIntMap(in.InFlight)
	}
	if in.QueueDepth != nil {
		out.QueueDepth = cloneIntMap(in.QueueDepth)
	}
	if in.IdleGapMS != nil {
		out.IdleGapMS = cloneInt64Map(in.IdleGapMS)
	}
	if in.FailedChunks != nil {
		out.FailedChunks = append([]ChunkFailureRef(nil), in.FailedChunks...)
	}
	return out
}

func (m *Manager) PreviewCleanup(id string) (CleanupPreview, error) {
	m.mu.RLock()
	st := m.jobs[id]
	m.mu.RUnlock()
	if st == nil {
		return CleanupPreview{}, fmt.Errorf("job not found")
	}
	p := CleanupPreview{
		DownloadID:   id,
		ManifestPath: st.job.ManifestPath,
		FinalFile:    st.job.OutputPath,
		Actions:      []string{},
	}
	if st.job.Status != "done" {
		p.Reason = "download not completed"
		return p, fmt.Errorf("%s", p.Reason)
	}
	finalInfo, err := os.Stat(st.job.OutputPath)
	if err != nil {
		p.Reason = "final file missing"
		return p, fmt.Errorf("%s", p.Reason)
	}
	if finalInfo.Size() != st.job.TotalBytes {
		p.Reason = "final size mismatch"
		return p, fmt.Errorf("%s", p.Reason)
	}
	if st.job.ChunksDone != st.job.ChunksTotal || st.job.ChunksFailed != 0 || st.job.ChunksPending != 0 || st.job.ChunksSending != 0 {
		p.Reason = "chunk counters not safe for cleanup"
		return p, fmt.Errorf("%s", p.Reason)
	}
	sessionDir := sessionDirFor(st.job.ID, st.job.OutputPath)
	root := st.rootDir
	if strings.TrimSpace(root) == "" {
		root = m.baseDir
	}
	rootAbs, _ := filepath.Abs(root)
	sessAbs, _ := filepath.Abs(sessionDir)
	if !isUnder(sessAbs, rootAbs) {
		p.Reason = "cleanup path outside allowed download root"
		return p, fmt.Errorf("%s", p.Reason)
	}
	manifestAbs, err := filepath.Abs(st.job.ManifestPath)
	if err != nil {
		p.Reason = "invalid manifest path"
		return p, fmt.Errorf("%s", p.Reason)
	}
	if !isUnder(manifestAbs, sessAbs) {
		p.Reason = "manifest path outside session directory"
		return p, fmt.Errorf("%s", p.Reason)
	}
	files, err := filepath.Glob(filepath.Join(sessAbs, "chunk*.part"))
	if err != nil {
		p.Reason = "failed to enumerate chunks"
		return p, err
	}
	var total int64
	for _, f := range files {
		if stf, err := os.Stat(f); err == nil {
			total += stf.Size()
		}
	}
	p.ChunkFiles = len(files)
	p.ChunkBytes = total
	p.Safe = true
	p.Actions = append(p.Actions, "delete_chunks")
	if m.cfg.KeepManifests {
		p.Actions = append(p.Actions, "keep_manifest")
	} else {
		p.Actions = append(p.Actions, "delete_manifest")
	}
	return p, nil
}

func (m *Manager) Cleanup(id string, req CleanupRequest) (CleanupResult, error) {
	preview, err := m.PreviewCleanup(id)
	if err != nil {
		return CleanupResult{}, err
	}
	deleteChunks := m.cfg.CleanupCompletedChunks
	if req.DeleteChunks != nil {
		deleteChunks = *req.DeleteChunks
	}
	deleteManifest := !m.cfg.KeepManifests
	if req.DeleteManifest != nil {
		deleteManifest = *req.DeleteManifest
	}
	deleteEmptyDir := m.cfg.CleanupEmptyDownloadDirs
	if req.DeleteEmptyDir != nil {
		deleteEmptyDir = *req.DeleteEmptyDir
	}

	res := CleanupResult{DownloadID: id, Warnings: []string{}}
	sessionDir := filepath.Dir(preview.ManifestPath)

	if deleteChunks {
		files, _ := filepath.Glob(filepath.Join(sessionDir, "chunk*.part"))
		for _, f := range files {
			info, _ := os.Stat(f)
			if err := os.Remove(f); err != nil {
				res.Warnings = append(res.Warnings, fmt.Sprintf("could not remove %s: %v", filepath.Base(f), err))
				continue
			}
			res.DeletedChunks++
			if info != nil {
				res.FreedBytes += info.Size()
			}
		}
	}

	if deleteManifest && strings.TrimSpace(preview.ManifestPath) != "" {
		if err := os.Remove(preview.ManifestPath); err == nil || os.IsNotExist(err) {
			res.ManifestDeleted = true
		} else {
			res.Warnings = append(res.Warnings, "could not remove manifest: "+err.Error())
		}
	}

	remaining, _ := filepath.Glob(filepath.Join(sessionDir, "chunk*.part"))
	res.RemainingChunks = len(remaining)
	if deleteEmptyDir {
		entries, err := os.ReadDir(sessionDir)
		if err == nil && len(entries) == 0 {
			if err := os.Remove(sessionDir); err == nil {
				res.DirDeleted = true
			}
		}
	}
	return res, nil
}
