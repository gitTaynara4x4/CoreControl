package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"
)

const gatewayVersion = "1.1.0"

var defaultServerURL = "https://apps-corecontrol.9ywrah.easypanel.host"

type Config struct {
	ServerURL       string `json:"server_url"`
	EnrollmentToken string `json:"enrollment_token,omitempty"`
	AgentSecret     string `json:"agent_secret,omitempty"`
	DeviceID        int    `json:"device_id,omitempty"`
}

type enrollRequest struct {
	EnrollmentToken string `json:"enrollment_token"`
	DeviceKind      string `json:"device_kind"`
	DeviceUID       string `json:"device_uid"`
	Name            string `json:"name"`
	Hostname        string `json:"hostname"`
	OSName          string `json:"os_name"`
	OSVersion       string `json:"os_version"`
	AgentVersion    string `json:"agent_version"`
}

type enrollResponse struct {
	DeviceID    int    `json:"device_id"`
	AgentSecret string `json:"agent_secret"`
}

type pendingCommand struct {
	ID      int             `json:"id"`
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload"`
}

type commandPollResponse struct {
	Command *pendingCommand `json:"command"`
}

type commandResultRequest struct {
	DeviceUID string                 `json:"device_uid"`
	OK        bool                   `json:"ok"`
	Result    map[string]interface{} `json:"result,omitempty"`
	Error     string                 `json:"error,omitempty"`
}

type wakePeerPayload struct {
	MACAddress string `json:"mac_address"`
}

type Gateway struct {
	cfg        Config
	configPath string
	deviceUID  string
	hostname   string
	client     *http.Client
	logger     *log.Logger
}

func main() {
	runMode := flag.Bool("run", false, "executa o gateway instalado")
	installToken := flag.String("install-token", "", "autorização de instalação")
	server := flag.String("server", defaultServerURL, "URL da VPS CoreControl")
	uninstall := flag.Bool("uninstall", false, "remove o gateway instalado")
	flag.Parse()

	if *uninstall {
		if err := uninstallGateway(); err != nil {
			showMessage("CoreControl Gateway", "Não foi possível remover o Gateway: "+err.Error(), true)
			os.Exit(1)
		}
		showMessage("CoreControl Gateway", "Gateway removido.", false)
		return
	}

	token := strings.TrimSpace(*installToken)
	if token == "" {
		token = tokenFromExecutableName()
	}

	if runtime.GOOS == "windows" && !*runMode {
		if token == "" {
			showMessage("CoreControl Gateway", "Este instalador não possui uma autorização válida. Gere o Gateway novamente no CoreControl.", true)
			os.Exit(2)
		}
		if err := installGateway(token, normalizeServerURL(*server)); err != nil {
			showMessage("CoreControl Gateway", "Não foi possível instalar o Gateway: "+err.Error(), true)
			os.Exit(1)
		}
		return
	}

	cfgPath := gatewayConfigPath()
	cfg, err := loadConfig(cfgPath)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		fatalToLog(cfgPath, fmt.Errorf("configuração inválida: %w", err))
	}
	if cfg.ServerURL == "" {
		cfg.ServerURL = normalizeServerURL(*server)
	}
	if cfg.EnrollmentToken == "" && cfg.AgentSecret == "" && token != "" {
		cfg.EnrollmentToken = token
	}
	if cfg.EnrollmentToken == "" && cfg.AgentSecret == "" {
		fatalToLog(cfgPath, errors.New("Gateway sem autorização de instalação"))
	}
	if err := saveConfig(cfgPath, cfg); err != nil {
		fatalToLog(cfgPath, err)
	}

	logger := openLogger(cfgPath)
	gw := &Gateway{
		cfg:        cfg,
		configPath: cfgPath,
		client:     &http.Client{Timeout: 20 * time.Second},
		logger:     logger,
	}
	gw.hostname, _ = os.Hostname()
	gw.deviceUID = stableGatewayUID(gw.hostname)
	logger.Printf("CoreControl Gateway %s iniciado em %s", gatewayVersion, gw.hostname)
	if err := gw.run(); err != nil {
		logger.Printf("Gateway encerrado: %v", err)
		time.Sleep(5 * time.Second)
		os.Exit(1)
	}
}

