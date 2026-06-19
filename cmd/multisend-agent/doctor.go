package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strings"
	"time"

	"lab/multinet/internal/config"
	"lab/multinet/internal/ifmonitor"
)

type doctorReporter struct {
	pass int
	warn int
	fail int
	skip int
}

func (r *doctorReporter) line(kind, msg string) {
	fmt.Printf("%-5s %s\n", kind, msg)
}
func (r *doctorReporter) Pass(msg string) { r.pass++; r.line("PASS", msg) }
func (r *doctorReporter) Warn(msg string) { r.warn++; r.line("WARN", msg) }
func (r *doctorReporter) Fail(msg string) { r.fail++; r.line("FAIL", msg) }
func (r *doctorReporter) Skip(msg string) { r.skip++; r.line("SKIP", msg) }

func RunDoctor() int {
	r := &doctorReporter{}
	cfgPath, _ := config.ConfigPath()
	cfg, cfgExists, cfgRaw, cfgBOM, cfgErr := readConfigForDoctor(cfgPath)

	printDoctorHeader(r, cfgPath, cfg)
	printDoctorPaths(r, cfg, cfgPath)
	printInstalledFiles(r)
	validateAndPrintConfig(r, cfg, cfgExists, cfgRaw, cfgBOM, cfgErr)
	printAgentProcess(r)
	rt := readRuntimeState()
	testLocalAPI(r, cfg, rt)
	printFirewallChecks(r)
	printAutostart(r)
	printExplorerContext(r)
	printInterfaces(r, cfg)

	printSummary(r)
	if r.fail > 0 {
		return 1
	}
	return 0
}

type runtimeState struct {
	LocalAPIPort  int `json:"local_api_port"`
	TransferPort  int `json:"transfer_port"`
	DiscoveryPort int `json:"discovery_port"`
	PID           int `json:"pid"`
}

func readRuntimeState() runtimeState {
	local := os.Getenv("LOCALAPPDATA")
	if local == "" {
		return runtimeState{}
	}
	p := filepath.Join(local, "MultiSend", "runtime.json")
	b, err := os.ReadFile(p)
	if err != nil {
		return runtimeState{}
	}
	var rt runtimeState
	_ = json.Unmarshal(b, &rt)
	return rt
}

func printDoctorHeader(r *doctorReporter, cfgPath string, cfg config.Config) {
	fmt.Println("=== MultiSend Doctor ===")
	host, _ := os.Hostname()
	u, _ := user.Current()
	username := ""
	if u != nil {
		username = u.Username
	}
	appVersion := cfg.AppVersion
	if appVersion == "" {
		appVersion = "0.1.0"
	}
	fmt.Println("[Header]")
	fmt.Printf("app_version: %s\n", appVersion)
	fmt.Printf("node_id: %s\n", cfg.NodeID)
	fmt.Printf("display_name: %s\n", cfg.DisplayName)
	fmt.Printf("hostname: %s\n", host)
	fmt.Printf("user: %s\n", username)
	fmt.Printf("time: %s\n", time.Now().Format(time.RFC3339))
	fmt.Printf("config_path: %s\n", cfgPath)
	fmt.Println()
}

func printDoctorPaths(r *doctorReporter, cfg config.Config, cfgPath string) {
	fmt.Println("[Paths]")
	local := os.Getenv("LOCALAPPDATA")
	appdata := os.Getenv("APPDATA")
	paths := []string{
		`C:\Program Files\MultiSend`,
		`C:\Program Files\MultiSend\bin`,
		filepath.Join(appdata, "MultiSend"),
		cfgPath,
		filepath.Join(local, "MultiSend", "logs"),
		filepath.Join(local, "MultiSend", "logs", "agent.log"),
		`C:\ProgramData\MultiSend`,
		`C:\ProgramData\MultiSend\logs`,
		`C:\ProgramData\MultiSend\install-state.json`,
		cfg.ReceivePath,
	}
	for _, p := range paths {
		if p == "" {
			r.Skip("empty path")
			continue
		}
		if _, err := os.Stat(p); err == nil {
			r.Pass(p)
		} else {
			r.Skip(p)
		}
	}
	fmt.Println()
}

