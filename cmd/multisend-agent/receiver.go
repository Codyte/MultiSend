package main

// ====================== BEGIN NAV INDEX ======================
// NAV INDEX — auto-generated symbol map (refresh via the navindex skill)
//   L56    app.runReceiver
//   L79    app.handleConn
//   L204   receiveChunkToTemp
//   L235   receiverChunkMatches
//   L253   app.publishReceiverChunk
//   L274   app.updateReceiverManifest
//   L286   app.updateReceiverManifestFile
//   L342   app.ensureReceiverSession
//   L356   app.updateReceiverSession
//   L369   app.maybeScheduleReceiverFinalize
//   L390   app.finalizeReceiverSession
//   L469   app.failReceiverFinalize
//   L478   app.markReceiverDone
//   L494   countManifestChunksByStatus
//   L504   validateReceiverManifestComplete
//   L520   validateReceiverFinalSize
//   L531   validateReceiverPath
//   L553   isUnder
//   L567   isPullHeader
//   L571   validateReceiveRelPath
//   L589   resolveReceiveRelRoot
//   L612   sanitizePublishName
//   L624   receiverTargetInfo
//   L643   receiverFinalPath
//   L653   pullPublishRoot
//   L661   extractZipSafe
//   L716   mergeReceiverChunks
//   L748   cleanupReceiverChunks
// ======================= END NAV INDEX =======================

import (
	"archive/zip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/Codyte/MultiSend/internal/config"
	"github.com/Codyte/MultiSend/internal/manifest"
	"github.com/Codyte/MultiSend/internal/proto"
)

func (a *app) runReceiver(ctx context.Context) {
	ln, err := net.Listen("tcp", fmt.Sprintf("0.0.0.0:%d", a.cfg.SelectedPorts.Transfer))
	if err != nil {
		log.Printf("receiver listen error: %v", err)
		return
	}
	defer ln.Close()
	go func() {
		<-ctx.Done()
		_ = ln.Close()
	}()
	for {
		conn, err := ln.Accept()
		if err != nil {
			if strings.Contains(strings.ToLower(err.Error()), "closed") {
				return
			}
			continue
		}
		go a.handleConn(conn)
	}
}

