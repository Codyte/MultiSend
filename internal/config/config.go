package config

// ====================== BEGIN NAV INDEX ======================
// NAV INDEX — auto-generated symbol map (refresh via the navindex skill)
//   L37    GenerateSecret
//   L47    Config.SecretBytes
//   L60    Config.AuthSecret
//   L67    type SelectedPorts
//   L74    type Config
//   L126   NormalizeInterfacePolicy
//   L135   DefaultConfig
//   L189   NormalizeDownloadPipelineMode
//   L198   NormalizeDownloadChannelStrategy
//   L207   NormalizePullFolderResult
//   L216   ConfigPath
//   L224   LoadOrCreate
//   L257   LoadIfExists
//   L280   Save
//   L313   migrateLegacyConfig
//   L465   backupConfig
// ======================= END NAV INDEX =======================

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Codyte/MultiSend/internal/ports"
)

// GenerateSecret returns a fresh 256-bit hex secret for node authentication.
func GenerateSecret() string {
	buf := make([]byte, 32)
	// Supported Go versions fail closed if the operating-system CSPRNG is
	// unavailable; never substitute predictable entropy for an auth secret.
	_, _ = rand.Read(buf)
	return hex.EncodeToString(buf)
}

// SecretBytes returns the raw secret bytes used to key the HMAC, or nil when no
// secret is configured.
func (c Config) SecretBytes() []byte {
	s := strings.TrimSpace(c.NodeSecret)
	if s == "" {
		return nil
	}
	if b, err := hex.DecodeString(s); err == nil && len(b) > 0 {
		return b
	}
	return []byte(s)
}

// AuthSecret returns the secret bytes only when authentication is enforced;
// otherwise nil (authentication disabled).
func (c Config) AuthSecret() []byte {
	if !c.RequireAuth {
		return nil
	}
	return c.SecretBytes()
}

type SelectedPorts struct {
	Transfer  int `json:"transfer"`
	Discovery int `json:"discovery"`
	LocalAPI  int `json:"local_api"`
	Control   int `json:"control"`
}

type Config struct {
	SchemaVersion            int           `json:"schema_version"`
	AppVersion               string        `json:"app_version"`
	NodeID                   string        `json:"node_id"`
	DisplayName              string        `json:"display_name"`
	ReceivePath              string        `json:"receive_path"`
	TransferPort             int           `json:"transfer_port,omitempty"`
	DiscoveryPort            int           `json:"discovery_port,omitempty"`
	DiscoveryMode            string        `json:"discovery"`
	InterfacesMode           string        `json:"interfaces"`
	Scheduler                string        `json:"scheduler"`
	ChunkMode                string        `json:"chunk_mode"`
	ChunkSizeMB              int           `json:"chunk_size_mb"`
	StartOnLogin             bool          `json:"start_agent_on_login"`
	LocalAPIPort             int           `json:"local_api_port,omitempty"`
	ControlPort              int           `json:"control_port,omitempty"`
	TransferPortRange        ports.Range   `json:"transfer_port_range"`
	DiscoveryPortRange       ports.Range   `json:"discovery_port_range"`
	LocalAPIPortRange        ports.Range   `json:"local_api_port_range"`
	ControlPortRange         ports.Range   `json:"control_port_range"`
	SelectedPorts            SelectedPorts `json:"selected_ports"`
	InterfacePolicy          string        `json:"interface_policy"`
	InterfaceRefresh         int           `json:"interface_refresh_seconds"`
	AllowNewInterfaces       bool          `json:"allow_new_interfaces_during_transfer"`
	IgnoreVirtual            bool          `json:"ignore_virtual_interfaces"`
	IgnoreVPN                bool          `json:"ignore_vpn_interfaces"`
	IgnoreLinkLocal          bool          `json:"ignore_link_local"`
	AllowedIfTypes           []string      `json:"allowed_interface_types"`
	ManualInterfaces         []string      `json:"manual_interfaces"`
	IgnoredInterfaces        []string      `json:"ignored_interfaces"`
	CleanupCompletedChunks   bool          `json:"cleanup_completed_chunks"`
	KeepManifests            bool          `json:"keep_manifests"`
	CleanupEmptyDownloadDirs bool          `json:"cleanup_empty_download_dirs"`
	DownloadPipelineMode     string        `json:"download_pipeline_mode"`
	DownloadInFlightPerChan  int           `json:"download_in_flight_per_channel"`
	DownloadChannelStrategy  string        `json:"download_channel_strategy"`
	DownloadSplitCablePct    int           `json:"download_split_cable_pct"`
	PullFolderResult         string        `json:"pull_folder_result"`
	// NodeSecret is a hex-encoded shared secret used to authenticate transfers
	// (HMAC on the wire protocol) and the control API. It is auto-generated on
	// first run. Paired nodes must share the same value for RequireAuth to work.
	NodeSecret string `json:"node_secret,omitempty"`
	// RequireAuth enforces HMAC verification on the receiver and a bearer token
	// on the control API. Off by default so existing unpaired setups keep working;
	// turn on once the same NodeSecret has been provisioned on every paired node.
	RequireAuth bool `json:"require_auth"`
	// RemoteSendRoots whitelists the directories a remote peer may request through
	// /remote-send. Requests for paths outside these roots are rejected. Empty
	// defaults to the receive path, preventing arbitrary file exfiltration.
	RemoteSendRoots []string `json:"remote_send_roots,omitempty"`
}

