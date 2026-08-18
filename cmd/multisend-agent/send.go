package main

// ====================== BEGIN NAV INDEX ======================
// NAV INDEX — auto-generated symbol map (refresh via the navindex skill)
//   L39    app.health
//   L47    app.peers
//   L55    app.send
//   L97    app.startSend
//   L202   app.jobs
//   L210   app.jobByID
//   L272   app.remoteSend
//   L345   app.remoteJobByID
//   L408   app.createJob
//   L450   app.resumeJob
//   L508   app.cancelJob
//   L529   app.updateJobProgress
//   L651   app.finishJob
//   L680   app.setJobManifest
//   L695   app.refreshJobFromManifestLocked
// ======================= END NAV INDEX =======================

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Codyte/MultiSend/internal/config"
	"github.com/Codyte/MultiSend/internal/discovery"
	"github.com/Codyte/MultiSend/internal/manifest"
	"github.com/Codyte/MultiSend/internal/transfer"
)

func (a *app) health(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeAPIError(w, r, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}
	writeJSON(w, a.healthSnapshot())
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
	writeJSON(w, a.jobsSnapshot())
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