func (a *app) handleConn(conn net.Conn) {
	defer conn.Close()
	h, err := proto.ReadHeader(conn)
	if err != nil {
		log.Printf("read header: %v", err)
		return
	}
	if err := proto.ValidateHeader(h); err != nil {
		log.Printf("reject invalid chunk header: %v", err)
		_ = proto.WriteChunkAck(conn, proto.ChunkAck{Type: "chunk_ack", TransferID: h.TransferID, ChunkIndex: h.ChunkIndex, Status: "failed", Error: err.Error()})
		return
	}
	if !proto.VerifyHeader(h, a.cfg.AuthSecret()) {
		log.Printf("reject unauthenticated chunk transfer=%s index=%d", h.TransferID, h.ChunkIndex)
		_ = proto.WriteChunkAck(conn, proto.ChunkAck{
			Type:       "chunk_ack",
			TransferID: h.TransferID,
			ChunkIndex: h.ChunkIndex,
			Status:     "unauthorized",
			Error:      "authentication required",
		})
		return
	}
	name := filepath.Base(h.FileName)
	safeTransferID := strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			return r
		}
		return -1
	}, h.TransferID)
	if safeTransferID == "" {
		safeTransferID = "unknown"
	}
	rootPath := a.cfg.ReceivePath
	if h.Lab {
		labRoot, err := validateLabReceivePath(a.cfg.ReceivePath, h.LabPath)
		if err != nil {
			log.Printf("reject lab receive path: %v", err)
			return
		}
		rootPath = labRoot
	} else if isPullHeader(h) {
		pullRoot, err := resolveReceiveRelRoot(a.cfg.ReceivePath, h.ReceiveRelPath)
		if err != nil {
			log.Printf("reject pull receive path: %v", err)
			return
		}
		rootPath = pullRoot
	}
	rootAbs, _ := filepath.Abs(rootPath)
	sessionBase := rootAbs
	if isPullHeader(h) {
		sessionBase = filepath.Join(rootAbs, "_pull_sessions")
	}
	sessionDir := filepath.Join(sessionBase, fmt.Sprintf("%s_%s", name, safeTransferID))
	sessionAbs, _ := filepath.Abs(sessionDir)
	if !isUnder(sessionAbs, rootAbs) {
		log.Printf("reject path traversal: %s", sessionAbs)
		return
	}
	if err := os.MkdirAll(sessionDir, 0o755); err != nil {
		log.Printf("mkdir session: %v", err)
		return
	}
	a.ensureReceiverSession(sessionDir, name, h)
	target := filepath.Join(sessionDir, fmt.Sprintf("%s.chunk%06d", name, h.ChunkIndex+1))
	targetAbs, _ := filepath.Abs(target)
	if !isUnder(targetAbs, sessionAbs) {
		log.Printf("reject chunk path traversal: %s", targetAbs)
		return
	}
	if receiverChunkMatches(target, h) {
		if err := a.updateReceiverManifest(sessionDir, name, h, h.PartSize); err != nil {
			log.Printf("save manifest for existing chunk: %v", err)
			return
		}
		if err := proto.WriteChunkAck(conn, proto.ChunkAck{
			Type:          "chunk_ack",
			TransferID:    h.TransferID,
			ChunkIndex:    h.ChunkIndex,
			Status:        "done",
			BytesReceived: h.PartSize,
		}); err != nil {
			log.Printf("write ack existing: %v", err)
		}
		a.maybeScheduleReceiverFinalize(sessionDir)
		return
	}
	tmpPath, written, err := receiveChunkToTemp(conn, sessionDir, h)
	if err != nil {
		if errors.Is(err, errReceiverChunkHashMismatch) {
			log.Printf("reject chunk hash mismatch transfer=%s index=%d", h.TransferID, h.ChunkIndex)
			_ = proto.WriteChunkAck(conn, proto.ChunkAck{
				Type:       "chunk_ack",
				TransferID: h.TransferID,
				ChunkIndex: h.ChunkIndex,
				Status:     "hash_mismatch",
				Error:      err.Error(),
			})
		} else {
			log.Printf("receive chunk payload: %v", err)
		}
		return
	}
	defer os.Remove(tmpPath)
	if err := a.publishReceiverChunk(sessionDir, name, target, tmpPath, h, written); err != nil {
		log.Printf("publish chunk: %v", err)
		return
	}
	if err := proto.WriteChunkAck(conn, proto.ChunkAck{
		Type:          "chunk_ack",
		TransferID:    h.TransferID,
		ChunkIndex:    h.ChunkIndex,
		Status:        "done",
		BytesReceived: written,
	}); err != nil {
		log.Printf("write ack: %v", err)
		return
	}
	a.maybeScheduleReceiverFinalize(sessionDir)
	log.Printf("received %s (%d bytes)", filepath.Base(target), h.PartSize)
}

var errReceiverChunkHashMismatch = errors.New("chunk integrity check failed")

func receiveChunkToTemp(conn net.Conn, sessionDir string, h proto.Header) (string, int64, error) {
	tmp, err := os.CreateTemp(sessionDir, ".incoming-*.tmp")
	if err != nil {
		return "", 0, err
	}
	tmpPath := tmp.Name()
	cleanup := func() {
		_ = tmp.Close()
		_ = os.Remove(tmpPath)
	}
	hasher := sha256.New()
	written, err := io.CopyN(io.MultiWriter(tmp, hasher), conn, h.PartSize)
	if err != nil {
		cleanup()
		return "", written, err
	}
	if h.ChunkSHA256 != "" && !strings.EqualFold(hex.EncodeToString(hasher.Sum(nil)), h.ChunkSHA256) {
		cleanup()
		return "", written, errReceiverChunkHashMismatch
	}
	if err := tmp.Sync(); err != nil {
		cleanup()
		return "", written, err
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return "", written, err
	}
	return tmpPath, written, nil
}

func receiverChunkMatches(path string, h proto.Header) bool {
	info, err := os.Stat(path)
	if err != nil || !info.Mode().IsRegular() || info.Size() != h.PartSize {
		return false
	}
	if h.ChunkSHA256 == "" {
		return true
	}
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	hasher := sha256.New()
	_, copyErr := io.Copy(hasher, f)
	closeErr := f.Close()
	return copyErr == nil && closeErr == nil && strings.EqualFold(hex.EncodeToString(hasher.Sum(nil)), h.ChunkSHA256)
}

