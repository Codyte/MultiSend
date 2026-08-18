package main

// ====================== BEGIN NAV INDEX ======================
// NAV INDEX — auto-generated symbol map (refresh via the navindex skill)
//   L58    TestControlJSONClientsSendBearerToken
//   L83    TestControlJSONClientRejectsOversizedResponse
//   L96    TestControlAPITokenOnlyWhenAuthRequired
//   L107   TestControlAuthFailsClosedAndAcceptsCaseInsensitiveScheme
//   L124   TestRemoteSendPathAllowedResolvesSymlinkEscape
//   L152   TestStartSendRejectsInvalidChunkSizeBeforeCreatingJob
//   L166   TestSendRejectsLabReceivePathWhenLabFalse
//   L193   TestSendRejectsRelativePathAndInvalidChunkSize
//   L223   TestSendRejectsUnsafeCleanupPath
//   L237   TestPullRejectsInvalidChunkSizeBeforeDiscovery
//   L247   TestInterfacesHandlerPayload
//   L292   TestWriteAPIErrorReturnsJSON
//   L314   TestFormatRemoteAPIErrorIncludesRequestID
//   L324   TestMergeReceiverChunksOrdersByIndex
//   L353   TestValidateReceiverFinalSize
//   L367   TestCleanupReceiverChunksOnlyRemovesChunks
//   L395   TestFinalizeReceiverSessionIdempotent
//   L439   TestUpdateReceiverManifestSerializesConcurrentChunks
//   L485   TestHandleConnPublishesVerifiedChunkAtomically
//   L560   TestUpdateReceiverManifestRejectsIdentityChange
//   L573   TestUpdateReceiverManifestPreservesCorruptManifest
//   L590   TestValidateReceiverPathRejectsOutside
//   L598   TestParseFileSourceAcceptsUNCAndFileURL
//   L615   TestValidateOutputUnderReceiveReturnsRelativeDestination
//   L633   TestReceivePathsRejectSymbolicLinkEscape
//   L658   TestPrepareRemoteSendSourceFolderCreatesRelativeZip
//   L693   TestZipDirectoryRejectsSymbolicLinks
//   L708   TestReceiverTargetInfoForPullPublishesOutsideSession
//   L722   TestExtractZipSafeRejectsTraversal
// ======================= END NAV INDEX =======================

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Codyte/MultiSend/internal/config"
	"github.com/Codyte/MultiSend/internal/ifmonitor"
	"github.com/Codyte/MultiSend/internal/manifest"
	"github.com/Codyte/MultiSend/internal/proto"
)

func TestControlJSONClientsSendBearerToken(t *testing.T) {
	const token = "paired-node-secret"
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		if got := r.Header.Get("Authorization"); got != "Bearer "+token {
			t.Errorf("Authorization = %q, want bearer token", got)
		}
		writeJSON(w, map[string]any{"ok": true})
	}))
	defer server.Close()

	var postResult map[string]any
	if err := postJSON(server.URL, map[string]any{"value": 1}, &postResult, token); err != nil {
		t.Fatalf("postJSON: %v", err)
	}
	var getResult map[string]any
	if err := getJSON(server.URL, &getResult, token); err != nil {
		t.Fatalf("getJSON: %v", err)
	}
	if requests != 2 || postResult["ok"] != true || getResult["ok"] != true {
		t.Fatalf("unexpected client results: requests=%d post=%v get=%v", requests, postResult, getResult)
	}
}

func TestControlJSONClientRejectsOversizedResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(bytes.Repeat([]byte("x"), int(maxRemoteAPIResponseBodyBytes)+1))
	}))
	defer server.Close()

	var result map[string]any
	err := getJSON(server.URL, &result, "")
	if err == nil || !strings.Contains(err.Error(), "response body exceeds") {
		t.Fatalf("expected bounded response error, got %v", err)
	}
}

func TestControlAPITokenOnlyWhenAuthRequired(t *testing.T) {
	a := &app{cfg: config.Config{NodeSecret: " shared-secret "}}
	if got := a.controlAPIToken(); got != "" {
		t.Fatalf("token must not be sent when authentication is disabled: %q", got)
	}
	a.cfg.RequireAuth = true
	if got := a.controlAPIToken(); got != "shared-secret" {
		t.Fatalf("control token = %q", got)
	}
}

