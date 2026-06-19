package netif

import (
	"net"
	"strings"
)

func DetectUsableIPv4() ([]net.IP, error) {
	ifs, err := net.Interfaces()
	if err != nil {
		return nil, err
	}
	ignored := []string{"loopback", "hyper-v", "vethernet", "tailscale", "vpn", "virtualbox", "vmware", "docker", "wsl", "bluetooth"}
	out := make([]net.IP, 0, 4)
	for _, ifc := range ifs {
		if ifc.Flags&net.FlagUp == 0 || ifc.Flags&net.FlagLoopback != 0 {
			continue
		}
		lname := strings.ToLower(ifc.Name)
		skip := false
		for _, k := range ignored {
			if strings.Contains(lname, k) {
				skip = true
				break
			}
		}
		if skip {
			continue
		}
		addrs, err := ifc.Addrs()
		if err != nil {
			continue
		}
		for _, a := range addrs {
			ip := ipFromAddr(a)
			if ip == nil {
				continue
			}
			if ip.IsLoopback() || ip.IsUnspecified() {
				continue
			}
			if ip[0] == 169 && ip[1] == 254 {
				continue
			}
			out = append(out, ip)
		}
	}
	return dedupe(out), nil
}

func ipFromAddr(addr net.Addr) net.IP {
	switch v := addr.(type) {
	case *net.IPNet:
		return v.IP.To4()
	case *net.IPAddr:
		return v.IP.To4()
	default:
		return nil
	}
}

func dedupe(in []net.IP) []net.IP {
	seen := map[string]bool{}
	out := make([]net.IP, 0, len(in))
	for _, ip := range in {
		s := ip.String()
		if seen[s] {
			continue
		}
		seen[s] = true
		out = append(out, ip)
	}
	return out
}
