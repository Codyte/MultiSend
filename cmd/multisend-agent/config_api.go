package main

// ====================== BEGIN NAV INDEX ======================
// NAV INDEX — auto-generated symbol map (refresh via the navindex skill)
//   L33    type editableConfig
//   L57    type configRuntimeView
//   L67    type configAPIResponse
//   L75    app.configHandler
//   L117   app.loadPersistedConfig
//   L133   configResponse
//   L151   editableConfigFrom
//   L177   applyEditableConfig
//   L268   normalizeSafeDirectory
//   L299   normalizeStringList
//   L324   oneOf
// ======================= END NAV INDEX =======================

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode"

	"github.com/Codyte/MultiSend/internal/config"
	"github.com/Codyte/MultiSend/internal/ports"
)

type editableConfig struct {
	DisplayName              string   `json:"display_name"`
	ReceivePath              string   `json:"receive_path"`
	InterfacePolicy          string   `json:"interface_policy"`
	InterfaceRefresh         int      `json:"interface_refresh_seconds"`
	AllowNewInterfaces       bool     `json:"allow_new_interfaces_during_transfer"`
	IgnoreVirtual            bool     `json:"ignore_virtual_interfaces"`
	IgnoreVPN                bool     `json:"ignore_vpn_interfaces"`
	IgnoreLinkLocal          bool     `json:"ignore_link_local"`
	AllowedIfTypes           []string `json:"allowed_interface_types"`
	ManualInterfaces         []string `json:"manual_interfaces"`
	IgnoredInterfaces        []string `json:"ignored_interfaces"`
	CleanupCompletedChunks   bool     `json:"cleanup_completed_chunks"`
	KeepManifests            bool     `json:"keep_manifests"`
	CleanupEmptyDownloadDirs bool     `json:"cleanup_empty_download_dirs"`
	PullFolderResult         string   `json:"pull_folder_result"`
	DownloadPipelineMode     string   `json:"download_pipeline_mode"`
	DownloadInFlightPerChan  int      `json:"download_in_flight_per_channel"`
	DownloadChannelStrategy  string   `json:"download_channel_strategy"`
	DownloadSplitCablePct    int      `json:"download_split_cable_pct"`
	RequireAuth              bool     `json:"require_auth"`
	RemoteSendRoots          []string `json:"remote_send_roots"`
}

type configRuntimeView struct {
	NodeID             string               `json:"node_id"`
	AppVersion         string               `json:"app_version"`
	SelectedPorts      config.SelectedPorts `json:"selected_ports"`
	TransferPortRange  ports.Range          `json:"transfer_port_range"`
	DiscoveryPortRange ports.Range          `json:"discovery_port_range"`
	LocalAPIPortRange  ports.Range          `json:"local_api_port_range"`
	ControlPortRange   ports.Range          `json:"control_port_range"`
}

type configAPIResponse struct {
	Config           editableConfig    `json:"config"`
	Runtime          configRuntimeView `json:"runtime"`
	SecretConfigured bool              `json:"secret_configured"`
	SecretExposed    bool              `json:"secret_exposed"`
	RestartRequired  bool              `json:"restart_required"`
}

func (a *app) configHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		cfg, err := a.loadPersistedConfig()
		if err != nil {
			writeAPIErrorDetail(w, r, http.StatusInternalServerError, "config_read_failed", "could not read configuration", err.Error())
			return
		}
		writeJSON(w, configResponse(cfg, false))
	case http.MethodPut:
		var req editableConfig
		if !decodeJSONRequest(w, r, &req) {
			return
		}
		a.configMu.Lock()
		defer a.configMu.Unlock()
		current, err := a.loadPersistedConfig()
		if err != nil {
			writeAPIErrorDetail(w, r, http.StatusInternalServerError, "config_read_failed", "could not read configuration", err.Error())
			return
		}
		updated, err := applyEditableConfig(current, req)
		if err != nil {
			writeAPIError(w, r, http.StatusBadRequest, "invalid_config", err.Error())
			return
		}
		if err := os.MkdirAll(updated.ReceivePath, 0o755); err != nil {
			writeAPIErrorDetail(w, r, http.StatusBadRequest, "receive_path_unavailable", "could not create receive path", err.Error())
			return
		}
		if err := config.Save(a.configPath, updated); err != nil {
			writeAPIErrorDetail(w, r, http.StatusInternalServerError, "config_save_failed", "could not save configuration", err.Error())
			return
		}
		log.Printf("config_updated path=%q display_name=%q receive_path=%q require_auth=%t remote_roots=%d restart_required=true",
			a.configPath, updated.DisplayName, updated.ReceivePath, updated.RequireAuth, len(updated.RemoteSendRoots))
		writeJSON(w, configResponse(updated, true))
	default:
		writeAPIError(w, r, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
	}
}

