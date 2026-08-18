package transfer

// ====================== BEGIN NAV INDEX ======================
// NAV INDEX — auto-generated symbol map (refresh via the navindex skill)
//   L75    type localChannel
//   L80    type PeerTarget
//   L84    type SendOptions
//   L99    type InterfaceBinding
//   L105   type SendResult
//   L110   type ProgressSnapshot
//   L127   type ChannelDetail
//   L138   type ProgressFunc
//   L140   SendFileSplit
//   L145   SendFileSplitWithProgress
//   L150   SendFileSplitWithProgressCtx
//   L155   SendFileSplitWithResultCtx
//   L407   buildIdentity
//   L415   SenderIdentity
//   L442   senderManifestPath
//   L451   loadOrCreateManifest
//   L475   manifestComplete
//   L487   newManifest
//   L512   pathDir
//   L514   channelStatesDynamic
//   L533   retryablePending
//   L544   startProgressLoop
//   L602   buildDynamicChannels
//   L637   findChannel
//   L646   type channelStats
//   L651   channelStats.set
//   L657   channelStats.get
//   L666   channelStats.snapshot
//   L678   channelStats.updateFromChannels
//   L699   sendChunkCtx
//   L762   isLoopbackTarget
//   L775   type progressReader
//   L783   progressReader.counter
//   L793   progressReader.Read
//   L806   progressReader.rollback
//   L819   type manifestSaver
//   L826   newManifestSaver
//   L831   manifestSaver.save
//   L844   manifestSaver.flush
//   L855   hashSection
//   L867   contentFingerprint
// ======================= END NAV INDEX =======================

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Codyte/MultiSend/internal/chunk"
	"github.com/Codyte/MultiSend/internal/manifest"
	"github.com/Codyte/MultiSend/internal/proto"
	"github.com/Codyte/MultiSend/internal/scheduler"
)

// maxAttemptsPerChunk bounds retries for a LAN peer transfer. It is deliberately
// lower than the download manager's maxChunkAttempts: on a local network a chunk
// that fails repeatedly usually means the peer/interface is gone, so we fail fast
// rather than spin. See download.maxChunkAttempts.
const maxAttemptsPerChunk = 3

type localChannel struct {
	name string
	ip   string
}

type PeerTarget struct {
	Address string
}

type SendOptions struct {
	ExplicitResume          bool
	ChunkSize               int64
	Lab                     bool
	LabReceivePath          string
	ReceiveRelPath          string
	PublishName             string
	FolderResult            string
	InterfaceProvider       func() []InterfaceBinding
	InterfaceRefreshSeconds int
	AllowNewInterfaces      bool
	// AuthSecret keys the HMAC applied to each chunk header. Nil disables signing.
	AuthSecret []byte
}

type InterfaceBinding struct {
	Name string
	Type string
	IP   string
}

type SendResult struct {
	ManifestPath string
	TransferID   string
}

type ProgressSnapshot struct {
	CableBytes      int64
	WifiBytes       int64
	TotalBytes      int64
	CableTotal      int64
	WifiTotal       int64
	TotalSize       int64
	ChunksTotal     int64
	ChunksDone      int64
	ChunksFailed    int64
	ChunksPending   int64
	ChunksSending   int64
	ResumeSupported bool
	ManifestPath    string
	ChannelDetails  map[string]ChannelDetail
}

type ChannelDetail struct {
	State         string
	InterfaceName string
	LocalIP       string
	BytesSent     int64
	MbpsNow       float64
	Mbps30s       float64
	Failures      int64
	LastError     string
}

type ProgressFunc func(ProgressSnapshot)

func SendFileSplit(filePath string, target PeerTarget, localIPs []string) error {
	_, err := SendFileSplitWithResultCtx(context.Background(), filePath, target, localIPs, nil, SendOptions{})
	return err
}

func SendFileSplitWithProgress(filePath string, target PeerTarget, localIPs []string, progress ProgressFunc) error {
	_, err := SendFileSplitWithResultCtx(context.Background(), filePath, target, localIPs, progress, SendOptions{})
	return err
}

