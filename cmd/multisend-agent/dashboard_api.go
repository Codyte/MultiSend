package main

import (
	"net/http"
	"sort"

	"github.com/Codyte/MultiSend/internal/config"
	"github.com/Codyte/MultiSend/internal/ifmonitor"
)

func (a *app) dashboardHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeAPIError(w, r, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}
	writeJSON(w, map[string]any{
		"health":     a.healthSnapshot(),
		"peers":      a.disc.Peers(),
		"interfaces": a.interfacesSnapshot(),
		"downloads":  a.downloads.List(),
		"jobs":       a.jobsSnapshot(),
		"pulls":      a.listPulls(),
	})
}

func (a *app) healthSnapshot() map[string]any {
	return map[string]any{
		"ok":             true,
		"node_id":        a.cfg.NodeID,
		"name":           a.cfg.DisplayName,
		"agent_log_path": agentLogPath(),
	}
}

func (a *app) jobsSnapshot() []jobDetails {
	a.jobsMu.Lock()
	list := make([]jobDetails, 0, len(a.jobsByID))
	for _, state := range a.jobsByID {
		a.refreshJobFromManifestLocked(&state.details)
		list = append(list, state.details)
	}
	a.jobsMu.Unlock()
	sort.Slice(list, func(i, j int) bool { return list[i].StartedAt > list[j].StartedAt })
	return list
}

func (a *app) interfacesSnapshot() map[string]any {
	infos := detectUsableInterfaces(a.cfg)
	usable := make([]ifmonitor.InterfaceInfo, 0, len(infos))
	for _, info := range infos {
		if info.Usable {
			usable = append(usable, info)
		}
	}
	return map[string]any{
		"policy":                               config.NormalizeInterfacePolicy(a.cfg.InterfacePolicy),
		"supported_policies":                   []string{"auto", "manual", "all"},
		"refresh_seconds":                      a.cfg.InterfaceRefresh,
		"allow_new_interfaces_during_transfer": a.cfg.AllowNewInterfaces,
		"ignore_virtual_interfaces":            a.cfg.IgnoreVirtual,
		"ignore_vpn_interfaces":                a.cfg.IgnoreVPN,
		"ignore_link_local":                    a.cfg.IgnoreLinkLocal,
		"allowed_interface_types":              append([]string(nil), a.cfg.AllowedIfTypes...),
		"manual_interfaces":                    append([]string(nil), a.cfg.ManualInterfaces...),
		"ignored_interfaces":                   append([]string(nil), a.cfg.IgnoredInterfaces...),
		"usable_interfaces":                    usable,
		"interfaces":                           infos,
	}
}
