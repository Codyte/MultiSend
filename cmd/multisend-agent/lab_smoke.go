package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"lab/multinet/internal/config"
)

type labRuntimeState struct {
	LocalAPIPort int `json:"local_api_port"`
	TransferPort int `json:"transfer_port"`
}

type smokeJob struct {
	ID              string  `json:"id"`
	Status          string  `json:"status"`
	Message         string  `json:"message"`
	LastError       string  `json:"last_error"`
	Percent         float64 `json:"percent"`
	BytesSent       int64   `json:"bytes_sent"`
	TotalBytes      int64   `json:"total_bytes"`
	MbpsNow         float64 `json:"mbps_now"`
	Mbps30s         float64 `json:"mbps_30s"`
	ChunksTotal     int64   `json:"chunks_total"`
	ChunksDone      int64   `json:"chunks_done"`
	ChunksFailed    int64   `json:"chunks_failed"`
	ChunksPending   int64   `json:"chunks_pending"`
	ChunksSending   int64   `json:"chunks_sending"`
	ResumeSupported bool    `json:"resume_supported"`
	ManifestPath    string  `json:"manifest_path"`
	FailedChunks    []struct {
		Index     int64  `json:"index"`
		Attempts  int    `json:"attempts"`
		LastError string `json:"last_error"`
		Channel   string `json:"channel"`
	} `json:"failed_chunks"`
}

type smokeResult struct {
	pass int
	warn int
	fail int
}

func (r *smokeResult) passf(ok bool) string {
	if ok {
		r.pass++
		return "PASS"
	}
	r.fail++
	return "FAIL"
}
func (r *smokeResult) warnf(msg string) { fmt.Println("WARN:", msg); r.warn++ }