func NormalizeInterfacePolicy(v string) string {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "manual", "all":
		return strings.ToLower(strings.TrimSpace(v))
	default:
		return "auto"
	}
}

func DefaultConfig() Config {
	host := os.Getenv("COMPUTERNAME")
	if host == "" {
		host = "multisend-node"
	}
	nodeID := fmt.Sprintf("%s-%d", strings.ToUpper(host), time.Now().UnixNano()%100000000)
	userProfile := os.Getenv("USERPROFILE")
	if userProfile == "" {
		userProfile = `C:\Users\Public`
	}
	return Config{
		SchemaVersion:            4,
		AppVersion:               "0.1.0",
		NodeID:                   nodeID,
		DisplayName:              host,
		ReceivePath:              filepath.Join(userProfile, "Downloads", "MultiSend"),
		TransferPort:             55202,
		DiscoveryPort:            55203,
		DiscoveryMode:            "auto",
		InterfacesMode:           "auto",
		Scheduler:                "auto",
		ChunkMode:                "auto",
		ChunkSizeMB:              32,
		StartOnLogin:             true,
		LocalAPIPort:             55204,
		ControlPort:              55205,
		TransferPortRange:        ports.Range{Start: 56200, End: 56210},
		DiscoveryPortRange:       ports.Range{Start: 56211, End: 56220},
		LocalAPIPortRange:        ports.Range{Start: 56221, End: 56230},
		ControlPortRange:         ports.Range{Start: 56231, End: 56240},
		SelectedPorts:            SelectedPorts{Transfer: 56200, Discovery: 56211, LocalAPI: 56221, Control: 56231},
		InterfacePolicy:          "auto",
		InterfaceRefresh:         5,
		AllowNewInterfaces:       true,
		IgnoreVirtual:            true,
		IgnoreVPN:                true,
		IgnoreLinkLocal:          true,
		AllowedIfTypes:           []string{"ethernet", "wifi", "usb_ethernet", "unknown"},
		ManualInterfaces:         []string{},
		IgnoredInterfaces:        []string{"loopback", "hyper-v", "tailscale", "vpn", "virtualbox", "vmware", "docker", "wsl", "bluetooth"},
		CleanupCompletedChunks:   false,
		KeepManifests:            true,
		CleanupEmptyDownloadDirs: false,
		DownloadPipelineMode:     "pipelined",
		DownloadInFlightPerChan:  2,
		DownloadChannelStrategy:  "dynamic",
		DownloadSplitCablePct:    50,
		PullFolderResult:         "zip",
		NodeSecret:               GenerateSecret(),
		RequireAuth:              false,
		RemoteSendRoots:          []string{},
	}
}

