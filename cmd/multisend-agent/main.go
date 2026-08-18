package main

// ====================== BEGIN NAV INDEX ======================
// NAV INDEX — auto-generated symbol map (refresh via the navindex skill)
//   L57    type channelProgress
//   L71    type channelsProgress
//   L76    type jobDetails
//   L103   type receiverSession
//   L119   type failedChunkRef
//   L126   type progressSample
//   L133   type jobState
//   L153   type receiverSessionState
//   L160   type app
//   L175   type pullJobState
//   L193   type sendTargetOptions
//   L202   type apiErrorResponse
//   L212   type preparedRemoteSource
//   L223   agentLogPath
//   L234   setupAgentLogging
//   L256   rotateLogIfLarge
//   L269   main
//   L334   app.selectAndPersistPorts
//   L364   app.writeRuntimeState
//   L385   app.runLocalAPI
//   L398   app.runControlAPI
//   L411   writeJSON
//   L416   writeAPIError
//   L420   writeAPIErrorDetail
//   L446   remoteAddrForLog
// ======================= END NAV INDEX =======================

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/Codyte/MultiSend/internal/config"
	"github.com/Codyte/MultiSend/internal/discovery"
	"github.com/Codyte/MultiSend/internal/download"
	"github.com/Codyte/MultiSend/internal/ifmonitor"
	"github.com/Codyte/MultiSend/internal/ports"
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
		log.Printf("runtime state skipped: LOCALAPPDATA is empty")
		return
	}
	runtimePath := filepath.Join(local, "MultiSend", "runtime.json")
	payload := map[string]any{
		"pid":            os.Getpid(),
		"local_api_port": a.cfg.SelectedPorts.LocalAPI,
		"transfer_port":  a.cfg.SelectedPorts.Transfer,
		"discovery_port": a.cfg.SelectedPorts.Discovery,
		"control_port":   a.cfg.SelectedPorts.Control,
		"started_at":     time.Now().Format(time.RFC3339),
		"agent_log_path": agentLogPath(),
	}
	if err := saveJSONAtomic(runtimePath, payload); err != nil {
		log.Printf("runtime state write failed path=%q err=%v", runtimePath, err)
	}
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