func printInstalledFiles(r *doctorReporter) {
	fmt.Println("[Installed files]")
	base := `C:\Program Files\MultiSend\bin`
	names := []string{"multisend-agent.exe", "multisend.exe", "multirecv.exe", "multisend-launcher.ps1"}
	for _, n := range names {
		p := filepath.Join(base, n)
		st, err := os.Stat(p)
		if err != nil {
			r.Skip(fmt.Sprintf("%s (not installed)", n))
			continue
		}
		ver := fileVersion(p)
		if ver == "" {
			ver = "n/a"
		}
		r.Pass(fmt.Sprintf("%s | size=%d | modified=%s | version=%s", n, st.Size(), st.ModTime().Format(time.RFC3339), ver))
	}
	fmt.Println()
}

func validateAndPrintConfig(r *doctorReporter, cfg config.Config, exists bool, raw []byte, hasBOM bool, readErr error) {
	fmt.Println("[User config]")
	if !exists {
		r.Fail("config.json not found")
		fmt.Println()
		return
	}
	if readErr != nil {
		r.Fail("config.json read/parse failed: " + readErr.Error())
		fmt.Println()
		return
	}
	if hasBOM {
		r.Warn("config.json has UTF-8 BOM")
	} else {
		r.Pass("config.json UTF-8 BOM: not found")
	}
	var anyObj any
	if err := json.Unmarshal(raw, &anyObj); err != nil {
		r.Fail("config.json JSON invalid: " + err.Error())
		fmt.Println()
		return
	}
	norm, _ := json.MarshalIndent(anyObj, "", "  ")
	fmt.Println(string(norm))

	if cfg.SchemaVersion >= 1 {
		r.Pass(fmt.Sprintf("schema_version=%d", cfg.SchemaVersion))
	} else {
		r.Fail("schema_version invalid")
	}
	if strings.TrimSpace(cfg.NodeID) != "" {
		r.Pass("node_id present")
	} else {
		r.Fail("node_id missing")
	}
	if strings.TrimSpace(cfg.ReceivePath) != "" {
		r.Pass("receive_path present")
	} else {
		r.Fail("receive_path missing")
	}
	validatePort := func(name string, p int) {
		if p >= 1 && p <= 65535 {
			r.Pass(fmt.Sprintf("%s=%d", name, p))
		} else {
			r.Fail(fmt.Sprintf("%s invalid: %d", name, p))
		}
	}
	validatePort("transfer_port(selected)", cfg.SelectedPorts.Transfer)
	validatePort("discovery_port(selected)", cfg.SelectedPorts.Discovery)
	validatePort("local_api_port(selected)", cfg.SelectedPorts.LocalAPI)
	validatePort("transfer_port_range.start", cfg.TransferPortRange.Start)
	validatePort("transfer_port_range.end", cfg.TransferPortRange.End)
	validatePort("discovery_port_range.start", cfg.DiscoveryPortRange.Start)
	validatePort("discovery_port_range.end", cfg.DiscoveryPortRange.End)
	validatePort("local_api_port_range.start", cfg.LocalAPIPortRange.Start)
	validatePort("local_api_port_range.end", cfg.LocalAPIPortRange.End)
	if cfg.TransferPort == 55202 || cfg.LocalAPIPort == 55204 {
		r.Warn("legacy fixed ports still present in compat fields")
	}
	r.Pass(fmt.Sprintf("interface_policy=%s", cfg.InterfacePolicy))
	r.Pass(fmt.Sprintf("interface_refresh_seconds=%d", cfg.InterfaceRefresh))
	r.Pass(fmt.Sprintf("allow_new_interfaces_during_transfer=%t", cfg.AllowNewInterfaces))
	r.Pass(fmt.Sprintf("pull_folder_result=%s", config.NormalizePullFolderResult(cfg.PullFolderResult)))
	if len(cfg.AllowedIfTypes) > 0 {
		r.Pass("allowed_interface_types=" + strings.Join(cfg.AllowedIfTypes, ","))
	}
	fmt.Println()
}