func (g *Gateway) run() error {
	for g.cfg.AgentSecret == "" {
		if err := g.enroll(); err != nil {
			g.logger.Printf("vinculação pendente: %v", err)
			time.Sleep(10 * time.Second)
			continue
		}
	}
	if err := g.sendHeartbeat(); err != nil {
		g.logger.Printf("heartbeat inicial: %v", err)
	}

	heartbeat := time.NewTicker(30 * time.Second)
	poll := time.NewTicker(3 * time.Second)
	defer heartbeat.Stop()
	defer poll.Stop()

	for {
		select {
		case <-heartbeat.C:
			if err := g.sendHeartbeat(); err != nil {
				g.logger.Printf("heartbeat: %v", err)
			}
		case <-poll.C:
			if err := g.pollCommand(); err != nil {
				g.logger.Printf("fila de comandos: %v", err)
			}
		}
	}
}

func (g *Gateway) enroll() error {
	payload := enrollRequest{
		EnrollmentToken: g.cfg.EnrollmentToken,
		DeviceKind:      "gateway",
		DeviceUID:       g.deviceUID,
		Name:            "CoreControl Box - " + g.hostname,
		Hostname:        g.hostname,
		OSName:          "CoreControl Box " + runtime.GOOS,
		OSVersion:       runtime.GOOS + "/" + runtime.GOARCH,
		AgentVersion:    gatewayVersion,
	}
	var result enrollResponse
	if err := g.postJSON("/api/agent/enroll", payload, "", &result); err != nil {
		return fmt.Errorf("vinculação do Gateway falhou: %w", err)
	}
	if result.AgentSecret == "" {
		return errors.New("servidor não devolveu credencial do Gateway")
	}
	g.cfg.AgentSecret = result.AgentSecret
	g.cfg.DeviceID = result.DeviceID
	g.cfg.EnrollmentToken = ""
	if err := saveConfig(g.configPath, g.cfg); err != nil {
		return fmt.Errorf("vinculado, mas a credencial não pôde ser salva: %w", err)
	}
	g.logger.Printf("Gateway vinculado; device_id=%d", result.DeviceID)
	return nil
}

func (g *Gateway) sendHeartbeat() error {
	ipLocal, cidrs := localNetworkInfo()
	primary := ""
	if len(cidrs) > 0 {
		primary = cidrs[0]
	}
	payload := map[string]interface{}{
		"device_uid": g.deviceUID,
		"ip_local":   ipLocal,
		"extra": map[string]interface{}{
			"agent_version":     gatewayVersion,
			"gateway_mode":      true,
			"wol_relay_capable": true,
			"network_cidr":      primary,
			"network_cidrs":     cidrs,
			"runtime":           runtime.GOOS + "/" + runtime.GOARCH,
		},
	}
	return g.postJSON("/api/agent/telemetry", payload, g.cfg.AgentSecret, nil)
}

func (g *Gateway) pollCommand() error {
	var response commandPollResponse
	path := "/api/agent/commands/next?device_uid=" + url.QueryEscape(g.deviceUID)
	if err := g.getJSON(path, g.cfg.AgentSecret, &response); err != nil {
		return err
	}
	if response.Command == nil {
		return nil
	}

	result, execErr := executeGatewayCommand(*response.Command)
	request := commandResultRequest{
		DeviceUID: g.deviceUID,
		OK:        execErr == nil,
		Result:    result,
	}
	if execErr != nil {
		request.Error = execErr.Error()
	}
	resultPath := fmt.Sprintf("/api/agent/commands/%d/result", response.Command.ID)
	if err := g.postJSON(resultPath, request, g.cfg.AgentSecret, nil); err != nil {
		return err
	}
	if execErr != nil {
		g.logger.Printf("comando %d falhou: %v", response.Command.ID, execErr)
	} else {
		g.logger.Printf("comando %d concluído", response.Command.ID)
	}
	return nil
}

