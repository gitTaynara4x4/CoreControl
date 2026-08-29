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
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

const agentVersion = "0.8.0"

type Config struct {
	ServerURL         string `json:"server_url"`
	EnrollmentToken   string `json:"enrollment_token,omitempty"`
	AgentSecret       string `json:"agent_secret,omitempty"`
	DeviceID          int    `json:"device_id,omitempty"`
	IntervalSeconds   int    `json:"interval_seconds"`
	AllowInsecureHTTP bool   `json:"allow_insecure_http"`
	Name              string `json:"name,omitempty"`
	Sector            string `json:"sector,omitempty"`
	Location          string `json:"location,omitempty"`
}

type MachineSnapshot struct {
	DeviceUID            string   `json:"device_uid"`
	Hostname             string   `json:"hostname"`
	Manufacturer         string   `json:"manufacturer"`
	Model                string   `json:"model"`
	SerialNumber         string   `json:"serial_number"`
	OSName               string   `json:"os_name"`
	OSVersion            string   `json:"os_version"`
	CPUPercent           *float64 `json:"cpu_percent"`
	MemoryPercent        *float64 `json:"memory_percent"`
	MemoryUsedGB         *float64 `json:"memory_used_gb"`
	MemoryTotalGB        *float64 `json:"memory_total_gb"`
	DiskPercent          *float64 `json:"disk_percent"`
	DiskFreeGB           *float64 `json:"disk_free_gb"`
	DiskTotalGB          *float64 `json:"disk_total_gb"`
	TemperatureC         *float64 `json:"temperature_c"`
	UptimeSeconds        *int64   `json:"uptime_seconds"`
	IPLocal              string   `json:"ip_local"`
	NetworkName          string   `json:"network_name"`
	DefenderActive       *bool    `json:"defender_active"`
	FirewallActive       *bool    `json:"firewall_active"`
	Profile              string   `json:"profile"`
	RemoteAgentInstalled bool     `json:"remote_agent_installed"`
	RemoteAgentRunning   bool     `json:"remote_agent_running"`
	RemoteServiceName    string   `json:"remote_service_name"`
}

type enrollRequest struct {
	EnrollmentToken string `json:"enrollment_token"`
	DeviceUID       string `json:"device_uid"`
	Name            string `json:"name"`
	Hostname        string `json:"hostname"`
	Sector          string `json:"sector,omitempty"`
	Location        string `json:"location,omitempty"`
	Manufacturer    string `json:"manufacturer,omitempty"`
	Model           string `json:"model,omitempty"`
	SerialNumber    string `json:"serial_number,omitempty"`
	OSName          string `json:"os_name,omitempty"`
	OSVersion       string `json:"os_version,omitempty"`
	AgentVersion    string `json:"agent_version"`
}

type enrollResponse struct {
	DeviceID    int    `json:"device_id"`
	AgentSecret string `json:"agent_secret"`
}