func SendFileSplitWithProgressCtx(ctx context.Context, filePath string, target PeerTarget, localIPs []string, progress ProgressFunc) error {
	_, err := SendFileSplitWithResultCtx(ctx, filePath, target, localIPs, progress, SendOptions{})
	return err
}

func SendFileSplitWithResultCtx(ctx context.Context, filePath string, target PeerTarget, localIPs []string, progress ProgressFunc, opts SendOptions) (SendResult, error) {
	if len(localIPs) == 0 {
		return SendResult{}, fmt.Errorf("no usable local IP")
	}
	absPath, err := filepath.Abs(filePath)
	if err != nil {
		return SendResult{}, err
	}
	f, err := os.Open(absPath)
	if err != nil {
		return SendResult{}, err
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		return SendResult{}, err
	}
	base := filepath.Base(absPath)
	chunkSize := opts.ChunkSize
	if chunkSize <= 0 {
		chunkSize = chunk.AutoChunkSize(st.Size())
	}
	plan, err := chunk.BuildPlan(st.Size(), chunkSize)
	if err != nil {
		return SendResult{}, err
	}
	fingerprint, err := contentFingerprint(f, st.Size())
	if err != nil {
		return SendResult{}, err
	}
	identity := buildIdentity(absPath, st.Size(), st.ModTime(), target.Address, chunkSize, fingerprint)
	manifestPath := senderManifestPath(identity)
	mf, err := loadOrCreateManifest(manifestPath, identity, absPath, base, st, chunkSize, target.Address, plan, opts.ExplicitResume)
	if err != nil {
		return SendResult{}, err
	}
	if err := manifest.SaveAtomic(manifestPath, mf); err != nil {
		return SendResult{}, err
	}
	saver := newManifestSaver(manifestPath, 750*time.Millisecond)

	var sentCable, sentWifi int64
	sched := scheduler.NewWeightedScheduler()
	chStats := &channelStats{m: map[string]*ChannelDetail{
		"cable": {State: "offline"},
		"wifi":  {State: "offline"},
	}}
	stopProgress := startProgressLoop(progress, &sentCable, &sentWifi, st.Size(), 0, 0, mf, manifestPath, chStats)
	defer stopProgress()

	channels := make([]localChannel, 0, 2)
	if opts.Lab || isLoopbackTarget(target.Address) {
		channels = append(channels, localChannel{name: "loopback", ip: ""})
		// Track the loopback channel under its own key so its telemetry is
		// actually updated by the worker (B-7); also mirror to "cable" for UIs
		// that only render the cable/wifi pair.
		chStats.set("loopback", ChannelDetail{State: "active", InterfaceName: "loopback", BytesSent: 0})
		chStats.set("cable", ChannelDetail{State: "active", InterfaceName: "loopback", BytesSent: 0})
	} else {
		channels = buildDynamicChannels(localIPs, opts.InterfaceProvider)
		chStats.updateFromChannels(channels)
	}

	sendCtx, sendCancel := context.WithCancel(ctx)
	defer sendCancel()
	var mu sync.Mutex
	var wg sync.WaitGroup
	var hardErr atomic.Value
	inFlight := map[string]int{}

	// One worker per channel, with a floor of 1. When dynamic interface discovery
	// is enabled, keep at least 2 workers so a second interface that appears
	// mid-transfer is actually used (B-6).
	workerCount := len(channels)
	if workerCount < 1 {
		workerCount = 1
	}
	if opts.AllowNewInterfaces && workerCount < 2 {
		workerCount = 2
	}
	for i := 0; i < workerCount; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			lastRefresh := time.Time{}
			for {
				if sendCtx.Err() != nil || hardErr.Load() != nil {
					return
				}
				if !opts.Lab && !isLoopbackTarget(target.Address) && opts.AllowNewInterfaces {
					refreshSec := opts.InterfaceRefreshSeconds
					if refreshSec <= 0 {
						refreshSec = 5
					}
					if lastRefresh.IsZero() || time.Since(lastRefresh) >= time.Duration(refreshSec)*time.Second {
						updated := buildDynamicChannels(localIPs, opts.InterfaceProvider)
						mu.Lock()
						channels = updated
						chStats.updateFromChannels(channels)
						mu.Unlock()
						lastRefresh = time.Now()
					}
				}
				mu.Lock()
				pending := retryablePending(mf)
				if len(pending) == 0 {
					mu.Unlock()
					return
				}
				idx, pickCh, ok := sched.NextChunk(pending, channelStatesDynamic(channels, inFlight, chStats.snapshot()))
				if !ok {
					mu.Unlock()
					time.Sleep(80 * time.Millisecond)
					continue
				}
				selected, ok := findChannel(channels, pickCh)
				if !ok {
					mu.Unlock()
					time.Sleep(80 * time.Millisecond)
					continue
				}
				chunkState := mf.MarkAndGetSending(idx, selected.name)
				inFlight[selected.name]++
				saver.save(mf, false)
				if s, ok := chStats.get(selected.name); ok {
					s.State = "active"
					chStats.set(selected.name, s)
				}
				mu.Unlock()
				if chunkState == nil {
					continue
				}

				start := time.Now()
				chunkHash, hashErr := hashSection(f, chunkState.Offset, chunkState.Size)
				if hashErr != nil {
					mu.Lock()
					if inFlight[selected.name] > 0 {
						inFlight[selected.name]--
					}
					mf.MarkFailed(chunkState.Index, hashErr.Error())
					hardErr.Store(fmt.Errorf("hash chunk %d: %w", chunkState.Index, hashErr))
					saver.save(mf, true)
					mu.Unlock()
					sendCancel()
					return
				}
				reader := io.NewSectionReader(f, chunkState.Offset, chunkState.Size)
				header := proto.Header{
					MessageType:    "chunk_start",
					TransferID:     mf.TransferID,
					FileName:       base,
					PartIndex:      int(chunkState.Index) + 1,
					TotalParts:     len(mf.Chunks),
					PartSize:       chunkState.Size,
					ChunkIndex:     chunkState.Index,
					Offset:         chunkState.Offset,
					TotalBytes:     st.Size(),
					Lab:            opts.Lab,
					LabPath:        opts.LabReceivePath,
					ReceiveRelPath: opts.ReceiveRelPath,
					PublishName:    opts.PublishName,
					FolderResult:   opts.FolderResult,
					ChunkSHA256:    chunkHash,
				}
				proto.SignHeader(&header, opts.AuthSecret)
				err := sendChunkCtx(sendCtx, target.Address, selected.ip, header, reader, &sentCable, &sentWifi, selected.name)
				if err != nil {
					mu.Lock()
					if inFlight[selected.name] > 0 {
						inFlight[selected.name]--
					}
					mf.MarkFailed(chunkState.Index, err.Error())
					if c := mf.GetChunk(chunkState.Index); c != nil {
						if c.Attempts < maxAttemptsPerChunk {
							c.Status = manifest.StatusPending
						} else {
							hardErr.Store(fmt.Errorf("chunk %d exceeded retries", c.Index))
						}
					}
					saver.save(mf, hardErr.Load() != nil)
					if s, ok := chStats.get(selected.name); ok {
						s.Failures++
						s.LastError = err.Error()
						if s.Failures >= 3 {
							s.State = "cooldown"
						} else {
							s.State = "offline"
						}
						chStats.set(selected.name, s)
					}
					mu.Unlock()
					sched.ReportFailure(selected.name, err)
					if sendCtx.Err() != nil {
						return
					}
					if hardErr.Load() != nil {
						sendCancel()
						return
					}
					continue
				}
				sched.ReportSuccess(selected.name, chunkState.Size, time.Since(start))
				mu.Lock()
				if inFlight[selected.name] > 0 {
					inFlight[selected.name]--
				}
				mf.MarkDone(chunkState.Index, chunkState.Size, chunkHash)
				saver.save(mf, false)
				if s, ok := chStats.get(selected.name); ok {
					s.State = "active"
					s.LastError = ""
					s.BytesSent += chunkState.Size
					chStats.set(selected.name, s)
				}
				mu.Unlock()
				if opts.Lab {
					time.Sleep(30 * time.Millisecond)
				}
			}
		}()
	}

	wg.Wait()
	if ctx.Err() != nil {
		mu.Lock()
		mf.MarkCanceledPending()
		saver.save(mf, true)
		mu.Unlock()
		return SendResult{ManifestPath: manifestPath, TransferID: mf.TransferID}, ctx.Err()
	}
	if v := hardErr.Load(); v != nil {
		mu.Lock()
		saver.save(mf, true)
		mu.Unlock()
		return SendResult{ManifestPath: manifestPath, TransferID: mf.TransferID}, v.(error)
	}
	mu.Lock()
	defer mu.Unlock()
	// Flush the final manifest state so a crash right after completion still
	// records every done chunk for resume/verification.
	if err := saver.flush(mf); err != nil {
		return SendResult{ManifestPath: manifestPath, TransferID: mf.TransferID}, fmt.Errorf("persist final manifest: %w", err)
	}
	for _, c := range mf.Chunks {
		if c.Status != manifest.StatusDone {
			return SendResult{ManifestPath: manifestPath, TransferID: mf.TransferID}, fmt.Errorf("transfer incomplete")
		}
	}
	return SendResult{ManifestPath: manifestPath, TransferID: mf.TransferID}, nil
}

