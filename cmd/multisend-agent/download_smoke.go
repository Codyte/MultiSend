package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"lab/multinet/internal/config"
)

func runDownloadSmoke() int {
	started := time.Now()
	globalDeadline := started.Add(5 * time.Minute)
	cfg, _, ok, err := config.LoadIfExists()
	if err != nil || !ok {
		fmt.Println("FAIL: config is not available:", err)
		return 1
	}
	rt, err := readRuntime()
	if err != nil || rt.LocalAPIPort <= 0 {
		rt.LocalAPIPort = cfg.SelectedPorts.LocalAPI
		if rt.LocalAPIPort <= 0 {
			fmt.Println("FAIL: runtime:", err)
			return 1
		}
	}
	api := fmt.Sprintf("http://127.0.0.1:%d", rt.LocalAPIPort)
	if err := apiGET(api+"/health", nil); err != nil {
		fmt.Println("FAIL: agent local API is not running")
		return 1
	}
	base := filepath.Join(cfg.ReceivePath, "_lab", "download")
	srcDir := filepath.Join(base, "src")
	outDir := filepath.Join(base, "out")
	_ = os.MkdirAll(srcDir, 0o755)
	_ = os.MkdirAll(outDir, 0o755)
	src := filepath.Join(srcDir, "download-test.bin")
	const size = 256 * 1024 * 1024
	if _, err := ensureDeterministicFile(src, size); err != nil {
		fmt.Println("FAIL:", err)
		return 1
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		fmt.Println("FAIL:", err)
		return 1
	}
	defer ln.Close()
	url := "http://" + ln.Addr().String() + "/file"
	srv := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f, err := os.Open(src)
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		defer f.Close()
		st, _ := f.Stat()
		w.Header().Set("Accept-Ranges", "bytes")
		w.Header().Set("Content-Length", strconv.FormatInt(st.Size(), 10))
		if r.Method == http.MethodHead {
			return
		}
		rg := r.Header.Get("Range")
		if rg == "" {
			http.Error(w, "range required", http.StatusRequestedRangeNotSatisfiable)
			return
		}
		var s, e int64
		if _, err := fmt.Sscanf(rg, "bytes=%d-%d", &s, &e); err != nil {
			http.Error(w, "bad range", http.StatusRequestedRangeNotSatisfiable)
			return
		}
		if e < s || s < 0 || e >= st.Size() {
			http.Error(w, "invalid range", http.StatusRequestedRangeNotSatisfiable)
			return
		}
		w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", s, e, st.Size()))
		w.WriteHeader(http.StatusPartialContent)
		rd := io.NewSectionReader(f, s, e-s+1)
		buf := make([]byte, 64*1024)
		remaining := e - s + 1
		for remaining > 0 {
			n := int64(len(buf))
			if remaining < n {
				n = remaining
			}
			rn, rerr := rd.Read(buf[:n])
			if rn > 0 {
				_, _ = w.Write(buf[:rn])
				remaining -= int64(rn)
				time.Sleep(2 * time.Millisecond)
			}
			if rerr != nil {
				break
			}
		}
	})}
	go func() { _ = srv.Serve(ln) }()
	defer func() { _ = srv.Close() }()

	var created map[string]any
	req := map[string]any{"url": url, "output_dir": outDir, "chunk_size_mb": 4}
	if err := apiPOST(api+"/downloads", req, &created); err != nil {
		fmt.Println("FAIL start:", err)
		return 1
	}
	id, _ := created["id"].(string)
	if id == "" {
		fmt.Println("FAIL: no download id")
		return 1
	}
	canceled := false
	lastBytes := int64(-1)
	lastMove := time.Now()
	failZeroSince := time.Time{}
	for time.Now().Before(globalDeadline) {
		time.Sleep(1 * time.Second)
		var st map[string]any
		if err := apiGET(api+"/downloads/"+id, &st); err != nil {
			fmt.Println("FAIL poll:", err)
			return 1
		}
		bytesDone := toInt(st["bytes_done"])
		total := toInt(st["total_bytes"])
		percent := toFloat(st["percent"])
		cd := toInt(st["chunks_done"])
		ct := toInt(st["chunks_total"])
		cp := toInt(st["chunks_pending"])
		cf := toInt(st["chunks_failed"])
		cs := toInt(st["chunks_sending"])
		mbps := toFloat(st["mbps_now"])
		mf := fmt.Sprintf("%v", st["manifest_path"])
		fmt.Printf("progress: id=%s status=%v bytes=%d/%d percent=%.2f chunks=%d/%d pending=%d failed=%d sending=%d mbps_now=%.2f manifest=%s\n",
			id, st["status"], bytesDone, total, percent, cd, ct, cp, cf, cs, mbps, mf)
		if bytesDone != lastBytes {
			lastBytes = bytesDone
			lastMove = time.Now()
		}
		if cf > 0 && bytesDone == 0 {
			if failZeroSince.IsZero() {
				failZeroSince = time.Now()
			}
			if time.Since(failZeroSince) > 5*time.Second {
				fmt.Println("FAIL: chunks_failed > 0 and bytes_done == 0 for >5s")
				printDownloadFailedChunks(st)
				printDownloadManifestErrors(mf)
				return 1
			}
		} else {
			failZeroSince = time.Time{}
		}
		if strings.EqualFold(fmt.Sprintf("%v", st["status"]), "failed") {
			fmt.Println("FAIL: status=failed message=", st["message"])
			printDownloadFailedChunks(st)
			printDownloadManifestErrors(mf)
			return 1
		}
		if strings.EqualFold(fmt.Sprintf("%v", st["status"]), "done") {
			fmt.Println("WARN: download completed before cancel, retrying with stronger throttle")
			return 1
		}
		if !canceled && (time.Since(started) > 3*time.Second || cd >= 1) {
			_ = apiPOST(api+"/downloads/"+id+"/cancel", map[string]any{}, nil)
			canceled = true
		}
		if time.Since(lastMove) > 20*time.Second {
			fmt.Println("FAIL: no progress timeout >20s")
			printDownloadFailedChunks(st)
			printDownloadManifestErrors(mf)
			return 1
		}
		if canceled && strings.EqualFold(fmt.Sprintf("%v", st["status"]), "canceled") {
			break
		}
	}
	if !canceled {
		fmt.Println("FAIL: could not cancel")
		return 1
	}
	_ = apiPOST(api+"/downloads/"+id+"/resume", map[string]any{}, nil)
	var final map[string]any
	lastBytes = -1
	lastMove = time.Now()
	for time.Now().Before(globalDeadline) {
		time.Sleep(1 * time.Second)
		if err := apiGET(api+"/downloads/"+id, &final); err != nil {
			fmt.Println("FAIL poll2:", err)
			return 1
		}
		bytesDone := toInt(final["bytes_done"])
		total := toInt(final["total_bytes"])
		percent := toFloat(final["percent"])
		cd := toInt(final["chunks_done"])
		ct := toInt(final["chunks_total"])
		cp := toInt(final["chunks_pending"])
		cf := toInt(final["chunks_failed"])
		cs := toInt(final["chunks_sending"])
		mbps := toFloat(final["mbps_now"])
		mf := fmt.Sprintf("%v", final["manifest_path"])
		fmt.Printf("resume: id=%s status=%v bytes=%d/%d percent=%.2f chunks=%d/%d pending=%d failed=%d sending=%d mbps_now=%.2f manifest=%s\n",
			id, final["status"], bytesDone, total, percent, cd, ct, cp, cf, cs, mbps, mf)
		if bytesDone != lastBytes {
			lastBytes = bytesDone
			lastMove = time.Now()
		}
		if strings.EqualFold(fmt.Sprintf("%v", final["status"]), "failed") {
			fmt.Println("FAIL: resume status=failed message=", final["message"])
			printDownloadFailedChunks(final)
			printDownloadManifestErrors(mf)
			return 1
		}
		if strings.EqualFold(fmt.Sprintf("%v", final["status"]), "done") {
			break
		}
		if time.Since(lastMove) > 20*time.Second {
			fmt.Println("FAIL: resume no progress timeout >20s")
			printDownloadFailedChunks(final)
			printDownloadManifestErrors(mf)
			return 1
		}
	}
	if time.Now().After(globalDeadline) {
		fmt.Println("FAIL: global timeout >5m")
		return 1
	}
	if !strings.EqualFold(fmt.Sprintf("%v", final["status"]), "done") {
		fmt.Println("FAIL: status=", final["status"])
		return 1
	}
	outPath := fmt.Sprintf("%v", final["output_path"])
	st, err := os.Stat(outPath)
	if err != nil || st.Size() != size {
		fmt.Println("FAIL: output size mismatch")
		return 1
	}
	fmt.Println("=== MultiSend Download Smoke ===")
	fmt.Println("result: PASS")
	return 0
}

