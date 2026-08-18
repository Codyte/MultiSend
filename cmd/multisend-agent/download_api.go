package main

import (
	"log"
	"net/http"
	"sort"
	"strings"

	"github.com/Codyte/MultiSend/internal/download"
	"github.com/Codyte/MultiSend/internal/netif"
	"github.com/Codyte/MultiSend/internal/transfer"
)

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
	writeJSON(w, a.interfacesSnapshot())
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