func printAgentProcess(r *doctorReporter) {
	fmt.Println("[Agent process]")
	currentPID := os.Getpid()
	type proc struct {
		ProcessId      int    `json:"ProcessId"`
		ExecutablePath string `json:"ExecutablePath"`
		CreationDate   string `json:"CreationDate"`
	}
	out, err := runPowerShell(`Get-CimInstance Win32_Process -Filter "name='multisend-agent.exe'" | Select-Object ProcessId,ExecutablePath,CreationDate | ConvertTo-Json -Depth 4 -Compress`)
	if err != nil {
		r.Skip("could not query process list")
		fmt.Println()
		return
	}
	plist := decodeJSONList[proc](out)
	if len(plist) == 0 {
		r.Skip("multisend-agent is not running")
		fmt.Println()
		return
	}
	external := make([]proc, 0, len(plist))
	for _, p := range plist {
		if p.ProcessId == currentPID {
			r.Skip(fmt.Sprintf("PID=%d current doctor process", p.ProcessId))
			continue
		}
		external = append(external, p)
		r.Pass(fmt.Sprintf("PID=%d Path=%s Start=%s", p.ProcessId, p.ExecutablePath, p.CreationDate))
	}
	if len(external) == 0 {
		r.Skip("agent listener not running")
	}
	if len(external) > 1 {
		r.Warn("more than one multisend-agent process is running")
	}
	fmt.Println()
}

func testLocalAPI(r *doctorReporter, cfg config.Config, rt runtimeState) {
	fmt.Println("[Local API]")
	port := cfg.SelectedPorts.LocalAPI
	if rt.LocalAPIPort > 0 {
		port = rt.LocalAPIPort
	}
	if port == 0 {
		port = 56221
	}
	urls := []string{
		fmt.Sprintf("http://127.0.0.1:%d/health", port),
		fmt.Sprintf("http://127.0.0.1:%d/peers", port),
		fmt.Sprintf("http://127.0.0.1:%d/jobs", port),
	}
	client := &http.Client{Timeout: 3 * time.Second}
	for _, u := range urls {
		resp, err := client.Get(u)
		if err != nil {
			r.Warn("offline: " + u)
			continue
		}
		_ = resp.Body.Close()
		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			r.Pass(fmt.Sprintf("%s -> %d", u, resp.StatusCode))
		} else {
			r.Warn(fmt.Sprintf("%s -> %d", u, resp.StatusCode))
		}
	}
	fmt.Println()
}

func printFirewallChecks(r *doctorReporter) {
	fmt.Println("[Firewall]")
	type fwRow struct {
		DisplayName   string `json:"DisplayName"`
		Enabled       string `json:"Enabled"`
		Profile       string `json:"Profile"`
		Direction     string `json:"Direction"`
		Action        string `json:"Action"`
		Protocol      string `json:"Protocol"`
		LocalPort     string `json:"LocalPort"`
		RemoteAddress string `json:"RemoteAddress"`
	}
	script := `$names=@('MultiSend Agent TCP 56200-56210','MultiSend Discovery UDP 56211-56220','MultiSend Agent TCP 55202','MultiSend Discovery UDP 55203');$out=@();foreach($n in $names){$r=Get-NetFirewallRule -DisplayName $n -ErrorAction SilentlyContinue;if($r){$p=$r|Get-NetFirewallPortFilter;$a=$r|Get-NetFirewallAddressFilter;$out += [pscustomobject]@{DisplayName=$r.DisplayName;Enabled=[string]$r.Enabled;Profile=[string]$r.Profile;Direction=[string]$r.Direction;Action=[string]$r.Action;Protocol=[string]$p.Protocol;LocalPort=[string]$p.LocalPort;RemoteAddress=[string]$a.RemoteAddress}}};$out|ConvertTo-Json -Depth 5 -Compress`
	out, err := runPowerShell(script)
	if err != nil {
		r.Skip("firewall query unavailable")
		fmt.Println()
		return
	}
	rows := decodeJSONList[fwRow](out)
	if len(rows) == 0 {
		r.Warn("firewall rules not found")
		fmt.Println()
		return
	}
	for _, row := range rows {
		r.Pass(fmt.Sprintf("%s Enabled=%s Profile=%s Direction=%s Action=%s Protocol=%s LocalPort=%s RemoteAddress=%s", row.DisplayName, row.Enabled, row.Profile, row.Direction, row.Action, row.Protocol, row.LocalPort, row.RemoteAddress))
		if !strings.EqualFold(strings.TrimSpace(row.RemoteAddress), "LocalSubnet") {
			r.Warn(fmt.Sprintf("%s remote address is not LocalSubnet (%s)", row.DisplayName, row.RemoteAddress))
		}
		if strings.Contains(strings.ToLower(row.Profile), "public") {
			r.Warn(fmt.Sprintf("%s includes Public profile", row.DisplayName))
		}
		if strings.EqualFold(strings.TrimSpace(row.RemoteAddress), "Any") || strings.EqualFold(strings.TrimSpace(row.RemoteAddress), "*") {
			r.Warn(fmt.Sprintf("%s allows Any remote address", row.DisplayName))
		}
	}
	fmt.Println()
}

