package ports

import (
	"bufio"
	"fmt"
	"net"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
)

type Range struct {
	Start int `json:"start"`
	End   int `json:"end"`
}

func (r Range) Normalize() Range {
	if r.Start > r.End {
		r.Start, r.End = r.End, r.Start
	}
	return r
}

func (r Range) Contains(port int) bool {
	r = r.Normalize()
	return port >= r.Start && port <= r.End
}

func ParseRange(s string) (Range, error) {
	parts := strings.Split(strings.TrimSpace(s), "-")
	if len(parts) != 2 {
		return Range{}, fmt.Errorf("invalid range")
	}
	a, err := strconv.Atoi(strings.TrimSpace(parts[0]))
	if err != nil {
		return Range{}, err
	}
	b, err := strconv.Atoi(strings.TrimSpace(parts[1]))
	if err != nil {
		return Range{}, err
	}
	r := Range{Start: a, End: b}.Normalize()
	if !IsValidPort(r.Start) || !IsValidPort(r.End) {
		return Range{}, fmt.Errorf("invalid port bounds")
	}
	return r, nil
}

func IsValidPort(port int) bool {
	return port >= 1 && port <= 65535
}

func IsPortAvailableTCP(host string, port int) bool {
	if !IsValidPort(port) {
		return false
	}
	ln, err := net.Listen("tcp", net.JoinHostPort(host, strconv.Itoa(port)))
	if err != nil {
		return false
	}
	_ = ln.Close()
	return true
}

func IsPortAvailableUDP(host string, port int) bool {
	if !IsValidPort(port) {
		return false
	}
	addr, err := net.ResolveUDPAddr("udp4", net.JoinHostPort(host, strconv.Itoa(port)))
	if err != nil {
		return false
	}
	conn, err := net.ListenUDP("udp4", addr)
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}

func PickTCP(host string, rg Range) (int, error) {
	rg = rg.Normalize()
	ex := WindowsExcludedTCPRanges()
	for p := rg.Start; p <= rg.End; p++ {
		if IsExcluded(p, ex) {
			continue
		}
		if IsPortAvailableTCP(host, p) {
			return p, nil
		}
	}
	return 0, fmt.Errorf("no available tcp port in range %d-%d", rg.Start, rg.End)
}

func PickUDP(host string, rg Range) (int, error) {
	rg = rg.Normalize()
	ex := WindowsExcludedUDPRanges()
	for p := rg.Start; p <= rg.End; p++ {
		if IsExcluded(p, ex) {
			continue
		}
		if IsPortAvailableUDP(host, p) {
			return p, nil
		}
	}
	return 0, fmt.Errorf("no available udp port in range %d-%d", rg.Start, rg.End)
}

func IsExcluded(port int, ranges []Range) bool {
	for _, r := range ranges {
		if r.Contains(port) {
			return true
		}
	}
	return false
}

func WindowsExcludedTCPRanges() []Range {
	return windowsExcludedRanges("tcp")
}

func WindowsExcludedUDPRanges() []Range {
	return windowsExcludedRanges("udp")
}

func windowsExcludedRanges(proto string) []Range {
	cmd := exec.Command("netsh", "interface", "ipv4", "show", "excludedportrange", "protocol="+proto)
	out, err := cmd.Output()
	if err != nil {
		return nil
	}
	re := regexp.MustCompile(`^\s*(\d+)\s+(\d+)\s*`) // start end
	sc := bufio.NewScanner(strings.NewReader(string(out)))
	res := make([]Range, 0, 32)
	for sc.Scan() {
		line := sc.Text()
		m := re.FindStringSubmatch(line)
		if len(m) != 3 {
			continue
		}
		a, _ := strconv.Atoi(m[1])
		b, _ := strconv.Atoi(m[2])
		r := Range{Start: a, End: b}.Normalize()
		if IsValidPort(r.Start) && IsValidPort(r.End) {
			res = append(res, r)
		}
	}
	return res
}