func TestControlAuthFailsClosedAndAcceptsCaseInsensitiveScheme(t *testing.T) {
	a := &app{cfg: config.Config{RequireAuth: true}}
	req := httptest.NewRequest(http.MethodGet, "/remote-jobs/job-1", nil)
	if a.controlAuthOK(req) {
		t.Fatal("authentication must fail closed when the configured secret is empty")
	}
	a.cfg.NodeSecret = "shared-secret"
	req.Header.Set("Authorization", "bearer shared-secret")
	if !a.controlAuthOK(req) {
		t.Fatal("Bearer authentication scheme must be case-insensitive")
	}
	req.Header.Set("Authorization", "Bearer wrong-secret")
	if a.controlAuthOK(req) {
		t.Fatal("wrong bearer token must be rejected")
	}
}

func TestRemoteSendPathAllowedResolvesSymlinkEscape(t *testing.T) {
	root := t.TempDir()
	inside := filepath.Join(root, "inside.bin")
	if err := os.WriteFile(inside, []byte("inside"), 0o600); err != nil {
		t.Fatal(err)
	}
	a := &app{cfg: config.Config{RemoteSendRoots: []string{root}}}
	if !a.isRemoteSendPathAllowed(inside) {
		t.Fatal("regular file inside shared root must be allowed")
	}

	outside := t.TempDir()
	secret := filepath.Join(outside, "secret.bin")
	if err := os.WriteFile(secret, []byte("secret"), 0o600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, "escape")
	if err := os.Symlink(outside, link); err != nil {
		if os.Getenv("MULTISEND_REQUIRE_SYMLINK_TEST") == "1" {
			t.Fatalf("directory symlink required for elevated validation: %v", err)
		}
		t.Skipf("directory symlink unavailable on this machine: %v", err)
	}
	if a.isRemoteSendPathAllowed(filepath.Join(link, "secret.bin")) {
		t.Fatal("shared-root symlink must not expose a file outside the resolved root")
	}
}

func TestStartSendRejectsInvalidChunkSizeBeforeCreatingJob(t *testing.T) {
	source := filepath.Join(t.TempDir(), "source.bin")
	if err := os.WriteFile(source, []byte("payload"), 0o600); err != nil {
		t.Fatal(err)
	}
	a := &app{jobsByID: map[string]*jobState{}}
	if _, err := a.startSend(source, "", "127.0.0.1:56200", sendTargetOptions{}, 1025); err == nil {
		t.Fatal("oversized chunk must be rejected")
	}
	if len(a.jobsByID) != 0 {
		t.Fatal("invalid request must not create a job")
	}
}

func TestSendRejectsLabReceivePathWhenLabFalse(t *testing.T) {
	a := &app{
		cfg: config.Config{
			ReceivePath: `C:\Users\Public\Downloads\MultiSend`,
			SelectedPorts: config.SelectedPorts{
				Transfer: 56200,
			},
		},
	}

	reqBody := map[string]any{
		"file_path":        `C:\tmp\file.bin`,
		"peer_address":     "127.0.0.1:56200",
		"lab":              false,
		"lab_receive_path": `C:\Users\Public\Downloads\MultiSend\_lab\receive`,
	}
	b, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/send", bytes.NewReader(b))
	rr := httptest.NewRecorder()

	a.send(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d body=%s", rr.Code, rr.Body.String())
	}
}

func TestSendRejectsRelativePathAndInvalidChunkSize(t *testing.T) {
	tests := []struct {
		name string
		body map[string]any
		want string
	}{
		{
			name: "relative path",
			body: map[string]any{"file_path": `relative\file.bin`, "peer_address": "127.0.0.1:56200"},
			want: "file_path must be absolute",
		},
		{
			name: "oversized chunk",
			body: map[string]any{"file_path": `C:\tmp\file.bin`, "peer_address": "127.0.0.1:56200", "chunk_size_mb": 1025},
			want: "chunk_size_mb must be between 0 and 1024",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := &app{cfg: config.Config{SelectedPorts: config.SelectedPorts{Transfer: 56200}}}
			body, _ := json.Marshal(tt.body)
			rr := httptest.NewRecorder()
			a.send(rr, httptest.NewRequest(http.MethodPost, "/send", bytes.NewReader(body)))
			if rr.Code != http.StatusBadRequest || !strings.Contains(rr.Body.String(), tt.want) {
				t.Fatalf("expected 400 containing %q, got %d body=%s", tt.want, rr.Code, rr.Body.String())
			}
		})
	}
}

