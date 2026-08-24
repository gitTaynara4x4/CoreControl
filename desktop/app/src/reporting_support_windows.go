//go:build windows

package main

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unsafe"
)

type MetricsSnapshot struct {
	CapturedAt time.Time `json:"captured_at"`
	Health     int       `json:"health"`
	CPU        float64   `json:"cpu_percent"`
	Memory     float64   `json:"memory_percent"`
	Disk       float64   `json:"disk_percent"`
	LatencyMS  int64     `json:"latency_ms"`
	InternetOK bool      `json:"internet_ok"`
}

type OptimizationComparison struct {
	Profile string          `json:"profile"`
	At      time.Time       `json:"at"`
	Before  MetricsSnapshot `json:"before"`
	After   MetricsSnapshot `json:"after"`
}

func metricsSnapshot(s SystemInfo) MetricsSnapshot {
	return MetricsSnapshot{CapturedAt: time.Now(), Health: health(s), CPU: s.CPU, Memory: s.Memory, Disk: s.Disk, LatencyMS: s.LatencyMS, InternetOK: s.InternetOK}
}

func comparisonPath() string { return filepath.Join(dataDir(), "optimization-comparison.json") }

func saveOptimizationComparison(profile string, before, after SystemInfo) error {
	c := OptimizationComparison{Profile: profile, At: time.Now(), Before: metricsSnapshot(before), After: metricsSnapshot(after)}
	b, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dataDir(), 0755); err != nil {
		return err
	}
	return os.WriteFile(comparisonPath(), b, 0600)
}

func loadOptimizationComparison() (*OptimizationComparison, error) {
	b, err := os.ReadFile(comparisonPath())
	if err != nil {
		return nil, err
	}
	var c OptimizationComparison
	if err := json.Unmarshal(b, &c); err != nil {
		return nil, err
	}
	return &c, nil
}

func signedDelta(v float64) string {
	if v > 0 {
		return fmt.Sprintf("+%.1f", v)
	}
	return fmt.Sprintf("%.1f", v)
}

func (a *App) generateComparisonReport() {
	c, err := loadOptimizationComparison()
	if err != nil {
		message("Comparação ainda não disponível", "Aplique ou restaure um perfil de otimização primeiro. O CoreControl registrará automaticamente uma amostra antes e depois.", MB_OK|MB_ICONINFORMATION)
		return
	}
	dir := filepath.Join(dataDir(), "Relatorios")
	_ = os.MkdirAll(dir, 0755)
	path := filepath.Join(dir, "CoreControl_Antes_Depois_"+time.Now().Format("20060102_150405")+".html")
	rows := func(label, unit string, before, after float64) string {
		return fmt.Sprintf(`<tr><td>%s</td><td>%.1f%s</td><td>%.1f%s</td><td>%s%s</td></tr>`, label, before, unit, after, unit, signedDelta(after-before), unit)
	}
	body := rows("Saúde", "/100", float64(c.Before.Health), float64(c.After.Health)) +
		rows("CPU", "%", c.Before.CPU, c.After.CPU) +
		rows("Memória", "%", c.Before.Memory, c.After.Memory) +
		rows("Disco", "%", c.Before.Disk, c.After.Disk) +
		rows("Latência", " ms", float64(c.Before.LatencyMS), float64(c.After.LatencyMS))
	content := fmt.Sprintf(`<!doctype html><html lang="pt-BR"><head><meta charset="utf-8"><title>CoreControl - Antes e depois</title><style>body{font-family:Segoe UI,Arial;background:#f5f7fb;color:#17233a;margin:32px}.wrap{max-width:920px;margin:auto;background:#fff;border:1px solid #dfe6ef;border-radius:14px;padding:30px}h1{margin:0 0 6px}.muted{color:#6b7890}table{width:100%%;border-collapse:collapse;margin-top:24px}th,td{padding:12px;border-bottom:1px solid #e5eaf0;text-align:left}th{background:#f7f9fc}.note{margin-top:22px;padding:12px;background:#f7f9fc;border-radius:8px;font-size:13px;color:#60708a}</style></head><body><div class="wrap"><h1>Comparação antes e depois</h1><p class="muted">Perfil: %s • registrado em %s</p><table><thead><tr><th>Métrica</th><th>Antes</th><th>Depois</th><th>Variação</th></tr></thead><tbody>%s</tbody></table><div class="note">As medições representam amostras capturadas imediatamente antes e depois da operação. CPU, memória e latência variam naturalmente; este relatório não é um benchmark de desempenho.</div></div></body></html>`, html(c.Profile), c.At.Format("02/01/2006 15:04"), body)
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		message("Relatório", err.Error(), MB_OK|MB_ICONERROR)
		return
	}
	a.addHistory("Comparação antes/depois gerada", path)
	procShellExecute.Call(0, uintptr(unsafe.Pointer(utf16("open"))), uintptr(unsafe.Pointer(utf16(path))), 0, 0, SW_SHOWNORMAL)
}

