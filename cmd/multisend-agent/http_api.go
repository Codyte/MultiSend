package main

import (
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	urlpkg "net/url"
	"strings"
	"time"
)

const maxAPIRequestBodyBytes int64 = 1 << 20

func (a *app) localHandler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", a.health)
	mux.HandleFunc("/peers", a.peers)
	mux.HandleFunc("/send", a.send)
	mux.HandleFunc("/pulls", a.pullsHandler)
	mux.HandleFunc("/pulls/", a.pullByID)
	mux.HandleFunc("/jobs", a.jobs)
	mux.HandleFunc("/jobs/", a.jobByID)
	mux.HandleFunc("/receiver/sessions", a.receiverSessionsHandler)
	mux.HandleFunc("/receiver/sessions/", a.receiverSessionByID)
	mux.HandleFunc("/downloads", a.downloadsHandler)
	mux.HandleFunc("/downloads/", a.downloadByID)
	mux.HandleFunc("/interfaces", a.interfacesHandler)
	mux.HandleFunc("/agent/shutdown", a.shutdownAgent)
	mux.HandleFunc("/api/v1/dashboard", a.dashboardHandler)
	mux.HandleFunc("/api/v1/config", a.configHandler)
	mux.Handle("/ui/", http.StripPrefix("/ui", webUIHandler()))
	mux.HandleFunc("/ui", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/ui/", http.StatusTemporaryRedirect)
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		http.Redirect(w, r, "/ui/", http.StatusTemporaryRedirect)
	})
	return localRequestGuard(mux)
}

func (a *app) shutdownAgent(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeAPIError(w, r, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}
	if a.stop == nil {
		writeAPIError(w, r, http.StatusServiceUnavailable, "shutdown_unavailable", "agent shutdown is unavailable")
		return
	}
	writeJSON(w, map[string]string{"status": "stopping"})
	go func() {
		time.Sleep(100 * time.Millisecond)
		a.stop()
	}()
}

func (a *app) controlHandler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/remote-send", a.remoteSend)
	mux.HandleFunc("/remote-jobs/", a.remoteJobByID)
	return mux
}

func newAgentHTTPServer(addr string, handler http.Handler) *http.Server {
	return &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    1 << 20,
	}
}

func localRequestGuard(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("X-Frame-Options", "DENY")

		if !isLoopbackHost(r.Host) {
			writeAPIError(w, r, http.StatusForbidden, "invalid_host", "local API requires a loopback host")
			return
		}
		if !isSameRequestOrigin(r) {
			writeAPIError(w, r, http.StatusForbidden, "invalid_origin", "request origin does not match the local API")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func isLoopbackHost(hostport string) bool {
	host := strings.TrimSpace(hostport)
	if parsed, _, err := net.SplitHostPort(host); err == nil {
		host = parsed
	}
	host = strings.Trim(strings.ToLower(host), "[]")
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func isSameRequestOrigin(r *http.Request) bool {
	origin := strings.TrimSpace(r.Header.Get("Origin"))
	if origin == "" {
		return true
	}
	u, err := urlpkg.Parse(origin)
	if err != nil || u.Scheme != "http" || u.User != nil || u.Path != "" {
		return false
	}
	return strings.EqualFold(u.Host, r.Host)
}

func decodeJSONRequest(w http.ResponseWriter, r *http.Request, dst any) bool {
	return decodeJSONRequestAllowEmpty(w, r, dst, false)
}

func decodeOptionalJSONRequest(w http.ResponseWriter, r *http.Request, dst any) bool {
	return decodeJSONRequestAllowEmpty(w, r, dst, true)
}

func decodeJSONRequestAllowEmpty(w http.ResponseWriter, r *http.Request, dst any, allowEmpty bool) bool {
	if r.Body == nil {
		if allowEmpty {
			return true
		}
		writeAPIError(w, r, http.StatusBadRequest, "bad_json", "request body is required")
		return false
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxAPIRequestBodyBytes)
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(dst); err != nil {
		if allowEmpty && errors.Is(err, io.EOF) {
			return true
		}
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			writeAPIError(w, r, http.StatusRequestEntityTooLarge, "body_too_large", "request body is too large")
			return false
		}
		writeAPIErrorDetail(w, r, http.StatusBadRequest, "bad_json", "invalid JSON request", err.Error())
		return false
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		writeAPIError(w, r, http.StatusBadRequest, "bad_json", "request body must contain one JSON value")
		return false
	}
	return true
}