func printAutostart(r *doctorReporter) {
	fmt.Println("[Autostart]")
	checks := []struct {
		label  string
		hive   string
		subKey string
	}{
		{"HKLM Run", "HKLM", `Software\Microsoft\Windows\CurrentVersion\Run`},
		{"HKCU Run", "HKCU", `Software\Microsoft\Windows\CurrentVersion\Run`},
	}
	for _, c := range checks {
		val, ok := regQueryValue(c.hive, c.subKey, "MultiSendAgent")
		if ok {
			r.Pass(fmt.Sprintf("%s MultiSendAgent=%s", c.label, val))
		} else {
			r.Skip(c.label + " MultiSendAgent not set")
		}
	}
	fmt.Println()
}

func printExplorerContext(r *doctorReporter) {
	fmt.Println("[Explorer context menu]")
	keys := []struct {
		label string
		hive  string
		path  string
	}{
		{"HKLM\\Software\\Classes\\*\\shell\\MultiSend", "HKLM", `Software\Classes\*\shell\MultiSend`},
		{"HKCU\\Software\\Classes\\*\\shell\\MultiSend", "HKCU", `Software\Classes\*\shell\MultiSend`},
		{"HKCR\\*\\shell\\MultiSend", "HKCR", `*\shell\MultiSend`},
	}
	for _, k := range keys {
		mui, ok := regQueryValue(k.hive, k.path, "MUIVerb")
		if !ok {
			r.Skip(k.label)
			continue
		}
		icon, _ := regQueryValue(k.hive, k.path, "Icon")
		cmd, _ := regQueryDefault(k.hive, k.path+`\command`)
		r.Pass(fmt.Sprintf("%s MUIVerb=%s Icon=%s", k.label, mui, icon))
		if cmd != "" {
			r.Pass(fmt.Sprintf("%s command=%s", k.label, cmd))
			if strings.Contains(strings.ToLower(cmd), strings.ToLower(`multisend-launcher.ps1`)) {
				r.Pass("command points to multisend-launcher.ps1")
			} else {
				r.Warn("command does not point to multisend-launcher.ps1")
			}
		} else {
			r.Warn(fmt.Sprintf("%s command missing", k.label))
		}
	}
	fmt.Println()
}

func printInterfaces(r *doctorReporter, cfg config.Config) {
	fmt.Println("[Network interfaces]")
	rows := ifmonitor.DetectUsableInterfaces(cfg)
	if len(rows) == 0 {
		r.Skip("no interfaces found")
		fmt.Println()
		return
	}
	for _, n := range rows {
		if n.Usable {
			r.Pass(fmt.Sprintf("%s | type=%s | IPv4=%s | usable", n.Name, n.Type, n.IPv4))
		} else {
			r.Skip(fmt.Sprintf("%s | type=%s | ignored (%s)", n.Name, n.Type, n.IgnoreReason))
		}
	}
	fmt.Println()
}