func supportDir() string { return filepath.Join(dataDir(), "Suporte") }

func openLocalFolder(path string) {
	_ = os.MkdirAll(path, 0755)
	procShellExecute.Call(0, uintptr(unsafe.Pointer(utf16("open"))), uintptr(unsafe.Pointer(utf16(path))), 0, 0, SW_SHOWNORMAL)
}

func zipAddJSON(z *zip.Writer, name string, value any) error {
	w, err := z.Create(name)
	if err != nil {
		return err
	}
	b, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	_, err = w.Write(b)
	return err
}

func (a *App) generateSupportPackage() {
	a.mu.RLock()
	sys := a.sys
	history := append([]HistoryItem(nil), a.history...)
	user := a.user
	company := a.company
	server := a.serverURL
	centralOK := a.centralOK
	a.mu.RUnlock()
	if len(history) > 100 {
		history = history[len(history)-100:]
	}
	dir := supportDir()
	if err := os.MkdirAll(dir, 0755); err != nil {
		message("Suporte", err.Error(), MB_OK|MB_ICONERROR)
		return
	}
	path := filepath.Join(dir, "CoreControl_Suporte_"+time.Now().Format("20060102_150405")+".zip")
	f, err := os.Create(path)
	if err != nil {
		message("Suporte", err.Error(), MB_OK|MB_ICONERROR)
		return
	}
	z := zip.NewWriter(f)
	summary := map[string]any{"generated_at": time.Now(), "corecontrol_version": appVersion, "server": server, "central_connected": centralOK, "user": user.Name, "company": companyName(company)}
	err = zipAddJSON(z, "resumo.json", summary)
	if err == nil {
		err = zipAddJSON(z, "diagnostico.json", sys)
	}
	if err == nil {
		err = zipAddJSON(z, "historico-recente.json", history)
	}
	if err == nil {
		w, e := z.Create("LEIA-ME.txt")
		if e == nil {
			_, e = w.Write([]byte("Pacote técnico do CoreControl. Contém somente diagnóstico técnico e histórico do aplicativo. Não contém documentos, conversas, senhas ou o token da sessão.\r\n"))
		}
		err = e
	}
	closeErr := z.Close()
	fileErr := f.Close()
	if err == nil {
		err = closeErr
	}
	if err == nil {
		err = fileErr
	}
	if err != nil {
		_ = os.Remove(path)
		message("Suporte", "Não foi possível gerar o pacote: "+err.Error(), MB_OK|MB_ICONERROR)
		return
	}
	a.addHistory("Pacote de suporte gerado", path)
	message("Pacote de suporte pronto", "O pacote técnico foi gerado sem documentos, conversas ou senhas.\n\n"+path, MB_OK|MB_ICONINFORMATION)
	openLocalFolder(dir)
}

func cleanServerURL(v string) string { return strings.TrimRight(strings.TrimSpace(v), "/") }
