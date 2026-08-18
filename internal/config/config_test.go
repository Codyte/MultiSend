package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestNormalizeInterfacePolicy(t *testing.T) {
	cases := map[string]string{
		"auto":     "auto",
		" manual ": "manual",
		"ALL":      "all",
		"":         "auto",
		"weird":    "auto",
	}
	for in, want := range cases {
		if got := NormalizeInterfacePolicy(in); got != want {
			t.Fatalf("normalize %q: got %q want %q", in, got, want)
		}
	}
}

func TestNormalizeDownloadPipelineMode(t *testing.T) {
	cases := map[string]string{
		"legacy":    "legacy",
		" LEGACY ":  "legacy",
		"pipelined": "pipelined",
		"weird":     "legacy",
		"":          "legacy",
	}
	for in, want := range cases {
		if got := NormalizeDownloadPipelineMode(in); got != want {
			t.Fatalf("normalize pipeline %q: got %q want %q", in, got, want)
		}
	}
}

func TestNormalizeDownloadChannelStrategy(t *testing.T) {
	cases := map[string]string{
		"dynamic":       "dynamic",
		" strict_split": "strict_split",
		"STRICT_SPLIT":  "strict_split",
		"":              "dynamic",
		"weird":         "dynamic",
	}
	for in, want := range cases {
		if got := NormalizeDownloadChannelStrategy(in); got != want {
			t.Fatalf("normalize strategy %q: got %q want %q", in, got, want)
		}
	}
}

func TestNormalizePullFolderResult(t *testing.T) {
	cases := map[string]string{
		"zip":       "zip",
		" ZIP ":     "zip",
		"extract":   "extract",
		"EXTRACT":   "extract",
		"":          "zip",
		"something": "zip",
	}
	for in, want := range cases {
		if got := NormalizePullFolderResult(in); got != want {
			t.Fatalf("normalize pull folder result %q: got %q want %q", in, got, want)
		}
	}
}

func TestSaveAtomicallyReplacesValidConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	cfg := DefaultConfig()
	cfg.DisplayName = "first"
	if err := Save(path, cfg); err != nil {
		t.Fatalf("first save: %v", err)
	}
	cfg.DisplayName = "second"
	if err := Save(path, cfg); err != nil {
		t.Fatalf("replace save: %v", err)
	}

	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	var got Config
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatalf("saved config is invalid JSON: %v", err)
	}
	if got.DisplayName != "second" {
		t.Fatalf("expected replacement config, got %q", got.DisplayName)
	}
	temps, err := filepath.Glob(filepath.Join(dir, ".multisend-config-*.tmp"))
	if err != nil {
		t.Fatalf("glob temp files: %v", err)
	}
	if len(temps) != 0 {
		t.Fatalf("temporary files remained after save: %v", temps)
	}
}

func TestLoadIfExistsPreservesExplicitFalseAndDefaultsOmittedBooleans(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("APPDATA", dir)
	path := filepath.Join(dir, "MultiSend", "config.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	raw := []byte(`{
  "schema_version": 4,
  "allow_new_interfaces_during_transfer": false,
  "ignore_virtual_interfaces": false
}`)
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		t.Fatal(err)
	}

	cfg, gotPath, exists, err := LoadIfExists()
	if err != nil || !exists || gotPath != path {
		t.Fatalf("load failed: path=%q exists=%t err=%v", gotPath, exists, err)
	}
	if cfg.AllowNewInterfaces || cfg.IgnoreVirtual {
		t.Fatalf("explicit false values were overwritten: %+v", cfg)
	}
	if !cfg.IgnoreVPN || !cfg.IgnoreLinkLocal {
		t.Fatalf("omitted booleans did not retain defaults: %+v", cfg)
	}
}
