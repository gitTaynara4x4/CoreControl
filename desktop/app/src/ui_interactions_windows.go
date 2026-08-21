//go:build windows

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"
	"unsafe"
)

func (a *App) click(x, y int32) {
	if h := a.hitAt(x, y); h != nil {
		a.action(h.Action, h.Value)
	}
}

func (a *App) hitAt(x, y int32) *Hit {
	for i := range a.hits {
		if a.hits[i].Rect.contains(x, y) {
			return &a.hits[i]
		}
	}
	return nil
}

func (a *App) mouseMove(x, y int32) {
	a.mouseX, a.mouseY = x, y
	h := a.hitAt(x, y)
	active := h != nil
	var hoverRect Rect
	if active {
		hoverRect = h.Rect
	}
	changed := active != a.hoverActive || (active && hoverRect != a.hoverRect)
	a.hoverActive = active
	a.hoverRect = hoverRect
	a.setPointerCursor(active)
	if changed {
		a.invalidate()
	}
}

func (a *App) isHovered(r Rect) bool {
	return a.hoverActive && a.hoverRect == r
}

func (a *App) setPointerCursor(pointer bool) {
	id := uintptr(IDC_ARROW)
	if pointer {
		id = IDC_HAND
	}
	cursor, _, _ := procLoadCursor.Call(0, id)
	procSetCursor.Call(cursor)
}

func (a *App) isClickableControl(hwnd syscall.Handle) bool {
	if hwnd == 0 || hwnd == a.hwnd {
		return false
	}
	id, _, _ := procGetDlgCtrlID.Call(uintptr(hwnd))
	switch int(id) {
	case idLogin, idShowRegister, idForgotPassword, idRegister, idShowLogin:
		return true
	default:
		return false
	}
}

func (a *App) action(action string, value int) {
	switch action {
	case "page":
		a.page = value
		a.invalidate()
	case "logout":
		a.logout()
	case "reauth":
		a.logout()
	case "profile":
		a.profile = value
		a.invalidate()
	case "cancel-profile":
		a.profile = 0
		a.invalidate()
	case "apply-profile":
		if a.profile == 0 {
			message("CoreTuner", "Selecione um perfil primeiro.", MB_OK|MB_ICONINFORMATION)
			return
		}
		plan, err := optimizationPlan(a.profile, runningOnBattery())
		if err != nil {
			message("CoreTuner", err.Error(), MB_OK|MB_ICONERROR)
			return
		}
		if message("Confirmar otimização segura", optimizationConfirmation(plan), MB_YESNO|MB_ICONQUESTION) != IDYES {
			return
		}
		a.mu.Lock()
		if a.optimizationBusy {
			a.mu.Unlock()
			return
		}
		a.optimizationBusy = true
		a.mu.Unlock()
		a.invalidate()
		result, applyErr := applyOptimizationProfile(a.profile)
		a.mu.Lock()
		a.optimizationBusy = false
		a.mu.Unlock()
		a.refreshOptimizationSummary()
		detail := summarizeOptimizationResult(result)
		if applyErr != nil {
			if detail != "" {
				detail += "\n\n"
			}
			detail += applyErr.Error()
			a.addHistory("Otimização incompleta", detail)
			message("Otimização não concluída", detail, MB_OK|MB_ICONERROR)
			return
		}
		if result.Restored {
			a.addHistory("Configurações restauradas", detail)
			a.profile = 0
		} else {
			a.addHistory("Perfil aplicado", detail)
		}
		message("CoreTuner", detail, MB_OK|MB_ICONINFORMATION)
	case "refresh-local":
		go a.refreshLocal(false)
	case "refresh-central":
		go a.refreshCentralStatus()
	case "refresh-all":
		go a.refreshLocal(false)
		go a.refreshCentralStatus()
	case "test-internet":
		go func() { a.refreshLocal(false); a.addHistory("Teste de internet", "Teste concluído") }()
	case "test-audio":
		go func() {
			a.refreshLocal(false)
			a.addHistory("Teste de áudio", "Detecção de áudio e microfone atualizada")
		}()
	case "report":
		a.generateReport()
	case "open-web":
		a.openWeb()
	}
}
func (a *App) openWeb() {
	a.mu.RLock()
	u := a.serverURL
	a.mu.RUnlock()
	procShellExecute.Call(0, uintptr(unsafe.Pointer(utf16("open"))), uintptr(unsafe.Pointer(utf16(u+"/central"))), 0, 0, SW_SHOWNORMAL)
}

