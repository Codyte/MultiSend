package transfer

import "testing"

func TestIsLoopbackTarget(t *testing.T) {
	cases := []struct {
		addr string
		want bool
	}{
		{addr: "127.0.0.1:56200", want: true},
		{addr: "localhost:56200", want: true},
		{addr: "[::1]:56200", want: true},
		{addr: "192.168.0.250:56200", want: false},
		{addr: "10.0.0.20:56200", want: false},
	}
	for _, tc := range cases {
		got := isLoopbackTarget(tc.addr)
		if got != tc.want {
			t.Fatalf("addr=%s got=%v want=%v", tc.addr, got, tc.want)
		}
	}
}

func TestBuildDynamicChannels(t *testing.T) {
	ch := buildDynamicChannels(nil, func() []InterfaceBinding {
		return []InterfaceBinding{
			{Name: "Ethernet", Type: "ethernet", IP: "192.168.0.10"},
			{Name: "Wi-Fi", Type: "wifi", IP: "192.168.0.11"},
		}
	})
	if len(ch) != 2 {
		t.Fatalf("expected 2 channels got %d", len(ch))
	}
	if ch[0].name != "cable" || ch[1].name != "wifi" {
		t.Fatalf("unexpected channels: %+v", ch)
	}
}