func (a *app) loadPersistedConfig() (config.Config, error) {
	path := strings.TrimSpace(a.configPath)
	if path == "" {
		return config.Config{}, fmt.Errorf("configuration path is unavailable")
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return config.Config{}, err
	}
	var cfg config.Config
	if err := json.Unmarshal(b, &cfg); err != nil {
		return config.Config{}, err
	}
	return cfg, nil
}

func configResponse(cfg config.Config, restartRequired bool) configAPIResponse {
	return configAPIResponse{
		Config:           editableConfigFrom(cfg),
		SecretConfigured: strings.TrimSpace(cfg.NodeSecret) != "",
		SecretExposed:    false,
		RestartRequired:  restartRequired,
		Runtime: configRuntimeView{
			NodeID:             cfg.NodeID,
			AppVersion:         cfg.AppVersion,
			SelectedPorts:      cfg.SelectedPorts,
			TransferPortRange:  cfg.TransferPortRange,
			DiscoveryPortRange: cfg.DiscoveryPortRange,
			LocalAPIPortRange:  cfg.LocalAPIPortRange,
			ControlPortRange:   cfg.ControlPortRange,
		},
	}
}

func editableConfigFrom(cfg config.Config) editableConfig {
	return editableConfig{
		DisplayName:              cfg.DisplayName,
		ReceivePath:              cfg.ReceivePath,
		InterfacePolicy:          config.NormalizeInterfacePolicy(cfg.InterfacePolicy),
		InterfaceRefresh:         cfg.InterfaceRefresh,
		AllowNewInterfaces:       cfg.AllowNewInterfaces,
		IgnoreVirtual:            cfg.IgnoreVirtual,
		IgnoreVPN:                cfg.IgnoreVPN,
		IgnoreLinkLocal:          cfg.IgnoreLinkLocal,
		AllowedIfTypes:           append([]string(nil), cfg.AllowedIfTypes...),
		ManualInterfaces:         append([]string(nil), cfg.ManualInterfaces...),
		IgnoredInterfaces:        append([]string(nil), cfg.IgnoredInterfaces...),
		CleanupCompletedChunks:   cfg.CleanupCompletedChunks,
		KeepManifests:            cfg.KeepManifests,
		CleanupEmptyDownloadDirs: cfg.CleanupEmptyDownloadDirs,
		PullFolderResult:         config.NormalizePullFolderResult(cfg.PullFolderResult),
		DownloadPipelineMode:     config.NormalizeDownloadPipelineMode(cfg.DownloadPipelineMode),
		DownloadInFlightPerChan:  cfg.DownloadInFlightPerChan,
		DownloadChannelStrategy:  config.NormalizeDownloadChannelStrategy(cfg.DownloadChannelStrategy),
		DownloadSplitCablePct:    cfg.DownloadSplitCablePct,
		RequireAuth:              cfg.RequireAuth,
		RemoteSendRoots:          append([]string(nil), cfg.RemoteSendRoots...),
	}
}