func printSummary(r *doctorReporter) {
	fmt.Println("[Summary]")
	fmt.Printf("PASS count: %d\n", r.pass)
	fmt.Printf("WARN count: %d\n", r.warn)
	fmt.Printf("FAIL count: %d\n", r.fail)
	fmt.Printf("SKIP count: %d\n", r.skip)
	result := "HEALTHY"
	if r.fail > 0 {
		result = "BROKEN"
	} else if r.warn > 0 {
		result = "DEGRADED"
	}
	fmt.Printf("result: %s\n", result)
	fmt.Println("=== DONE ===")
}

func readConfigForDoctor(path string) (config.Config, bool, []byte, bool, error) {
	if path == "" {
		return config.Config{}, false, nil, false, fmt.Errorf("config path unavailable")
	}
	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return config.Config{}, false, nil, false, nil
		}
		return config.Config{}, true, nil, false, err
	}
	hasBOM := len(b) >= 3 && b[0] == 0xEF && b[1] == 0xBB && b[2] == 0xBF
	clean := b
	if hasBOM {
		clean = b[3:]
	}
	var cfg config.Config
	if err := json.Unmarshal(clean, &cfg); err != nil {
		return config.Config{}, true, clean, hasBOM, err
	}
	return cfg, true, clean, hasBOM, nil
}

func fileVersion(path string) string {
	script := fmt.Sprintf(`(Get-Item '%s').VersionInfo.FileVersion`, strings.ReplaceAll(path, `'`, `''`))
	out, err := runPowerShell(script)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func runPowerShell(script string) ([]byte, error) {
	cmd := exec.Command("powershell.exe", "-NoProfile", "-Command", script)
	var out bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err != nil {
		if stderr.Len() > 0 {
			return nil, fmt.Errorf("%s", strings.TrimSpace(stderr.String()))
		}
		return nil, err
	}
	return bytes.TrimSpace(out.Bytes()), nil
}

func decodeJSONList[T any](b []byte) []T {
	b = bytes.TrimSpace(b)
	if len(b) == 0 || bytes.Equal(b, []byte("null")) {
		return nil
	}
	var arr []T
	if err := json.Unmarshal(b, &arr); err == nil {
		return arr
	}
	var single T
	if err := json.Unmarshal(b, &single); err == nil {
		return []T{single}
	}
	return nil
}

func regQueryValue(hive, subKey, valueName string) (string, bool) {
	native := hive + `\` + subKey
	args := []string{"query", native, "/v", valueName}
	out, err := runReg(args...)
	if err != nil {
		return "", false
	}
	return parseRegValue(out, valueName)
}

func regQueryDefault(hive, subKey string) (string, bool) {
	native := hive + `\` + subKey
	out, err := runReg("query", native, "/ve")
	if err != nil {
		return "", false
	}
	return parseRegDefault(out)
}

func runReg(args ...string) ([]byte, error) {
	cmd := exec.Command("reg.exe", args...)
	var out bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err != nil {
		if stderr.Len() > 0 {
			return nil, fmt.Errorf("%s", strings.TrimSpace(stderr.String()))
		}
		return nil, err
	}
	return out.Bytes(), nil
}

func parseRegValue(out []byte, valueName string) (string, bool) {
	needle := strings.ToLower(valueName)
	for _, ln := range strings.Split(string(out), "\n") {
		l := strings.TrimSpace(ln)
		if l == "" {
			continue
		}
		if strings.Contains(strings.ToLower(l), needle) && strings.Contains(l, "REG_") {
			parts := strings.Fields(l)
			if len(parts) >= 3 {
				return strings.Join(parts[2:], " "), true
			}
		}
	}
	return "", false
}

func parseRegDefault(out []byte) (string, bool) {
	for _, ln := range strings.Split(string(out), "\n") {
		l := strings.TrimSpace(ln)
		if l == "" || strings.HasPrefix(strings.ToUpper(l), "HKEY_") {
			continue
		}
		if strings.Contains(l, "REG_") {
			parts := strings.Fields(l)
			if len(parts) >= 3 {
				return strings.Join(parts[2:], " "), true
			}
		}
	}
	return "", false
}