func recommendations(s SystemInfo) []string {
	var r []string
	if strings.Contains(strings.ToLower(s.DiskType), "hdd") || strings.Contains(strings.ToLower(s.DiskType), "unspecified") {
		r = append(r, "Considere instalar um SSD para reduzir a lentidão.")
	}
	if s.TotalRAMGB > 0 && s.TotalRAMGB < 7.5 {
		r = append(r, "Aumente a memória RAM para pelo menos 8 GB.")
	}
	if s.Memory > 85 {
		r = append(r, "A memória está muito ocupada; revise abas e programas abertos.")
	}
	if s.Disk > 90 {
		r = append(r, "O disco está com pouco espaço livre.")
	}
	if !s.InternetOK {
		r = append(r, "A conexão com a internet precisa ser verificada.")
	}
	if !s.AudioOK || !s.MicOK {
		r = append(r, "Verifique o headset e o dispositivo padrão do Windows.")
	}
	if len(r) == 0 {
		r = append(r, "O computador está em condições adequadas para atendimento.")
	}
	return r
}
func (a *App) generateReport() {
	a.mu.RLock()
	s := a.sys
	comp := companyName(a.company)
	a.mu.RUnlock()
	dir := filepath.Join(dataDir(), "Relatorios")
	os.MkdirAll(dir, 0755)
	path := filepath.Join(dir, "CoreTuner_Relatorio_"+time.Now().Format("20060102_150405")+".html")
	var rows strings.Builder
	for _, v := range recommendations(s) {
		fmt.Fprintf(&rows, "<li>%s</li>", html(v))
	}
	content := fmt.Sprintf(`<!doctype html><html lang="pt-BR"><head><meta charset="utf-8"><title>Relatório CoreTuner</title><style>body{font-family:Segoe UI,Arial;color:#10213d;background:#f5f7fb;margin:30px}.wrap{max-width:980px;margin:auto;background:#fff;padding:34px;border:1px solid #dfe6f0;border-radius:14px}h1{color:#1265f6}h2{margin-top:28px;border-bottom:1px solid #e3e8f0;padding-bottom:8px}.grid{display:grid;grid-template-columns:1fr 1fr;gap:12px}.item{background:#f7f9fc;padding:14px;border-radius:8px}.score{font-size:42px;font-weight:700}.muted{color:#687892}@media print{body{background:#fff;margin:0}.wrap{border:0}}</style></head><body><div class="wrap"><h1>CoreTuner</h1><p class="muted">Relatório gerado em %s • Empresa %s</p><h2>Este computador</h2><div class="grid"><div class="item"><b>Nome</b><br>%s</div><div class="item"><b>Sistema</b><br>%s</div><div class="item"><b>Processador</b><br>%s</div><div class="item"><b>Memória</b><br>%.1f GB • %.0f%% em uso</div><div class="item"><b>Armazenamento</b><br>%s • %.0f%% em uso</div><div class="item"><b>Saúde</b><br><span class="score">%d/100</span></div></div><h2>Recomendações</h2><ul>%s</ul><p class="muted">O CoreTuner coleta somente informações técnicas e não acessa documentos, conversas ou senhas.</p></div></body></html>`, time.Now().Format("02/01/2006 15:04"), html(comp), html(s.Hostname), html(s.OS), html(s.CPUName), s.TotalRAMGB, s.Memory, html(nz(s.DiskType, s.DiskName)), s.Disk, health(s), rows.String())
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		message("Relatório", err.Error(), MB_OK|MB_ICONERROR)
		return
	}
	a.addHistory("Relatório gerado", path)
	procShellExecute.Call(0, uintptr(unsafe.Pointer(utf16("open"))), uintptr(unsafe.Pointer(utf16(path))), 0, 0, SW_SHOWNORMAL)
}

func html(s string) string {
	r := strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", "\"", "&quot;")
	return r.Replace(s)
}

func (a *App) addHistory(title, detail string) {
	a.mu.Lock()
	a.history = append(a.history, HistoryItem{At: time.Now(), Title: title, Detail: detail})
	if len(a.history) > 500 {
		a.history = a.history[len(a.history)-500:]
	}
	h := append([]HistoryItem(nil), a.history...)
	a.mu.Unlock()
	b, _ := json.MarshalIndent(h, "", "  ")
	os.MkdirAll(dataDir(), 0755)
	_ = os.WriteFile(filepath.Join(dataDir(), "history.json"), b, 0600)
	a.invalidate()
}