func NormalizeDownloadPipelineMode(v string) string {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "pipelined":
		return "pipelined"
	default:
		return "legacy"
	}
}

func NormalizeDownloadChannelStrategy(v string) string {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "strict_split":
		return "strict_split"
	default:
		return "dynamic"
	}
}

func NormalizePullFolderResult(v string) string {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "extract":
		return "extract"
	default:
		return "zip"
	}
}

func ConfigPath() (string, error) {
	cfgRoot, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(cfgRoot, "MultiSend", "config.json"), nil
}

func LoadOrCreate() (Config, string, error) {
	path, err := ConfigPath()
	if err != nil {
		return Config{}, "", err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return Config{}, "", err
	}
	if _, err := os.Stat(path); err == nil {
		raw, err := os.ReadFile(path)
		if err != nil {
			return Config{}, path, err
		}
		cfg := DefaultConfig()
		if err := json.Unmarshal(raw, &cfg); err != nil {
			return Config{}, path, err
		}
		migrated := migrateLegacyConfig(&cfg)
		if migrated {
			_ = backupConfig(path, raw)
			if err := Save(path, cfg); err != nil {
				return Config{}, path, err
			}
		}
		return cfg, path, nil
	}
	cfg := DefaultConfig()
	if err := Save(path, cfg); err != nil {
		return Config{}, path, err
	}
	return cfg, path, nil
}

func LoadIfExists() (Config, string, bool, error) {
	path, err := ConfigPath()
	if err != nil {
		return Config{}, "", false, err
	}
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return Config{}, path, false, nil
		}
		return Config{}, path, false, err
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return Config{}, path, true, err
	}
	cfg := DefaultConfig()
	if err := json.Unmarshal(b, &cfg); err != nil {
		return Config{}, path, true, err
	}
	_ = migrateLegacyConfig(&cfg)
	return cfg, path, true, nil
}

func Save(path string, cfg Config) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".multisend-config-*.tmp")
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

