package main

// ====================== BEGIN NAV INDEX ======================
// NAV INDEX — auto-generated symbol map (refresh via the navindex skill)
//   L143   type channelProgress
//   L157   type channelsProgress
//   L162   type jobDetails
//   L189   type receiverSession
//   L205   type failedChunkRef
//   L212   type progressSample
//   L219   type jobState
//   L239   type receiverSessionState
//   L246   type app
//   L261   type pullJobState
//   L279   type sendTargetOptions
//   L288   type apiErrorResponse
//   L298   type preparedRemoteSource
//   L309   agentLogPath
//   L320   setupAgentLogging
//   L342   rotateLogIfLarge
//   L355   main
//   L420   app.selectAndPersistPorts
//   L450   app.writeRuntimeState
//   L471   app.runReceiver
//   L494   app.handleConn
//   L624   app.updateReceiverManifest
//   L678   app.ensureReceiverSession
//   L692   app.updateReceiverSession
//   L705   app.maybeScheduleReceiverFinalize
//   L726   app.finalizeReceiverSession
//   L803   app.failReceiverFinalize
//   L812   app.markReceiverDone
//   L828   countManifestChunksByStatus
//   L838   validateReceiverManifestComplete
//   L854   validateReceiverFinalSize
//   L865   validateReceiverPath
//   L887   isUnder
//   L895   isPullHeader
//   L899   validateReceiveRelPath
//   L917   resolveReceiveRelRoot
//   L940   sanitizePublishName
//   L952   receiverTargetInfo
//   L971   receiverFinalPath
//   L981   pullPublishRoot
//   L989   extractZipSafe
//   L1044  mergeReceiverChunks
//   L1076  cleanupReceiverChunks
//   L1092  app.runLocalAPI
//   L1105  app.runControlAPI
//   L1118  app.health
//   L1126  app.peers
//   L1134  app.send
//   L1176  app.startSend
//   L1281  app.jobs
//   L1297  app.jobByID
//   L1359  app.remoteSend
//   L1432  app.remoteJobByID
//   L1495  app.pullsHandler
//   L1527  app.pullByID
//   L1575  app.createJob
//   L1617  app.resumeJob
//   L1675  app.cancelJob
//   L1696  app.updateJobProgress
//   L1818  app.finishJob
//   L1847  app.setJobManifest
//   L1862  app.refreshJobFromManifestLocked
//   L1913  app.receiverSessionsHandler
//   L1921  app.receiverSessionByID
//   L1940  app.listReceiverSessions
//   L1953  app.getReceiverSession
//   L1965  app.downloadsHandler
//   L1996  app.downloadByID
//   L2089  app.interfacesHandler
//   L2117  app.localIPs
//   L2129  app.enrichSendOptions
//   L2161  app.controlAuthOK
//   L2177  app.controlAPIToken
//   L2184  app.remoteSendRoots
//   L2194  app.isRemoteSendPathAllowed
//   L2211  resolveExistingPath
//   L2223  app.isRemoteAllowed
//   L2245  app.resolvePeerAddressByNodeID
//   L2263  parseFileSource
//   L2315  prepareRemoteSendSource
//   L2352  zipDirectory
//   L2414  app.findPeerByHost
//   L2451  validateOutputUnderReceive
//   L2474  app.remoteControlBase
//   L2487  postJSON
//   L2507  getJSON
//   L2522  decodeRemoteAPIResponse
//   L2539  setBearerToken
//   L2545  formatRemoteAPIError
//   L2570  app.createPull
//   L2656  validateChunkSizeMB
//   L2663  app.pullViewFromRemote
//   L2730  app.listPulls
//   L2750  app.refreshPull
//   L2760  app.pullRemoteAction
//   L2783  validateLabReceivePath
//   L2812  writeJSON
//   L2817  writeAPIError
//   L2821  writeAPIErrorDetail
//   L2847  remoteAddrForLog
// ======================= END NAV INDEX =======================

import (
	"archive/zip"
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	urlpkg "net/url"
	"os"
	"os/signal"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/Codyte/MultiSend/internal/config"
	"github.com/Codyte/MultiSend/internal/discovery"
	"github.com/Codyte/MultiSend/internal/download"
	"github.com/Codyte/MultiSend/internal/ifmonitor"
	"github.com/Codyte/MultiSend/internal/manifest"
	"github.com/Codyte/MultiSend/internal/netif"
	"github.com/Codyte/MultiSend/internal/ports"
	"github.com/Codyte/MultiSend/internal/proto"
	"github.com/Codyte/MultiSend/internal/transfer"
)

type channelProgress struct {
	BytesSent  int64   `json:"bytes_sent"`
	TotalBytes int64   `json:"total_bytes"`
	Percent    float64 `json:"percent"`
	MbpsNow    float64 `json:"mbps_now"`
	MbpsAvg    float64 `json:"mbps_avg"`
	Mbps30s    float64 `json:"mbps_30s"`
	State      string  `json:"state,omitempty"`
	Interface  string  `json:"interface_name,omitempty"`
	LocalIP    string  `json:"local_ip,omitempty"`
	Failures   int64   `json:"failures,omitempty"`
	LastError  string  `json:"last_error,omitempty"`
}

type channelsProgress struct {
	Cable channelProgress `json:"cable"`
	Wifi  channelProgress `json:"wifi"`
}

type jobDetails struct {
	ID              string           `json:"id"`
	FilePath        string           `json:"file_path,omitempty"`
	Status          string           `json:"status"`
	Message         string           `json:"message,omitempty"`
	BytesSent       int64            `json:"bytes_sent"`
	TotalBytes      int64            `json:"total_bytes"`
	Percent         float64          `json:"percent"`
	MbpsNow         float64          `json:"mbps_now"`
	MbpsAvg         float64          `json:"mbps_avg"`
	Mbps30s         float64          `json:"mbps_30s"`
	EtaSeconds      int64            `json:"eta_seconds"`
	Channels        channelsProgress `json:"channels"`
	StartedAt       string           `json:"started_at"`
	UpdatedAt       string           `json:"updated_at"`
	CompletedAt     string           `json:"completed_at,omitempty"`
	ChunksTotal     int64            `json:"chunks_total"`
	ChunksDone      int64            `json:"chunks_done"`
	ChunksFailed    int64            `json:"chunks_failed"`
	ChunksPending   int64            `json:"chunks_pending"`
	ChunksSending   int64            `json:"chunks_sending"`
	ResumeSupported bool             `json:"resume_supported"`
	ManifestPath    string           `json:"manifest_path,omitempty"`
	LastError       string           `json:"last_error,omitempty"`
	FailedChunks    []failedChunkRef `json:"failed_chunks,omitempty"`
}

type receiverSession struct {
	SessionDir      string `json:"session_dir"`
	TransferID      string `json:"transfer_id,omitempty"`
	FileName        string `json:"file_name,omitempty"`
	Status          string `json:"status,omitempty"`
	Message         string `json:"message,omitempty"`
	ChunksTotal     int64  `json:"chunks_total,omitempty"`
	ChunksDone      int64  `json:"chunks_done,omitempty"`
	ChunksFailed    int64  `json:"chunks_failed,omitempty"`
	ManifestPath    string `json:"manifest_path,omitempty"`
	FinalOutputPath string `json:"final_output_path,omitempty"`
	MergeStatus     string `json:"merge_status,omitempty"`
	MergeMessage    string `json:"merge_message,omitempty"`
	CleanupStatus   string `json:"cleanup_status,omitempty"`
}

type failedChunkRef struct {
	Index     int64  `json:"index"`
	Attempts  int    `json:"attempts"`
	LastError string `json:"last_error,omitempty"`
	Channel   string `json:"channel,omitempty"`
}

type progressSample struct {
	At    time.Time
	Total int64
	Cable int64
	Wifi  int64
}

type jobState struct {
	details        jobDetails
	filePath       string
	targetAddr     string
	lab            bool
	labPath        string
	receiveRelPath string
	publishName    string
	folderResult   string
	cleanupPath    string
	chunkSize      int64
	startedAt      time.Time
	lastAt         time.Time
	lastTotal      int64
	lastCable      int64
	lastWifi       int64
	samples        []progressSample
	cancelFunc     context.CancelFunc
}

type receiverSessionState struct {
	session    receiverSession
	timer      *time.Timer
	mu         sync.Mutex
	manifestMu sync.Mutex
}

type app struct {
	cfg                config.Config
	configPath         string
	configMu           sync.Mutex
	stop               context.CancelFunc
	disc               *discovery.Manager
	downloads          *download.Manager
	jobsMu             sync.RWMutex
	jobsByID           map[string]*jobState
	pullsMu            sync.RWMutex
	pullsByID          map[string]*pullJobState
	receiverSessionsMu sync.RWMutex
	receiverSessions   map[string]*receiverSessionState
}