func buildIdentity(absPath string, size int64, mod time.Time, target string, chunkSize int64, fingerprint string) string {
	raw := fmt.Sprintf("%s|%d|%d|%s|%d|%s", absPath, size, mod.Unix(), target, chunkSize, fingerprint)
	s := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(s[:16])
}

// SenderIdentity returns the identity used to select a resumable sender
// manifest for the current file contents and target.
func SenderIdentity(filePath, target string, chunkSize int64) (string, error) {
	absPath, err := filepath.Abs(filePath)
	if err != nil {
		return "", err
	}
	f, err := os.Open(absPath)
	if err != nil {
		return "", err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return "", err
	}
	if info.IsDir() {
		return "", fmt.Errorf("source is not a file")
	}
	if chunkSize <= 0 {
		chunkSize = chunk.AutoChunkSize(info.Size())
	}
	fingerprint, err := contentFingerprint(f, info.Size())
	if err != nil {
		return "", err
	}
	return buildIdentity(absPath, info.Size(), info.ModTime(), target, chunkSize, fingerprint), nil
}

func senderManifestPath(identity string) string {
	base := os.Getenv("USERPROFILE")
	if base == "" {
		base = `C:\Users\Public`
	}
	dir := filepath.Join(base, "Downloads", "MultiSend", "manifests")
	return filepath.Join(dir, fmt.Sprintf("%s.manifest.json", identity))
}