func executeGatewayCommand(command pendingCommand) (map[string]interface{}, error) {
	if command.Type != "power.wake_peer" {
		return nil, fmt.Errorf("comando não permitido no Gateway: %s", command.Type)
	}
	var payload wakePeerPayload
	if len(command.Payload) > 0 {
		if err := json.Unmarshal(command.Payload, &payload); err != nil {
			return nil, fmt.Errorf("payload inválido: %w", err)
		}
	}
	return sendWakeOnLAN(payload.MACAddress)
}

func sendWakeOnLAN(macText string) (map[string]interface{}, error) {
	mac, err := net.ParseMAC(strings.TrimSpace(macText))
	if err != nil || len(mac) != 6 {
		return nil, errors.New("endereço MAC inválido")
	}
	packet := make([]byte, 6+16*len(mac))
	for i := 0; i < 6; i++ {
		packet[i] = 0xFF
	}
	for i := 0; i < 16; i++ {
		copy(packet[6+i*len(mac):], mac)
	}

	broadcasts := localBroadcastAddresses()
	if len(broadcasts) == 0 {
		broadcasts = []string{"255.255.255.255"}
	}
	sent := 0
	var lastErr error
	for round := 0; round < 3; round++ {
		for _, host := range broadcasts {
			for _, port := range []int{9, 7} {
				conn, dialErr := net.DialUDP("udp4", nil, &net.UDPAddr{IP: net.ParseIP(host), Port: port})
				if dialErr != nil {
					lastErr = dialErr
					continue
				}
				_ = conn.SetWriteDeadline(time.Now().Add(2 * time.Second))
				_, writeErr := conn.Write(packet)
				_ = conn.Close()
				if writeErr != nil {
					lastErr = writeErr
					continue
				}
				sent++
			}
		}
		if round < 2 {
			time.Sleep(100 * time.Millisecond)
		}
	}
	result := map[string]interface{}{
		"mac_address":  strings.ToLower(mac.String()),
		"broadcasts":   broadcasts,
		"packets_sent": sent,
		"gateway":      true,
	}
	if sent == 0 {
		if lastErr == nil {
			lastErr = errors.New("nenhum Magic Packet foi enviado")
		}
		return result, lastErr
	}
	return result, nil
}

func localNetworkInfo() (string, []string) {
	seen := map[string]struct{}{}
	ipLocal := ""
	interfaces, err := net.Interfaces()
	if err == nil {
		for _, iface := range interfaces {
			if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
				continue
			}
			addrs, _ := iface.Addrs()
			for _, addr := range addrs {
				ipNet, ok := addr.(*net.IPNet)
				if !ok {
					continue
				}
				ip := ipNet.IP.To4()
				if ip == nil || !isPrivateIPv4(ip) {
					continue
				}
				if ipLocal == "" {
					ipLocal = ip.String()
				}
				ones, bits := ipNet.Mask.Size()
				if ones < 0 || bits != 32 {
					continue
				}
				networkIP := ip.Mask(ipNet.Mask)
				cidr := fmt.Sprintf("%s/%d", networkIP.String(), ones)
				seen[cidr] = struct{}{}
			}
		}
	}
	cidrs := make([]string, 0, len(seen))
	for cidr := range seen {
		cidrs = append(cidrs, cidr)
	}
	sort.Strings(cidrs)
	return ipLocal, cidrs
}

func localBroadcastAddresses() []string {
	seen := map[string]struct{}{"255.255.255.255": {}}
	interfaces, err := net.Interfaces()
	if err == nil {
		for _, iface := range interfaces {
			if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
				continue
			}
			addrs, _ := iface.Addrs()
			for _, addr := range addrs {
				ipNet, ok := addr.(*net.IPNet)
				if !ok {
					continue
				}
				ip := ipNet.IP.To4()
				if ip == nil || !isPrivateIPv4(ip) {
					continue
				}
				mask := ipNet.Mask
				ones, bits := mask.Size()
				if ones < 0 || bits != 32 || ones >= 32 {
					continue
				}
				broadcast := make(net.IP, net.IPv4len)
				for i := 0; i < net.IPv4len; i++ {
					broadcast[i] = ip[i] | ^mask[i]
				}
				seen[broadcast.String()] = struct{}{}
			}
		}
	}
	values := make([]string, 0, len(seen))
	for value := range seen {
		values = append(values, value)
	}
	sort.Strings(values)
	return values
}