type telemetryRequest struct {
	DeviceUID      string                 `json:"device_uid"`
	CPUPercent     *float64               `json:"cpu_percent,omitempty"`
	MemoryPercent  *float64               `json:"memory_percent,omitempty"`
	MemoryUsedGB   *float64               `json:"memory_used_gb,omitempty"`
	MemoryTotalGB  *float64               `json:"memory_total_gb,omitempty"`
	DiskPercent    *float64               `json:"disk_percent,omitempty"`
	DiskFreeGB     *float64               `json:"disk_free_gb,omitempty"`
	DiskTotalGB    *float64               `json:"disk_total_gb,omitempty"`
	TemperatureC   *float64               `json:"temperature_c,omitempty"`
	UptimeSeconds  *int64                 `json:"uptime_seconds,omitempty"`
	IPLocal        string                 `json:"ip_local,omitempty"`
	NetworkName    string                 `json:"network_name,omitempty"`
	DefenderActive *bool                  `json:"defender_active,omitempty"`
	FirewallActive *bool                  `json:"firewall_active,omitempty"`
	Profile        string                 `json:"profile,omitempty"`
	Extra          map[string]interface{} `json:"extra,omitempty"`
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

type Agent struct {
	configPath string
	cfg        Config
	client     *http.Client
	logger     *log.Logger
	mu         sync.Mutex
}

func main() {
	releaseInstance, instanceErr := acquireSingleInstance("Global\\CoreTunerAgent")
	if instanceErr != nil {
		fallbackLog("outra instância do CoreControl Agent já está em execução: %v", instanceErr)
		return
	}
	defer releaseInstance()

	configFlag := flag.String("config", "", "caminho do arquivo de configuração")
	onceFlag := flag.Bool("once", false, "coleta e envia apenas uma vez")
	flag.Parse()

	configPath, err := resolveConfigPath(*configFlag)
	if err != nil {
		fallbackLog("não foi possível resolver configuração: %v", err)
		return
	}
	cfg, err := loadConfig(configPath)
	if err != nil {
		fallbackLog("configuração inválida: %v", err)
		return
	}
	logger, closer := newLogger(configPath)
	if closer != nil {
		defer closer.Close()
	}
	agent := &Agent{
		configPath: configPath,
		cfg:        cfg,
		client:     &http.Client{Timeout: 25 * time.Second},
		logger:     logger,
	}
	if err := agent.validateServerURL(); err != nil {
		logger.Printf("configuração recusada: %v", err)
		return
	}
	logger.Printf("CoreControl Agent %s iniciado", agentVersion)

	if *onceFlag {
		if err := agent.runCycle(); err != nil {
			logger.Printf("falha: %v", err)
		}
		return
	}

	interval := time.Duration(agent.cfg.IntervalSeconds) * time.Second
	if interval < 15*time.Second {
		interval = 30 * time.Second
	}
	backoff := 5 * time.Second
	for {
		if err := agent.runCycle(); err != nil {
			logger.Printf("ciclo não enviado: %v", err)
			time.Sleep(backoff)
			if backoff < 5*time.Minute {
				backoff *= 2
			}
			continue
		}
		backoff = 5 * time.Second
		time.Sleep(interval)
	}
}

func resolveConfigPath(explicit string) (string, error) {
	if explicit != "" {
		return filepath.Abs(explicit)
	}
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	return filepath.Join(filepath.Dir(exe), "agent-config.json"), nil
}

func loadConfig(path string) (Config, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("não foi possível abrir %s: %w", path, err)
	}
	raw = bytes.TrimPrefix(raw, []byte{0xEF, 0xBB, 0xBF})
	var cfg Config
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return Config{}, err
	}
	cfg.ServerURL = strings.TrimRight(strings.TrimSpace(cfg.ServerURL), "/")
	if cfg.ServerURL == "" {
		return Config{}, errors.New("server_url é obrigatório")
	}
	if cfg.IntervalSeconds == 0 {
		cfg.IntervalSeconds = 30
	}
	return cfg, nil
}

func saveConfig(path string, cfg Config) error {
	raw, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	temp := path + ".tmp"
	backup := path + ".bak"
	if err := os.WriteFile(temp, raw, 0600); err != nil {
		return err
	}

	if runtime.GOOS != "windows" {
		return os.Rename(temp, path)
	}

	_ = os.Remove(backup)
	hadOriginal := false
	if _, statErr := os.Stat(path); statErr == nil {
		hadOriginal = true
		if err := os.Rename(path, backup); err != nil {
			_ = os.Remove(temp)
			return err
		}
	}
	if err := os.Rename(temp, path); err != nil {
		if hadOriginal {
			_ = os.Rename(backup, path)
		}
		return err
	}
	_ = os.Remove(backup)
	return nil
}

