package main

// ====================== BEGIN NAV INDEX ======================
// NAV INDEX — auto-generated symbol map (refresh via the navindex skill)
//   L56    app.pullsHandler
//   L88    app.pullByID
//   L136   app.controlAuthOK
//   L152   app.controlAPIToken
//   L159   app.remoteSendRoots
//   L169   app.isRemoteSendPathAllowed
//   L186   resolveExistingPath
//   L198   resolvePathWithExistingParent
//   L228   app.isRemoteAllowed
//   L250   app.resolvePeerAddressByNodeID
//   L268   parseFileSource
//   L320   prepareRemoteSendSource
//   L357   zipDirectory
//   L416   app.findPeerByHost
//   L453   validateOutputUnderReceive
//   L476   app.remoteControlBase
//   L489   postJSON
//   L509   getJSON
//   L524   decodeRemoteAPIResponse
//   L541   setBearerToken
//   L547   formatRemoteAPIError
//   L572   app.createPull
//   L658   validateChunkSizeMB
//   L665   app.pullViewFromRemote
//   L732   app.listPulls
//   L752   app.refreshPull
//   L762   app.pullRemoteAction
//   L785   validateLabReceivePath
// ======================= END NAV INDEX =======================

import (
	"archive/zip"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	urlpkg "net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/Codyte/MultiSend/internal/config"
	"github.com/Codyte/MultiSend/internal/discovery"
)

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

func resolvePathWithExistingParent(raw string) (string, error) {
	abs, err := filepath.Abs(strings.TrimSpace(raw))
	if err != nil {
		return "", err
	}
	ancestor := abs
	missing := make([]string, 0)
	for {
		if _, err := os.Lstat(ancestor); err == nil {
			break
		} else if !os.IsNotExist(err) {
			return "", err
		}
		parent := filepath.Dir(ancestor)
		if parent == ancestor {
			return "", fmt.Errorf("no existing parent for %s", abs)
		}
		missing = append(missing, filepath.Base(ancestor))
		ancestor = parent
	}
	resolved, err := filepath.EvalSymlinks(ancestor)
	if err != nil {
		return "", err
	}
	for i := len(missing) - 1; i >= 0; i-- {
		resolved = filepath.Join(resolved, missing[i])
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
			return fmt.Errorf("symbolic link is not supported: %s", path)
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
	outResolved, err := resolvePathWithExistingParent(outputDir)
	if err != nil {
		return "", err
	}
	rootResolved, err := resolveExistingPath(receiveRoot)
	if err != nil {
		return "", err
	}
	if !isUnder(outResolved, rootResolved) {
		return "", fmt.Errorf("output_dir must stay inside %s", rootResolved)
	}
	rel, err := filepath.Rel(rootResolved, outResolved)
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
