package ifmonitor

import (
	"net"
	"testing"

	"lab/multinet/internal/config"
)

func TestShouldIgnoreByName(t *testing.T) {
	cfg := config.DefaultConfig()
	if !shouldIgnoreByName(cfg, "vEthernet (Default Switch)") {
		t.Fatal("expected vEthernet to be ignored")
	}
	if !shouldIgnoreByName(cfg, "Tailscale") {
		t.Fatal("expected tailscale to be ignored")
	}
	if shouldIgnoreByName(cfg, "Ethernet") {
		t.Fatal("expected ethernet to be usable by name")
	}
}

func TestFirstUsableIPv4IgnoresLinkLocal(t *testing.T) {
	cfg := config.DefaultConfig()
	addrs := []net.Addr{
		&net.IPNet{IP: net.ParseIP("169.254.10.20"), Mask: net.CIDRMask(16, 32)},
		&net.IPNet{IP: net.ParseIP("192.168.0.10"), Mask: net.CIDRMask(24, 32)},
	}
	ip := firstUsableIPv4(addrs, cfg.IgnoreLinkLocal)
	if ip == nil || ip.String() != "192.168.0.10" {
		t.Fatalf("expected 192.168.0.10 got %v", ip)
	}
}

func TestPolicyModes(t *testing.T) {
	eth := InterfaceInfo{Name: "Ethernet", Type: "ethernet", Usable: true}
	cfg := config.DefaultConfig()

	cfg.InterfacePolicy = "auto"
	if !isAllowedByPolicy(cfg, eth) {
		t.Fatal("expected auto policy to allow ethernet")
	}

	cfg.InterfacePolicy = "manual"
	if isAllowedByPolicy(cfg, eth) {
		t.Fatal("expected manual policy to deny until listed")
	}
	cfg.ManualInterfaces = []string{"Ethernet"}
	if !isAllowedByPolicy(cfg, eth) {
		t.Fatal("expected manual policy to allow listed interface")
	}

	cfg.InterfacePolicy = "all"
	cfg.ManualInterfaces = nil
	if !isAllowedByPolicy(cfg, eth) {
		t.Fatal("expected all policy to allow everything")
	}

	if got := config.NormalizeInterfacePolicy("garbage"); got != "auto" {
		t.Fatalf("expected invalid policy to normalize to auto, got %s", got)
	}
}