func TestSendRejectsUnsafeCleanupPath(t *testing.T) {
	a := &app{cfg: config.Config{SelectedPorts: config.SelectedPorts{Transfer: 56200}}}
	body, _ := json.Marshal(map[string]any{
		"file_path":    filepath.Join(os.TempDir(), "source.bin"),
		"peer_address": "127.0.0.1:56200",
		"cleanup_path": os.TempDir(),
	})
	rr := httptest.NewRecorder()
	a.send(rr, httptest.NewRequest(http.MethodPost, "/send", bytes.NewReader(body)))
	if rr.Code != http.StatusBadRequest || !strings.Contains(rr.Body.String(), "invalid_cleanup_path") {
		t.Fatalf("expected unsafe cleanup rejection, got %d body=%s", rr.Code, rr.Body.String())
	}
}

func TestPullRejectsInvalidChunkSizeBeforeDiscovery(t *testing.T) {
	a := &app{}
	body := []byte(`{"source_url":"file://peer/share/file.bin","chunk_size_mb":1025}`)
	rr := httptest.NewRecorder()
	a.pullsHandler(rr, httptest.NewRequest(http.MethodPost, "/pulls", bytes.NewReader(body)))
	if rr.Code != http.StatusBadRequest || !strings.Contains(rr.Body.String(), "chunk_size_mb must be between 0 and 1024") {
		t.Fatalf("expected invalid chunk response, got %d body=%s", rr.Code, rr.Body.String())
	}
}

func TestInterfacesHandlerPayload(t *testing.T) {
	a := &app{
		cfg: config.Config{
			InterfacePolicy:    "ALL",
			InterfaceRefresh:   7,
			AllowNewInterfaces: true,
			IgnoreVirtual:      true,
			IgnoreVPN:          false,
			IgnoreLinkLocal:    true,
			AllowedIfTypes:     []string{"ethernet", "wifi"},
			ManualInterfaces:   []string{"Ethernet"},
			IgnoredInterfaces:  []string{"vEthernet"},
		},
	}
	origDetect := detectUsableInterfaces
	defer func() { detectUsableInterfaces = origDetect }()
	detectUsableInterfaces = func(cfg config.Config) []ifmonitor.InterfaceInfo {
		return []ifmonitor.InterfaceInfo{
			{Name: "Ethernet", Type: "ethernet", Usable: true},
			{Name: "Wi-Fi", Type: "wifi", Usable: false},
		}
	}

	req := httptest.NewRequest(http.MethodGet, "/interfaces", nil)
	rr := httptest.NewRecorder()
	a.interfacesHandler(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rr.Code, rr.Body.String())
	}
	var got map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got["policy"] != "all" {
		t.Fatalf("expected normalized policy all, got %#v", got["policy"])
	}
	if got["supported_policies"] == nil {
		t.Fatal("expected supported policies")
	}
	if got["usable_interfaces"] == nil {
		t.Fatal("expected usable interfaces field")
	}
}

func TestWriteAPIErrorReturnsJSON(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/pulls", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	rr := httptest.NewRecorder()

	writeAPIErrorDetail(rr, req, http.StatusBadRequest, "bad_json", "bad json", "unexpected EOF")

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d body=%s", rr.Code, rr.Body.String())
	}
	if ct := rr.Header().Get("Content-Type"); !strings.Contains(ct, "application/json") {
		t.Fatalf("expected json content-type, got %q", ct)
	}
	var got apiErrorResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.Error != "bad_json" || got.Message != "bad json" || got.Detail != "unexpected EOF" || got.RequestID == "" {
		t.Fatalf("unexpected error response: %+v", got)
	}
}

func TestFormatRemoteAPIErrorIncludesRequestID(t *testing.T) {
	body := []byte(`{"error":"file_path_required","message":"file_path is required","detail":"missing","status":400,"request_id":"req-123"}`)
	got := formatRemoteAPIError(http.StatusBadRequest, body)
	for _, want := range []string{"status=400", "code=file_path_required", "message=file_path is required", "detail=missing", "request_id=req-123"} {
		if !strings.Contains(got, want) {
			t.Fatalf("formatted error %q does not contain %q", got, want)
		}
	}
}