func loadOrCreateManifest(path, identity, absPath, fileName string, st os.FileInfo, chunkSize int64, targetAddr string, plan chunk.Plan, explicitResume bool) (*manifest.Manifest, error) {
	now := time.Now().UTC()
	if existing, err := manifest.Load(path); err == nil {
		compatible := existing.FileName == fileName && existing.TotalBytes == st.Size() && existing.ChunkSize == chunkSize && existing.Target.Address == targetAddr && existing.Source.Identity == identity
		if compatible {
			if explicitResume {
				existing.ActivateResume(maxAttemptsPerChunk)
				return existing, nil
			}
			if existing.HasCanceledChunks() || manifestComplete(existing) {
				return newManifest(identity, fileName, st.Size(), chunkSize, targetAddr, absPath, st.ModTime(), plan, now), nil
			}
			return existing, nil
		}
		if explicitResume {
			return nil, fmt.Errorf("manifest incompatible for resume")
		}
	}
	if explicitResume {
		return nil, fmt.Errorf("resume requested but manifest not found")
	}
	return newManifest(identity, fileName, st.Size(), chunkSize, targetAddr, absPath, st.ModTime(), plan, now), nil
}

func manifestComplete(m *manifest.Manifest) bool {
	if m == nil || len(m.Chunks) == 0 {
		return false
	}
	for _, chunk := range m.Chunks {
		if chunk.Status != manifest.StatusDone {
			return false
		}
	}
	return true
}

