package ifmonitor

import (
	"net"
	"strings"
	"time"

	"lab/multinet/internal/config"
)

type InterfaceInfo struct {
	Name         string
	Type         string
	IPv4         string
	Usable       bool
	IgnoreReason string
}

func DetectUsableInterfaces(cfg config.Config) []InterfaceInfo {
	ifs, err := net.Interfaces()
	if err != nil {
		return nil
	}
	out := make([]InterfaceInfo, 0, len(ifs))
	for _, ifc := range ifs {
		info := InterfaceInfo{Name: ifc.Name, Type: classifyType(ifc.Name), Usable: false}
		if ifc.Flags&net.FlagUp == 0 {
			info.IgnoreReason = "disconnected"
			out = append(out, info)
			continue
		}
		if ifc.Flags&net.FlagLoopback != 0 {
			info.IgnoreReason = "loopback"
			out = append(out, info)
			continue
		}
		if shouldIgnoreByName(cfg, ifc.Name) {
			info.IgnoreReason = "ignored-by-name"
			out = append(out, info)
			continue
		}
		addrs, err := ifc.Addrs()
		if err != nil {
			info.IgnoreReason = "addr-query-failed"
			out = append(out, info)
			continue
		}
		ip := firstUsableIPv4(addrs, cfg.IgnoreLinkLocal)
		if ip == nil {
			info.IgnoreReason = "no-usable-ipv4"
			out = append(out, info)
			continue
		}
		info.IPv4 = ip.String()
		if !isAllowedByPolicy(cfg, info) {
			if info.IgnoreReason == "" {
				info.IgnoreReason = "policy-blocked"
			}
			out = append(out, info)
			continue
		}
		info.Usable = true
		out = append(out, info)
	}
	return out
}

func Watch(stop <-chan struct{}, interval time.Duration, cfg config.Config) <-chan []InterfaceInfo {
	ch := make(chan []InterfaceInfo, 1)
	go func() {
		defer close(ch)
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-stop:
				return
			case <-ticker.C:
				ch <- DetectUsableInterfaces(cfg)
			}
		}
	}()
	return ch
}

func classifyType(name string) string {
	l := strings.ToLower(name)
	if strings.Contains(l, "wi-fi") || strings.Contains(l, "wifi") || strings.Contains(l, "wireless") {
		return "wifi"
	}
	if strings.Contains(l, "usb") && strings.Contains(l, "ethernet") {
		return "usb_ethernet"
	}
	if strings.Contains(l, "ethernet") {
		return "ethernet"
	}
	return "unknown"
}

func shouldIgnoreByName(cfg config.Config, name string) bool {
	l := strings.ToLower(name)
	for _, n := range cfg.IgnoredInterfaces {
		if strings.Contains(l, strings.ToLower(strings.TrimSpace(n))) {
			return true
		}
	}
	if cfg.IgnoreVirtual {
		for _, k := range []string{"hyper-v", "vethernet", "wsl", "docker", "vmware", "virtualbox"} {
			if strings.Contains(l, k) {
				return true
			}
		}
	}
	if cfg.IgnoreVPN {
		for _, k := range []string{"tailscale", "vpn"} {
			if strings.Contains(l, k) {
				return true
			}
		}
	}
	if strings.Contains(l, "bluetooth") {
		return true
	}
	return false
}

func firstUsableIPv4(addrs []net.Addr, ignoreLinkLocal bool) net.IP {
	for _, a := range addrs {
		var ip net.IP
		switch v := a.(type) {
		case *net.IPNet:
			ip = v.IP.To4()
		case *net.IPAddr:
			ip = v.IP.To4()
		}
		if ip == nil || ip.IsLoopback() || ip.IsUnspecified() {
			continue
		}
		if ignoreLinkLocal && ip[0] == 169 && ip[1] == 254 {
			continue
		}
		return ip
	}
	return nil
}

func isAllowedType(cfg config.Config, typ string) bool {
	for _, t := range cfg.AllowedIfTypes {
		if strings.EqualFold(strings.TrimSpace(t), typ) {
			return true
		}
	}
	return false
}

func isAllowedByPolicy(cfg config.Config, info InterfaceInfo) bool {
	p := config.NormalizeInterfacePolicy(cfg.InterfacePolicy)
	switch p {
	case "manual":
		for _, n := range cfg.ManualInterfaces {
			v := strings.TrimSpace(n)
			if strings.EqualFold(v, info.Name) || strings.EqualFold(v, info.IPv4) {
				return true
			}
		}
		return false
	case "all":
		return true
	default: // auto
		return isAllowedType(cfg, info.Type)
	}
}