func TestMergeReceiverChunksOrdersByIndex(t *testing.T) {
	sessionDir := t.TempDir()
	mf := &manifest.Manifest{
		FileName:   "payload.bin",
		TotalBytes: 6,
		Chunks: []manifest.ChunkState{
			{Index: 1, Size: 3, Status: manifest.StatusDone},
			{Index: 0, Size: 3, Status: manifest.StatusDone},
		},
	}
	if err := os.WriteFile(filepath.Join(sessionDir, "payload.bin.chunk000001"), []byte("abc"), 0o644); err != nil {
		t.Fatalf("write part1: %v", err)
	}
	if err := os.WriteFile(filepath.Join(sessionDir, "payload.bin.chunk000002"), []byte("def"), 0o644); err != nil {
		t.Fatalf("write part2: %v", err)
	}
	finalPath := filepath.Join(sessionDir, "payload.bin")
	if err := mergeReceiverChunks(finalPath, sessionDir, mf); err != nil {
		t.Fatalf("merge failed: %v", err)
	}
	got, err := os.ReadFile(finalPath)
	if err != nil {
		t.Fatalf("read final: %v", err)
	}
	if string(got) != "abcdef" {
		t.Fatalf("unexpected merge order: %q", string(got))
	}
}

func TestValidateReceiverFinalSize(t *testing.T) {
	dir := t.TempDir()
	finalPath := filepath.Join(dir, "payload.bin")
	if err := os.WriteFile(finalPath, []byte("abc"), 0o644); err != nil {
		t.Fatalf("write final: %v", err)
	}
	if err := validateReceiverFinalSize(finalPath, 3); err != nil {
		t.Fatalf("expected size match: %v", err)
	}
	if err := validateReceiverFinalSize(finalPath, 4); err == nil {
		t.Fatal("expected size mismatch")
	}
}

func TestCleanupReceiverChunksOnlyRemovesChunks(t *testing.T) {
	sessionDir := t.TempDir()
	chunkPath := filepath.Join(sessionDir, "payload.bin.chunk000001")
	manifestPath := filepath.Join(sessionDir, "manifest.json")
	finalPath := filepath.Join(sessionDir, "payload.bin")
	if err := os.WriteFile(chunkPath, []byte("abc"), 0o644); err != nil {
		t.Fatalf("write chunk: %v", err)
	}
	if err := os.WriteFile(manifestPath, []byte("{}"), 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
	if err := os.WriteFile(finalPath, []byte("abc"), 0o644); err != nil {
		t.Fatalf("write final: %v", err)
	}
	if err := cleanupReceiverChunks(sessionDir); err != nil {
		t.Fatalf("cleanup failed: %v", err)
	}
	if _, err := os.Stat(chunkPath); !os.IsNotExist(err) {
		t.Fatalf("chunk should be removed, err=%v", err)
	}
	if _, err := os.Stat(manifestPath); err != nil {
		t.Fatalf("manifest should remain: %v", err)
	}
	if _, err := os.Stat(finalPath); err != nil {
		t.Fatalf("final should remain: %v", err)
	}
}

func TestFinalizeReceiverSessionIdempotent(t *testing.T) {
	sessionDir := t.TempDir()
	mf := &manifest.Manifest{
		FileName:   "payload.bin",
		TotalBytes: 6,
		Chunks: []manifest.ChunkState{
			{Index: 0, Size: 3, Status: manifest.StatusDone},
			{Index: 1, Size: 3, Status: manifest.StatusDone},
		},
	}
	mfPath := filepath.Join(sessionDir, "manifest.json")
	if err := manifest.SaveAtomic(mfPath, mf); err != nil {
		t.Fatalf("save manifest: %v", err)
	}
	if err := os.WriteFile(filepath.Join(sessionDir, "payload.bin.chunk000001"), []byte("abc"), 0o644); err != nil {
		t.Fatalf("write part1: %v", err)
	}
	if err := os.WriteFile(filepath.Join(sessionDir, "payload.bin.chunk000002"), []byte("def"), 0o644); err != nil {
		t.Fatalf("write part2: %v", err)
	}
	a := &app{receiverSessions: map[string]*receiverSessionState{}}
	a.receiverSessions[sessionDir] = &receiverSessionState{session: receiverSession{SessionDir: sessionDir, MergeStatus: "pending"}}
	a.finalizeReceiverSession(sessionDir)
	a.finalizeReceiverSession(sessionDir)
	finalPath := filepath.Join(sessionDir, "payload.bin")
	got, err := os.ReadFile(finalPath)
	if err != nil {
		t.Fatalf("read final: %v", err)
	}
	if string(got) != "abcdef" {
		t.Fatalf("unexpected final content: %q", string(got))
	}
	if _, err := os.Stat(filepath.Join(sessionDir, "payload.bin.chunk000001")); !os.IsNotExist(err) {
		t.Fatalf("expected chunk cleanup, err=%v", err)
	}
	sess := a.getReceiverSession(sessionDir)
	if sess.MergeStatus != "done" || sess.CleanupStatus != "done" {
		t.Fatalf("unexpected session state: %+v", sess)
	}
	if sess.FinalOutputPath != finalPath {
		t.Fatalf("unexpected final output path: %+v", sess)
	}
}

func TestUpdateReceiverManifestSerializesConcurrentChunks(t *testing.T) {
	t.Parallel()
	sessionDir := t.TempDir()
	a := &app{receiverSessions: make(map[string]*receiverSessionState)}
	const chunkCount = 64
	const chunkSize = int64(1024)
	base := proto.Header{
		TransferID: "tx-concurrent",
		FileName:   "concurrent.bin",
		TotalBytes: chunkCount * chunkSize,
		PartSize:   chunkSize,
	}
	a.ensureReceiverSession(sessionDir, base.FileName, base)

	errs := make(chan error, chunkCount)
	var wg sync.WaitGroup
	for i := int64(0); i < chunkCount; i++ {
		wg.Add(1)
		go func(index int64) {
			defer wg.Done()
			h := base
			h.ChunkIndex = index
			h.Offset = index * chunkSize
			errs <- a.updateReceiverManifest(sessionDir, base.FileName, h, chunkSize)
		}(i)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatalf("updateReceiverManifest: %v", err)
		}
	}

	mf, err := manifest.Load(filepath.Join(sessionDir, "manifest.json"))
	if err != nil {
		t.Fatalf("load manifest: %v", err)
	}
	if got := len(mf.Chunks); got != chunkCount {
		t.Fatalf("chunk count = %d, want %d", got, chunkCount)
	}
	if got := countManifestChunksByStatus(mf, manifest.StatusDone); got != chunkCount {
		t.Fatalf("done count = %d, want %d", got, chunkCount)
	}
}