func (a *app) publishReceiverChunk(sessionDir, name, target, tmpPath string, h proto.Header, written int64) error {
	a.receiverSessionsMu.RLock()
	st := a.receiverSessions[sessionDir]
	a.receiverSessionsMu.RUnlock()
	if st == nil {
		return fmt.Errorf("receiver session is not initialized")
	}
	st.manifestMu.Lock()
	defer st.manifestMu.Unlock()
	if receiverChunkMatches(target, h) {
		return a.updateReceiverManifestFile(sessionDir, name, h, written)
	}
	if err := os.Remove(target); err != nil && !os.IsNotExist(err) {
		return err
	}
	if err := os.Rename(tmpPath, target); err != nil {
		return err
	}
	return a.updateReceiverManifestFile(sessionDir, name, h, written)
}

func (a *app) updateReceiverManifest(sessionDir, name string, h proto.Header, written int64) error {
	a.receiverSessionsMu.RLock()
	st := a.receiverSessions[sessionDir]
	a.receiverSessionsMu.RUnlock()
	if st == nil {
		return fmt.Errorf("receiver session is not initialized")
	}
	st.manifestMu.Lock()
	defer st.manifestMu.Unlock()
	return a.updateReceiverManifestFile(sessionDir, name, h, written)
}

func (a *app) updateReceiverManifestFile(sessionDir, name string, h proto.Header, written int64) error {
	mfPath := filepath.Join(sessionDir, "manifest.json")
	mf, lerr := manifest.Load(mfPath)
	if lerr != nil {
		if !os.IsNotExist(lerr) {
			return fmt.Errorf("load receiver manifest: %w", lerr)
		}
		mf = &manifest.Manifest{
			SchemaVersion: 1,
			TransferID:    h.TransferID,
			Type:          "p2p",
			FileName:      name,
			TotalBytes:    h.TotalBytes,
			ChunkSize:     h.PartSize,
			CreatedAt:     time.Now().UTC(),
			UpdatedAt:     time.Now().UTC(),
			Source:        manifest.SourceInfo{Type: "p2p"},
			Target:        receiverTargetInfo(sessionDir, name, h),
		}
	} else {
		wantTarget := receiverTargetInfo(sessionDir, name, h)
		if mf.TransferID != h.TransferID || mf.FileName != name || mf.TotalBytes != h.TotalBytes || mf.Target != wantTarget {
			return fmt.Errorf("receiver manifest identity mismatch")
		}
	}
	if chunk := mf.GetChunk(h.ChunkIndex); chunk != nil {
		if chunk.Offset != h.Offset || chunk.Size != h.PartSize {
			return fmt.Errorf("receiver manifest chunk mismatch")
		}
		mf.MarkDone(h.ChunkIndex, written, "")
	} else {
		mf.Chunks = append(mf.Chunks, manifest.ChunkState{
			Index:     h.ChunkIndex,
			Offset:    h.Offset,
			Size:      h.PartSize,
			Status:    manifest.StatusDone,
			BytesDone: written,
		})
	}
	if err := manifest.SaveAtomic(mfPath, mf); err != nil {
		return err
	}
	a.updateReceiverSession(sessionDir, func(s *receiverSession) {
		s.SessionDir = sessionDir
		s.TransferID = h.TransferID
		s.FileName = name
		s.ManifestPath = mfPath
		s.Status = "receiving"
		s.ChunksTotal = int64(len(mf.Chunks))
		s.ChunksDone = countManifestChunksByStatus(mf, manifest.StatusDone)
		s.ChunksFailed = countManifestChunksByStatus(mf, manifest.StatusFailed)
		s.MergeStatus = "pending"
	})
	return nil
}

func (a *app) ensureReceiverSession(sessionDir, name string, h proto.Header) {
	a.updateReceiverSession(sessionDir, func(s *receiverSession) {
		s.SessionDir = sessionDir
		s.TransferID = h.TransferID
		s.FileName = name
		if s.Status == "" {
			s.Status = "receiving"
		}
		if s.MergeStatus == "" {
			s.MergeStatus = "pending"
		}
	})
}