func newManifest(identity, fileName string, totalBytes, chunkSize int64, targetAddr, absPath string, modTime time.Time, plan chunk.Plan, now time.Time) *manifest.Manifest {
	m := &manifest.Manifest{
		SchemaVersion: 1,
		TransferID:    fmt.Sprintf("tx-%d", time.Now().UnixNano()),
		Type:          "p2p",
		FileName:      fileName,
		TotalBytes:    totalBytes,
		ChunkSize:     chunkSize,
		CreatedAt:     now,
		UpdatedAt:     now,
		Source: manifest.SourceInfo{
			Type:     "p2p",
			Identity: identity,
			AbsPath:  absPath,
			ModTime:  modTime.Unix(),
		},
		Target: manifest.TargetInfo{Type: "p2p", Address: targetAddr, Path: pathDir(absPath)},
		Chunks: make([]manifest.ChunkState, 0, len(plan.Chunks)),
	}
	for _, c := range plan.Chunks {
		m.Chunks = append(m.Chunks, manifest.ChunkState{Index: c.Index, Offset: c.Offset, Size: c.Size, Status: manifest.StatusPending})
	}
	return m
}

func pathDir(filePath string) string { return filepath.Dir(filePath) }

func channelStatesDynamic(channels []localChannel, inFlight map[string]int, stats map[string]ChannelDetail) []scheduler.Channel {
	out := make([]scheduler.Channel, 0, len(channels))
	for _, c := range channels {
		ch := scheduler.Channel{Name: c.name, Active: true}
		if inFlight != nil {
			ch.InFlight = inFlight[c.name]
		}
		if stats != nil {
			if s, ok := stats[c.name]; ok {
				ch.Mbps30s = s.Mbps30s
				ch.Failures = int(s.Failures)
				ch.LastError = s.LastError
			}
		}
		out = append(out, ch)
	}
	return out
}

func retryablePending(m *manifest.Manifest) []manifest.ChunkState {
	out := make([]manifest.ChunkState, 0)
	for _, c := range m.Chunks {
		if c.Status == manifest.StatusPending || (c.Status == manifest.StatusFailed && c.Attempts < maxAttemptsPerChunk) {
			out = append(out, c)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Index < out[j].Index })
	return out
}

func startProgressLoop(progress ProgressFunc, cable, wifi *int64, totalSize, cableTotal, wifiTotal int64, m *manifest.Manifest, manifestPath string, stats *channelStats) func() {
	if progress == nil {
		return func() {}
	}
	done := make(chan struct{})
	var once sync.Once

	emit := func() {
		c := atomic.LoadInt64(cable)
		w := atomic.LoadInt64(wifi)
		doneCnt, failedCnt, pendingCnt, sendingCnt := int64(0), int64(0), int64(0), int64(0)
		for _, ch := range m.Chunks {
			switch ch.Status {
			case manifest.StatusDone:
				doneCnt++
			case manifest.StatusFailed:
				failedCnt++
			case manifest.StatusSending:
				sendingCnt++
			case manifest.StatusPending, manifest.StatusCanceled:
				pendingCnt++
			}
		}
		progress(ProgressSnapshot{
			CableBytes:      c,
			WifiBytes:       w,
			TotalBytes:      c + w,
			CableTotal:      cableTotal,
			WifiTotal:       wifiTotal,
			TotalSize:       totalSize,
			ChunksTotal:     int64(len(m.Chunks)),
			ChunksDone:      doneCnt,
			ChunksFailed:    failedCnt,
			ChunksPending:   pendingCnt,
			ChunksSending:   sendingCnt,
			ResumeSupported: true,
			ManifestPath:    manifestPath,
			ChannelDetails:  stats.snapshot(),
		})
	}

	go func() {
		ticker := time.NewTicker(1 * time.Second)
		defer ticker.Stop()
		emit()
		for {
			select {
			case <-ticker.C:
				emit()
			case <-done:
				emit()
				return
			}
		}
	}()
	return func() { once.Do(func() { close(done) }) }
}