func TestHandleConnPublishesVerifiedChunkAtomically(t *testing.T) {
	payload := []byte("abcdef")
	digest := sha256.Sum256(payload)
	header := proto.Header{
		TransferID:  "tx-atomic",
		FileName:    "payload.bin",
		PartIndex:   1,
		TotalParts:  2,
		PartSize:    int64(len(payload)),
		ChunkIndex:  0,
		Offset:      0,
		TotalBytes:  int64(len(payload) * 2),
		ChunkSHA256: hex.EncodeToString(digest[:]),
	}
	a := &app{cfg: config.Config{ReceivePath: t.TempDir()}, receiverSessions: map[string]*receiverSessionState{}}

	send := func() {
		server, client := net.Pipe()
		done := make(chan struct{})
		go func() {
			a.handleConn(server)
			close(done)
		}()
		if err := client.SetDeadline(time.Now().Add(2 * time.Second)); err != nil {
			t.Fatalf("set deadline: %v", err)
		}
		if err := proto.WriteHeader(client, header); err != nil {
			t.Fatalf("write header: %v", err)
		}
		if _, err := client.Write(payload); err != nil {
			t.Fatalf("write payload: %v", err)
		}
		ack, err := proto.ReadChunkAck(client)
		if err != nil {
			t.Fatalf("read ack: %v", err)
		}
		if ack.Status != "done" || ack.BytesReceived != int64(len(payload)) {
			t.Fatalf("unexpected ack: %+v", ack)
		}
		_ = client.Close()
		<-done
	}

	send()
	sessionDir := filepath.Join(a.cfg.ReceivePath, "payload.bin_tx-atomic")
	target := filepath.Join(sessionDir, "payload.bin.chunk000001")
	if got, err := os.ReadFile(target); err != nil || !bytes.Equal(got, payload) {
		t.Fatalf("published chunk mismatch: data=%q err=%v", got, err)
	}
	if entries, err := filepath.Glob(filepath.Join(sessionDir, ".incoming-*.tmp")); err != nil || len(entries) != 0 {
		t.Fatalf("temporary chunks remain: entries=%v err=%v", entries, err)
	}
	if err := os.WriteFile(target, []byte("xxxxxx"), 0o600); err != nil {
		t.Fatalf("corrupt chunk fixture: %v", err)
	}
	if receiverChunkMatches(target, header) {
		t.Fatal("same-size corrupt chunk must not be accepted")
	}
	send()
	if got, err := os.ReadFile(target); err != nil || !bytes.Equal(got, payload) {
		t.Fatalf("retry did not replace corrupt chunk: data=%q err=%v", got, err)
	}

	a.receiverSessionsMu.RLock()
	state := a.receiverSessions[sessionDir]
	a.receiverSessionsMu.RUnlock()
	if state != nil {
		state.mu.Lock()
		if state.timer != nil {
			state.timer.Stop()
		}
		state.mu.Unlock()
	}
}