func (a *app) updateReceiverSession(sessionDir string, fn func(*receiverSession)) {
	a.receiverSessionsMu.Lock()
	defer a.receiverSessionsMu.Unlock()
	st := a.receiverSessions[sessionDir]
	if st == nil {
		st = &receiverSessionState{session: receiverSession{SessionDir: sessionDir, MergeStatus: "pending"}}
		a.receiverSessions[sessionDir] = st
	}
	st.mu.Lock()
	defer st.mu.Unlock()
	fn(&st.session)
}

func (a *app) maybeScheduleReceiverFinalize(sessionDir string) {
	a.receiverSessionsMu.RLock()
	st := a.receiverSessions[sessionDir]
	a.receiverSessionsMu.RUnlock()
	if st == nil {
		return
	}
	st.mu.Lock()
	if st.session.MergeStatus == "running" || st.session.MergeStatus == "done" {
		st.mu.Unlock()
		return
	}
	if st.timer != nil {
		st.timer.Stop()
	}
	st.session.MergeStatus = "pending"
	st.session.MergeMessage = ""
	st.timer = time.AfterFunc(3*time.Second, func() { a.finalizeReceiverSession(sessionDir) })
	st.mu.Unlock()
}

func (a *app) finalizeReceiverSession(sessionDir string) {
	a.receiverSessionsMu.RLock()
	st := a.receiverSessions[sessionDir]
	a.receiverSessionsMu.RUnlock()
	if st == nil {
		return
	}
	st.mu.Lock()
	if st.session.MergeStatus == "running" || st.session.MergeStatus == "done" {
		st.mu.Unlock()
		return
	}
	st.session.MergeStatus = "running"
	st.session.MergeMessage = ""
	st.session.CleanupStatus = "pending"
	st.mu.Unlock()
	st.manifestMu.Lock()
	defer st.manifestMu.Unlock()

	mfPath := filepath.Join(sessionDir, "manifest.json")
	mf, err := manifest.Load(mfPath)
	if err != nil {
		a.failReceiverFinalize(sessionDir, "load manifest: "+err.Error())
		return
	}
	if err := validateReceiverManifestComplete(mf); err != nil {
		a.failReceiverFinalize(sessionDir, err.Error())
		return
	}
	finalPath := receiverFinalPath(sessionDir, mf)
	if err := validateReceiverPath(sessionDir, finalPath); err != nil {
		a.failReceiverFinalize(sessionDir, err.Error())
		return
	}
	if info, err := os.Stat(finalPath); err == nil {
		if info.Size() == mf.TotalBytes {
			a.markReceiverDone(sessionDir, mf, finalPath, "already finalized")
			return
		}
		if err := os.Remove(finalPath); err != nil {
			a.failReceiverFinalize(sessionDir, "remove stale final: "+err.Error())
			return
		}
	}
	tmpPath := finalPath + ".tmp"
	if err := mergeReceiverChunks(tmpPath, sessionDir, mf); err != nil {
		_ = os.Remove(tmpPath)
		a.failReceiverFinalize(sessionDir, err.Error())
		return
	}
	if err := os.Rename(tmpPath, finalPath); err != nil {
		_ = os.Remove(tmpPath)
		a.failReceiverFinalize(sessionDir, "publish final: "+err.Error())
		return
	}
	if err := validateReceiverFinalSize(finalPath, mf.TotalBytes); err != nil {
		a.failReceiverFinalize(sessionDir, err.Error())
		return
	}
	finalOutputPath := finalPath
	if mf.Target.Address == "extract" {
		extractDir := strings.TrimSuffix(finalPath, ".zip")
		if extractDir == finalPath {
			extractDir = finalPath + ".extracted"
		}
		if err := extractZipSafe(finalPath, extractDir); err != nil {
			a.failReceiverFinalize(sessionDir, "extract zip: "+err.Error())
			return
		}
		_ = os.Remove(finalPath)
		finalOutputPath = extractDir
	}
	if err := cleanupReceiverChunks(sessionDir); err != nil {
		a.failReceiverFinalize(sessionDir, err.Error())
		return
	}
	a.markReceiverDone(sessionDir, mf, finalOutputPath, "merged and cleaned")
}

func (a *app) failReceiverFinalize(sessionDir, msg string) {
	log.Printf("receiver_finalize_failed session_dir=%q message=%q", sessionDir, msg)
	a.updateReceiverSession(sessionDir, func(s *receiverSession) {
		s.MergeStatus = "failed"
		s.MergeMessage = msg
		s.CleanupStatus = "failed"
	})
}

