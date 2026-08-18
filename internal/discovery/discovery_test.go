package discovery

import (
	"encoding/json"
	"net"
	"testing"
	"time"
)

func testManager() *Manager {
	return NewManager("self", "Self", "test", 56200, 56211, 56231, 56211, 56220, func() []string {
		return []string{"192.168.1.10"}
	})
}

func helloPacket(t *testing.T, h Hello) []byte {
	t.Helper()
	b, err := json.Marshal(h)
	if err != nil {
		t.Fatalf("marshal hello: %v", err)
	}
	return b
}

func TestHandlePacketValidatesPortsAndAddress(t *testing.T) {
	t.Parallel()
	m := testManager()
	valid := Hello{Type: "MULTISEND_HELLO", NodeID: "peer-1", Name: "Peer", TransferPort: 56200}
	m.handlePacket(helloPacket(t, valid), nil)
	m.handlePacket(helloPacket(t, Hello{Type: valid.Type, NodeID: "bad-transfer", TransferPort: 0}), &net.UDPAddr{IP: net.IPv4(192, 168, 1, 20)})
	m.handlePacket(helloPacket(t, Hello{Type: valid.Type, NodeID: "bad-control", TransferPort: 56200, ControlPort: 70000}), &net.UDPAddr{IP: net.IPv4(192, 168, 1, 20)})
	if got := m.Peers(); len(got) != 0 {
		t.Fatalf("invalid peers accepted: %+v", got)
	}

	m.handlePacket(helloPacket(t, valid), &net.UDPAddr{IP: net.IPv4(192, 168, 1, 20), Port: 56211})
	got := m.Peers()
	if len(got) != 1 || got[0].NodeID != valid.NodeID || got[0].Addr != "192.168.1.20" {
		t.Fatalf("valid peer not recorded: %+v", got)
	}
}

func TestPeersExpireSuppressProbesAndSortDeterministically(t *testing.T) {
	t.Parallel()
	m := testManager()
	now := time.Now()
	m.peersByID = map[string]Peer{
		"expired": {Hello: Hello{NodeID: "expired", Name: "Old"}, Addr: "192.168.1.30", LastSeen: now.Add(-peerTTL - time.Second)},
		"probe-1": {Hello: Hello{NodeID: "probe-1", Name: "Node 192.168.1.20"}, Addr: "192.168.1.20", LastSeen: now},
		"peer-b":  {Hello: Hello{NodeID: "peer-b", Name: "Same"}, Addr: "192.168.1.20", LastSeen: now},
		"peer-a":  {Hello: Hello{NodeID: "peer-a", Name: "same"}, Addr: "192.168.1.21", LastSeen: now},
	}
	got := m.Peers()
	if len(got) != 2 || got[0].NodeID != "peer-a" || got[1].NodeID != "peer-b" {
		t.Fatalf("peers = %+v", got)
	}
	if _, ok := m.peersByID["expired"]; ok {
		t.Fatal("expired peer was not pruned")
	}
}

func TestHelloPayloadAndProbeCooldown(t *testing.T) {
	t.Parallel()
	m := testManager()
	var h Hello
	if err := json.Unmarshal(m.helloPayload(), &h); err != nil {
		t.Fatalf("unmarshal hello: %v", err)
	}
	if h.NodeID != "self" || h.TransferPort != 56200 || h.DiscoveryPort != 56211 || h.ControlPort != 56231 || len(h.IPs) != 1 {
		t.Fatalf("hello = %+v", h)
	}
	now := time.Now()
	m.markProbed("192.168.1.20", now)
	if !m.wasProbedRecently("192.168.1.20", now.Add(time.Second), 2*time.Second) {
		t.Fatal("recent probe not detected")
	}
	if m.wasProbedRecently("192.168.1.20", now.Add(3*time.Second), 2*time.Second) {
		t.Fatal("expired cooldown still active")
	}
}

func TestDiscoveryHelpers(t *testing.T) {
	t.Parallel()
	if !shouldIgnoreInterfaceName("vEthernet (Default Switch)") || shouldIgnoreInterfaceName("Ethernet") {
		t.Fatal("interface name filter mismatch")
	}
	_, network, err := net.ParseCIDR("192.168.1.0/24")
	if err != nil {
		t.Fatal(err)
	}
	if !belongsToAnyNet(net.IPv4(192, 168, 1, 50), []*net.IPNet{network}) || belongsToAnyNet(net.IPv4(10, 0, 0, 1), []*net.IPNet{network}) {
		t.Fatal("network membership mismatch")
	}
}