func TestUpdateReceiverManifestRejectsIdentityChange(t *testing.T) {
	sessionDir := t.TempDir()
	a := &app{receiverSessions: map[string]*receiverSessionState{}}
	header := proto.Header{TransferID: "tx-1", FileName: "payload.bin", PartIndex: 1, TotalParts: 1, PartSize: 3, ChunkIndex: 0, TotalBytes: 3}
	a.ensureReceiverSession(sessionDir, header.FileName, header)
	if err := manifest.SaveAtomic(filepath.Join(sessionDir, "manifest.json"), &manifest.Manifest{TransferID: "other", FileName: header.FileName, TotalBytes: header.TotalBytes}); err != nil {
		t.Fatalf("save fixture: %v", err)
	}
	if err := a.updateReceiverManifest(sessionDir, header.FileName, header, header.PartSize); err == nil {
		t.Fatal("expected manifest identity mismatch")
	}
}

func TestUpdateReceiverManifestPreservesCorruptManifest(t *testing.T) {
	sessionDir := t.TempDir()
	a := &app{receiverSessions: map[string]*receiverSessionState{}}
	header := proto.Header{TransferID: "tx-1", FileName: "payload.bin", PartIndex: 1, TotalParts: 1, PartSize: 3, ChunkIndex: 0, TotalBytes: 3}
	a.ensureReceiverSession(sessionDir, header.FileName, header)
	manifestPath := filepath.Join(sessionDir, "manifest.json")
	if err := os.WriteFile(manifestPath, []byte("{"), 0o600); err != nil {
		t.Fatalf("write corrupt fixture: %v", err)
	}
	if err := a.updateReceiverManifest(sessionDir, header.FileName, header, header.PartSize); err == nil {
		t.Fatal("expected corrupt manifest error")
	}
	if raw, err := os.ReadFile(manifestPath); err != nil || string(raw) != "{" {
		t.Fatalf("corrupt manifest was overwritten: data=%q err=%v", raw, err)
	}
}

func TestValidateReceiverPathRejectsOutside(t *testing.T) {
	root := filepath.Join(t.TempDir(), "root")
	outside := filepath.Join(t.TempDir(), "other", "file.bin")
	if err := validateReceiverPath(root, outside); err == nil {
		t.Fatal("expected path guard rejection")
	}
}

func TestParseFileSourceAcceptsUNCAndFileURL(t *testing.T) {
	host, sourcePath, err := parseFileSource(`\\SRVTECHNEO\Arquivos\Projetos\file.txt`)
	if err != nil {
		t.Fatalf("parse UNC: %v", err)
	}
	if host != "SRVTECHNEO" || sourcePath != `\\SRVTECHNEO\Arquivos\Projetos\file.txt` {
		t.Fatalf("unexpected UNC parse host=%q path=%q", host, sourcePath)
	}
	host, sourcePath, err = parseFileSource(`file://srvtechneo/Arquivos/Projetos/file.txt`)
	if err != nil {
		t.Fatalf("parse file URL: %v", err)
	}
	if host != "srvtechneo" || sourcePath != `\\srvtechneo\Arquivos\Projetos\file.txt` {
		t.Fatalf("unexpected file URL parse host=%q path=%q", host, sourcePath)
	}
}

func TestValidateOutputUnderReceiveReturnsRelativeDestination(t *testing.T) {
	root := filepath.Join(t.TempDir(), "MultiSend")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatalf("mkdir receive root: %v", err)
	}
	output := filepath.Join(root, "Downloads", "LAN")
	rel, err := validateOutputUnderReceive(output, root)
	if err != nil {
		t.Fatalf("validate output: %v", err)
	}
	if rel != filepath.Join("Downloads", "LAN") {
		t.Fatalf("unexpected relative path: %q", rel)
	}
	if _, err := validateOutputUnderReceive(filepath.Join(t.TempDir(), "outside"), root); err == nil {
		t.Fatal("expected outside output to be rejected")
	}
}

