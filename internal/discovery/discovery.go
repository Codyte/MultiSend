package discovery

// ====================== BEGIN NAV INDEX ======================
// NAV INDEX — auto-generated symbol map (refresh via the navindex skill)
//   L53    type Hello
//   L65    type Peer
//   L71    type Manager
//   L86    NewManager
//   L109   Manager.Start
//   L116   Manager.Peers
//   L141   Manager.announceLoop
//   L155   Manager.helloPayload
//   L174   Manager.sendHelloMulticast
//   L191   Manager.sendHelloBroadcast
//   L212   Manager.listenMulticastLoop
//   L240   Manager.listenAnyLoop
//   L267   Manager.handlePacket
//   L289   Manager.hasFreshRealPeerForAddr
//   L301   itoa
//   L324   Manager.broadcastTargets
//   L371   shouldIgnoreInterfaceName
//   L382   Manager.probeLoop
//   L398   Manager.nextProbeInterval
//   L414   Manager.runProbeCycle
//   L454   Manager.localIPv4Nets
//   L492   Manager.wasProbedRecently
//   L499   Manager.markProbed
//   L505   Manager.upsertProbePeer
//   L534   candidateIPsFromARP
//   L575   belongsToAnyNet
// ======================= END NAV INDEX =======================

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"net"
	"os/exec"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/Codyte/MultiSend/internal/ports"
)

const multicastAddr = "239.255.42.99"
const peerTTL = 45 * time.Second

type Hello struct {
	Type          string   `json:"type"`
	NodeID        string   `json:"node_id"`
	Name          string   `json:"name"`
	TransferPort  int      `json:"port"`
	DiscoveryPort int      `json:"discovery_port,omitempty"`
	ControlPort   int      `json:"control_port,omitempty"`
	IPs           []string `json:"ips"`
	Version       string   `json:"version"`
	TimestampUnix int64    `json:"ts"`
}

type Peer struct {
	Hello
	Addr     string    `json:"addr"`
	LastSeen time.Time `json:"last_seen"`
}

type Manager struct {
	nodeID       string
	name         string
	version      string
	tcpPort      int
	udpPort      int
	controlPort  int
	udpPortStart int
	udpPortEnd   int
	getIPs       func() []string
	mu           sync.RWMutex
	peersByID    map[string]Peer
	lastProbe    map[string]time.Time
}

func NewManager(nodeID, name, version string, tcpPort, udpPort, controlPort, udpPortStart, udpPortEnd int, getIPs func() []string) *Manager {
	if udpPortStart <= 0 || udpPortEnd <= 0 {
		udpPortStart = udpPort
		udpPortEnd = udpPort
	}
	if udpPortStart > udpPortEnd {
		udpPortStart, udpPortEnd = udpPortEnd, udpPortStart
	}
	return &Manager{
		nodeID:       nodeID,
		name:         name,
		version:      version,
		tcpPort:      tcpPort,
		udpPort:      udpPort,
		controlPort:  controlPort,
		udpPortStart: udpPortStart,
		udpPortEnd:   udpPortEnd,
		getIPs:       getIPs,
		peersByID:    map[string]Peer{},
		lastProbe:    map[string]time.Time{},
	}
}

func (m *Manager) Start(ctx context.Context) {
	go m.listenMulticastLoop(ctx)
	go m.listenAnyLoop(ctx)
	go m.announceLoop(ctx)
	go m.probeLoop(ctx)
}

func (m *Manager) Peers() []Peer {
	m.mu.Lock()
	defer m.mu.Unlock()
	now := time.Now()
	out := make([]Peer, 0, len(m.peersByID))
	for id, p := range m.peersByID {
		if now.Sub(p.LastSeen) > peerTTL {
			delete(m.peersByID, id)
			continue
		}
		if strings.HasPrefix(id, "probe-") && m.hasFreshRealPeerForAddr(now, p.Addr) {
			continue
		}
		out = append(out, p)
	}
	sort.Slice(out, func(i, j int) bool {
		left, right := strings.ToLower(out[i].Name), strings.ToLower(out[j].Name)
		if left == right {
			return out[i].NodeID < out[j].NodeID
		}
		return left < right
	})
	return out
}

