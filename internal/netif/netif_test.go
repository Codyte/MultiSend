package netif

import (
	"net"
	"testing"
)

func TestIPFromAddrAndDedupe(t *testing.T) {
	t.Parallel()
	ip := net.IPv4(192, 168, 1, 10)
	if got := ipFromAddr(&net.IPNet{IP: ip, Mask: net.CIDRMask(24, 32)}); !got.Equal(ip) {
		t.Fatalf("IPNet = %v", got)
	}
	if got := ipFromAddr(&net.IPAddr{IP: ip}); !got.Equal(ip) {
		t.Fatalf("IPAddr = %v", got)
	}
	if got := ipFromAddr(&net.TCPAddr{IP: ip}); got != nil {
		t.Fatalf("unsupported address = %v", got)
	}

	got := dedupe([]net.IP{ip, net.IPv4(10, 0, 0, 1), ip})
	if len(got) != 2 || !got[0].Equal(ip) || !got[1].Equal(net.IPv4(10, 0, 0, 1)) {
		t.Fatalf("dedupe = %v", got)
	}
}

func TestDetectUsableIPv4Invariants(t *testing.T) {
	ips, err := DetectUsableIPv4()
	if err != nil {
		t.Fatalf("DetectUsableIPv4: %v", err)
	}
	seen := map[string]bool{}
	for _, ip := range ips {
		ip4 := ip.To4()
		if ip4 == nil || ip4.IsLoopback() || ip4.IsUnspecified() || ip4.IsLinkLocalUnicast() {
			t.Fatalf("unusable address returned: %v", ip)
		}
		if seen[ip4.String()] {
			t.Fatalf("duplicate address returned: %v", ip)
		}
		seen[ip4.String()] = true
	}
}