func toFloat(v any) float64 {
	switch t := v.(type) {
	case float64:
		return t
	case int64:
		return float64(t)
	case json.Number:
		f, _ := t.Float64()
		return f
	default:
		return 0
	}
}
func toInt(v any) int64 {
	switch t := v.(type) {
	case float64:
		return int64(t)
	case int64:
		return t
	case json.Number:
		i, _ := t.Int64()
		return i
	default:
		return 0
	}
}

func printDownloadFailedChunks(st map[string]any) {
	raw, ok := st["failed_chunks"]
	if !ok {
		fmt.Println("failed_chunks: none")
		return
	}
	b, _ := json.Marshal(raw)
	fmt.Println("failed_chunks:", string(b))
}

func printDownloadManifestErrors(manifestPath string) {
	if strings.TrimSpace(manifestPath) == "" {
		fmt.Println("manifest: empty")
		return
	}
	b, err := os.ReadFile(manifestPath)
	if err != nil {
		fmt.Println("manifest read error:", err)
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
		fmt.Println("manifest parse error:", err)
		return
	}
	n := 0
	for _, c := range doc.Chunks {
		if c.Status != "failed" {
			continue
		}
		fmt.Printf("failed chunk idx=%d attempts=%d channel=%s err=%q\n", c.Index, c.Attempts, c.Channel, c.LastError)
		n++
		if n >= 5 {
			break
		}
	}
}