func (a *app) markReceiverDone(sessionDir string, mf *manifest.Manifest, finalPath, msg string) {
	log.Printf("receiver_finalize_done session_dir=%q transfer_id=%s final=%q message=%q", sessionDir, mf.TransferID, finalPath, msg)
	a.updateReceiverSession(sessionDir, func(s *receiverSession) {
		s.MergeStatus = "done"
		s.MergeMessage = msg
		s.CleanupStatus = "done"
		s.Status = "done"
		s.Message = ""
		s.ManifestPath = filepath.Join(sessionDir, "manifest.json")
		s.FinalOutputPath = finalPath
		s.ChunksTotal = int64(len(mf.Chunks))
		s.ChunksDone = countManifestChunksByStatus(mf, manifest.StatusDone)
		s.ChunksFailed = countManifestChunksByStatus(mf, manifest.StatusFailed)
	})
}

func countManifestChunksByStatus(mf *manifest.Manifest, status string) int64 {
	var n int64
	for _, c := range mf.Chunks {
		if c.Status == status {
			n++
		}
	}
	return n
}

func validateReceiverManifestComplete(mf *manifest.Manifest) error {
	if mf == nil {
		return fmt.Errorf("manifest is nil")
	}
	if mf.TotalBytes <= 0 {
		return fmt.Errorf("invalid total bytes")
	}
	if countManifestChunksByStatus(mf, manifest.StatusDone) != int64(len(mf.Chunks)) {
		return fmt.Errorf("chunks_done must equal chunks_total")
	}
	if countManifestChunksByStatus(mf, manifest.StatusFailed) != 0 {
		return fmt.Errorf("chunks_failed must be zero")
	}
	return nil
}

func validateReceiverFinalSize(finalPath string, totalBytes int64) error {
	info, err := os.Stat(finalPath)
	if err != nil {
		return err
	}
	if info.Size() != totalBytes {
		return fmt.Errorf("final size mismatch got=%d want=%d", info.Size(), totalBytes)
	}
	return nil
}

func validateReceiverPath(root, target string) error {
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return err
	}
	targetAbs, err := filepath.Abs(target)
	if err != nil {
		return err
	}
	if isUnder(targetAbs, rootAbs) {
		return nil
	}
	pullRoot := pullPublishRoot(rootAbs)
	if pullRoot != "" && isUnder(targetAbs, pullRoot) && !isUnder(targetAbs, filepath.Join(pullRoot, "_pull_sessions")) {
		return nil
	}
	if !isUnder(targetAbs, rootAbs) {
		return fmt.Errorf("path outside receive root")
	}
	return nil
}

func isUnder(path, root string) bool {
	p, err := filepath.Abs(path)
	if err != nil {
		return false
	}
	r, err := filepath.Abs(root)
	if err != nil {
		return false
	}
	lp := strings.ToLower(p)
	lr := strings.ToLower(r)
	return lp == lr || strings.HasPrefix(lp, lr+string(os.PathSeparator))
}

func isPullHeader(h proto.Header) bool {
	return strings.TrimSpace(h.ReceiveRelPath) != "" || strings.TrimSpace(h.PublishName) != "" || strings.TrimSpace(h.FolderResult) != ""
}

func validateReceiveRelPath(rel string) (string, error) {
	rel = strings.TrimSpace(rel)
	if rel == "" || rel == "." {
		return ".", nil
	}
	if filepath.IsAbs(rel) {
		return "", fmt.Errorf("path must be relative")
	}
	clean := filepath.Clean(rel)
	if clean == "." {
		return ".", nil
	}
	if clean == ".." || strings.HasPrefix(clean, ".."+string(os.PathSeparator)) {
		return "", fmt.Errorf("path traversal is not allowed")
	}
	return clean, nil
}

func resolveReceiveRelRoot(receivePath, rel string) (string, error) {
	clean, err := validateReceiveRelPath(rel)
	if err != nil {
		return "", err
	}
	rootResolved, err := resolveExistingPath(receivePath)
	if err != nil {
		return "", err
	}
	target := receivePath
	if clean != "." {
		target = filepath.Join(receivePath, clean)
	}
	targetResolved, err := resolvePathWithExistingParent(target)
	if err != nil {
		return "", err
	}
	if !isUnder(targetResolved, rootResolved) {
		return "", fmt.Errorf("path must stay inside %s", rootResolved)
	}
	return targetResolved, nil
}