func isPrivateIPv4(ip net.IP) bool {
	v := ip.To4()
	if v == nil {
		return false
	}
	return v[0] == 10 || (v[0] == 172 && v[1] >= 16 && v[1] <= 31) || (v[0] == 192 && v[1] == 168)
}

func stableGatewayUID(hostname string) string {
	parts := []string{strings.ToLower(strings.TrimSpace(hostname)), runtime.GOOS, runtime.GOARCH}
	ifaces, _ := net.Interfaces()
	for _, iface := range ifaces {
		if len(iface.HardwareAddr) > 0 {
			parts = append(parts, strings.ToLower(iface.HardwareAddr.String()))
		}
	}
	sort.Strings(parts)
	sum := sha256.Sum256([]byte(strings.Join(parts, "|")))
	return "gateway-" + hex.EncodeToString(sum[:12])
}

func tokenFromExecutableName() string {
	exe, err := os.Executable()
	if err != nil {
		return ""
	}
	name := filepath.Base(exe)
	lower := strings.ToLower(name)
	marker := "corecontrolgateway--"
	idx := strings.Index(lower, marker)
	if idx < 0 {
		return ""
	}
	token := name[idx+len(marker):]
	token = strings.TrimSuffix(token, filepath.Ext(token))
	return strings.TrimSpace(token)
}

func normalizeServerURL(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		value = defaultServerURL
	}
	return strings.TrimRight(value, "/")
}

func gatewayConfigPath() string {
	if runtime.GOOS == "windows" {
		base := os.Getenv("ProgramData")
		if base == "" {
			base = `C:\ProgramData`
		}
		return filepath.Join(base, "CoreControl", "Gateway", "gateway-config.json")
	}
	return "/var/lib/corecontrol-gateway/gateway-config.json"
}

func loadConfig(path string) (Config, error) {
	var cfg Config
	raw, err := os.ReadFile(path)
	if err != nil {
		return cfg, err
	}
	err = json.Unmarshal(raw, &cfg)
	cfg.ServerURL = normalizeServerURL(cfg.ServerURL)
	return cfg, err
}

func saveConfig(path string, cfg Config) error {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	cfg.ServerURL = normalizeServerURL(cfg.ServerURL)
	raw, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, raw, 0600)
}

func openLogger(configPath string) *log.Logger {
	logPath := filepath.Join(filepath.Dir(configPath), "gateway.log")
	_ = os.MkdirAll(filepath.Dir(logPath), 0700)
	file, err := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		return log.New(io.Discard, "gateway ", log.LstdFlags|log.LUTC)
	}
	return log.New(file, "gateway ", log.LstdFlags|log.LUTC)
}

func fatalToLog(configPath string, err error) {
	logger := openLogger(configPath)
	logger.Printf("ERRO FATAL: %v", err)
	showMessage("CoreControl Gateway", err.Error(), true)
	os.Exit(1)
}

func (g *Gateway) getJSON(path, bearer string, output interface{}) error {
	req, err := http.NewRequest(http.MethodGet, g.cfg.ServerURL+path, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "CoreControlGateway/"+gatewayVersion)
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	resp, err := g.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("servidor respondeu %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	if output != nil && len(body) > 0 {
		return json.Unmarshal(body, output)
	}
	return nil
}

func (g *Gateway) postJSON(path string, payload interface{}, bearer string, output interface{}) error {
	raw, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	req, err := http.NewRequest(http.MethodPost, g.cfg.ServerURL+path, bytes.NewReader(raw))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "CoreControlGateway/"+gatewayVersion)
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	resp, err := g.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("servidor respondeu %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	if output != nil && len(body) > 0 {
		return json.Unmarshal(body, output)
	}
	return nil
}