type pullJobState struct {
	ID             string    `json:"id"`
	SourceURL      string    `json:"source_url"`
	OutputDir      string    `json:"output_dir"`
	OutputPath     string    `json:"output_path,omitempty"`
	SourceType     string    `json:"source_type,omitempty"`
	FolderResult   string    `json:"folder_result,omitempty"`
	RemoteNodeID   string    `json:"remote_node_id"`
	RemotePeerAddr string    `json:"remote_peer_addr"`
	RemoteJobID    string    `json:"remote_job_id"`
	Status         string    `json:"status"`
	Message        string    `json:"message,omitempty"`
	StartedAt      time.Time `json:"started_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

var detectUsableInterfaces = ifmonitor.DetectUsableInterfaces

type sendTargetOptions struct {
	Lab            bool
	LabReceivePath string
	ReceiveRelPath string
	PublishName    string
	FolderResult   string
	CleanupPath    string
}

type apiErrorResponse struct {
	Error     string `json:"error"`
	Message   string `json:"message"`
	Detail    string `json:"detail,omitempty"`
	Status    int    `json:"status"`
	Method    string `json:"method,omitempty"`
	Path      string `json:"path,omitempty"`
	RequestID string `json:"request_id,omitempty"`
}

type preparedRemoteSource struct {
	SourceType   string
	PublishName  string
	FolderResult string
	CleanupPath  string
}

const agentLogMaxBytes int64 = 8 * 1024 * 1024

var activeAgentLogPath string

func agentLogPath() string {
	if activeAgentLogPath != "" {
		return activeAgentLogPath
	}
	local := os.Getenv("LOCALAPPDATA")
	if local == "" {
		return ""
	}
	return filepath.Join(local, "MultiSend", "logs", "agent.log")
}

func setupAgentLogging() (string, func()) {
	path := agentLogPath()
	if path == "" {
		log.SetFlags(log.LstdFlags | log.Lmicroseconds)
		return "", nil
	}
	activeAgentLogPath = path
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		log.Printf("agent log mkdir failed path=%s err=%v", path, err)
		return path, nil
	}
	rotateLogIfLarge(path, agentLogMaxBytes)
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		log.Printf("agent log open failed path=%s err=%v", path, err)
		return path, nil
	}
	log.SetOutput(io.MultiWriter(os.Stderr, f))
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)
	return path, func() { _ = f.Close() }
}

func rotateLogIfLarge(path string, maxBytes int64) {
	if maxBytes <= 0 {
		return
	}
	st, err := os.Stat(path)
	if err != nil || st.Size() < maxBytes {
		return
	}
	backup := path + ".1"
	_ = os.Remove(backup)
	_ = os.Rename(path, backup)
}

func main() {
	logPath, closeLog := setupAgentLogging()
	if closeLog != nil {
		defer closeLog()
	}
	if logPath != "" {
		log.Printf("agent log path=%s", logPath)
	}

	configOnly := flag.Bool("print-config", false, "print redacted effective config and exit")
	doctor := flag.Bool("doctor", false, "print diagnostic report and exit")
	labSmoke := flag.Bool("lab-smoke", false, "run local lab smoke against running agent and exit")
	downloadSmoke := flag.Bool("download-smoke", false, "run local download smoke and exit")
	flag.Parse()

	if *doctor {
		os.Exit(RunDoctor())
	}
	if *labSmoke {
		os.Exit(runLabSmoke())
	}
	if *downloadSmoke {
		os.Exit(runDownloadSmoke())
	}

	cfg, path, err := config.LoadOrCreate()
	if err != nil {
		log.Fatalf("config: %v", err)
	}
	if err := os.MkdirAll(cfg.ReceivePath, 0o755); err != nil {
		log.Fatalf("receive path: %v", err)
	}
	if *configOnly {
		cfg.NodeSecret = "[redacted]"
		b, _ := json.MarshalIndent(cfg, "", "  ")
		fmt.Printf("config path: %s\n%s\n", path, string(b))
		return
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	a := &app{cfg: cfg, configPath: path, stop: stop, jobsByID: map[string]*jobState{}, pullsByID: map[string]*pullJobState{}, receiverSessions: map[string]*receiverSessionState{}}
	a.restoreOperationHistory()
	if err := a.selectAndPersistPorts(path); err != nil {
		log.Fatalf("select ports: %v", err)
	}
	a.writeRuntimeState()
	a.downloads = download.NewManager(cfg)
	a.disc = discovery.NewManager(
		cfg.NodeID, cfg.DisplayName, cfg.AppVersion,
		cfg.SelectedPorts.Transfer, cfg.SelectedPorts.Discovery, cfg.SelectedPorts.Control,
		cfg.DiscoveryPortRange.Start, cfg.DiscoveryPortRange.End, a.localIPs,
	)
	a.disc.Start(ctx)

	go a.runReceiver(ctx)
	go a.runLocalAPI(ctx)
	go a.runControlAPI(ctx)

	log.Printf("multisend-agent running node=%s transfer=%d discovery=%d api=%d control=%d", cfg.NodeID, cfg.SelectedPorts.Transfer, cfg.SelectedPorts.Discovery, cfg.SelectedPorts.LocalAPI, cfg.SelectedPorts.Control)
	<-ctx.Done()
	log.Printf("multisend-agent stopping")
}

func (a *app) selectAndPersistPorts(cfgPath string) error {
	pickTransfer, err := ports.PickTCP("0.0.0.0", a.cfg.TransferPortRange)
	if err != nil {
		return err
	}
	pickDiscovery, err := ports.PickUDP("0.0.0.0", a.cfg.DiscoveryPortRange)
	if err != nil {
		return err
	}
	pickAPI, err := ports.PickTCP("127.0.0.1", a.cfg.LocalAPIPortRange)
	if err != nil {
		return err
	}
	pickControl, err := ports.PickTCP("0.0.0.0", a.cfg.ControlPortRange)
	if err != nil {
		return err
	}
	a.cfg.SelectedPorts = config.SelectedPorts{
		Transfer:  pickTransfer,
		Discovery: pickDiscovery,
		LocalAPI:  pickAPI,
		Control:   pickControl,
	}
	a.cfg.TransferPort = pickTransfer
	a.cfg.DiscoveryPort = pickDiscovery
	a.cfg.LocalAPIPort = pickAPI
	a.cfg.ControlPort = pickControl
	return config.Save(cfgPath, a.cfg)
}

func (a *app) writeRuntimeState() {
	local := os.Getenv("LOCALAPPDATA")
	if local == "" {
		return
	}
	dir := filepath.Join(local, "MultiSend")
	_ = os.MkdirAll(dir, 0o755)
	runtimePath := filepath.Join(dir, "runtime.json")
	payload := map[string]any{
		"pid":            os.Getpid(),
		"local_api_port": a.cfg.SelectedPorts.LocalAPI,
		"transfer_port":  a.cfg.SelectedPorts.Transfer,
		"discovery_port": a.cfg.SelectedPorts.Discovery,
		"control_port":   a.cfg.SelectedPorts.Control,
		"started_at":     time.Now().Format(time.RFC3339),
		"agent_log_path": agentLogPath(),
	}
	b, _ := json.MarshalIndent(payload, "", "  ")
	_ = os.WriteFile(runtimePath, b, 0o644)
}

func (a *app) runReceiver(ctx context.Context) {
	ln, err := net.Listen("tcp", fmt.Sprintf("0.0.0.0:%d", a.cfg.SelectedPorts.Transfer))
	if err != nil {
		log.Printf("receiver listen error: %v", err)
		return
	}
	defer ln.Close()
	go func() {
		<-ctx.Done()
		_ = ln.Close()
	}()
	for {
		conn, err := ln.Accept()
		if err != nil {
			if strings.Contains(strings.ToLower(err.Error()), "closed") {
				return
			}
			continue
		}
		go a.handleConn(conn)
	}
}

func (a *app) handleConn(conn net.Conn) {
	defer conn.Close()
	h, err := proto.ReadHeader(conn)
	if err != nil {
		log.Printf("read header: %v", err)
		return
	}
	if !proto.VerifyHeader(h, a.cfg.AuthSecret()) {
		log.Printf("reject unauthenticated chunk transfer=%s index=%d", h.TransferID, h.ChunkIndex)
		_ = proto.WriteChunkAck(conn, proto.ChunkAck{
			Type:       "chunk_ack",
			TransferID: h.TransferID,
			ChunkIndex: h.ChunkIndex,
			Status:     "unauthorized",
			Error:      "authentication required",
		})
		return
	}
	name := filepath.Base(h.FileName)
	safeTransferID := strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			return r
		}
		return -1
	}, h.TransferID)
	if safeTransferID == "" {
		safeTransferID = "unknown"
	}
	rootPath := a.cfg.ReceivePath
	if h.Lab {
		labRoot, err := validateLabReceivePath(a.cfg.ReceivePath, h.LabPath)
		if err != nil {
			log.Printf("reject lab receive path: %v", err)
			return
		}
		rootPath = labRoot
	} else if isPullHeader(h) {
		pullRoot, err := resolveReceiveRelRoot(a.cfg.ReceivePath, h.ReceiveRelPath)
		if err != nil {
			log.Printf("reject pull receive path: %v", err)
			return
		}
		rootPath = pullRoot
	}
	rootAbs, _ := filepath.Abs(rootPath)
	sessionBase := rootAbs
	if isPullHeader(h) {
		sessionBase = filepath.Join(rootAbs, "_pull_sessions")
	}
	sessionDir := filepath.Join(sessionBase, fmt.Sprintf("%s_%s", name, safeTransferID))
	sessionAbs, _ := filepath.Abs(sessionDir)
	if !isUnder(sessionAbs, rootAbs) {
		log.Printf("reject path traversal: %s", sessionAbs)
		return
	}
	if err := os.MkdirAll(sessionDir, 0o755); err != nil {
		log.Printf("mkdir session: %v", err)
		return
	}
	a.ensureReceiverSession(sessionDir, name, h)
	target := filepath.Join(sessionDir, fmt.Sprintf("%s.chunk%06d", name, h.ChunkIndex+1))
	targetAbs, _ := filepath.Abs(target)
	if !strings.HasPrefix(strings.ToLower(targetAbs), strings.ToLower(sessionAbs)) {
		log.Printf("reject chunk path traversal: %s", targetAbs)
		return
	}
	if st, err := os.Stat(target); err == nil && st.Size() == h.PartSize {
		if err := a.updateReceiverManifest(sessionDir, name, h, h.PartSize); err != nil {
			log.Printf("save manifest for existing chunk: %v", err)
			return
		}
		if err := proto.WriteChunkAck(conn, proto.ChunkAck{
			Type:          "chunk_ack",
			TransferID:    h.TransferID,
			ChunkIndex:    h.ChunkIndex,
			Status:        "done",
			BytesReceived: h.PartSize,
		}); err != nil {
			log.Printf("write ack existing: %v", err)
		}
		a.maybeScheduleReceiverFinalize(sessionDir)
		return
	}
	f, err := os.Create(target)
	if err != nil {
		log.Printf("create target: %v", err)
		return
	}
	defer f.Close()
	hasher := sha256.New()
	written, err := io.CopyN(io.MultiWriter(f, hasher), conn, h.PartSize)
	if err != nil {
		log.Printf("copy payload: %v", err)
		return
	}
	// SEC-3: verify the payload against the sender-supplied digest before acking.
	if h.ChunkSHA256 != "" {
		got := hex.EncodeToString(hasher.Sum(nil))
		if !strings.EqualFold(got, h.ChunkSHA256) {
			log.Printf("reject chunk hash mismatch transfer=%s index=%d", h.TransferID, h.ChunkIndex)
			_ = f.Close()
			_ = os.Remove(target)
			_ = proto.WriteChunkAck(conn, proto.ChunkAck{
				Type:       "chunk_ack",
				TransferID: h.TransferID,
				ChunkIndex: h.ChunkIndex,
				Status:     "hash_mismatch",
				Error:      "chunk integrity check failed",
			})
			return
		}
	}
	if err := a.updateReceiverManifest(sessionDir, name, h, written); err != nil {
		log.Printf("save manifest: %v", err)
		return
	}
	if err := proto.WriteChunkAck(conn, proto.ChunkAck{
		Type:          "chunk_ack",
		TransferID:    h.TransferID,
		ChunkIndex:    h.ChunkIndex,
		Status:        "done",
		BytesReceived: written,
	}); err != nil {
		log.Printf("write ack: %v", err)
		return
	}
	a.maybeScheduleReceiverFinalize(sessionDir)
	log.Printf("received %s (%d bytes)", filepath.Base(target), h.PartSize)
}

func (a *app) updateReceiverManifest(sessionDir, name string, h proto.Header, written int64) error {
	a.receiverSessionsMu.RLock()
	st := a.receiverSessions[sessionDir]
	a.receiverSessionsMu.RUnlock()
	if st == nil {
		return fmt.Errorf("receiver session is not initialized")
	}
	st.manifestMu.Lock()
	defer st.manifestMu.Unlock()

	mfPath := filepath.Join(sessionDir, "manifest.json")
	mf, lerr := manifest.Load(mfPath)
	if lerr != nil {
		mf = &manifest.Manifest{
			SchemaVersion: 1,
			TransferID:    h.TransferID,
			Type:          "p2p",
			FileName:      name,
			TotalBytes:    h.TotalBytes,
			ChunkSize:     h.PartSize,
			CreatedAt:     time.Now().UTC(),
			UpdatedAt:     time.Now().UTC(),
			Source:        manifest.SourceInfo{Type: "p2p"},
			Target:        receiverTargetInfo(sessionDir, name, h),
		}
	}
	if chunk := mf.GetChunk(h.ChunkIndex); chunk != nil {
		mf.MarkDone(h.ChunkIndex, written, "")
	} else {
		mf.Chunks = append(mf.Chunks, manifest.ChunkState{
			Index:     h.ChunkIndex,
			Offset:    h.Offset,
			Size:      h.PartSize,
			Status:    manifest.StatusDone,
			BytesDone: written,
		})
	}
	if err := manifest.SaveAtomic(mfPath, mf); err != nil {
		return err
	}
	a.updateReceiverSession(sessionDir, func(s *receiverSession) {
		s.SessionDir = sessionDir
		s.TransferID = h.TransferID
		s.FileName = name
		s.ManifestPath = mfPath
		s.Status = "receiving"
		s.ChunksTotal = int64(len(mf.Chunks))
		s.ChunksDone = countManifestChunksByStatus(mf, manifest.StatusDone)
		s.ChunksFailed = countManifestChunksByStatus(mf, manifest.StatusFailed)
		s.MergeStatus = "pending"
	})
	return nil
}

func (a *app) ensureReceiverSession(sessionDir, name string, h proto.Header) {
	a.updateReceiverSession(sessionDir, func(s *receiverSession) {
		s.SessionDir = sessionDir
		s.TransferID = h.TransferID
		s.FileName = name
		if s.Status == "" {
			s.Status = "receiving"
		}
		if s.MergeStatus == "" {
			s.MergeStatus = "pending"
		}
	})
}

func (a *app) updateReceiverSession(sessionDir string, fn func(*receiverSession)) {
	a.receiverSessionsMu.Lock()
	defer a.receiverSessionsMu.Unlock()
	st := a.receiverSessions[sessionDir]
	if st == nil {
		st = &receiverSessionState{session: receiverSession{SessionDir: sessionDir, MergeStatus: "pending"}}
		a.receiverSessions[sessionDir] = st
	}
	st.mu.Lock()
	defer st.mu.Unlock()
	fn(&st.session)
}

func (a *app) maybeScheduleReceiverFinalize(sessionDir string) {
	a.receiverSessionsMu.RLock()
	st := a.receiverSessions[sessionDir]
	a.receiverSessionsMu.RUnlock()
	if st == nil {
		return
	}
	st.mu.Lock()
	if st.session.MergeStatus == "running" || st.session.MergeStatus == "done" {
		st.mu.Unlock()
		return
	}
	if st.timer != nil {
		st.timer.Stop()
	}
	st.session.MergeStatus = "pending"
	st.session.MergeMessage = ""
	st.timer = time.AfterFunc(3*time.Second, func() { a.finalizeReceiverSession(sessionDir) })
	st.mu.Unlock()
}

func (a *app) finalizeReceiverSession(sessionDir string) {
	a.receiverSessionsMu.RLock()
	st := a.receiverSessions[sessionDir]
	a.receiverSessionsMu.RUnlock()
	if st == nil {
		return
	}
	st.mu.Lock()
	if st.session.MergeStatus == "running" || st.session.MergeStatus == "done" {
		st.mu.Unlock()
		return
	}
	st.session.MergeStatus = "running"
	st.session.MergeMessage = ""
	st.session.CleanupStatus = "pending"
	st.mu.Unlock()

	mfPath := filepath.Join(sessionDir, "manifest.json")
	mf, err := manifest.Load(mfPath)
	if err != nil {
		a.failReceiverFinalize(sessionDir, "load manifest: "+err.Error())
		return
	}
	if err := validateReceiverManifestComplete(mf); err != nil {
		a.failReceiverFinalize(sessionDir, err.Error())
		return
	}
	finalPath := receiverFinalPath(sessionDir, mf)
	if err := validateReceiverPath(sessionDir, finalPath); err != nil {
		a.failReceiverFinalize(sessionDir, err.Error())
		return
	}
	if info, err := os.Stat(finalPath); err == nil {
		if info.Size() == mf.TotalBytes {
			a.markReceiverDone(sessionDir, mf, finalPath, "already finalized")
			return
		}
		if err := os.Remove(finalPath); err != nil {
			a.failReceiverFinalize(sessionDir, "remove stale final: "+err.Error())
			return
		}
	}
	tmpPath := finalPath + ".tmp"
	if err := mergeReceiverChunks(tmpPath, sessionDir, mf); err != nil {
		_ = os.Remove(tmpPath)
		a.failReceiverFinalize(sessionDir, err.Error())
		return
	}
	if err := os.Rename(tmpPath, finalPath); err != nil {
		_ = os.Remove(tmpPath)
		a.failReceiverFinalize(sessionDir, "publish final: "+err.Error())
		return
	}
	if err := validateReceiverFinalSize(finalPath, mf.TotalBytes); err != nil {
		a.failReceiverFinalize(sessionDir, err.Error())
		return
	}
	finalOutputPath := finalPath
	if mf.Target.Address == "extract" {
		extractDir := strings.TrimSuffix(finalPath, ".zip")
		if extractDir == finalPath {
			extractDir = finalPath + ".extracted"
		}
		if err := extractZipSafe(finalPath, extractDir); err != nil {
			a.failReceiverFinalize(sessionDir, "extract zip: "+err.Error())
			return
		}
		_ = os.Remove(finalPath)
		finalOutputPath = extractDir
	}
	if err := cleanupReceiverChunks(sessionDir); err != nil {
		a.failReceiverFinalize(sessionDir, err.Error())
		return
	}
	a.markReceiverDone(sessionDir, mf, finalOutputPath, "merged and cleaned")
}

func (a *app) failReceiverFinalize(sessionDir, msg string) {
	log.Printf("receiver_finalize_failed session_dir=%q message=%q", sessionDir, msg)
	a.updateReceiverSession(sessionDir, func(s *receiverSession) {
		s.MergeStatus = "failed"
		s.MergeMessage = msg
		s.CleanupStatus = "failed"
	})
}

func (a *app) markReceiverDone(sessionDir string, mf *manifest.Manifest, finalPath, msg string) {
	log.Printf("receiver_finalize_done session_dir=%q transfer_id=%s final=%q message=%q", sessionDir, mf.TransferID, finalPath, msg)
	a.updateReceiverSession(sessionDir, func(s *receiverSession) {
		s.MergeStatus = "done"
		s.MergeMessage = msg
		s.CleanupStatus = "done"
		s.Status = "done"
		s.Message = ""
		s.ManifestPath = filepath.Join(sessionDir, "manifest.json")
		s.FinalOutputPath = finalPath
		s.ChunksTotal = int64(len(mf.Chunks))
		s.ChunksDone = countManifestChunksByStatus(mf, manifest.StatusDone)
		s.ChunksFailed = countManifestChunksByStatus(mf, manifest.StatusFailed)
	})
}

func countManifestChunksByStatus(mf *manifest.Manifest, status string) int64 {
	var n int64
	for _, c := range mf.Chunks {
		if c.Status == status {
			n++
		}
	}
	return n
}

func validateReceiverManifestComplete(mf *manifest.Manifest) error {
	if mf == nil {
		return fmt.Errorf("manifest is nil")
	}
	if mf.TotalBytes <= 0 {
		return fmt.Errorf("invalid total bytes")
	}
	if countManifestChunksByStatus(mf, manifest.StatusDone) != int64(len(mf.Chunks)) {
		return fmt.Errorf("chunks_done must equal chunks_total")
	}
	if countManifestChunksByStatus(mf, manifest.StatusFailed) != 0 {
		return fmt.Errorf("chunks_failed must be zero")
	}
	return nil
}

func validateReceiverFinalSize(finalPath string, totalBytes int64) error {
	info, err := os.Stat(finalPath)
	if err != nil {
		return err
	}
	if info.Size() != totalBytes {
		return fmt.Errorf("final size mismatch got=%d want=%d", info.Size(), totalBytes)
	}
	return nil
}

func validateReceiverPath(root, target string) error {
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return err
	}
	targetAbs, err := filepath.Abs(target)
	if err != nil {
		return err
	}
	if isUnder(targetAbs, rootAbs) {
		return nil
	}
	pullRoot := pullPublishRoot(rootAbs)
	if pullRoot != "" && isUnder(targetAbs, pullRoot) && !isUnder(targetAbs, filepath.Join(pullRoot, "_pull_sessions")) {
		return nil
	}
	if !isUnder(targetAbs, rootAbs) {
		return fmt.Errorf("path outside receive root")
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

func isPullHeader(h proto.Header) bool {
	return strings.TrimSpace(h.ReceiveRelPath) != "" || strings.TrimSpace(h.PublishName) != "" || strings.TrimSpace(h.FolderResult) != ""
}

func validateReceiveRelPath(rel string) (string, error) {
	rel = strings.TrimSpace(rel)
	if rel == "" || rel == "." {
		return ".", nil
	}
	if filepath.IsAbs(rel) {
		return "", fmt.Errorf("path must be relative")
	}
	clean := filepath.Clean(rel)
	if clean == "." {
		return ".", nil
	}
	if clean == ".." || strings.HasPrefix(clean, ".."+string(os.PathSeparator)) {
		return "", fmt.Errorf("path traversal is not allowed")
	}
	return clean, nil
}

func resolveReceiveRelRoot(receivePath, rel string) (string, error) {
	clean, err := validateReceiveRelPath(rel)
	if err != nil {
		return "", err
	}
	rootAbs, err := filepath.Abs(receivePath)
	if err != nil {
		return "", err
	}
	target := rootAbs
	if clean != "." {
		target = filepath.Join(rootAbs, clean)
	}
	targetAbs, err := filepath.Abs(target)
	if err != nil {
		return "", err
	}
	if !isUnder(targetAbs, rootAbs) {
		return "", fmt.Errorf("path must stay inside %s", rootAbs)
	}
	return targetAbs, nil
}

func sanitizePublishName(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return ""
	}
	name = filepath.Base(strings.ReplaceAll(name, "/", string(os.PathSeparator)))
	if name == "." || name == string(os.PathSeparator) {
		return ""
	}
	return name
}

func receiverTargetInfo(sessionDir, name string, h proto.Header) manifest.TargetInfo {
	if !isPullHeader(h) {
		return manifest.TargetInfo{Type: "p2p", Path: sessionDir}
	}
	publishName := sanitizePublishName(h.PublishName)
	if publishName == "" {
		publishName = filepath.Base(name)
	}
	root := pullPublishRoot(sessionDir)
	if root == "" {
		root = sessionDir
	}
	return manifest.TargetInfo{
		Type:    "file",
		Path:    filepath.Join(root, publishName),
		Address: config.NormalizePullFolderResult(h.FolderResult),
	}
}

func receiverFinalPath(sessionDir string, mf *manifest.Manifest) string {
	if mf != nil && mf.Target.Type == "file" && strings.TrimSpace(mf.Target.Path) != "" {
		return mf.Target.Path
	}
	if mf == nil {
		return sessionDir
	}
	return filepath.Join(sessionDir, mf.FileName)
}

func pullPublishRoot(sessionDir string) string {
	parent := filepath.Dir(sessionDir)
	if strings.EqualFold(filepath.Base(parent), "_pull_sessions") {
		return filepath.Dir(parent)
	}
	return ""
}

func extractZipSafe(zipPath, destDir string) error {
	destAbs, err := filepath.Abs(destDir)
	if err != nil {
		return err
	}
	if _, err := os.Stat(destAbs); err == nil {
		return fmt.Errorf("destination already exists: %s", destAbs)
	} else if !os.IsNotExist(err) {
		return err
	}
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return err
	}
	defer r.Close()
	for _, f := range r.File {
		target := filepath.Join(destAbs, f.Name)
		targetAbs, err := filepath.Abs(target)
		if err != nil {
			return err
		}
		if !isUnder(targetAbs, destAbs) {
			return fmt.Errorf("zip entry outside destination: %s", f.Name)
		}
		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(targetAbs, 0o755); err != nil {
				return err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(targetAbs), 0o755); err != nil {
			return err
		}
		src, err := f.Open()
		if err != nil {
			return err
		}
		dst, err := os.OpenFile(targetAbs, os.O_CREATE|os.O_WRONLY|os.O_EXCL, f.Mode())
		if err != nil {
			_ = src.Close()
			return err
		}
		_, copyErr := io.Copy(dst, src)
		closeErr := dst.Close()
		_ = src.Close()
		if copyErr != nil {
			return copyErr
		}
		if closeErr != nil {
			return closeErr
		}
	}
	return nil
}

func mergeReceiverChunks(finalPath, sessionDir string, mf *manifest.Manifest) error {
	if err := validateReceiverPath(sessionDir, finalPath); err != nil {
		return err
	}
	out, err := os.Create(finalPath)
	if err != nil {
		return err
	}
	defer out.Close()
	chunks := append([]manifest.ChunkState(nil), mf.Chunks...)
	sort.Slice(chunks, func(i, j int) bool { return chunks[i].Index < chunks[j].Index })
	for _, c := range chunks {
		part := filepath.Join(sessionDir, fmt.Sprintf("%s.chunk%06d", mf.FileName, c.Index+1))
		if err := validateReceiverPath(sessionDir, part); err != nil {
			return err
		}
		f, err := os.Open(part)
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
	return nil
}

func cleanupReceiverChunks(sessionDir string) error {
	files, err := filepath.Glob(filepath.Join(sessionDir, "*.chunk*"))
	if err != nil {
		return err
	}
	for _, f := range files {
		if strings.HasSuffix(f, ".tmp") {
			continue
		}
		if err := os.Remove(f); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	return nil
}

func (a *app) runLocalAPI(ctx context.Context) {
	srv := newAgentHTTPServer(net.JoinHostPort("127.0.0.1", strconv.Itoa(a.cfg.SelectedPorts.LocalAPI)), a.localHandler())
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
	}()
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Printf("api error: %v", err)
	}
}

func (a *app) runControlAPI(ctx context.Context) {
	srv := newAgentHTTPServer(net.JoinHostPort("0.0.0.0", strconv.Itoa(a.cfg.SelectedPorts.Control)), a.controlHandler())
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
	}()
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Printf("control api error: %v", err)
	}
}

func (a *app) health(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeAPIError(w, r, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}
	writeJSON(w, map[string]any{"ok": true, "node_id": a.cfg.NodeID, "name": a.cfg.DisplayName, "agent_log_path": agentLogPath()})
}

func (a *app) peers(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeAPIError(w, r, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}
	writeJSON(w, a.disc.Peers())
}

func (a *app) send(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeAPIError(w, r, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}
	var req struct {
		FilePath       string `json:"file_path"`
		PeerNodeID     string `json:"peer_node_id"`
		PeerAddress    string `json:"peer_address"`
		Lab            bool   `json:"lab"`
		LabReceivePath string `json:"lab_receive_path"`
		ChunkSizeMB    int64  `json:"chunk_size_mb"`
		CleanupPath    string `json:"cleanup_path"`
	}
	if !decodeJSONRequest(w, r, &req) {
		return
	}
	if req.FilePath == "" || (req.PeerNodeID == "" && req.PeerAddress == "") {
		writeAPIError(w, r, http.StatusBadRequest, "missing_destination", "file_path and (peer_node_id or peer_address) are required")
		return
	}
	if err := validateChunkSizeMB(req.ChunkSizeMB); err != nil {
		writeAPIError(w, r, http.StatusBadRequest, "invalid_chunk_size", err.Error())
		return
	}
	cleanupPath, err := validateManagedCleanupPath(req.FilePath, req.CleanupPath)
	if err != nil {
		writeAPIError(w, r, http.StatusBadRequest, "invalid_cleanup_path", err.Error())
		return
	}
	resp, err := a.startSend(req.FilePath, req.PeerNodeID, req.PeerAddress, sendTargetOptions{
		Lab:            req.Lab,
		LabReceivePath: req.LabReceivePath,
		CleanupPath:    cleanupPath,
	}, req.ChunkSizeMB)
	if err != nil {
		writeAPIError(w, r, http.StatusBadRequest, "send_start_failed", err.Error())
		return
	}
	writeJSON(w, resp)
}

func (a *app) startSend(filePath, peerNodeID, peerAddress string, targetOpts sendTargetOptions, chunkSizeMB int64) (map[string]string, error) {
	if err := validateChunkSizeMB(chunkSizeMB); err != nil {
		return nil, err
	}
	filePath = strings.TrimSpace(filePath)
	if !filepath.IsAbs(filePath) {
		return nil, fmt.Errorf("file_path must be absolute")
	}
	filePath = filepath.Clean(filePath)
	sendOpts := transfer.SendOptions{}
	if chunkSizeMB > 0 {
		sendOpts.ChunkSize = chunkSizeMB * 1024 * 1024
	}
	if !targetOpts.Lab && strings.TrimSpace(targetOpts.LabReceivePath) != "" {
		return nil, fmt.Errorf("lab_receive_path requires lab=true")
	}
	if targetOpts.Lab {
		labPath, err := validateLabReceivePath(a.cfg.ReceivePath, targetOpts.LabReceivePath)
		if err != nil {
			return nil, fmt.Errorf("invalid lab_receive_path: %w", err)
		}
		sendOpts.Lab = true
		sendOpts.LabReceivePath = labPath
	}
	if !targetOpts.Lab {
		if strings.TrimSpace(targetOpts.ReceiveRelPath) != "" {
			rel, err := validateReceiveRelPath(targetOpts.ReceiveRelPath)
			if err != nil {
				return nil, fmt.Errorf("invalid receive_rel_path: %w", err)
			}
			sendOpts.ReceiveRelPath = rel
		}
		sendOpts.PublishName = sanitizePublishName(targetOpts.PublishName)
		if sendOpts.ReceiveRelPath != "" || sendOpts.PublishName != "" || strings.TrimSpace(targetOpts.FolderResult) != "" {
			sendOpts.FolderResult = config.NormalizePullFolderResult(targetOpts.FolderResult)
		}
	}

	st, err := os.Stat(filePath)
	if err != nil {
		return nil, fmt.Errorf("file not found")
	}
	if st.IsDir() {
		return nil, fmt.Errorf("directory is not supported in v1")
	}

	targetAddr := ""
	if peerAddress != "" {
		targetAddr = peerAddress
		if !strings.Contains(targetAddr, ":") {
			targetAddr = fmt.Sprintf("%s:%d", targetAddr, a.cfg.SelectedPorts.Transfer)
		}
	} else {
		peers := a.disc.Peers()
		var selected *discovery.Peer
		for i := range peers {
			if peers[i].NodeID == peerNodeID {
				selected = &peers[i]
				break
			}
		}
		if selected == nil {
			return nil, fmt.Errorf("peer not found")
		}
		targetIP := selected.Addr
		if targetIP == "" && len(selected.IPs) > 0 {
			targetIP = selected.IPs[0]
		}
		targetAddr = fmt.Sprintf("%s:%d", targetIP, selected.TransferPort)
	}

	jobID := fmt.Sprintf("job-%d", time.Now().UnixNano())
	jobCtx := a.createJob(jobID, st.Size(), filePath, targetAddr, sendOpts, targetOpts.CleanupPath)
	log.Printf("send_start job_id=%s target=%s file=%q size=%d lab=%t receive_rel=%q publish_name=%q folder_result=%q cleanup_path=%q",
		jobID, targetAddr, filePath, st.Size(), sendOpts.Lab, sendOpts.ReceiveRelPath, sendOpts.PublishName, sendOpts.FolderResult, targetOpts.CleanupPath)

	go func() {
		ips := a.localIPs()
		result, err := transfer.SendFileSplitWithResultCtx(jobCtx, filePath, transfer.PeerTarget{Address: targetAddr}, ips, func(s transfer.ProgressSnapshot) {
			a.updateJobProgress(jobID, s)
		}, a.enrichSendOptions(sendOpts))
		if err != nil {
			if errors.Is(err, context.Canceled) {
				log.Printf("send_canceled job_id=%s target=%s file=%q", jobID, targetAddr, filePath)
				a.finishJob(jobID, "canceled", "transfer canceled by user")
				return
			}
			log.Printf("send_failed job_id=%s target=%s file=%q err=%v", jobID, targetAddr, filePath, err)
			a.finishJob(jobID, "failed", err.Error())
			return
		}
		a.setJobManifest(jobID, result.ManifestPath)
		log.Printf("send_done job_id=%s transfer_id=%s manifest=%q target=%s file=%q", jobID, result.TransferID, result.ManifestPath, targetAddr, filePath)
		a.finishJob(jobID, "done", "")
		if targetOpts.CleanupPath != "" {
			if err := removeManagedCleanupPath(filePath, targetOpts.CleanupPath); err != nil {
				log.Printf("send_cleanup_failed job_id=%s cleanup_path=%q err=%v", jobID, targetOpts.CleanupPath, err)
			} else {
				log.Printf("send_cleanup_done job_id=%s cleanup_path=%q", jobID, targetOpts.CleanupPath)
			}
		}
	}()
	return map[string]string{"job_id": jobID, "status": "running"}, nil
}

func (a *app) jobs(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeAPIError(w, r, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}
	a.jobsMu.Lock()
	list := make([]jobDetails, 0, len(a.jobsByID))
	for _, s := range a.jobsByID {
		a.refreshJobFromManifestLocked(&s.details)
		list = append(list, s.details)
	}
	a.jobsMu.Unlock()
	sort.Slice(list, func(i, j int) bool { return list[i].StartedAt > list[j].StartedAt })
	writeJSON(w, list)
}

func (a *app) jobByID(w http.ResponseWriter, r *http.Request) {
	rel := strings.TrimPrefix(r.URL.Path, "/jobs/")
	rel = strings.Trim(rel, "/")
	if rel == "" {
		writeAPIError(w, r, http.StatusBadRequest, "job_id_required", "job id required")
		return
	}
	parts := strings.Split(rel, "/")
	id := parts[0]

	if len(parts) == 2 && parts[1] == "cancel" {
		if r.Method != http.MethodPost {
			writeAPIError(w, r, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}
		if a.cancelJob(id) {
			writeJSON(w, map[string]string{"job_id": id, "status": "canceling"})
			return
		}
		writeAPIError(w, r, http.StatusNotFound, "job_not_found", "job not found")
		return
	}
	if len(parts) == 2 && parts[1] == "resume" {
		if r.Method != http.MethodPost {
			writeAPIError(w, r, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}
		if err := a.resumeJob(id); err != nil {
			writeAPIError(w, r, http.StatusBadRequest, "resume_failed", err.Error())
			return
		}
		writeJSON(w, map[string]string{"job_id": id, "status": "running"})
		return
	}
	if len(parts) == 1 && r.Method == http.MethodDelete {
		if err := a.forgetJob(id); err != nil {
			writeAPIError(w, r, http.StatusBadRequest, "job_forget_failed", err.Error())
			return
		}
		log.Printf("job_history_removed job_id=%s", id)
		writeJSON(w, map[string]string{"job_id": id, "status": "removed"})
		return
	}

	if r.Method != http.MethodGet {
		writeAPIError(w, r, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}
	a.jobsMu.RLock()
	state := a.jobsByID[id]
	a.jobsMu.RUnlock()
	if state == nil {
		writeAPIError(w, r, http.StatusNotFound, "job_not_found", "job not found")
		return
	}
	a.jobsMu.Lock()
	a.refreshJobFromManifestLocked(&state.details)
	details := state.details
	a.jobsMu.Unlock()
	writeJSON(w, details)
}

func (a *app) remoteSend(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeAPIError(w, r, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}
	if !a.isRemoteAllowed(r.RemoteAddr) {
		writeAPIError(w, r, http.StatusForbidden, "forbidden", "forbidden")
		return
	}
	if !a.controlAuthOK(r) {
		writeAPIError(w, r, http.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}
	var req struct {
		FilePath          string `json:"file_path"`
		TargetNodeID      string `json:"target_node_id"`
		TargetPeerAddress string `json:"target_peer_address"`
		TargetReceivePath string `json:"target_receive_relpath"`
		FolderResult      string `json:"folder_result"`
		ChunkSizeMB       int64  `json:"chunk_size_mb"`
	}
	if !decodeJSONRequest(w, r, &req) {
		return
	}
	if strings.TrimSpace(req.FilePath) == "" {
		writeAPIError(w, r, http.StatusBadRequest, "file_path_required", "file_path is required")
		return
	}
	if err := validateChunkSizeMB(req.ChunkSizeMB); err != nil {
		writeAPIError(w, r, http.StatusBadRequest, "invalid_chunk_size", err.Error())
		return
	}
	// SEC-1: a remote peer may only request files inside the configured shared
	// roots. This blocks arbitrary file exfiltration via /remote-send.
	if !a.isRemoteSendPathAllowed(req.FilePath) {
		log.Printf("reject remote-send outside shared roots: %q from %s", req.FilePath, r.RemoteAddr)
		writeAPIError(w, r, http.StatusForbidden, "path_not_shared", "file_path is outside the shared roots")
		return
	}
	peerAddress := strings.TrimSpace(req.TargetPeerAddress)
	if peerAddress == "" {
		peerAddress = a.resolvePeerAddressByNodeID(strings.TrimSpace(req.TargetNodeID))
	}
	if peerAddress == "" {
		writeAPIError(w, r, http.StatusBadRequest, "target_peer_not_found", "target peer not found")
		return
	}
	sendPath, prepared, err := prepareRemoteSendSource(req.FilePath, config.NormalizePullFolderResult(req.FolderResult))
	if err != nil {
		writeAPIError(w, r, http.StatusBadRequest, "prepare_remote_source_failed", err.Error())
		return
	}
	resp, err := a.startSend(sendPath, "", peerAddress, sendTargetOptions{
		ReceiveRelPath: req.TargetReceivePath,
		PublishName:    prepared.PublishName,
		FolderResult:   prepared.FolderResult,
		CleanupPath:    prepared.CleanupPath,
	}, req.ChunkSizeMB)
	if err != nil {
		if prepared.CleanupPath != "" {
			if cleanupErr := removeManagedCleanupPath(sendPath, prepared.CleanupPath); cleanupErr != nil {
				log.Printf("remote_send_prepare_cleanup_failed path=%q err=%v", prepared.CleanupPath, cleanupErr)
			}
		}
		writeAPIError(w, r, http.StatusBadRequest, "remote_send_start_failed", err.Error())
		return
	}
	resp["source_type"] = prepared.SourceType
	resp["publish_name"] = prepared.PublishName
	resp["folder_result"] = prepared.FolderResult
	writeJSON(w, resp)
}

func (a *app) remoteJobByID(w http.ResponseWriter, r *http.Request) {
	if !a.isRemoteAllowed(r.RemoteAddr) {
		writeAPIError(w, r, http.StatusForbidden, "forbidden", "forbidden")
		return
	}
	if !a.controlAuthOK(r) {
		writeAPIError(w, r, http.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}
	rel := strings.TrimPrefix(r.URL.Path, "/remote-jobs/")
	rel = strings.Trim(rel, "/")
	if rel == "" {
		writeAPIError(w, r, http.StatusBadRequest, "job_id_required", "job id required")
		return
	}
	parts := strings.Split(rel, "/")
	id := parts[0]
	if len(parts) == 1 {
		if r.Method != http.MethodGet {
			writeAPIError(w, r, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}
		a.jobsMu.RLock()
		state := a.jobsByID[id]
		a.jobsMu.RUnlock()
		if state == nil {
			writeAPIError(w, r, http.StatusNotFound, "job_not_found", "job not found")
			return
		}
		a.jobsMu.Lock()
		a.refreshJobFromManifestLocked(&state.details)
		details := state.details
		a.jobsMu.Unlock()
		writeJSON(w, details)
		return
	}
	if len(parts) == 2 && parts[1] == "cancel" {
		if r.Method != http.MethodPost {
			writeAPIError(w, r, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}
		if a.cancelJob(id) {
			writeJSON(w, map[string]string{"job_id": id, "status": "canceling"})
			return
		}
		writeAPIError(w, r, http.StatusNotFound, "job_not_found", "job not found")
		return
	}
	if len(parts) == 2 && parts[1] == "resume" {
		if r.Method != http.MethodPost {
			writeAPIError(w, r, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}
		if err := a.resumeJob(id); err != nil {
			writeAPIError(w, r, http.StatusBadRequest, "resume_failed", err.Error())
			return
		}
		writeJSON(w, map[string]string{"job_id": id, "status": "running"})
		return
	}
	writeAPIError(w, r, http.StatusNotFound, "not_found", "not found")
}

func (a *app) pullsHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, a.listPulls())
	case http.MethodPost:
		var req struct {
			SourceURL   string `json:"source_url"`
			OutputDir   string `json:"output_dir"`
			ChunkSizeMB int64  `json:"chunk_size_mb"`
		}
		if !decodeJSONRequest(w, r, &req) {
			return
		}
		if err := validateChunkSizeMB(req.ChunkSizeMB); err != nil {
			writeAPIError(w, r, http.StatusBadRequest, "invalid_chunk_size", err.Error())
			return
		}
		p, err := a.createPull(req.SourceURL, req.OutputDir, req.ChunkSizeMB)
		if err != nil {
			if strings.Contains(err.Error(), "remote-send failed") {
				writeAPIError(w, r, http.StatusBadGateway, "remote_send_failed", err.Error())
			} else {
				writeAPIError(w, r, http.StatusBadRequest, "pull_start_failed", err.Error())
			}
			return
		}
		writeJSON(w, p)
	default:
		writeAPIError(w, r, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
	}
}

func (a *app) pullByID(w http.ResponseWriter, r *http.Request) {
	rel := strings.TrimPrefix(r.URL.Path, "/pulls/")
	rel = strings.Trim(rel, "/")
	if rel == "" {
		writeAPIError(w, r, http.StatusBadRequest, "pull_id_required", "pull id required")
		return
	}
	parts := strings.Split(rel, "/")
	id := parts[0]
	if len(parts) == 1 && r.Method == http.MethodGet {
		p, err := a.refreshPull(id)
		if err != nil {
			writeAPIError(w, r, http.StatusBadRequest, "pull_refresh_failed", err.Error())
			return
		}
		writeJSON(w, p)
		return
	}
	if len(parts) == 2 && parts[1] == "cancel" && r.Method == http.MethodPost {
		p, err := a.pullRemoteAction(id, "cancel")
		if err != nil {
			writeAPIError(w, r, http.StatusBadRequest, "pull_cancel_failed", err.Error())
			return
		}
		writeJSON(w, p)
		return
	}
	if len(parts) == 2 && parts[1] == "resume" && r.Method == http.MethodPost {
		p, err := a.pullRemoteAction(id, "resume")
		if err != nil {
			writeAPIError(w, r, http.StatusBadRequest, "pull_resume_failed", err.Error())
			return
		}
		writeJSON(w, p)
		return
	}
	if len(parts) == 1 && r.Method == http.MethodDelete {
		if err := a.forgetPull(id); err != nil {
			writeAPIError(w, r, http.StatusBadRequest, "pull_forget_failed", err.Error())
			return
		}
		log.Printf("pull_history_removed pull_id=%s", id)
		writeJSON(w, map[string]string{"id": id, "status": "removed"})
		return
	}
	writeAPIError(w, r, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
}

func (a *app) createJob(id string, totalBytes int64, filePath, targetAddr string, opts transfer.SendOptions, cleanupPath string) context.Context {
	now := time.Now()
	ctx, cancel := context.WithCancel(context.Background())
	d := jobDetails{
		ID:              id,
		FilePath:        filePath,
		Status:          "running",
		BytesSent:       0,
		TotalBytes:      totalBytes,
		Percent:         0,
		MbpsNow:         0,
		MbpsAvg:         0,
		Mbps30s:         0,
		EtaSeconds:      -1,
		ResumeSupported: true,
		StartedAt:       now.Format(time.RFC3339),
		UpdatedAt:       now.Format(time.RFC3339),
	}
	a.jobsMu.Lock()
	state := &jobState{
		details:        d,
		filePath:       filePath,
		targetAddr:     targetAddr,
		lab:            opts.Lab,
		labPath:        opts.LabReceivePath,
		receiveRelPath: opts.ReceiveRelPath,
		publishName:    opts.PublishName,
		folderResult:   opts.FolderResult,
		cleanupPath:    cleanupPath,
		chunkSize:      opts.ChunkSize,
		startedAt:      now,
		lastAt:         now,
		cancelFunc:     cancel,
		samples:        make([]progressSample, 0, 40),
	}
	a.jobsByID[id] = state
	a.persistJobLocked(state)
	a.pruneJobsLocked()
	a.jobsMu.Unlock()
	return ctx
}

func (a *app) resumeJob(id string) error {
	a.jobsMu.Lock()
	state := a.jobsByID[id]
	if state == nil {
		a.jobsMu.Unlock()
		return fmt.Errorf("job not found")
	}
	if state.details.Status != "canceled" && state.details.Status != "failed" {
		a.jobsMu.Unlock()
		return fmt.Errorf("job is not resumable")
	}
	if !state.details.ResumeSupported {
		a.jobsMu.Unlock()
		return fmt.Errorf("job is not safely resumable")
	}
	if state.filePath == "" || state.targetAddr == "" {
		a.jobsMu.Unlock()
		return fmt.Errorf("missing job request context")
	}
	ctx, cancel := context.WithCancel(context.Background())
	state.cancelFunc = cancel
	state.details.Status = "running"
	state.details.Message = ""
	state.details.UpdatedAt = time.Now().Format(time.RFC3339)
	a.persistJobLocked(state)
	log.Printf("job_resume_requested job_id=%s file=%q target=%s manifest=%q", id, state.filePath, state.targetAddr, state.details.ManifestPath)
	resumeOptions := state.persistedSendOptions()
	a.jobsMu.Unlock()

	go func(jobID, filePath, targetAddr, cleanupPath string, opts transfer.SendOptions) {
		ips := a.localIPs()
		result, err := transfer.SendFileSplitWithResultCtx(ctx, filePath, transfer.PeerTarget{Address: targetAddr}, ips, func(s transfer.ProgressSnapshot) {
			a.updateJobProgress(jobID, s)
		}, a.enrichSendOptions(opts))
		if err != nil {
			if errors.Is(err, context.Canceled) {
				log.Printf("send_resume_canceled job_id=%s target=%s file=%q", jobID, targetAddr, filePath)
				a.finishJob(jobID, "canceled", "transfer canceled by user")
				return
			}
			log.Printf("send_resume_failed job_id=%s target=%s file=%q err=%v", jobID, targetAddr, filePath, err)
			a.finishJob(jobID, "failed", err.Error())
			return
		}
		a.setJobManifest(jobID, result.ManifestPath)
		log.Printf("send_resume_done job_id=%s transfer_id=%s manifest=%q target=%s file=%q", jobID, result.TransferID, result.ManifestPath, targetAddr, filePath)
		a.finishJob(jobID, "done", "")
		if cleanupPath != "" {
			if err := removeManagedCleanupPath(filePath, cleanupPath); err != nil {
				log.Printf("send_cleanup_failed job_id=%s cleanup_path=%q err=%v", jobID, cleanupPath, err)
			} else {
				log.Printf("send_cleanup_done job_id=%s cleanup_path=%q", jobID, cleanupPath)
			}
		}
	}(id, state.filePath, state.targetAddr, state.cleanupPath, resumeOptions)
	return nil
}

func (a *app) cancelJob(id string) bool {
	a.jobsMu.Lock()
	defer a.jobsMu.Unlock()
	state := a.jobsByID[id]
	if state == nil {
		return false
	}
	if state.details.Status == "done" || state.details.Status == "failed" || state.details.Status == "canceled" {
		return true
	}
	log.Printf("job_cancel_requested job_id=%s status=%s file=%q target=%s", id, state.details.Status, state.filePath, state.targetAddr)
	state.details.Status = "canceling"
	state.details.Message = "cancel requested"
	state.details.UpdatedAt = time.Now().Format(time.RFC3339)
	if state.cancelFunc != nil {
		state.cancelFunc()
	}
	a.persistJobLocked(state)
	return true
}

func (a *app) updateJobProgress(id string, s transfer.ProgressSnapshot) {
	now := time.Now()
	a.jobsMu.Lock()
	defer a.jobsMu.Unlock()
	state := a.jobsByID[id]
	if state == nil {
		return
	}
	d := &state.details
	if d.Status == "done" || d.Status == "failed" || d.Status == "canceled" {
		return
	}

	d.TotalBytes = s.TotalSize
	d.BytesSent = s.TotalBytes
	if d.TotalBytes > 0 {
		d.Percent = (float64(d.BytesSent) * 100) / float64(d.TotalBytes)
	}

	d.Channels.Cable.TotalBytes = s.CableTotal
	d.Channels.Cable.BytesSent = s.CableBytes
	if s.CableTotal > 0 {
		d.Channels.Cable.Percent = (float64(s.CableBytes) * 100) / float64(s.CableTotal)
	}
	d.Channels.Wifi.TotalBytes = s.WifiTotal
	d.Channels.Wifi.BytesSent = s.WifiBytes
	if s.WifiTotal > 0 {
		d.Channels.Wifi.Percent = (float64(s.WifiBytes) * 100) / float64(s.WifiTotal)
	}
	d.ChunksTotal = s.ChunksTotal
	d.ChunksDone = s.ChunksDone
	d.ChunksFailed = s.ChunksFailed
	d.ChunksPending = s.ChunksPending
	d.ChunksSending = s.ChunksSending
	d.ResumeSupported = s.ResumeSupported
	manifestAttached := d.ManifestPath == "" && s.ManifestPath != ""
	if s.ManifestPath != "" {
		d.ManifestPath = s.ManifestPath
	}
	if cd, ok := s.ChannelDetails["cable"]; ok {
		d.Channels.Cable.State = cd.State
		d.Channels.Cable.Interface = cd.InterfaceName
		d.Channels.Cable.LocalIP = cd.LocalIP
		d.Channels.Cable.Failures = cd.Failures
		d.Channels.Cable.LastError = cd.LastError
	}
	if wd, ok := s.ChannelDetails["wifi"]; ok {
		d.Channels.Wifi.State = wd.State
		d.Channels.Wifi.Interface = wd.InterfaceName
		d.Channels.Wifi.LocalIP = wd.LocalIP
		d.Channels.Wifi.Failures = wd.Failures
		d.Channels.Wifi.LastError = wd.LastError
	}
	a.refreshJobFromManifestLocked(d)

	deltaSec := now.Sub(state.lastAt).Seconds()
	if deltaSec > 0 {
		deltaTotal := d.BytesSent - state.lastTotal
		if deltaTotal >= 0 {
			d.MbpsNow = (float64(deltaTotal) * 8) / (deltaSec * 1_000_000)
		}
		deltaCable := s.CableBytes - state.lastCable
		if deltaCable >= 0 {
			d.Channels.Cable.MbpsNow = (float64(deltaCable) * 8) / (deltaSec * 1_000_000)
		}
		deltaWifi := s.WifiBytes - state.lastWifi
		if deltaWifi >= 0 {
			d.Channels.Wifi.MbpsNow = (float64(deltaWifi) * 8) / (deltaSec * 1_000_000)
		}
	}

	elapsed := now.Sub(state.startedAt).Seconds()
	if elapsed > 0 {
		d.MbpsAvg = (float64(d.BytesSent) * 8) / (elapsed * 1_000_000)
		d.Channels.Cable.MbpsAvg = (float64(s.CableBytes) * 8) / (elapsed * 1_000_000)
		d.Channels.Wifi.MbpsAvg = (float64(s.WifiBytes) * 8) / (elapsed * 1_000_000)
	}

	state.samples = append(state.samples, progressSample{At: now, Total: d.BytesSent, Cable: s.CableBytes, Wifi: s.WifiBytes})
	cutoff := now.Add(-30 * time.Second)
	idx := 0
	for idx < len(state.samples) && state.samples[idx].At.Before(cutoff) {
		idx++
	}
	if idx > 0 {
		state.samples = append([]progressSample(nil), state.samples[idx:]...)
	}
	if len(state.samples) >= 2 {
		first := state.samples[0]
		last := state.samples[len(state.samples)-1]
		windowSec := last.At.Sub(first.At).Seconds()
		if windowSec > 0 {
			d.Mbps30s = (float64(last.Total-first.Total) * 8) / (windowSec * 1_000_000)
			d.Channels.Cable.Mbps30s = (float64(last.Cable-first.Cable) * 8) / (windowSec * 1_000_000)
			d.Channels.Wifi.Mbps30s = (float64(last.Wifi-first.Wifi) * 8) / (windowSec * 1_000_000)
		}
	}

	if d.MbpsNow > 0 && d.TotalBytes > d.BytesSent {
		remainingBytes := d.TotalBytes - d.BytesSent
		bps := d.MbpsNow * 1_000_000 / 8
		if bps > 0 {
			d.EtaSeconds = int64(float64(remainingBytes) / bps)
		} else {
			d.EtaSeconds = -1
		}
	} else if d.BytesSent >= d.TotalBytes && d.TotalBytes > 0 {
		d.EtaSeconds = 0
	} else {
		d.EtaSeconds = -1
	}

	d.UpdatedAt = now.Format(time.RFC3339)
	state.lastAt = now
	state.lastTotal = d.BytesSent
	state.lastCable = s.CableBytes
	state.lastWifi = s.WifiBytes
	if manifestAttached {
		a.persistJobLocked(state)
	}
}

func (a *app) finishJob(id, status, message string) {
	now := time.Now()
	a.jobsMu.Lock()
	defer a.jobsMu.Unlock()
	state := a.jobsByID[id]
	if state == nil {
		return
	}
	state.details.Status = status
	state.details.Message = message
	log.Printf("job_finish job_id=%s status=%s message=%q manifest=%q file=%q target=%s", id, status, message, state.details.ManifestPath, state.filePath, state.targetAddr)
	if message != "" && status == "failed" {
		state.details.LastError = message
	}
	state.details.UpdatedAt = now.Format(time.RFC3339)
	state.details.CompletedAt = now.Format(time.RFC3339)
	state.details.EtaSeconds = 0
	state.details.MbpsNow = 0
	state.details.Channels.Cable.MbpsNow = 0
	state.details.Channels.Wifi.MbpsNow = 0
	if state.cancelFunc != nil {
		state.cancelFunc()
		state.cancelFunc = nil
	}
	a.refreshJobFromManifestLocked(&state.details)
	a.persistJobLocked(state)
	a.pruneJobsLocked()
}

func (a *app) setJobManifest(id, manifestPath string) {
	if manifestPath == "" {
		return
	}
	a.jobsMu.Lock()
	defer a.jobsMu.Unlock()
	state := a.jobsByID[id]
	if state == nil {
		return
	}
	state.details.ManifestPath = manifestPath
	a.refreshJobFromManifestLocked(&state.details)
	a.persistJobLocked(state)
}

func (a *app) refreshJobFromManifestLocked(d *jobDetails) {
	if d == nil || d.ManifestPath == "" {
		return
	}
	mf, err := manifest.Load(d.ManifestPath)
	if err != nil {
		return
	}
	failed := make([]failedChunkRef, 0, 5)
	var doneCnt, failCnt, pendingCnt, sendingCnt, doneBytes int64
	for _, ch := range mf.Chunks {
		switch ch.Status {
		case manifest.StatusDone:
			doneCnt++
			doneBytes += ch.BytesDone
		case manifest.StatusFailed:
			failCnt++
		case manifest.StatusSending:
			sendingCnt++
		default:
			pendingCnt++
		}
		if ch.Status != manifest.StatusFailed {
			continue
		}
		if d.LastError == "" && ch.LastError != "" {
			d.LastError = ch.LastError
		}
		if len(failed) < 5 {
			failed = append(failed, failedChunkRef{
				Index:     ch.Index,
				Attempts:  ch.Attempts,
				LastError: ch.LastError,
				Channel:   ch.Channel,
			})
		}
	}
	d.ChunksTotal = int64(len(mf.Chunks))
	d.ChunksDone = doneCnt
	d.ChunksFailed = failCnt
	d.ChunksPending = pendingCnt
	d.ChunksSending = sendingCnt
	if doneBytes > d.BytesSent {
		d.BytesSent = doneBytes
		if d.TotalBytes > 0 {
			d.Percent = (float64(d.BytesSent) * 100) / float64(d.TotalBytes)
		}
	}
	d.FailedChunks = failed
}

func (a *app) receiverSessionsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeAPIError(w, r, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}
	writeJSON(w, a.listReceiverSessions())
}

func (a *app) receiverSessionByID(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeAPIError(w, r, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}
	rel := strings.TrimPrefix(r.URL.Path, "/receiver/sessions/")
	rel = strings.Trim(rel, "/")
	if rel == "" {
		writeAPIError(w, r, http.StatusBadRequest, "session_id_required", "session id required")
		return
	}
	s := a.getReceiverSession(rel)
	if s.SessionDir == "" {
		writeAPIError(w, r, http.StatusNotFound, "session_not_found", "session not found")
		return
	}
	writeJSON(w, s)
}

func (a *app) listReceiverSessions() []receiverSession {
	a.receiverSessionsMu.RLock()
	out := make([]receiverSession, 0, len(a.receiverSessions))
	for _, st := range a.receiverSessions {
		st.mu.Lock()
		out = append(out, st.session)
		st.mu.Unlock()
	}
	a.receiverSessionsMu.RUnlock()
	sort.Slice(out, func(i, j int) bool { return out[i].SessionDir > out[j].SessionDir })
	return out
}

func (a *app) getReceiverSession(sessionDir string) receiverSession {
	a.receiverSessionsMu.RLock()
	st := a.receiverSessions[sessionDir]
	a.receiverSessionsMu.RUnlock()
	if st == nil {
		return receiverSession{}
	}
	st.mu.Lock()
	defer st.mu.Unlock()
	return st.session
}

func (a *app) downloadsHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, a.downloads.List())
	case http.MethodPost:
		var req download.StartRequest
		if !decodeJSONRequest(w, r, &req) {
			return
		}
		if strings.TrimSpace(req.URL) == "" {
			writeAPIError(w, r, http.StatusBadRequest, "url_required", "url is required")
			return
		}
		job, err := a.downloads.Start(req)
		if err != nil {
			msg := err.Error()
			errCode := "bad_request"
			if strings.HasPrefix(msg, "download_probe_failed:") {
				errCode = "download_probe_failed"
				msg = strings.TrimSpace(strings.TrimPrefix(msg, "download_probe_failed:"))
			}
			writeAPIErrorDetail(w, r, http.StatusBadRequest, errCode, msg, "url="+req.URL)
			return
		}
		log.Printf("download_start job_id=%s url=%q output=%q manifest=%q total=%d", job.ID, req.URL, job.OutputPath, job.ManifestPath, job.TotalBytes)
		writeJSON(w, job)
	default:
		writeAPIError(w, r, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
	}
}

func (a *app) downloadByID(w http.ResponseWriter, r *http.Request) {
	rel := strings.TrimPrefix(r.URL.Path, "/downloads/")
	rel = strings.Trim(rel, "/")
	if rel == "" {
		writeAPIError(w, r, http.StatusBadRequest, "download_id_required", "download id required")
		return
	}
	parts := strings.Split(rel, "/")
	id := parts[0]
	if len(parts) == 2 && parts[1] == "cancel" {
		if r.Method != http.MethodPost {
			writeAPIError(w, r, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}
		if !a.downloads.Cancel(id) {
			writeAPIError(w, r, http.StatusNotFound, "job_not_found", "job not found")
			return
		}
		log.Printf("download_cancel_requested job_id=%s", id)
		writeJSON(w, map[string]string{"id": id, "status": "canceled"})
		return
	}
	if len(parts) == 2 && parts[1] == "resume" {
		if r.Method != http.MethodPost {
			writeAPIError(w, r, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}
		job, err := a.downloads.Resume(id)
		if err != nil {
			writeAPIError(w, r, http.StatusBadRequest, "download_resume_failed", err.Error())
			return
		}
		log.Printf("download_resume_requested job_id=%s output=%q manifest=%q", id, job.OutputPath, job.ManifestPath)
		writeJSON(w, job)
		return
	}
	if len(parts) == 1 && r.Method == http.MethodDelete {
		res, err := a.downloads.Forget(id)
		if err != nil {
			writeAPIError(w, r, http.StatusBadRequest, "download_forget_failed", err.Error())
			return
		}
		log.Printf("download_history_removed job_id=%s deleted_chunks=%d freed_bytes=%d manifest_deleted=%t dir_deleted=%t warnings=%d",
			id, res.DeletedChunks, res.FreedBytes, res.ManifestDeleted, res.DirDeleted, len(res.Warnings))
		writeJSON(w, res)
		return
	}
	if len(parts) == 3 && parts[1] == "cleanup" && parts[2] == "preview" {
		if r.Method != http.MethodGet {
			writeAPIError(w, r, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}
		p, err := a.downloads.PreviewCleanup(id)
		if err != nil {
			writeAPIError(w, r, http.StatusBadRequest, "cleanup_preview_failed", err.Error())
			return
		}
		log.Printf("download_cleanup_preview job_id=%s chunk_files=%d chunk_bytes=%d manifest=%q final_file=%q safe=%t",
			id, p.ChunkFiles, p.ChunkBytes, p.ManifestPath, p.FinalFile, p.Safe)
		writeJSON(w, p)
		return
	}
	if len(parts) == 2 && parts[1] == "cleanup" {
		if r.Method != http.MethodPost {
			writeAPIError(w, r, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}
		var req download.CleanupRequest
		if !decodeOptionalJSONRequest(w, r, &req) {
			return
		}
		res, err := a.downloads.Cleanup(id, req)
		if err != nil {
			writeAPIError(w, r, http.StatusBadRequest, "cleanup_failed", err.Error())
			return
		}
		log.Printf("download_cleanup job_id=%s deleted_chunks=%d freed_bytes=%d manifest_deleted=%t dir_deleted=%t remaining_chunks=%d warnings=%d",
			id, res.DeletedChunks, res.FreedBytes, res.ManifestDeleted, res.DirDeleted, res.RemainingChunks, len(res.Warnings))
		writeJSON(w, res)
		return
	}
	if r.Method != http.MethodGet {
		writeAPIError(w, r, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}
	job, ok := a.downloads.Get(id)
	if !ok {
		writeAPIError(w, r, http.StatusNotFound, "job_not_found", "job not found")
		return
	}
	writeJSON(w, job)
}

func (a *app) interfacesHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeAPIError(w, r, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}
	infos := detectUsableInterfaces(a.cfg)
	usable := make([]ifmonitor.InterfaceInfo, 0, len(infos))
	for _, info := range infos {
		if info.Usable {
			usable = append(usable, info)
		}
	}
	writeJSON(w, map[string]any{
		"policy":                               config.NormalizeInterfacePolicy(a.cfg.InterfacePolicy),
		"supported_policies":                   []string{"auto", "manual", "all"},
		"refresh_seconds":                      a.cfg.InterfaceRefresh,
		"allow_new_interfaces_during_transfer": a.cfg.AllowNewInterfaces,
		"ignore_virtual_interfaces":            a.cfg.IgnoreVirtual,
		"ignore_vpn_interfaces":                a.cfg.IgnoreVPN,
		"ignore_link_local":                    a.cfg.IgnoreLinkLocal,
		"allowed_interface_types":              append([]string(nil), a.cfg.AllowedIfTypes...),
		"manual_interfaces":                    append([]string(nil), a.cfg.ManualInterfaces...),
		"ignored_interfaces":                   append([]string(nil), a.cfg.IgnoredInterfaces...),
		"usable_interfaces":                    usable,
		"interfaces":                           infos,
	})
}

func (a *app) localIPs() []string {
	ips, err := netif.DetectUsableIPv4()
	if err != nil {
		return nil
	}
	out := make([]string, 0, len(ips))
	for _, ip := range ips {
		out = append(out, ip.String())
	}
	return out
}

func (a *app) enrichSendOptions(opts transfer.SendOptions) transfer.SendOptions {
	refresh := a.cfg.InterfaceRefresh
	if refresh <= 0 {
		refresh = 5
	}
	opts.InterfaceRefreshSeconds = refresh
	opts.AllowNewInterfaces = a.cfg.AllowNewInterfaces
	opts.AuthSecret = a.cfg.AuthSecret()
	opts.InterfaceProvider = func() []transfer.InterfaceBinding {
		infos := detectUsableInterfaces(a.cfg)
		out := make([]transfer.InterfaceBinding, 0, len(infos))
		for _, it := range infos {
			if !it.Usable || strings.TrimSpace(it.IPv4) == "" {
				continue
			}
			out = append(out, transfer.InterfaceBinding{
				Name: it.Name,
				Type: it.Type,
				IP:   it.IPv4,
			})
		}
		return out
	}
	return opts
}

// remoteSendRoots returns the directories a remote peer may pull from. When the
// config leaves it empty we default to the receive path, the folder already
// designated for MultiSend sharing.
// controlAuthOK enforces a bearer token on the control API when RequireAuth is
// on. The token is the shared NodeSecret. When auth is disabled it always
// allows, preserving backward compatibility for unpaired setups.
func (a *app) controlAuthOK(r *http.Request) bool {
	secret := strings.TrimSpace(a.cfg.NodeSecret)
	if !a.cfg.RequireAuth {
		return true
	}
	if secret == "" {
		return false
	}
	parts := strings.Fields(r.Header.Get("Authorization"))
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return false
	}
	provided := parts[1]
	return subtle.ConstantTimeCompare([]byte(provided), []byte(secret)) == 1
}

func (a *app) controlAPIToken() string {
	if !a.cfg.RequireAuth {
		return ""
	}
	return strings.TrimSpace(a.cfg.NodeSecret)
}

func (a *app) remoteSendRoots() []string {
	if len(a.cfg.RemoteSendRoots) == 0 {
		return []string{a.cfg.ReceivePath}
	}
	return a.cfg.RemoteSendRoots
}

// isRemoteSendPathAllowed reports whether p resolves inside one of the resolved
// shared roots. Resolving every parent prevents symlinks and junctions from
// turning a lexically safe path into an arbitrary file read.
func (a *app) isRemoteSendPathAllowed(p string) bool {
	resolved, err := resolveExistingPath(p)
	if err != nil {
		return false
	}
	for _, root := range a.remoteSendRoots() {
		rootResolved, err := resolveExistingPath(root)
		if err != nil {
			continue
		}
		if isUnder(resolved, rootResolved) {
			return true
		}
	}
	return false
}

func resolveExistingPath(raw string) (string, error) {
	abs, err := filepath.Abs(strings.TrimSpace(raw))
	if err != nil {
		return "", err
	}
	resolved, err := filepath.EvalSymlinks(abs)
	if err != nil {
		return "", err
	}
	return filepath.Abs(resolved)
}

func (a *app) isRemoteAllowed(remoteAddr string) bool {
	host, _, err := net.SplitHostPort(strings.TrimSpace(remoteAddr))
	if err != nil {
		host = strings.TrimSpace(remoteAddr)
	}
	ip := net.ParseIP(host)
	if ip == nil {
		return false
	}
	for _, p := range a.disc.Peers() {
		if p.Addr == host {
			return true
		}
		for _, pip := range p.IPs {
			if pip == host {
				return true
			}
		}
	}
	return false
}

func (a *app) resolvePeerAddressByNodeID(nodeID string) string {
	if nodeID == "" {
		return ""
	}
	peers := a.disc.Peers()
	for i := range peers {
		if peers[i].NodeID != nodeID {
			continue
		}
		targetIP := peers[i].Addr
		if targetIP == "" && len(peers[i].IPs) > 0 {
			targetIP = peers[i].IPs[0]
		}
		return fmt.Sprintf("%s:%d", targetIP, peers[i].TransferPort)
	}
	return ""
}

func parseFileSource(raw string) (host string, sourcePath string, err error) {
	raw = strings.TrimSpace(raw)
	if strings.HasPrefix(raw, `\\`) {
		trimmed := strings.TrimLeft(raw, `\`)
		parts := strings.SplitN(trimmed, `\`, 2)
		if len(parts) != 2 || strings.TrimSpace(parts[0]) == "" || strings.TrimSpace(parts[1]) == "" {
			return "", "", fmt.Errorf("UNC path must be \\\\host\\share\\path")
		}
		return strings.TrimSpace(parts[0]), `\\` + strings.TrimSpace(parts[0]) + `\` + strings.TrimSpace(parts[1]), nil
	}
	// If the UI passes file://\\SRVTECHNEO\Arquivos, convert it to standard file URL
	if strings.HasPrefix(raw, "file://\\\\") {
		trimmed := strings.TrimPrefix(raw, "file://\\\\")
		raw = "file://" + strings.ReplaceAll(trimmed, "\\", "/")
	}

	u, err := urlpkg.Parse(raw)
	if err != nil {
		return "", "", fmt.Errorf("invalid source_url: %v", err)
	}
	if !strings.EqualFold(u.Scheme, "file") {
		return "", "", fmt.Errorf("source_url must use file://")
	}
	host = strings.TrimSpace(u.Host)
	if host == "" {
		return "", "", fmt.Errorf("file:// host is required")
	}
	p := strings.TrimSpace(u.EscapedPath())
	if p == "" {
		p = strings.TrimSpace(u.Path)
	}
	if p == "" {
		return "", "", fmt.Errorf("file path is required")
	}
	unescaped, uerr := urlpkg.PathUnescape(p)
	if uerr != nil {
		return "", "", fmt.Errorf("invalid file path encoding")
	}
	trimmed := strings.TrimLeft(strings.TrimSpace(unescaped), "/\\")
	if trimmed == "" {
		return "", "", fmt.Errorf("file path is required")
	}
	trimmed = strings.ReplaceAll(trimmed, "/", "\\")
	// file://host/share/path -> UNC path expected by Windows APIs.
	sourcePath = "\\\\" + host + "\\" + trimmed
	sourcePath = strings.TrimSpace(sourcePath)
	if sourcePath == "" {
		return "", "", fmt.Errorf("file path is required")
	}
	return host, sourcePath, nil
}

func prepareRemoteSendSource(sourcePath, folderResult string) (string, preparedRemoteSource, error) {
	folderResult = config.NormalizePullFolderResult(folderResult)
	info, err := os.Lstat(sourcePath)
	if err != nil {
		return "", preparedRemoteSource{}, fmt.Errorf("source path not accessible: %w", err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return "", preparedRemoteSource{}, fmt.Errorf("source symlink is not supported")
	}
	if !info.IsDir() {
		return sourcePath, preparedRemoteSource{
			SourceType:   "file",
			PublishName:  filepath.Base(sourcePath),
			FolderResult: "zip",
		}, nil
	}
	tmpDir, err := os.MkdirTemp("", "multisend-pull-*")
	if err != nil {
		return "", preparedRemoteSource{}, err
	}
	base := sanitizePublishName(filepath.Base(sourcePath))
	if base == "" {
		base = "folder"
	}
	zipPath := filepath.Join(tmpDir, base+".zip")
	if err := zipDirectory(sourcePath, zipPath); err != nil {
		_ = os.RemoveAll(tmpDir)
		return "", preparedRemoteSource{}, err
	}
	return zipPath, preparedRemoteSource{
		SourceType:   "folder",
		PublishName:  base + ".zip",
		FolderResult: folderResult,
		CleanupPath:  tmpDir,
	}, nil
}

func zipDirectory(sourceDir, zipPath string) error {
	sourceAbs, err := filepath.Abs(sourceDir)
	if err != nil {
		return err
	}
	out, err := os.Create(zipPath)
	if err != nil {
		return err
	}
	zw := zip.NewWriter(out)
	walkErr := filepath.WalkDir(sourceAbs, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if path == sourceAbs {
			return nil
		}
		rel, err := filepath.Rel(sourceAbs, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if d.IsDir() {
			_, err := zw.Create(rel + "/")
			return err
		}
		h, err := zip.FileInfoHeader(info)
		if err != nil {
			return err
		}
		h.Name = rel
		h.Method = zip.Deflate
		w, err := zw.CreateHeader(h)
		if err != nil {
			return err
		}
		in, err := os.Open(path)
		if err != nil {
			return err
		}
		_, copyErr := io.Copy(w, in)
		closeErr := in.Close()
		if copyErr != nil {
			return copyErr
		}
		return closeErr
	})
	zipCloseErr := zw.Close()
	fileCloseErr := out.Close()
	return errors.Join(walkErr, zipCloseErr, fileCloseErr)
}

func (a *app) findPeerByHost(host string) *discovery.Peer {
	target := strings.ToLower(strings.TrimSpace(host))
	if target == "" {
		return nil
	}
	peers := a.disc.Peers()
	for _, p := range peers {
		if strings.EqualFold(p.Addr, target) || strings.EqualFold(p.Name, target) || strings.EqualFold(p.NodeID, target) {
			cp := p
			return &cp
		}
		for _, ip := range p.IPs {
			if strings.EqualFold(ip, target) {
				cp := p
				return &cp
			}
		}
	}

	ips, err := net.LookupIP(target)
	if err == nil {
		for _, resolvedIP := range ips {
			ipStr := resolvedIP.String()
			for _, p := range peers {
				for _, pIP := range p.IPs {
					if strings.EqualFold(pIP, ipStr) {
						cp := p
						return &cp
					}
				}
			}
		}
	}

	return nil
}

func validateOutputUnderReceive(outputDir, receiveRoot string) (string, error) {
	outAbs, err := filepath.Abs(outputDir)
	if err != nil {
		return "", err
	}
	rootAbs, err := filepath.Abs(receiveRoot)
	if err != nil {
		return "", err
	}
	if !isUnder(outAbs, rootAbs) {
		return "", fmt.Errorf("output_dir must stay inside %s", rootAbs)
	}
	rel, err := filepath.Rel(rootAbs, outAbs)
	if err != nil {
		return "", err
	}
	rel = filepath.Clean(rel)
	if rel == "." {
		return ".", nil
	}
	return validateReceiveRelPath(rel)
}

func (a *app) remoteControlBase(peer *discovery.Peer) string {
	if peer == nil || peer.ControlPort <= 0 {
		return ""
	}
	ip := peer.Addr
	if ip == "" && len(peer.IPs) > 0 {
		ip = peer.IPs[0]
	}
	return fmt.Sprintf("http://%s:%d", ip, peer.ControlPort)
}

const maxRemoteAPIResponseBodyBytes int64 = 1 << 20

func postJSON(url string, body any, out any, bearerToken string) error {
	b, err := json.Marshal(body)
	if err != nil {
		return err
	}
	client := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequest(http.MethodPost, url, strings.NewReader(string(b)))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	setBearerToken(req, bearerToken)
	res, err := client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	return decodeRemoteAPIResponse(res, out)
}

func getJSON(url string, out any, bearerToken string) error {
	client := &http.Client{Timeout: 8 * time.Second}
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	setBearerToken(req, bearerToken)
	res, err := client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	return decodeRemoteAPIResponse(res, out)
}

func decodeRemoteAPIResponse(res *http.Response, out any) error {
	body, err := io.ReadAll(io.LimitReader(res.Body, maxRemoteAPIResponseBodyBytes+1))
	if err != nil {
		return err
	}
	if int64(len(body)) > maxRemoteAPIResponseBodyBytes {
		return fmt.Errorf("remote response body exceeds %d bytes", maxRemoteAPIResponseBodyBytes)
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return errors.New(formatRemoteAPIError(res.StatusCode, body))
	}
	if out == nil {
		return nil
	}
	return json.Unmarshal(body, out)
}

func setBearerToken(req *http.Request, token string) {
	if token = strings.TrimSpace(token); token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
}

func formatRemoteAPIError(status int, body []byte) string {
	raw := strings.TrimSpace(string(body))
	if raw == "" {
		return fmt.Sprintf("status=%d", status)
	}
	var apiErr apiErrorResponse
	if err := json.Unmarshal(body, &apiErr); err == nil && (apiErr.Message != "" || apiErr.Error != "") {
		parts := []string{fmt.Sprintf("status=%d", status)}
		if apiErr.Error != "" {
			parts = append(parts, "code="+apiErr.Error)
		}
		if apiErr.Message != "" {
			parts = append(parts, "message="+apiErr.Message)
		}
		if apiErr.Detail != "" {
			parts = append(parts, "detail="+apiErr.Detail)
		}
		if apiErr.RequestID != "" {
			parts = append(parts, "request_id="+apiErr.RequestID)
		}
		return strings.Join(parts, " ")
	}
	return fmt.Sprintf("status=%d body=%s", status, raw)
}

func (a *app) createPull(sourceURL, outputDir string, chunkSizeMB int64) (map[string]any, error) {
	if err := validateChunkSizeMB(chunkSizeMB); err != nil {
		return nil, err
	}
	if strings.TrimSpace(outputDir) == "" {
		outputDir = a.cfg.ReceivePath
	}
	folderResult := config.NormalizePullFolderResult(a.cfg.PullFolderResult)
	host, sourcePath, err := parseFileSource(sourceURL)
	if err != nil {
		log.Printf("pull_parse_failed source_url=%q err=%v", sourceURL, err)
		return nil, err
	}
	peer := a.findPeerByHost(host)
	if peer == nil {
		log.Printf("pull_peer_not_found source_url=%q host=%s", sourceURL, host)
		return nil, fmt.Errorf("peer not found for host: %s", host)
	}
	if peer.ControlPort <= 0 {
		log.Printf("pull_peer_no_control host=%s node_id=%s addr=%s", host, peer.NodeID, peer.Addr)
		return nil, fmt.Errorf("peer sem suporte a Receber com MultiSend (atualize o agente)")
	}
	relPath, err := validateOutputUnderReceive(outputDir, a.cfg.ReceivePath)
	if err != nil {
		log.Printf("pull_output_invalid source_url=%q output_dir=%q receive_path=%q err=%v", sourceURL, outputDir, a.cfg.ReceivePath, err)
		return nil, err
	}
	controlBase := a.remoteControlBase(peer)
	if controlBase == "" {
		log.Printf("pull_control_unavailable host=%s node_id=%s addr=%s", host, peer.NodeID, peer.Addr)
		return nil, fmt.Errorf("peer control endpoint unavailable")
	}
	req := map[string]any{
		"file_path":              sourcePath,
		"target_node_id":         a.cfg.NodeID,
		"target_receive_relpath": relPath,
		"folder_result":          folderResult,
		"chunk_size_mb":          chunkSizeMB,
	}
	log.Printf("pull_remote_send_request host=%s node_id=%s control=%s source=%q output_dir=%q receive_rel=%q folder_result=%s chunk_size_mb=%d",
		host, peer.NodeID, controlBase, sourcePath, outputDir, relPath, folderResult, chunkSizeMB)
	var remoteResp map[string]any
	if err := postJSON(controlBase+"/remote-send", req, &remoteResp, a.controlAPIToken()); err != nil {
		log.Printf("pull_remote_send_failed host=%s node_id=%s control=%s source=%q err=%v", host, peer.NodeID, controlBase, sourcePath, err)
		return nil, err
	}
	remoteJobID := fmt.Sprintf("%v", remoteResp["job_id"])
	if strings.TrimSpace(remoteJobID) == "" {
		log.Printf("pull_remote_send_missing_job host=%s node_id=%s response=%v", host, peer.NodeID, remoteResp)
		return nil, fmt.Errorf("remote send did not return job_id")
	}
	sourceType := fmt.Sprintf("%v", remoteResp["source_type"])
	publishName := sanitizePublishName(fmt.Sprintf("%v", remoteResp["publish_name"]))
	if publishName == "" {
		publishName = filepath.Base(sourcePath)
	}
	expectedOutput := filepath.Join(outputDir, publishName)
	if sourceType == "folder" && folderResult == "extract" {
		expectedOutput = strings.TrimSuffix(expectedOutput, ".zip")
	}
	pullID := fmt.Sprintf("pull-%d", time.Now().UnixNano())
	now := time.Now()
	state := &pullJobState{
		ID:             pullID,
		SourceURL:      sourceURL,
		OutputDir:      outputDir,
		OutputPath:     expectedOutput,
		SourceType:     sourceType,
		FolderResult:   folderResult,
		RemoteNodeID:   peer.NodeID,
		RemotePeerAddr: peer.Addr,
		RemoteJobID:    remoteJobID,
		Status:         "running",
		StartedAt:      now,
		UpdatedAt:      now,
	}
	a.pullsMu.Lock()
	a.pullsByID[pullID] = state
	a.persistPullLocked(state)
	a.prunePullsLocked()
	a.pullsMu.Unlock()
	log.Printf("pull_start pull_id=%s remote_job_id=%s host=%s node_id=%s source_type=%s output=%q folder_result=%s",
		pullID, remoteJobID, host, peer.NodeID, sourceType, expectedOutput, folderResult)
	return a.pullViewFromRemote(state)
}

func validateChunkSizeMB(value int64) error {
	if value < 0 || value > 1024 {
		return fmt.Errorf("chunk_size_mb must be between 0 and 1024")
	}
	return nil
}

func (a *app) pullViewFromRemote(st *pullJobState) (map[string]any, error) {
	peer := a.findPeerByHost(st.RemotePeerAddr)
	if peer == nil {
		return nil, fmt.Errorf("remote peer unavailable")
	}
	base := a.remoteControlBase(peer)
	if base == "" {
		return nil, fmt.Errorf("remote control unavailable")
	}
	var remote jobDetails
	if err := getJSON(base+"/remote-jobs/"+st.RemoteJobID, &remote, a.controlAPIToken()); err != nil {
		log.Printf("pull_refresh_failed pull_id=%s remote_job_id=%s base=%s err=%v", st.ID, st.RemoteJobID, base, err)
		return nil, err
	}
	now := time.Now()
	a.pullsMu.Lock()
	st.Status = remote.Status
	st.Message = remote.Message
	st.UpdatedAt = now
	outputPath := st.OutputPath
	if strings.TrimSpace(outputPath) == "" {
		outputPath = filepath.Join(st.OutputDir, filepath.Base(remote.FilePath))
	}
	a.persistPullLocked(st)
	a.prunePullsLocked()
	a.pullsMu.Unlock()
	if remote.Status == "failed" || remote.Status == "canceled" || remote.Status == "done" {
		log.Printf("pull_remote_status pull_id=%s remote_job_id=%s status=%s message=%q manifest=%q output=%q",
			st.ID, st.RemoteJobID, remote.Status, remote.Message, remote.ManifestPath, outputPath)
	}
	return map[string]any{
		"id":               st.ID,
		"status":           remote.Status,
		"message":          remote.Message,
		"percent":          remote.Percent,
		"bytes_done":       remote.BytesSent,
		"total_bytes":      remote.TotalBytes,
		"output_path":      outputPath,
		"source_type":      st.SourceType,
		"folder_result":    st.FolderResult,
		"manifest_path":    remote.ManifestPath,
		"resume_supported": true,
		"chunks_total":     remote.ChunksTotal,
		"chunks_done":      remote.ChunksDone,
		"chunks_failed":    remote.ChunksFailed,
		"chunks_pending":   remote.ChunksPending,
		"chunks_sending":   remote.ChunksSending,
		"mbps_avg":         remote.MbpsAvg,
		"channels": map[string]any{
			"cable": map[string]any{
				"state":      remote.Channels.Cable.State,
				"bytes_sent": remote.Channels.Cable.BytesSent,
				"mbps_now":   remote.Channels.Cable.MbpsNow,
				"mbps_avg":   remote.Channels.Cable.MbpsAvg,
				"local_ip":   remote.Channels.Cable.LocalIP,
			},
			"wifi": map[string]any{
				"state":      remote.Channels.Wifi.State,
				"bytes_sent": remote.Channels.Wifi.BytesSent,
				"mbps_now":   remote.Channels.Wifi.MbpsNow,
				"mbps_avg":   remote.Channels.Wifi.MbpsAvg,
				"local_ip":   remote.Channels.Wifi.LocalIP,
			},
		},
	}, nil
}

func (a *app) listPulls() []map[string]any {
	a.pullsMu.RLock()
	states := make([]*pullJobState, 0, len(a.pullsByID))
	for _, s := range a.pullsByID {
		states = append(states, s)
	}
	a.pullsMu.RUnlock()
	sort.Slice(states, func(i, j int) bool { return states[i].StartedAt.After(states[j].StartedAt) })
	out := make([]map[string]any, 0, len(states))
	for _, s := range states {
		v, err := a.pullViewFromRemote(s)
		if err != nil {
			out = append(out, map[string]any{"id": s.ID, "status": "failed", "message": err.Error()})
			continue
		}
		out = append(out, v)
	}
	return out
}

func (a *app) refreshPull(id string) (map[string]any, error) {
	a.pullsMu.RLock()
	st := a.pullsByID[id]
	a.pullsMu.RUnlock()
	if st == nil {
		return nil, fmt.Errorf("pull job not found")
	}
	return a.pullViewFromRemote(st)
}

func (a *app) pullRemoteAction(id, action string) (map[string]any, error) {
	a.pullsMu.RLock()
	st := a.pullsByID[id]
	a.pullsMu.RUnlock()
	if st == nil {
		return nil, fmt.Errorf("pull job not found")
	}
	peer := a.findPeerByHost(st.RemotePeerAddr)
	if peer == nil {
		return nil, fmt.Errorf("remote peer unavailable")
	}
	base := a.remoteControlBase(peer)
	if base == "" {
		return nil, fmt.Errorf("remote control unavailable")
	}
	if err := postJSON(base+"/remote-jobs/"+st.RemoteJobID+"/"+action, map[string]any{}, nil, a.controlAPIToken()); err != nil {
		log.Printf("pull_remote_action_failed pull_id=%s remote_job_id=%s action=%s err=%v", st.ID, st.RemoteJobID, action, err)
		return nil, err
	}
	log.Printf("pull_remote_action pull_id=%s remote_job_id=%s action=%s", st.ID, st.RemoteJobID, action)
	return a.pullViewFromRemote(st)
}

func validateLabReceivePath(receivePath, requested string) (string, error) {
	if strings.TrimSpace(requested) == "" {
		return "", fmt.Errorf("lab_receive_path is required when lab=true")
	}
	baseAbs, err := filepath.Abs(receivePath)
	if err != nil {
		return "", err
	}
	allowedRoot := filepath.Join(baseAbs, "_lab", "receive")
	allowedAbs, err := filepath.Abs(allowedRoot)
	if err != nil {
		return "", err
	}
	reqInput := requested
	if !filepath.IsAbs(reqInput) {
		reqInput = filepath.Join(baseAbs, reqInput)
	}
	reqAbs, err := filepath.Abs(reqInput)
	if err != nil {
		return "", err
	}
	baseLower := strings.ToLower(allowedAbs)
	reqLower := strings.ToLower(reqAbs)
	if reqLower != baseLower && !strings.HasPrefix(reqLower, baseLower+string(os.PathSeparator)) {
		return "", fmt.Errorf("path must stay inside %s", allowedAbs)
	}
	return reqAbs, nil
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(v)
}

func writeAPIError(w http.ResponseWriter, r *http.Request, status int, code, message string) {
	writeAPIErrorDetail(w, r, status, code, message, "")
}

func writeAPIErrorDetail(w http.ResponseWriter, r *http.Request, status int, code, message, detail string) {
	if strings.TrimSpace(code) == "" {
		code = http.StatusText(status)
	}
	if strings.TrimSpace(message) == "" {
		message = http.StatusText(status)
	}
	reqID := fmt.Sprintf("req-%d", time.Now().UnixNano())
	resp := apiErrorResponse{
		Error:     code,
		Message:   message,
		Detail:    detail,
		Status:    status,
		RequestID: reqID,
	}
	if r != nil {
		resp.Method = r.Method
		resp.Path = r.URL.Path
	}
	log.Printf("api_error request_id=%s status=%d code=%s method=%s path=%s remote=%s message=%q detail=%q",
		resp.RequestID, resp.Status, resp.Error, resp.Method, resp.Path, remoteAddrForLog(r), resp.Message, resp.Detail)
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(resp)
}

func remoteAddrForLog(r *http.Request) string {
	if r == nil {
		return ""
	}
	return r.RemoteAddr
}