func newLogger(configPath string) (*log.Logger, *os.File) {
	logDir := filepath.Join(filepath.Dir(configPath), "logs")
	_ = os.MkdirAll(logDir, 0750)
	file, err := os.OpenFile(filepath.Join(logDir, "agent.log"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0640)
	if err != nil {
		return log.New(io.Discard, "", log.LstdFlags), nil
	}
	return log.New(file, "", log.LstdFlags|log.LUTC), file
}

func fallbackLog(format string, args ...interface{}) {
	_, _ = fmt.Fprintf(os.Stderr, format+"\n", args...)
}

func (a *Agent) validateServerURL() error {
	parsed, err := url.Parse(a.cfg.ServerURL)
	if err != nil || parsed.Host == "" {
		return errors.New("server_url inválido")
	}
	if parsed.Scheme == "https" {
		return nil
	}
	host := strings.Split(parsed.Host, ":")[0]
	local := host == "127.0.0.1" || host == "localhost" || host == "::1"
	if parsed.Scheme == "http" && (local || a.cfg.AllowInsecureHTTP) {
		return nil
	}
	return errors.New("HTTPS é obrigatório; HTTP só é aceito em teste local ou com allow_insecure_http=true")
}

func (a *Agent) runCycle() error {
	a.mu.Lock()
	defer a.mu.Unlock()

	snapshot, err := collectSnapshot()
	if err != nil {
		return err
	}
	if a.cfg.AgentSecret == "" {
		if a.cfg.EnrollmentToken == "" {
			return errors.New("o agente ainda não foi vinculado e não há enrollment_token")
		}
		name := strings.TrimSpace(a.cfg.Name)
		if name == "" {
			name = snapshot.Hostname
		}
		req := enrollRequest{
			EnrollmentToken: a.cfg.EnrollmentToken,
			DeviceUID:       snapshot.DeviceUID,
			Name:            name,
			Hostname:        snapshot.Hostname,
			Sector:          a.cfg.Sector,
			Location:        a.cfg.Location,
			Manufacturer:    snapshot.Manufacturer,
			Model:           snapshot.Model,
			SerialNumber:    snapshot.SerialNumber,
			OSName:          snapshot.OSName,
			OSVersion:       snapshot.OSVersion,
			AgentVersion:    agentVersion,
		}
		var resp enrollResponse
		if err := a.postJSON("/api/agent/enroll", req, "", &resp); err != nil {
			return fmt.Errorf("vinculação falhou: %w", err)
		}
		if resp.AgentSecret == "" {
			return errors.New("servidor não devolveu credencial do agente")
		}
		a.cfg.AgentSecret = resp.AgentSecret
		a.cfg.DeviceID = resp.DeviceID
		a.cfg.EnrollmentToken = ""
		if err := saveConfig(a.configPath, a.cfg); err != nil {
			return fmt.Errorf("vinculado, mas não foi possível salvar credencial: %w", err)
		}
		a.logger.Printf("computador vinculado com sucesso; device_id=%d", resp.DeviceID)
	}

	activity := collectForegroundActivity()
	payload := telemetryRequest{
		DeviceUID:      snapshot.DeviceUID,
		CPUPercent:     snapshot.CPUPercent,
		MemoryPercent:  snapshot.MemoryPercent,
		MemoryUsedGB:   snapshot.MemoryUsedGB,
		MemoryTotalGB:  snapshot.MemoryTotalGB,
		DiskPercent:    snapshot.DiskPercent,
		DiskFreeGB:     snapshot.DiskFreeGB,
		DiskTotalGB:    snapshot.DiskTotalGB,
		TemperatureC:   snapshot.TemperatureC,
		UptimeSeconds:  snapshot.UptimeSeconds,
		IPLocal:        snapshot.IPLocal,
		NetworkName:    snapshot.NetworkName,
		DefenderActive: snapshot.DefenderActive,
		FirewallActive: snapshot.FirewallActive,
		Profile:        snapshot.Profile,
		Extra: map[string]interface{}{
			"agent_version":          agentVersion,
			"activity":               activity,
			"runtime":                runtime.GOOS + "/" + runtime.GOARCH,
			"remote_agent_installed": snapshot.RemoteAgentInstalled,
			"remote_agent_running":   snapshot.RemoteAgentRunning,
			"remote_service_name":    snapshot.RemoteServiceName,
		},
	}
	if err := a.postJSON("/api/agent/telemetry", payload, a.cfg.AgentSecret, nil); err != nil {
		return fmt.Errorf("telemetria falhou: %w", err)
	}
	a.logger.Printf("telemetria enviada; cpu=%s ram=%s disco=%s", ptrText(snapshot.CPUPercent), ptrText(snapshot.MemoryPercent), ptrText(snapshot.DiskPercent))
	if err := a.pollAndExecuteCommand(snapshot.DeviceUID); err != nil {
		a.logger.Printf("fila de comandos: %v", err)
	}
	return nil
}

func (a *Agent) pollAndExecuteCommand(deviceUID string) error {
	var response commandPollResponse
	path := "/api/agent/commands/next?device_uid=" + url.QueryEscape(deviceUID)
	if err := a.getJSON(path, a.cfg.AgentSecret, &response); err != nil {
		return err
	}
	if response.Command == nil {
		return nil
	}

	a.logger.Printf("comando recebido; id=%d tipo=%s", response.Command.ID, response.Command.Type)
	result, execErr := executeAgentCommand(*response.Command)
	request := commandResultRequest{
		DeviceUID: deviceUID,
		OK:        execErr == nil,
		Result:    result,
	}
	if execErr != nil {
		request.Error = execErr.Error()
	}
	resultPath := fmt.Sprintf("/api/agent/commands/%d/result", response.Command.ID)
	if err := a.postJSON(resultPath, request, a.cfg.AgentSecret, nil); err != nil {
		return fmt.Errorf("não foi possível devolver resultado do comando %d: %w", response.Command.ID, err)
	}
	if execErr != nil {
		a.logger.Printf("comando finalizado com falha; id=%d erro=%v", response.Command.ID, execErr)
	} else {
		a.logger.Printf("comando finalizado; id=%d", response.Command.ID)
	}
	return nil
}

func (a *Agent) getJSON(path string, bearer string, output interface{}) error {
	req, err := http.NewRequest(http.MethodGet, a.cfg.ServerURL+path, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "CoreControlAgent/"+agentVersion)
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	resp, err := a.client.Do(req)
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

func ptrText(value *float64) string {
	if value == nil {
		return "n/d"
	}
	return fmt.Sprintf("%.0f%%", *value)
}

func (a *Agent) postJSON(path string, payload interface{}, bearer string, output interface{}) error {
	raw, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	req, err := http.NewRequest(http.MethodPost, a.cfg.ServerURL+path, bytes.NewReader(raw))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "CoreControlAgent/"+agentVersion)
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	resp, err := a.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("servidor respondeu %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	if output != nil && len(body) > 0 {
		if err := json.Unmarshal(body, output); err != nil {
			return err
		}
	}
	return nil
}

func collectSnapshot() (MachineSnapshot, error) {
	if runtime.GOOS != "windows" {
		return collectFallbackSnapshot()
	}
	return collectWindowsSnapshotNative()
}

func collectFallbackSnapshot() (MachineSnapshot, error) {
	hostname, _ := os.Hostname()
	uid := stableFallbackUID(hostname, runtime.GOOS+runtime.GOARCH)
	return MachineSnapshot{DeviceUID: uid, Hostname: hostname, OSName: runtime.GOOS, OSVersion: runtime.GOARCH, Profile: "Nenhum"}, nil
}

func stableFallbackUID(parts ...string) string {
	sum := sha256.Sum256([]byte(strings.Join(parts, "|")))
	return "fallback-" + hex.EncodeToString(sum[:16])
}