func (m *Manager) announceLoop(ctx context.Context) {
	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()
	for {
		m.sendHelloMulticast()
		m.sendHelloBroadcast()
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (m *Manager) helloPayload() []byte {
	h := Hello{
		Type:          "MULTISEND_HELLO",
		NodeID:        m.nodeID,
		Name:          m.name,
		TransferPort:  m.tcpPort,
		DiscoveryPort: m.udpPort,
		ControlPort:   m.controlPort,
		IPs:           m.getIPs(),
		Version:       m.version,
		TimestampUnix: time.Now().Unix(),
	}
	b, err := json.Marshal(h)
	if err != nil {
		return nil
	}
	return b
}

func (m *Manager) sendHelloMulticast() {
	addr, err := net.ResolveUDPAddr("udp4", net.JoinHostPort(multicastAddr, itoa(m.udpPort)))
	if err != nil {
		return
	}
	conn, err := net.DialUDP("udp4", nil, addr)
	if err != nil {
		return
	}
	defer conn.Close()
	b := m.helloPayload()
	if b == nil {
		return
	}
	_, _ = conn.Write(b)
}

func (m *Manager) sendHelloBroadcast() {
	b := m.helloPayload()
	if b == nil {
		return
	}
	for port := m.udpPortStart; port <= m.udpPortEnd; port++ {
		targets := m.broadcastTargets()
		targets = append(targets, &net.UDPAddr{IP: net.IPv4bcast, Port: port})
		for _, addr := range targets {
			addr.Port = port
			conn, err := net.DialUDP("udp4", nil, addr)
			if err != nil {
				continue
			}
			_ = conn.SetWriteBuffer(64 * 1024)
			_, _ = conn.Write(b)
			_ = conn.Close()
		}
	}
}

func (m *Manager) listenMulticastLoop(ctx context.Context) {
	group := net.ParseIP(multicastAddr)
	laddr := &net.UDPAddr{IP: group, Port: m.udpPort}
	conn, err := net.ListenMulticastUDP("udp4", nil, laddr)
	if err != nil {
		return
	}
	defer conn.Close()
	_ = conn.SetReadBuffer(64 * 1024)
	buf := make([]byte, 65535)
	for {
		_ = conn.SetReadDeadline(time.Now().Add(1 * time.Second))
		n, addr, err := conn.ReadFromUDP(buf)
		if err != nil {
			if ne, ok := err.(net.Error); ok && ne.Timeout() {
				select {
				case <-ctx.Done():
					return
				default:
					continue
				}
			}
			continue
		}
		m.handlePacket(buf[:n], addr)
	}
}

func (m *Manager) listenAnyLoop(ctx context.Context) {
	laddr := &net.UDPAddr{IP: net.IPv4zero, Port: m.udpPort}
	conn, err := net.ListenUDP("udp4", laddr)
	if err != nil {
		return
	}
	defer conn.Close()
	_ = conn.SetReadBuffer(64 * 1024)
	buf := make([]byte, 65535)
	for {
		_ = conn.SetReadDeadline(time.Now().Add(1 * time.Second))
		n, addr, err := conn.ReadFromUDP(buf)
		if err != nil {
			if ne, ok := err.(net.Error); ok && ne.Timeout() {
				select {
				case <-ctx.Done():
					return
				default:
					continue
				}
			}
			continue
		}
		m.handlePacket(buf[:n], addr)
	}
}

func (m *Manager) handlePacket(packet []byte, addr *net.UDPAddr) {
	if addr == nil || addr.IP == nil {
		return
	}
	var h Hello
	if err := json.Unmarshal(packet, &h); err != nil {
		return
	}
	if h.Type != "MULTISEND_HELLO" || h.NodeID == "" || h.NodeID == m.nodeID {
		return
	}
	if !ports.IsValidPort(h.TransferPort) ||
		(h.DiscoveryPort != 0 && !ports.IsValidPort(h.DiscoveryPort)) ||
		(h.ControlPort != 0 && !ports.IsValidPort(h.ControlPort)) {
		return
	}
	p := Peer{Hello: h, Addr: addr.IP.String(), LastSeen: time.Now()}
	m.mu.Lock()
	m.peersByID[h.NodeID] = p
	m.mu.Unlock()
}

func (m *Manager) hasFreshRealPeerForAddr(now time.Time, addr string) bool {
	for id, p := range m.peersByID {
		if strings.HasPrefix(id, "probe-") {
			continue
		}
		if p.Addr == addr && now.Sub(p.LastSeen) <= peerTTL {
			return true
		}
	}
	return false
}

func itoa(v int) string {
	if v == 0 {
		return "0"
	}
	neg := false
	if v < 0 {
		neg = true
		v = -v
	}
	buf := [20]byte{}
	i := len(buf)
	for v > 0 {
		i--
		buf[i] = byte('0' + (v % 10))
		v /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}

func (m *Manager) broadcastTargets() []*net.UDPAddr {
	ifs, err := net.Interfaces()
	if err != nil {
		return nil
	}
	seen := map[string]bool{}
	out := make([]*net.UDPAddr, 0, 8)
	for _, ifc := range ifs {
		if ifc.Flags&net.FlagUp == 0 || ifc.Flags&net.FlagLoopback != 0 {
			continue
		}
		if shouldIgnoreInterfaceName(ifc.Name) {
			continue
		}
		addrs, err := ifc.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range addrs {
			ipNet, ok := addr.(*net.IPNet)
			if !ok || ipNet.IP == nil || ipNet.Mask == nil {
				continue
			}
			ip4 := ipNet.IP.To4()
			if ip4 == nil || len(ipNet.Mask) < 4 {
				continue
			}
			if ip4[0] == 169 && ip4[1] == 254 {
				continue
			}
			bcast := net.IPv4(
				ip4[0]|^ipNet.Mask[0],
				ip4[1]|^ipNet.Mask[1],
				ip4[2]|^ipNet.Mask[2],
				ip4[3]|^ipNet.Mask[3],
			)
			key := bcast.String()
			if key == "" || seen[key] {
				continue
			}
			seen[key] = true
			out = append(out, &net.UDPAddr{IP: bcast, Port: m.udpPort})
		}
	}
	return out
}

func shouldIgnoreInterfaceName(name string) bool {
	lower := strings.ToLower(name)
	ignored := []string{"loopback", "hyper-v", "vethernet", "tailscale", "vpn", "virtualbox", "vmware", "docker", "wsl", "bluetooth"}
	for _, k := range ignored {
		if strings.Contains(lower, k) {
			return true
		}
	}
	return false
}

func (m *Manager) probeLoop(ctx context.Context) {
	timer := time.NewTimer(3 * time.Second)
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
			m.runProbeCycle()
			base := m.nextProbeInterval()
			jitter := time.Duration(rand.Intn(1000)) * time.Millisecond
			timer.Reset(base + jitter)
		}
	}
}

func (m *Manager) nextProbeInterval() time.Duration {
	now := time.Now()
	m.mu.RLock()
	defer m.mu.RUnlock()
	stable := 0
	for _, p := range m.peersByID {
		if now.Sub(p.LastSeen) <= 15*time.Second {
			stable++
		}
	}
	if stable > 0 {
		return 20 * time.Second
	}
	return 6 * time.Second
}

func (m *Manager) runProbeCycle() {
	candidates := candidateIPsFromARP(m.localIPv4Nets())
	if len(candidates) == 0 {
		return
	}
	localSet := make(map[string]bool, 8)
	for _, ip := range m.getIPs() {
		localSet[ip] = true
	}

	maxHosts := 20
	if len(candidates) < maxHosts {
		maxHosts = len(candidates)
	}
	now := time.Now()
	checked := 0
	for _, ip := range candidates {
		if checked >= maxHosts {
			break
		}
		if localSet[ip] {
			continue
		}
		if m.wasProbedRecently(ip, now, 15*time.Second) {
			continue
		}
		checked++
		m.markProbed(ip, now)

		timeout := time.Duration(220+rand.Intn(260)) * time.Millisecond
		addr := net.JoinHostPort(ip, itoa(m.tcpPort))
		conn, err := net.DialTimeout("tcp", addr, timeout)
		if err != nil {
			continue
		}
		_ = conn.Close()
		m.upsertProbePeer(ip)
	}
}

func (m *Manager) localIPv4Nets() []*net.IPNet {
	ifs, err := net.Interfaces()
	if err != nil {
		return nil
	}
	out := make([]*net.IPNet, 0, 8)
	for _, ifc := range ifs {
		if ifc.Flags&net.FlagUp == 0 || ifc.Flags&net.FlagLoopback != 0 {
			continue
		}
		if shouldIgnoreInterfaceName(ifc.Name) {
			continue
		}
		addrs, err := ifc.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range addrs {
			ipNet, ok := addr.(*net.IPNet)
			if !ok || ipNet.IP == nil || ipNet.Mask == nil {
				continue
			}
			ip4 := ipNet.IP.To4()
			if ip4 == nil || len(ipNet.Mask) < 4 {
				continue
			}
			if ip4[0] == 169 && ip4[1] == 254 {
				continue
			}
			out = append(out, &net.IPNet{
				IP:   net.IPv4(ip4[0]&ipNet.Mask[0], ip4[1]&ipNet.Mask[1], ip4[2]&ipNet.Mask[2], ip4[3]&ipNet.Mask[3]),
				Mask: net.IPv4Mask(ipNet.Mask[0], ipNet.Mask[1], ipNet.Mask[2], ipNet.Mask[3]),
			})
		}
	}
	return out
}

func (m *Manager) wasProbedRecently(ip string, now time.Time, cooldown time.Duration) bool {
	m.mu.RLock()
	t, ok := m.lastProbe[ip]
	m.mu.RUnlock()
	return ok && now.Sub(t) < cooldown
}

func (m *Manager) markProbed(ip string, at time.Time) {
	m.mu.Lock()
	m.lastProbe[ip] = at
	m.mu.Unlock()
}

func (m *Manager) upsertProbePeer(ip string) {
	now := time.Now()
	id := fmt.Sprintf("probe-%s", ip)
	p := Peer{
		Hello: Hello{
			Type:         "MULTISEND_PROBE",
			NodeID:       id,
			Name:         "Node " + ip,
			TransferPort: m.tcpPort,
			ControlPort:  m.controlPort,
			IPs:          []string{ip},
			Version:      "probe",
		},
		Addr:     ip,
		LastSeen: now,
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	for existingID, existing := range m.peersByID {
		if strings.HasPrefix(existingID, "probe-") {
			continue
		}
		if existing.Addr == ip && now.Sub(existing.LastSeen) <= peerTTL {
			return
		}
	}
	m.peersByID[id] = p
}

func candidateIPsFromARP(localNets []*net.IPNet) []string {
	if len(localNets) == 0 {
		return nil
	}
	cmd := exec.Command("arp", "-a")
	out, err := cmd.Output()
	if err != nil {
		return nil
	}
	re := regexp.MustCompile(`\b(\d{1,3}(?:\.\d{1,3}){3})\b`)
	seen := map[string]bool{}
	list := make([]string, 0, 32)
	sc := bufio.NewScanner(strings.NewReader(string(out)))
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		matches := re.FindAllStringSubmatch(line, -1)
		for _, match := range matches {
			ip := match[1]
			parsed := net.ParseIP(ip)
			if parsed == nil || parsed.To4() == nil {
				continue
			}
			if strings.HasPrefix(ip, "127.") || strings.HasPrefix(ip, "169.254.") {
				continue
			}
			if !belongsToAnyNet(parsed.To4(), localNets) {
				continue
			}
			if !seen[ip] {
				seen[ip] = true
				list = append(list, ip)
			}
		}
	}
	sort.Strings(list)
	return list
}

func belongsToAnyNet(ip net.IP, nets []*net.IPNet) bool {
	for _, n := range nets {
		if n != nil && n.Contains(ip) {
			return true
		}
	}
	return false
}