func TestReceivePathsRejectSymbolicLinkEscape(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	link := filepath.Join(root, "linked")
	if err := os.Symlink(outside, link); err != nil {
		t.Skipf("symbolic links unavailable: %v", err)
	}
	for name, validate := range map[string]func() error{
		"output": func() error {
			_, err := validateOutputUnderReceive(filepath.Join(link, "new"), root)
			return err
		},
		"receive relative": func() error {
			_, err := resolveReceiveRelRoot(root, filepath.Join("linked", "new"))
			return err
		},
	} {
		t.Run(name, func(t *testing.T) {
			if err := validate(); err == nil {
				t.Fatal("expected symbolic-link escape rejection")
			}
		})
	}
}

func TestPrepareRemoteSendSourceFolderCreatesRelativeZip(t *testing.T) {
	src := filepath.Join(t.TempDir(), "folder")
	if err := os.MkdirAll(filepath.Join(src, "sub"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(src, "sub", "file.txt"), []byte("payload"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	zipPath, prepared, err := prepareRemoteSendSource(src, "extract")
	if err != nil {
		t.Fatalf("prepare: %v", err)
	}
	defer os.RemoveAll(prepared.CleanupPath)
	if prepared.SourceType != "folder" || prepared.FolderResult != "extract" || prepared.PublishName != "folder.zip" {
		t.Fatalf("unexpected prepared source: %+v", prepared)
	}
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		t.Fatalf("open zip: %v", err)
	}
	defer r.Close()
	found := false
	for _, f := range r.File {
		if f.Name == "sub/file.txt" {
			found = true
		}
		if filepath.IsAbs(f.Name) || strings.Contains(f.Name, "..") {
			t.Fatalf("zip entry is not relative and safe: %q", f.Name)
		}
	}
	if !found {
		t.Fatal("expected relative file in zip")
	}
}

func TestZipDirectoryRejectsSymbolicLinks(t *testing.T) {
	source := t.TempDir()
	outside := filepath.Join(t.TempDir(), "outside.txt")
	if err := os.WriteFile(outside, []byte("secret"), 0o600); err != nil {
		t.Fatalf("write outside fixture: %v", err)
	}
	if err := os.Symlink(outside, filepath.Join(source, "linked.txt")); err != nil {
		t.Skipf("symbolic links unavailable: %v", err)
	}
	err := zipDirectory(source, filepath.Join(t.TempDir(), "archive.zip"))
	if err == nil || !strings.Contains(err.Error(), "symbolic link is not supported") {
		t.Fatalf("expected symbolic-link rejection, got %v", err)
	}
}

func TestReceiverTargetInfoForPullPublishesOutsideSession(t *testing.T) {
	root := filepath.Join(t.TempDir(), "MultiSend")
	sessionDir := filepath.Join(root, "Downloads", "_pull_sessions", "payload_job")
	h := proto.Header{ReceiveRelPath: "Downloads", PublishName: "payload.bin"}
	target := receiverTargetInfo(sessionDir, "ignored.bin", h)
	want := filepath.Join(root, "Downloads", "payload.bin")
	if target.Type != "file" || target.Path != want {
		t.Fatalf("unexpected target: %+v want path %s", target, want)
	}
	if err := validateReceiverPath(sessionDir, target.Path); err != nil {
		t.Fatalf("expected pull publish path to be allowed: %v", err)
	}
}

func TestExtractZipSafeRejectsTraversal(t *testing.T) {
	dir := t.TempDir()
	zipPath := filepath.Join(dir, "bad.zip")
	f, err := os.Create(zipPath)
	if err != nil {
		t.Fatalf("create zip: %v", err)
	}
	zw := zip.NewWriter(f)
	w, err := zw.Create("../evil.txt")
	if err != nil {
		t.Fatalf("create entry: %v", err)
	}
	_, _ = w.Write([]byte("bad"))
	if err := zw.Close(); err != nil {
		t.Fatalf("close zip writer: %v", err)
	}
	if err := f.Close(); err != nil {
		t.Fatalf("close zip file: %v", err)
	}
	if err := extractZipSafe(zipPath, filepath.Join(dir, "out")); err == nil {
		t.Fatal("expected zip traversal to be rejected")
	}
}