func buildDynamicChannels(localIPs []string, provider func() []InterfaceBinding) []localChannel {
	if provider == nil {
		out := make([]localChannel, 0, len(localIPs))
		for i, ip := range localIPs {
			name := "wifi"
			if i == 0 {
				name = "cable"
			}
			out = append(out, localChannel{name: name, ip: ip})
		}
		return out
	}
	list := provider()
	var cableIP, wifiIP string
	for _, it := range list {
		if it.Type == "ethernet" || it.Type == "usb_ethernet" || it.Type == "unknown" {
			if cableIP == "" {
				cableIP = it.IP
			}
		} else if it.Type == "wifi" {
			if wifiIP == "" {
				wifiIP = it.IP
			}
		}
	}
	out := make([]localChannel, 0, 2)
	if cableIP != "" {
		out = append(out, localChannel{name: "cable", ip: cableIP})
	}
	if wifiIP != "" {
		out = append(out, localChannel{name: "wifi", ip: wifiIP})
	}
	return out
}

func findChannel(channels []localChannel, name string) (localChannel, bool) {
	for _, c := range channels {
		if c.name == name {
			return c, true
		}
	}
	return localChannel{}, false
}

type channelStats struct {
	mu sync.RWMutex
	m  map[string]*ChannelDetail
}

func (s *channelStats) set(name string, d ChannelDetail) {
	s.mu.Lock()
	defer s.mu.Unlock()
	cp := d
	s.m[name] = &cp
}
func (s *channelStats) get(name string) (ChannelDetail, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	v, ok := s.m[name]
	if !ok || v == nil {
		return ChannelDetail{}, false
	}
	return *v, true
}
func (s *channelStats) snapshot() map[string]ChannelDetail {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make(map[string]ChannelDetail, len(s.m))
	for k, v := range s.m {
		if v == nil {
			continue
		}
		out[k] = *v
	}
	return out
}
func (s *channelStats) updateFromChannels(channels []localChannel) {
	active := map[string]bool{}
	for _, ch := range channels {
		active[ch.name] = true
		cur, _ := s.get(ch.name)
		cur.State = "active"
		cur.LocalIP = ch.ip
		if cur.InterfaceName == "" {
			cur.InterfaceName = ch.name
		}
		s.set(ch.name, cur)
	}
	for _, n := range []string{"cable", "wifi"} {
		if !active[n] {
			cur, _ := s.get(n)
			cur.State = "offline"
			s.set(n, cur)
		}
	}
}

func sendChunkCtx(ctx context.Context, server, localIP string, h proto.Header, r io.Reader, sentCable, sentWifi *int64, channel string) (err error) {
	var d net.Dialer
	if strings.TrimSpace(localIP) != "" {
		parsedIP := net.ParseIP(localIP)
		if parsedIP == nil {
			return fmt.Errorf("invalid local IP: %s", localIP)
		}
		d.LocalAddr = &net.TCPAddr{IP: parsedIP}
	}
	conn, err := d.DialContext(ctx, "tcp", server)
	if err != nil {
		return err
	}
	defer conn.Close()
	pw := &progressReader{r: r, cable: sentCable, wifi: sentWifi, channel: channel}
	// B-1: a chunk only "counts" toward progress once it is fully acked. If the
	// attempt fails after streaming partial bytes, roll those bytes back so the
	// retry does not double-count and inflate the progress/ETA.
	defer func() {
		if err != nil {
			pw.rollback()
		}
	}()
	done := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			_ = conn.Close()
		case <-done:
		}
	}()
	if err = proto.WriteHeader(conn, h); err != nil {
		close(done)
		return err
	}
	written, copyErr := io.Copy(conn, pw)
	if copyErr != nil {
		close(done)
		err = copyErr
		return err
	}
	if written != h.PartSize {
		close(done)
		err = fmt.Errorf("chunk size mismatch wrote=%d want=%d", written, h.PartSize)
		return err
	}
	ack, ackErr := proto.ReadChunkAck(conn)
	close(done)
	if ackErr != nil {
		err = ackErr
		return err
	}
	if ack.Status != "done" || ack.ChunkIndex != h.ChunkIndex {
		err = fmt.Errorf("chunk ack failed status=%s index=%d", ack.Status, ack.ChunkIndex)
		return err
	}
	if ctx.Err() != nil {
		err = ctx.Err()
		return err
	}
	return nil
}