func runLabSmoke() int {
	res := &smokeResult{}
	cfg, _, ok, err := config.LoadIfExists()
	if err != nil || !ok {
		fmt.Println("FAIL: config is not available:", err)
		return 1
	}
	rt, rtErr := readRuntime()
	fmt.Println("=== MultiSend Lab Smoke ===")
	fmt.Println()
	fmt.Println("[Environment]")
	fmt.Printf("%s runtime.json\n", res.passf(rtErr == nil))
	if rtErr != nil {
		fmt.Println("error:", rtErr)
	}
	if rtErr != nil {
		fmt.Printf("%s local API\n", res.passf(false))
		fmt.Println("agent local API is not running")
		printLabSummary(res)
		return 1
	}

	apiBase := fmt.Sprintf("http://127.0.0.1:%d", rt.LocalAPIPort)
	healthOK := apiGET(apiBase+"/health", nil) == nil
	fmt.Printf("%s local API\n", res.passf(healthOK))
	if !healthOK {
		fmt.Println("agent local API is not running")
		printLabSummary(res)
		return 1
	}
	transferOpen := canDial(rt.TransferPort)
	fmt.Printf("%s transfer port\n", res.passf(transferOpen))

	receivePath := cfg.ReceivePath
	if strings.TrimSpace(receivePath) == "" {
		fmt.Printf("%s lab folders\n", res.passf(false))
		printLabSummary(res)
		return 1
	}
	labRoot := filepath.Join(receivePath, "_lab")
	sendDir := filepath.Join(labRoot, "send")
	recvDir := filepath.Join(labRoot, "receive")
	manDir := filepath.Join(labRoot, "manifests")
	logDir := filepath.Join(labRoot, "logs")
	tmpDir := filepath.Join(labRoot, "tmp")
	foldersOK := true
	for _, p := range []string{labRoot, sendDir, recvDir, manDir, logDir, tmpDir} {
		if err := os.MkdirAll(p, 0o755); err != nil {
			foldersOK = false
		}
	}
	fmt.Printf("%s lab folders\n", res.passf(foldersOK))

	fmt.Println()
	fmt.Println("[Test file]")
	testFile := filepath.Join(sendDir, "lab-test.bin")
	size := int64(256 * 1024 * 1024)
	reused, err := ensureDeterministicFile(testFile, size)
	if err != nil {
		fmt.Println("path:", testFile)
		fmt.Println("size:", size)
		fmt.Println("reused/recreated: error")
		fmt.Println("error:", err)
		printLabSummary(res)
		return 1
	}
	fmt.Println("path:", testFile)
	fmt.Println("size:", size)
	if reused {
		fmt.Println("reused/recreated: reused")
	} else {
		fmt.Println("reused/recreated: recreated")
	}
	now := time.Now()
	_ = os.Chtimes(testFile, now, now)

	sendReq := map[string]any{
		"file_path":        testFile,
		"peer_address":     fmt.Sprintf("127.0.0.1:%d", rt.TransferPort),
		"lab":              true,
		"lab_receive_path": recvDir,
		"chunk_size_mb":    4,
	}
	var sendResp map[string]string
	if err := apiPOST(apiBase+"/send", sendReq, &sendResp); err != nil {
		fmt.Println("FAIL: send failed:", err)
		printLabSummary(res)
		return 1
	}
	jobID := sendResp["job_id"]
	if jobID == "" {
		fmt.Println("FAIL: send did not return job_id")
		printLabSummary(res)
		return 1
	}

	fmt.Println()
	fmt.Println("[Send]")
	fmt.Println("job_id:", jobID)
	fmt.Println("transfer_port:", rt.TransferPort)
	fmt.Println("local_api_port:", rt.LocalAPIPort)
	fmt.Println("peer_address:", fmt.Sprintf("127.0.0.1:%d", rt.TransferPort))

	var beforeCancel smokeJob
	canceled := false
	var zeroFailedSince time.Time
	deadline := time.Now().Add(4 * time.Minute)
	for time.Now().Before(deadline) {
		time.Sleep(1 * time.Second)
		var st smokeJob
		if err := apiGET(apiBase+"/jobs/"+jobID, &st); err != nil {
			fmt.Println("FAIL: get job:", err)
			printLabSummary(res)
			return 1
		}
		beforeCancel = st
		fmt.Printf("progress: %.2f%% %d/%d chunks(done/pending/failed/sending)=%d/%d/%d/%d mbps_now=%.2f mbps_30s=%.2f\n",
			st.Percent, st.BytesSent, st.TotalBytes, st.ChunksDone, st.ChunksPending, st.ChunksFailed, st.ChunksSending, st.MbpsNow, st.Mbps30s)
		if st.Message != "" {
			fmt.Println("job.message:", st.Message)
		}
		if st.ManifestPath != "" {
			fmt.Println("manifest_path:", st.ManifestPath)
		}
		if st.ChunksFailed > 0 && st.BytesSent == 0 {
			if zeroFailedSince.IsZero() {
				zeroFailedSince = time.Now()
			}
			if time.Since(zeroFailedSince) > 5*time.Second {
				fmt.Println("FAIL: chunks_failed > 0 with bytes_sent == 0 for more than 5 seconds")
				printFailedChunkDetails(st.ManifestPath)
				res.fail++
				printLabSummary(res)
				return 1
			}
		} else {
			zeroFailedSince = time.Time{}
		}
		if st.Status == "done" {
			res.warnf("job finished before cancel; cancel validation skipped")
			break
		}
		if st.ChunksDone >= 1 || st.Percent >= 10 {
			_ = apiPOST(apiBase+"/jobs/"+jobID+"/cancel", map[string]any{}, nil)
			canceled = true
			break
		}
	}

	fmt.Println()
	fmt.Println("[Cancel]")
	cancelStatus := beforeCancel.Status
	if canceled {
		for i := 0; i < 60; i++ {
			time.Sleep(1 * time.Second)
			var st smokeJob
			if err := apiGET(apiBase+"/jobs/"+jobID, &st); err != nil {
				break
			}
			beforeCancel = st
			if st.Status == "canceled" || st.Status == "done" || st.Status == "failed" {
				break
			}
		}
		cancelStatus = beforeCancel.Status
	}
	fmt.Println("status:", cancelStatus)
	fmt.Println("chunks_done:", beforeCancel.ChunksDone)
	fmt.Println("chunks_total:", beforeCancel.ChunksTotal)
	fmt.Printf("percent: %.2f\n", beforeCancel.Percent)
	if beforeCancel.ManifestPath != "" {
		fmt.Println("manifest_path:", beforeCancel.ManifestPath)
	}
	cancelOK := true
	if canceled {
		if cancelStatus != "canceled" {
			cancelOK = false
		}
		if beforeCancel.ManifestPath == "" {
			cancelOK = false
		} else if _, err := os.Stat(beforeCancel.ManifestPath); err != nil {
			cancelOK = false
		}
	}
	if !canceled {
		cancelOK = false
	}
	fmt.Printf("%s cancel validation\n", res.passf(cancelOK))

	fmt.Println()
	fmt.Println("[Resume]")
	if err := apiPOST(apiBase+"/jobs/"+jobID+"/resume", map[string]any{}, nil); err != nil {
		fmt.Println("status: failed")
		fmt.Println("error:", err)
		res.fail++
		printLabSummary(res)
		return 1
	}
	var doneState smokeJob
	doneDeadline := time.Now().Add(8 * time.Minute)
	for time.Now().Before(doneDeadline) {
		time.Sleep(1 * time.Second)
		var st smokeJob
		if err := apiGET(apiBase+"/jobs/"+jobID, &st); err != nil {
			fmt.Println("FAIL: get job during resume:", err)
			printLabSummary(res)
			return 1
		}
		doneState = st
		if st.Status == "done" || st.Status == "failed" || st.Status == "canceled" {
			break
		}
	}
	fmt.Println("status:", doneState.Status)
	fmt.Println("chunks_done:", doneState.ChunksDone)
	fmt.Println("chunks_total:", doneState.ChunksTotal)
	fmt.Println("chunks_failed:", doneState.ChunksFailed)
	fmt.Println("chunks_pending:", doneState.ChunksPending)
	resumeOK := doneState.Status == "done" &&
		doneState.ChunksTotal > 1 &&
		doneState.ChunksDone == doneState.ChunksTotal &&
		doneState.ChunksFailed == 0 &&
		doneState.ChunksPending == 0 &&
		doneState.ResumeSupported &&
		doneState.ManifestPath != ""
	if doneState.ManifestPath != "" {
		if _, err := os.Stat(doneState.ManifestPath); err != nil {
			resumeOK = false
		}
	}
	fmt.Printf("%s resume validation\n", res.passf(resumeOK))

	fmt.Println()
	fmt.Println("[Receiver]")
	sessionPath, recvManifest, chunkCount := findLatestSession(recvDir)
	fmt.Println("receive/session path:", sessionPath)
	fmt.Println("receiver manifest path:", recvManifest)
	fmt.Println("chunk files count:", chunkCount)

	printLabSummary(res)
	if res.fail > 0 {
		return 1
	}
	return 0
}

