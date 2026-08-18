package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Codyte/MultiSend/internal/config"
	"github.com/Codyte/MultiSend/internal/discovery"
	"github.com/Codyte/MultiSend/internal/download"
	"github.com/Codyte/MultiSend/internal/ifmonitor"
)

func localRequest(method, target string) *http.Request {
	req := httptest.NewRequest(method, target, nil)
	req.Host = "127.0.0.1:56221"
	return req
}

func TestLocalHandlerRedirectsAndServesEmbeddedUI(t *testing.T) {
	a := &app{cfg: config.Config{DisplayName: "Test node"}}
	handler := a.localHandler()

	redirect := httptest.NewRecorder()
	handler.ServeHTTP(redirect, localRequest(http.MethodGet, "/"))
	if redirect.Code != http.StatusTemporaryRedirect || redirect.Header().Get("Location") != "/ui/" {
		t.Fatalf("unexpected redirect: status=%d location=%q", redirect.Code, redirect.Header().Get("Location"))
	}

	page := httptest.NewRecorder()
	handler.ServeHTTP(page, localRequest(http.MethodGet, "/ui/"))
	if page.Code != http.StatusOK {
		t.Fatalf("expected UI 200, got %d body=%s", page.Code, page.Body.String())
	}
	body := page.Body.String()
	if !strings.Contains(body, "MultiSend") {
		t.Fatal("embedded UI marker not found")
	}
	if !strings.Contains(body, `id="transfer-form"`) {
		t.Fatal("unified transfer form not found")
	}
	for _, legacyForm := range []string{`id="download-form"`, `id="send-form"`, `id="pull-form"`} {
		if strings.Contains(body, legacyForm) {
			t.Fatalf("legacy transfer form still embedded: %s", legacyForm)
		}
	}
	if csp := page.Header().Get("Content-Security-Policy"); !strings.Contains(csp, "default-src 'self'") {
		t.Fatalf("expected restrictive CSP, got %q", csp)
	}
}

func TestLocalRequestGuardRejectsExternalHostAndOrigin(t *testing.T) {
	handler := (&app{}).localHandler()

	externalHost := httptest.NewRequest(http.MethodGet, "/health", nil)
	externalHost.Host = "multisend.example:56221"
	hostResponse := httptest.NewRecorder()
	handler.ServeHTTP(hostResponse, externalHost)
	if hostResponse.Code != http.StatusForbidden {
		t.Fatalf("expected external host to be rejected, got %d", hostResponse.Code)
	}

	externalOrigin := localRequest(http.MethodGet, "/health")
	externalOrigin.Header.Set("Origin", "https://example.com")
	originResponse := httptest.NewRecorder()
	handler.ServeHTTP(originResponse, externalOrigin)
	if originResponse.Code != http.StatusForbidden {
		t.Fatalf("expected external origin to be rejected, got %d", originResponse.Code)
	}

	localOrigin := localRequest(http.MethodGet, "/health")
	localOrigin.Header.Set("Origin", "http://127.0.0.1:56221")
	localResponse := httptest.NewRecorder()
	handler.ServeHTTP(localResponse, localOrigin)
	if localResponse.Code != http.StatusOK {
		t.Fatalf("expected matching local origin, got %d body=%s", localResponse.Code, localResponse.Body.String())
	}
}

func TestReadOnlyHandlersRejectPost(t *testing.T) {
	a := &app{jobsByID: map[string]*jobState{}}
	for _, handler := range []struct {
		name string
		fn   http.HandlerFunc
	}{
		{name: "health", fn: a.health},
		{name: "jobs", fn: a.jobs},
		{name: "api/v1/dashboard", fn: a.dashboardHandler},
	} {
		t.Run(handler.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/"+handler.name, nil)
			response := httptest.NewRecorder()
			handler.fn(response, req)
			if response.Code != http.StatusMethodNotAllowed {
				t.Fatalf("expected 405, got %d", response.Code)
			}
		})
	}
}

func TestDashboardHandlerReturnsUnifiedSnapshot(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.NodeID = "node-test"
	cfg.DisplayName = "Test node"
	cfg.ReceivePath = t.TempDir()

	originalDetect := detectUsableInterfaces
	defer func() { detectUsableInterfaces = originalDetect }()
	detectUsableInterfaces = func(config.Config) []ifmonitor.InterfaceInfo {
		return []ifmonitor.InterfaceInfo{{Name: "Wi-Fi", IPv4: "192.0.2.10", Usable: true}}
	}

	a := &app{
		cfg:       cfg,
		disc:      discovery.NewManager(cfg.NodeID, cfg.DisplayName, cfg.AppVersion, 56200, 56211, 56231, 56211, 56220, func() []string { return nil }),
		downloads: download.NewManager(cfg),
		jobsByID:  map[string]*jobState{},
		pullsByID: map[string]*pullJobState{},
	}
	req := localRequest(http.MethodGet, "/api/v1/dashboard")
	response := httptest.NewRecorder()
	a.dashboardHandler(response, req)
	if response.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", response.Code, response.Body.String())
	}
	var payload map[string]json.RawMessage
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode dashboard: %v", err)
	}
	for _, key := range []string{"health", "peers", "interfaces", "downloads", "jobs", "pulls"} {
		if _, ok := payload[key]; !ok {
			t.Fatalf("dashboard field missing: %s", key)
		}
	}
	var health map[string]any
	if err := json.Unmarshal(payload["health"], &health); err != nil || health["node_id"] != cfg.NodeID || health["ok"] != true {
		t.Fatalf("unexpected health snapshot: payload=%v err=%v", health, err)
	}
}

func TestJSONRequestBodyLimit(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/send", strings.NewReader(`{"file_path":"`+strings.Repeat("x", int(maxAPIRequestBodyBytes))+`"}`))
	response := httptest.NewRecorder()
	(&app{}).send(response, req)
	if response.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("expected 413, got %d body=%s", response.Code, response.Body.String())
	}
}

func TestShutdownAgentRespondsBeforeCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	a := &app{stop: cancel}
	req := httptest.NewRequest(http.MethodPost, "/agent/shutdown", nil)
	response := httptest.NewRecorder()
	a.shutdownAgent(response, req)
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), "stopping") {
		t.Fatalf("unexpected shutdown response: status=%d body=%s", response.Code, response.Body.String())
	}
	select {
	case <-ctx.Done():
	case <-time.After(time.Second):
		t.Fatal("shutdown did not cancel the agent context")
	}
}

func TestNewAgentHTTPServerHasDefensiveTimeouts(t *testing.T) {
	server := newAgentHTTPServer("127.0.0.1:0", http.NotFoundHandler())
	if server.ReadHeaderTimeout != 5*time.Second || server.ReadTimeout != 30*time.Second || server.WriteTimeout != 30*time.Second || server.IdleTimeout != 60*time.Second {
		t.Fatalf("unexpected timeouts: header=%s read=%s write=%s idle=%s", server.ReadHeaderTimeout, server.ReadTimeout, server.WriteTimeout, server.IdleTimeout)
	}
	if server.MaxHeaderBytes != 1<<20 {
		t.Fatalf("unexpected max header bytes: %d", server.MaxHeaderBytes)
	}
}