func applyEditableConfig(cfg config.Config, req editableConfig) (config.Config, error) {
	displayName := strings.TrimSpace(req.DisplayName)
	if displayName == "" || len(displayName) > 128 || strings.IndexFunc(displayName, unicode.IsControl) >= 0 {
		return config.Config{}, fmt.Errorf("display_name must contain 1 to 128 printable characters")
	}
	receivePath, err := normalizeSafeDirectory(req.ReceivePath, false)
	if err != nil {
		return config.Config{}, fmt.Errorf("receive_path: %w", err)
	}
	if !oneOf(req.InterfacePolicy, "auto", "manual", "all") {
		return config.Config{}, fmt.Errorf("interface_policy must be auto, manual, or all")
	}
	if req.InterfaceRefresh < 2 || req.InterfaceRefresh > 60 {
		return config.Config{}, fmt.Errorf("interface_refresh_seconds must be between 2 and 60")
	}
	allowed, err := normalizeStringList(req.AllowedIfTypes, 32)
	if err != nil || len(allowed) == 0 {
		return config.Config{}, fmt.Errorf("allowed_interface_types must contain at least one valid entry")
	}
	manual, err := normalizeStringList(req.ManualInterfaces, 128)
	if err != nil {
		return config.Config{}, fmt.Errorf("manual_interfaces: %w", err)
	}
	ignored, err := normalizeStringList(req.IgnoredInterfaces, 128)
	if err != nil {
		return config.Config{}, fmt.Errorf("ignored_interfaces: %w", err)
	}
	if !oneOf(req.PullFolderResult, "zip", "extract") {
		return config.Config{}, fmt.Errorf("pull_folder_result must be zip or extract")
	}
	if !oneOf(req.DownloadPipelineMode, "legacy", "pipelined") {
		return config.Config{}, fmt.Errorf("download_pipeline_mode must be legacy or pipelined")
	}
	if req.DownloadInFlightPerChan < 1 || req.DownloadInFlightPerChan > 16 {
		return config.Config{}, fmt.Errorf("download_in_flight_per_channel must be between 1 and 16")
	}
	if !oneOf(req.DownloadChannelStrategy, "dynamic", "strict_split") {
		return config.Config{}, fmt.Errorf("download_channel_strategy must be dynamic or strict_split")
	}
	if req.DownloadSplitCablePct < 1 || req.DownloadSplitCablePct > 99 {
		return config.Config{}, fmt.Errorf("download_split_cable_pct must be between 1 and 99")
	}
	remoteRoots := make([]string, 0, len(req.RemoteSendRoots))
	seenRoots := map[string]struct{}{}
	for _, raw := range req.RemoteSendRoots {
		if strings.TrimSpace(raw) == "" {
			continue
		}
		root, err := normalizeSafeDirectory(raw, true)
		if err != nil {
			return config.Config{}, fmt.Errorf("remote_send_roots: %w", err)
		}
		key := strings.ToLower(root)
		if _, exists := seenRoots[key]; exists {
			continue
		}
		seenRoots[key] = struct{}{}
		remoteRoots = append(remoteRoots, root)
		if len(remoteRoots) > 128 {
			return config.Config{}, fmt.Errorf("remote_send_roots allows at most 128 directories")
		}
	}
	sort.Strings(remoteRoots)

	cfg.DisplayName = displayName
	cfg.ReceivePath = receivePath
	cfg.InterfacePolicy = strings.ToLower(strings.TrimSpace(req.InterfacePolicy))
	cfg.InterfaceRefresh = req.InterfaceRefresh
	cfg.AllowNewInterfaces = req.AllowNewInterfaces
	cfg.IgnoreVirtual = req.IgnoreVirtual
	cfg.IgnoreVPN = req.IgnoreVPN
	cfg.IgnoreLinkLocal = req.IgnoreLinkLocal
	cfg.AllowedIfTypes = allowed
	cfg.ManualInterfaces = manual
	cfg.IgnoredInterfaces = ignored
	cfg.CleanupCompletedChunks = req.CleanupCompletedChunks
	cfg.KeepManifests = req.KeepManifests
	cfg.CleanupEmptyDownloadDirs = req.CleanupEmptyDownloadDirs
	cfg.PullFolderResult = strings.ToLower(strings.TrimSpace(req.PullFolderResult))
	cfg.DownloadPipelineMode = strings.ToLower(strings.TrimSpace(req.DownloadPipelineMode))
	cfg.DownloadInFlightPerChan = req.DownloadInFlightPerChan
	cfg.DownloadChannelStrategy = strings.ToLower(strings.TrimSpace(req.DownloadChannelStrategy))
	cfg.DownloadSplitCablePct = req.DownloadSplitCablePct
	cfg.RequireAuth = req.RequireAuth
	cfg.RemoteSendRoots = remoteRoots
	if cfg.RequireAuth && strings.TrimSpace(cfg.NodeSecret) == "" {
		cfg.NodeSecret = config.GenerateSecret()
	}
	return cfg, nil
}

func normalizeSafeDirectory(raw string, mustExist bool) (string, error) {
	value := strings.TrimSpace(os.ExpandEnv(raw))
	if value == "" {
		return "", fmt.Errorf("path is required")
	}
	if len(value) > 4096 || strings.IndexFunc(value, unicode.IsControl) >= 0 {
		return "", fmt.Errorf("path must contain at most 4096 printable characters")
	}
	if !filepath.IsAbs(value) {
		return "", fmt.Errorf("path must be absolute")
	}
	abs, err := filepath.Abs(value)
	if err != nil {
		return "", fmt.Errorf("invalid path: %w", err)
	}
	abs = filepath.Clean(abs)
	if filepath.Dir(abs) == abs {
		return "", fmt.Errorf("filesystem root is not allowed")
	}
	if mustExist {
		info, err := os.Stat(abs)
		if err != nil {
			return "", fmt.Errorf("%s is not accessible", abs)
		}
		if !info.IsDir() {
			return "", fmt.Errorf("%s is not a directory", abs)
		}
	}
	return abs, nil
}

func normalizeStringList(values []string, maxItems int) ([]string, error) {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(values))
	for _, raw := range values {
		value := strings.TrimSpace(raw)
		if value == "" {
			continue
		}
		if len(value) > 256 || strings.IndexFunc(value, unicode.IsControl) >= 0 {
			return nil, fmt.Errorf("entries must contain at most 256 printable characters")
		}
		key := strings.ToLower(value)
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, value)
		if len(out) > maxItems {
			return nil, fmt.Errorf("at most %d entries are allowed", maxItems)
		}
	}
	sort.Strings(out)
	return out, nil
}

func oneOf(value string, allowed ...string) bool {
	value = strings.ToLower(strings.TrimSpace(value))
	for _, candidate := range allowed {
		if value == candidate {
			return true
		}
	}
	return false
}
