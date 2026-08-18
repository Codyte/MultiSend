package ports

import (
	"net"
	"testing"
)

func TestRangeParsingAndContainment(t *testing.T) {
	t.Parallel()
	rg, err := ParseRange(" 56210 - 56200 ")
	if err != nil {
		t.Fatalf("ParseRange: %v", err)
	}
	if rg != (Range{Start: 56200, End: 56210}) {
		t.Fatalf("range = %+v", rg)
	}
	if !rg.Contains(56200) || !rg.Contains(56210) || rg.Contains(56199) {
		t.Fatalf("unexpected containment for %+v", rg)
	}
	if !IsExcluded(56205, []Range{{Start: 56210, End: 56200}}) {
		t.Fatal("reversed exclusion range should be normalized")
	}
}

func TestParseRangeRejectsInvalidValues(t *testing.T) {
	t.Parallel()
	for _, value := range []string{"", "56200", "a-b", "0-10", "1-65536", "1-2-3"} {
		if _, err := ParseRange(value); err == nil {
			t.Errorf("ParseRange(%q) unexpectedly succeeded", value)
		}
	}
}

func TestPickRejectsInvalidRange(t *testing.T) {
	t.Parallel()
	invalid := Range{Start: 0, End: int(^uint(0) >> 1)}
	if _, err := PickTCP("127.0.0.1", invalid); err == nil {
		t.Fatal("PickTCP unexpectedly accepted invalid range")
	}
	if _, err := PickUDP("127.0.0.1", invalid); err == nil {
		t.Fatal("PickUDP unexpectedly accepted invalid range")
	}
}

func TestPortAvailabilityOnLoopback(t *testing.T) {
	tcp, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen TCP: %v", err)
	}
	tcpPort := tcp.Addr().(*net.TCPAddr).Port
	if IsPortAvailableTCP("127.0.0.1", tcpPort) {
		t.Fatal("bound TCP port reported available")
	}
	if err := tcp.Close(); err != nil {
		t.Fatalf("close TCP: %v", err)
	}
	if got, err := PickTCP("127.0.0.1", Range{Start: tcpPort, End: tcpPort}); err != nil || got != tcpPort {
		t.Fatalf("PickTCP = %d, %v", got, err)
	}

	udp, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 0})
	if err != nil {
		t.Fatalf("listen UDP: %v", err)
	}
	udpPort := udp.LocalAddr().(*net.UDPAddr).Port
	if IsPortAvailableUDP("127.0.0.1", udpPort) {
		t.Fatal("bound UDP port reported available")
	}
	if err := udp.Close(); err != nil {
		t.Fatalf("close UDP: %v", err)
	}
	if got, err := PickUDP("127.0.0.1", Range{Start: udpPort, End: udpPort}); err != nil || got != udpPort {
		t.Fatalf("PickUDP = %d, %v", got, err)
	}
}