func sanitizePublishName(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return ""
	}
	name = filepath.Base(strings.ReplaceAll(name, "/", string(os.PathSeparator)))
	if name == "." || name == string(os.PathSeparator) {
		return ""
	}
	return name
}

func receiverTargetInfo(sessionDir, name string, h proto.Header) manifest.TargetInfo {
	if !isPullHeader(h) {
		return manifest.TargetInfo{Type: "p2p", Path: sessionDir}
	}
	publishName := sanitizePublishName(h.PublishName)
	if publishName == "" {
		publishName = filepath.Base(name)
	}
	root := pullPublishRoot(sessionDir)
	if root == "" {
		root = sessionDir
	}
	return manifest.TargetInfo{
		Type:    "file",
		Path:    filepath.Join(root, publishName),
		Address: config.NormalizePullFolderResult(h.FolderResult),
	}
}

func receiverFinalPath(sessionDir string, mf *manifest.Manifest) string {
	if mf != nil && mf.Target.Type == "file" && strings.TrimSpace(mf.Target.Path) != "" {
		return mf.Target.Path
	}
	if mf == nil {
		return sessionDir
	}
	return filepath.Join(sessionDir, mf.FileName)
}

func pullPublishRoot(sessionDir string) string {
	parent := filepath.Dir(sessionDir)
	if strings.EqualFold(filepath.Base(parent), "_pull_sessions") {
		return filepath.Dir(parent)
	}
	return ""
}

func extractZipSafe(zipPath, destDir string) error {
	destAbs, err := filepath.Abs(destDir)
	if err != nil {
		return err
	}
	if _, err := os.Stat(destAbs); err == nil {
		return fmt.Errorf("destination already exists: %s", destAbs)
	} else if !os.IsNotExist(err) {
		return err
	}
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return err
	}
	defer r.Close()
	for _, f := range r.File {
		target := filepath.Join(destAbs, f.Name)
		targetAbs, err := filepath.Abs(target)
		if err != nil {
			return err
		}
		if !isUnder(targetAbs, destAbs) {
			return fmt.Errorf("zip entry outside destination: %s", f.Name)
		}
		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(targetAbs, 0o755); err != nil {
				return err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(targetAbs), 0o755); err != nil {
			return err
		}
		src, err := f.Open()
		if err != nil {
			return err
		}
		dst, err := os.OpenFile(targetAbs, os.O_CREATE|os.O_WRONLY|os.O_EXCL, f.Mode())
		if err != nil {
			_ = src.Close()
			return err
		}
		_, copyErr := io.Copy(dst, src)
		closeErr := dst.Close()
		_ = src.Close()
		if copyErr != nil {
			return copyErr
		}
		if closeErr != nil {
			return closeErr
		}
	}
	return nil
}

func mergeReceiverChunks(finalPath, sessionDir string, mf *manifest.Manifest) error {
	if err := validateReceiverPath(sessionDir, finalPath); err != nil {
		return err
	}
	out, err := os.Create(finalPath)
	if err != nil {
		return err
	}
	defer out.Close()
	chunks := append([]manifest.ChunkState(nil), mf.Chunks...)
	sort.Slice(chunks, func(i, j int) bool { return chunks[i].Index < chunks[j].Index })
	for _, c := range chunks {
		part := filepath.Join(sessionDir, fmt.Sprintf("%s.chunk%06d", mf.FileName, c.Index+1))
		if err := validateReceiverPath(sessionDir, part); err != nil {
			return err
		}
		f, err := os.Open(part)
		if err != nil {
			return err
		}
		n, err := io.Copy(out, f)
		_ = f.Close()
		if err != nil {
			return err
		}
		if n != c.Size {
			return fmt.Errorf("chunk size mismatch at index %d", c.Index)
		}
	}
	return nil
}

func cleanupReceiverChunks(sessionDir string) error {
	files, err := filepath.Glob(filepath.Join(sessionDir, "*.chunk*"))
	if err != nil {
		return err
	}
	for _, f := range files {
		if strings.HasSuffix(f, ".tmp") {
			continue
		}
		if err := os.Remove(f); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	return nil
}