func isLoopbackTarget(address string) bool {
	host := address
	if h, _, err := net.SplitHostPort(address); err == nil {
		host = h
	}
	host = strings.TrimSpace(strings.Trim(host, "[]"))
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

type progressReader struct {
	r       io.Reader
	cable   *int64
	wifi    *int64
	channel string
	counted int64 // bytes added to the live counters during this attempt
}

func (p *progressReader) counter() *int64 {
	switch p.channel {
	case "cable", "loopback", "local":
		return p.cable
	case "wifi":
		return p.wifi
	}
	return nil
}

func (p *progressReader) Read(buf []byte) (int, error) {
	n, err := p.r.Read(buf)
	if n > 0 {
		if c := p.counter(); c != nil {
			atomic.AddInt64(c, int64(n))
			atomic.AddInt64(&p.counted, int64(n))
		}
	}
	return n, err
}

// rollback removes the bytes this attempt added to the live counters. Called
// when the chunk attempt fails so a retry does not double-count (B-1).
func (p *progressReader) rollback() {
	counted := atomic.SwapInt64(&p.counted, 0)
	if counted == 0 {
		return
	}
	if c := p.counter(); c != nil {
		atomic.AddInt64(c, -counted)
	}
}

// manifestSaver coalesces manifest writes to avoid an fsync per chunk (B-3).
// All methods are called while the sender's mutex is held, so it needs no
// internal locking. Throttled errors are surfaced by the final flush (B-4).
type manifestSaver struct {
	path     string
	interval time.Duration
	last     time.Time
	lastErr  error
}

func newManifestSaver(path string, interval time.Duration) *manifestSaver {
	return &manifestSaver{path: path, interval: interval}
}

// save persists the manifest, but at most once per interval unless force is set.
func (s *manifestSaver) save(m *manifest.Manifest, force bool) {
	if !force && !s.last.IsZero() && time.Since(s.last) < s.interval {
		return
	}
	if err := manifest.SaveAtomic(s.path, m); err != nil {
		s.lastErr = err
		return
	}
	s.last = time.Now()
	s.lastErr = nil
}

// flush forces a final write and returns any persistent error.
func (s *manifestSaver) flush(m *manifest.Manifest) error {
	if err := manifest.SaveAtomic(s.path, m); err != nil {
		s.lastErr = err
		return err
	}
	s.last = time.Now()
	s.lastErr = nil
	return nil
}

// hashSection returns the lowercase hex SHA-256 of size bytes at offset.
func hashSection(r io.ReaderAt, offset, size int64) (string, error) {
	h := sha256.New()
	if _, err := io.Copy(h, io.NewSectionReader(r, offset, size)); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// contentFingerprint returns a cheap, deterministic fingerprint of a file's
// content used to detect in-place edits that keep the same size/mtime (B-2).
// It hashes the size plus the head and tail windows; for small files it hashes
// the whole content. It is a heuristic, not a full-file checksum.
func contentFingerprint(r io.ReaderAt, size int64) (string, error) {
	const window = 64 * 1024
	h := sha256.New()
	fmt.Fprintf(h, "size:%d\x00", size)
	if size <= 2*window {
		if size > 0 {
			if _, err := io.Copy(h, io.NewSectionReader(r, 0, size)); err != nil {
				return "", err
			}
		}
		return hex.EncodeToString(h.Sum(nil)), nil
	}
	if _, err := io.Copy(h, io.NewSectionReader(r, 0, window)); err != nil {
		return "", err
	}
	if _, err := io.Copy(h, io.NewSectionReader(r, size-window, window)); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