func migrateLegacyConfig(cfg *Config) bool {
	if cfg == nil {
		return false
	}
	def := DefaultConfig()
	migrated := false
	if cfg.SchemaVersion < 2 {
		cfg.SchemaVersion = 2
		migrated = true
	}
	if cfg.SchemaVersion < 3 {
		cfg.CleanupCompletedChunks = false
		cfg.KeepManifests = true
		cfg.CleanupEmptyDownloadDirs = false
		cfg.SchemaVersion = 3
		migrated = true
	}
	if cfg.SchemaVersion < 4 {
		cfg.SchemaVersion = 4
		migrated = true
	}
	if strings.TrimSpace(cfg.NodeSecret) == "" {
		cfg.NodeSecret = GenerateSecret()
		migrated = true
	}
	if cfg.RemoteSendRoots == nil {
		cfg.RemoteSendRoots = []string{}
		migrated = true
	}
	if cfg.TransferPortRange.Start == 0 || cfg.TransferPortRange.End == 0 {
		cfg.TransferPortRange = def.TransferPortRange
		migrated = true
	}
	if cfg.DiscoveryPortRange.Start == 0 || cfg.DiscoveryPortRange.End == 0 {
		cfg.DiscoveryPortRange = def.DiscoveryPortRange
		migrated = true
	}
	if cfg.LocalAPIPortRange.Start == 0 || cfg.LocalAPIPortRange.End == 0 {
		cfg.LocalAPIPortRange = def.LocalAPIPortRange
		migrated = true
	}
	if cfg.ControlPortRange.Start == 0 || cfg.ControlPortRange.End == 0 {
		cfg.ControlPortRange = def.ControlPortRange
		migrated = true
	}
	if cfg.SelectedPorts.Transfer == 0 {
		if cfg.TransferPort != 0 {
			cfg.SelectedPorts.Transfer = cfg.TransferPort
		} else {
			cfg.SelectedPorts.Transfer = cfg.TransferPortRange.Start
		}
		migrated = true
	}
	if cfg.SelectedPorts.Discovery == 0 {
		if cfg.DiscoveryPort != 0 {
			cfg.SelectedPorts.Discovery = cfg.DiscoveryPort
		} else {
			cfg.SelectedPorts.Discovery = cfg.DiscoveryPortRange.Start
		}
		migrated = true
	}
	if cfg.SelectedPorts.LocalAPI == 0 {
		if cfg.LocalAPIPort != 0 {
			cfg.SelectedPorts.LocalAPI = cfg.LocalAPIPort
		} else {
			cfg.SelectedPorts.LocalAPI = cfg.LocalAPIPortRange.Start
		}
		migrated = true
	}
	if cfg.SelectedPorts.Control == 0 {
		if cfg.ControlPort != 0 {
			cfg.SelectedPorts.Control = cfg.ControlPort
		} else {
			cfg.SelectedPorts.Control = cfg.ControlPortRange.Start
		}
		migrated = true
	}
	if cfg.TransferPort == 0 {
		cfg.TransferPort = cfg.SelectedPorts.Transfer
	}
	if cfg.DiscoveryPort == 0 {
		cfg.DiscoveryPort = cfg.SelectedPorts.Discovery
	}
	if cfg.LocalAPIPort == 0 {
		cfg.LocalAPIPort = cfg.SelectedPorts.LocalAPI
	}
	if cfg.ControlPort == 0 {
		cfg.ControlPort = cfg.SelectedPorts.Control
	}
	if strings.TrimSpace(cfg.InterfacePolicy) == "" {
		cfg.InterfacePolicy = def.InterfacePolicy
		migrated = true
	}
	policy := NormalizeInterfacePolicy(cfg.InterfacePolicy)
	if cfg.InterfacePolicy != policy {
		cfg.InterfacePolicy = policy
		migrated = true
	}
	if cfg.InterfaceRefresh <= 0 {
		cfg.InterfaceRefresh = def.InterfaceRefresh
		migrated = true
	}
	if len(cfg.AllowedIfTypes) == 0 {
		cfg.AllowedIfTypes = append([]string(nil), def.AllowedIfTypes...)
		migrated = true
	} else {
		hasUnknown := false
		for _, t := range cfg.AllowedIfTypes {
			if strings.EqualFold(t, "unknown") {
				hasUnknown = true
				break
			}
		}
		if !hasUnknown {
			cfg.AllowedIfTypes = append(cfg.AllowedIfTypes, "unknown")
			migrated = true
		}
	}
	if cfg.ManualInterfaces == nil {
		cfg.ManualInterfaces = []string{}
		migrated = true
	}
	if cfg.IgnoredInterfaces == nil {
		cfg.IgnoredInterfaces = []string{}
		migrated = true
	}
	mode := NormalizeDownloadPipelineMode(cfg.DownloadPipelineMode)
	if cfg.DownloadPipelineMode != mode {
		cfg.DownloadPipelineMode = mode
		migrated = true
	}
	if cfg.DownloadInFlightPerChan <= 0 {
		cfg.DownloadInFlightPerChan = def.DownloadInFlightPerChan
		migrated = true
	}
	strategy := NormalizeDownloadChannelStrategy(cfg.DownloadChannelStrategy)
	if cfg.DownloadChannelStrategy != strategy {
		cfg.DownloadChannelStrategy = strategy
		migrated = true
	}
	if cfg.DownloadSplitCablePct <= 0 || cfg.DownloadSplitCablePct >= 100 {
		cfg.DownloadSplitCablePct = def.DownloadSplitCablePct
		migrated = true
	}
	folderResult := NormalizePullFolderResult(cfg.PullFolderResult)
	if cfg.PullFolderResult != folderResult {
		cfg.PullFolderResult = folderResult
		migrated = true
	}
	return migrated
}

func backupConfig(path string, raw []byte) error {
	stamp := time.Now().Format("20060102-150405")
	bak := path + ".bak-" + stamp
	return os.WriteFile(bak, raw, 0o644)
}
