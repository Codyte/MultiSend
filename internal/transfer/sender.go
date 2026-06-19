package transfer

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

	"lab/multinet/internal/chunk"
	"lab/multinet/internal/manifest"
	"lab/multinet/internal/proto"
	"lab/multinet/internal/scheduler"
)

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
	identity := buildIdentity(absPath, st.Size(), st.ModTime(), target.Address, chunkSize)
	manifestPath := senderManifestPath(identity)
	mf, err := loadOrCreateManifest(manifestPath, identity, absPath, base, st, chunkSize, target.Address, plan, opts.ExplicitResume)
	if err != nil {
		return SendResult{}, err
	}
	if err := manifest.SaveAtomic(manifestPath, mf); err != nil {
		return SendResult{}, err
	}

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

	workerCount := 2
	if len(channels) == 1 {
		workerCount = 1
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
				_ = manifest.SaveAtomic(manifestPath, mf)
				if s, ok := chStats.get(selected.name); ok {
					s.State = "active"
					chStats.set(selected.name, s)
				}
				mu.Unlock()
				if chunkState == nil {
					continue
				}

				start := time.Now()
				reader := io.NewSectionReader(f, chunkState.Offset, chunkState.Size)
				err := sendChunkCtx(sendCtx, target.Address, selected.ip, proto.Header{
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
				}, reader, &sentCable, &sentWifi, selected.name)
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
					_ = manifest.SaveAtomic(manifestPath, mf)
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
				mf.MarkDone(chunkState.Index, chunkState.Size, "")
				_ = manifest.SaveAtomic(manifestPath, mf)
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
		_ = manifest.SaveAtomic(manifestPath, mf)
		mu.Unlock()
		return SendResult{ManifestPath: manifestPath, TransferID: mf.TransferID}, ctx.Err()
	}
	if v := hardErr.Load(); v != nil {
		return SendResult{ManifestPath: manifestPath, TransferID: mf.TransferID}, v.(error)
	}
	mu.Lock()
	defer mu.Unlock()
	for _, c := range mf.Chunks {
		if c.Status != manifest.StatusDone {
			return SendResult{ManifestPath: manifestPath, TransferID: mf.TransferID}, fmt.Errorf("transfer incomplete")
		}
	}
	return SendResult{ManifestPath: manifestPath, TransferID: mf.TransferID}, nil
}

func buildIdentity(absPath string, size int64, mod time.Time, target string, chunkSize int64) string {
	raw := fmt.Sprintf("%s|%d|%d|%s|%d", absPath, size, mod.Unix(), target, chunkSize)
	s := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(s[:16])
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
			if existing.HasCanceledChunks() {
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

func sendChunkCtx(ctx context.Context, server, localIP string, h proto.Header, r io.Reader, sentCable, sentWifi *int64, channel string) error {
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
	done := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			_ = conn.Close()
		case <-done:
		}
	}()
	if err := proto.WriteHeader(conn, h); err != nil {
		close(done)
		return err
	}
	pw := &progressReader{r: r, cable: sentCable, wifi: sentWifi, channel: channel}
	written, err := io.Copy(conn, pw)
	if err != nil {
		close(done)
		return err
	}
	if written != h.PartSize {
		close(done)
		return fmt.Errorf("chunk size mismatch wrote=%d want=%d", written, h.PartSize)
	}
	ack, err := proto.ReadChunkAck(conn)
	close(done)
	if err != nil {
		return err
	}
	if ack.Status != "done" || ack.ChunkIndex != h.ChunkIndex {
		return fmt.Errorf("chunk ack failed status=%s index=%d", ack.Status, ack.ChunkIndex)
	}
	if ctx.Err() != nil {
		return ctx.Err()
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
}

func (p *progressReader) Read(buf []byte) (int, error) {
	n, err := p.r.Read(buf)
	if n > 0 {
		if (p.channel == "cable" || p.channel == "loopback" || p.channel == "local") && p.cable != nil {
			atomic.AddInt64(p.cable, int64(n))
		}
		if p.channel == "wifi" && p.wifi != nil {
			atomic.AddInt64(p.wifi, int64(n))
		}
	}
	return n, err
}
