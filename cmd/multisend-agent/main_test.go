package main

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"lab/multinet/internal/config"
	"lab/multinet/internal/ifmonitor"
	"lab/multinet/internal/manifest"
	"lab/multinet/internal/proto"
)

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
