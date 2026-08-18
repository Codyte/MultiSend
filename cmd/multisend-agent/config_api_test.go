package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Codyte/MultiSend/internal/config"
)

func writeConfigFixture(t *testing.T, cfg config.Config) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.json")
	if err := config.Save(path, cfg); err != nil {
		t.Fatalf("save fixture: %v", err)
	}
	return path
}

func TestConfigGetRedactsNodeSecret(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.NodeSecret = "super-secret-value"
	a := &app{configPath: writeConfigFixture(t, cfg)}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/config", nil)
	response := httptest.NewRecorder()
	a.configHandler(response, req)
	if response.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", response.Code, response.Body.String())
	}
	if strings.Contains(response.Body.String(), cfg.NodeSecret) || strings.Contains(response.Body.String(), "node_secret") {
		t.Fatalf("secret leaked in config response: %s", response.Body.String())
	}
	var got configAPIResponse
	if err := json.Unmarshal(response.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !got.SecretConfigured || got.SecretExposed || got.RestartRequired {
		t.Fatalf("unexpected security metadata: %+v", got)
	}
}

func TestConfigPutValidatesAndPreservesRuntimeFields(t *testing.T) {
	root := t.TempDir()
	receivePath := filepath.Join(root, "receive")
	remoteRoot := filepath.Join(root, "shared")
	if err := os.MkdirAll(remoteRoot, 0o755); err != nil {
		t.Fatalf("mkdir remote root: %v", err)
	}
	cfg := config.DefaultConfig()
	cfg.NodeID = "stable-node-id"
	cfg.NodeSecret = "stable-secret"
	path := writeConfigFixture(t, cfg)
	a := &app{configPath: path}
	update := editableConfigFrom(cfg)
	update.DisplayName = " Updated node "
	update.ReceivePath = receivePath
	update.InterfacePolicy = "manual"
	update.InterfaceRefresh = 8
	update.AllowedIfTypes = []string{"wifi", "ethernet", "wifi"}
	update.ManualInterfaces = []string{" Wi-Fi ", "Wi-Fi"}
	update.RemoteSendRoots = []string{remoteRoot, remoteRoot}
	update.RequireAuth = true
	b, _ := json.Marshal(update)
	req := httptest.NewRequest(http.MethodPut, "/api/v1/config", bytes.NewReader(b))
	response := httptest.NewRecorder()
	a.configHandler(response, req)
	if response.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", response.Code, response.Body.String())
	}
	var result configAPIResponse
	if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !result.RestartRequired || result.Config.DisplayName != "Updated node" {
		t.Fatalf("unexpected update response: %+v", result)
	}
	if len(result.Config.AllowedIfTypes) != 2 || len(result.Config.ManualInterfaces) != 1 || len(result.Config.RemoteSendRoots) != 1 {
		t.Fatalf("lists were not normalized: %+v", result.Config)
	}
	if _, err := os.Stat(receivePath); err != nil {
		t.Fatalf("receive path was not created: %v", err)
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read persisted config: %v", err)
	}
	var persisted config.Config
	if err := json.Unmarshal(raw, &persisted); err != nil {
		t.Fatalf("decode persisted config: %v", err)
	}
	if persisted.NodeID != "stable-node-id" || persisted.NodeSecret != "stable-secret" {
		t.Fatalf("runtime/security identity changed: node_id=%q secret=%q", persisted.NodeID, persisted.NodeSecret)
	}
	if persisted.DisplayName != "Updated node" || persisted.ReceivePath != receivePath || persisted.InterfacePolicy != "manual" {
		t.Fatalf("editable fields were not persisted: %+v", persisted)
	}
}

func TestConfigPutRejectsUnsafeRootWithoutChangingFile(t *testing.T) {
	cfg := config.DefaultConfig()
	path := writeConfigFixture(t, cfg)
	before, _ := os.ReadFile(path)
	a := &app{configPath: path}
	update := editableConfigFrom(cfg)
	update.ReceivePath = string(os.PathSeparator)
	b, _ := json.Marshal(update)
	req := httptest.NewRequest(http.MethodPut, "/api/v1/config", bytes.NewReader(b))
	response := httptest.NewRecorder()
	a.configHandler(response, req)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d body=%s", response.Code, response.Body.String())
	}
	after, _ := os.ReadFile(path)
	if !bytes.Equal(before, after) {
		t.Fatal("invalid request changed the configuration file")
	}
}

func TestConfigPutRejectsRelativeReceivePath(t *testing.T) {
	cfg := config.DefaultConfig()
	path := writeConfigFixture(t, cfg)
	a := &app{configPath: path}
	update := editableConfigFrom(cfg)
	update.ReceivePath = filepath.Join("relative", "downloads")
	b, _ := json.Marshal(update)
	response := httptest.NewRecorder()
	a.configHandler(response, httptest.NewRequest(http.MethodPut, "/api/v1/config", bytes.NewReader(b)))
	if response.Code != http.StatusBadRequest || !strings.Contains(response.Body.String(), "path must be absolute") {
		t.Fatalf("expected relative path rejection, got %d body=%s", response.Code, response.Body.String())
	}
}

func TestConfigPutGeneratesSecretWhenAuthIsEnabled(t *testing.T) {
	root := t.TempDir()
	cfg := config.DefaultConfig()
	cfg.ReceivePath = filepath.Join(root, "receive")
	cfg.NodeSecret = ""
	path := writeConfigFixture(t, cfg)
	a := &app{configPath: path}
	update := editableConfigFrom(cfg)
	update.RequireAuth = true
	b, _ := json.Marshal(update)
	response := httptest.NewRecorder()
	a.configHandler(response, httptest.NewRequest(http.MethodPut, "/api/v1/config", bytes.NewReader(b)))
	if response.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", response.Code, response.Body.String())
	}
	raw, _ := os.ReadFile(path)
	var persisted config.Config
	_ = json.Unmarshal(raw, &persisted)
	if persisted.NodeSecret == "" || !persisted.RequireAuth {
		t.Fatalf("auth secret was not generated: %+v", persisted)
	}
	if strings.Contains(response.Body.String(), persisted.NodeSecret) {
		t.Fatal("generated secret leaked in response")
	}
}