func readRuntime() (labRuntimeState, error) {
	local := os.Getenv("LOCALAPPDATA")
	if local == "" {
		return labRuntimeState{}, fmt.Errorf("LOCALAPPDATA is empty")
	}
	p := filepath.Join(local, "MultiSend", "runtime.json")
	b, err := os.ReadFile(p)
	if err != nil {
		return labRuntimeState{}, err
	}
	var rt labRuntimeState
	if err := json.Unmarshal(b, &rt); err != nil {
		return labRuntimeState{}, err
	}
	if rt.LocalAPIPort <= 0 || rt.TransferPort <= 0 {
		cfg, _, ok, _ := config.LoadIfExists()
		if ok {
			if rt.LocalAPIPort <= 0 {
				rt.LocalAPIPort = cfg.SelectedPorts.LocalAPI
			}
			if rt.TransferPort <= 0 {
				rt.TransferPort = cfg.SelectedPorts.Transfer
			}
		}
		if rt.LocalAPIPort <= 0 || rt.TransferPort <= 0 {
			return labRuntimeState{}, fmt.Errorf("runtime ports are invalid")
		}
	}
	return rt, nil
}

func ensureDeterministicFile(path string, size int64) (bool, error) {
	if st, err := os.Stat(path); err == nil {
		if st.Size() == size {
			return true, nil
		}
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
	if err != nil {
		return false, err
	}
	defer f.Close()
	buf := make([]byte, 1024*1024)
	for i := range buf {
		buf[i] = byte((i*31 + 7) % 251)
	}
	var written int64
	for written < size {
		n := int64(len(buf))
		if size-written < n {
			n = size - written
		}
		if _, err := f.Write(buf[:n]); err != nil {
			return false, err
		}
		written += n
	}
	return false, nil
}

func apiGET(url string, out any) error {
	resp, err := http.Get(url) // #nosec G107 local fixed URL
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("status %d: %s", resp.StatusCode, strings.TrimSpace(string(b)))
	}
	if out == nil {
		return nil
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

func apiPOST(url string, payload any, out any) error {
	b, _ := json.Marshal(payload)
	resp, err := http.Post(url, "application/json", bytes.NewReader(b)) // #nosec G107 local fixed URL
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	if out == nil {
		return nil
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

func canDial(port int) bool {
	c, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), 700*time.Millisecond)
	if err != nil {
		return false
	}
	_ = c.Close()
	return true
}

func findLatestSession(root string) (string, string, int) {
	entries, err := os.ReadDir(root)
	if err != nil {
		return "", "", 0
	}
	var latest string
	var latestTime time.Time
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		if info.ModTime().After(latestTime) {
			latestTime = info.ModTime()
			latest = filepath.Join(root, e.Name())
		}
	}
	if latest == "" {
		return "", "", 0
	}
	files, _ := os.ReadDir(latest)
	count := 0
	manifestPath := ""
	for _, f := range files {
		n := strings.ToLower(f.Name())
		if strings.HasSuffix(n, ".chunk001") || strings.Contains(n, ".chunk") {
			count++
		}
		if n == "manifest.json" {
			manifestPath = filepath.Join(latest, f.Name())
		}
	}
	return latest, manifestPath, count
}

func printLabSummary(res *smokeResult) {
	fmt.Println()
	fmt.Println("[Summary]")
	fmt.Println("PASS count:", res.pass)
	fmt.Println("WARN count:", res.warn)
	fmt.Println("FAIL count:", res.fail)
	result := "PASS"
	if res.fail > 0 {
		result = "FAIL"
	}
	fmt.Println("result:", result)
}

func printFailedChunkDetails(manifestPath string) {
	if strings.TrimSpace(manifestPath) == "" {
		fmt.Println("failed chunks detail: manifest_path is empty")
		return
	}
	b, err := os.ReadFile(manifestPath)
	if err != nil {
		fmt.Println("failed chunks detail read error:", err)
		return
	}
	var doc struct {
		Chunks []struct {
			Index     int64  `json:"index"`
			Status    string `json:"status"`
			Attempts  int    `json:"attempts"`
			LastError string `json:"last_error"`
			Channel   string `json:"channel"`
		} `json:"chunks"`
	}
	if err := json.Unmarshal(b, &doc); err != nil {
		fmt.Println("failed chunks detail parse error:", err)
		return
	}
	fmt.Println("failed chunks detail:")
	for _, ch := range doc.Chunks {
		if ch.Status != "failed" {
			continue
		}
		fmt.Printf("index=%d status=%s attempts=%d last_error=%q channel=%q\n", ch.Index, ch.Status, ch.Attempts, ch.LastError, ch.Channel)
	}
}
