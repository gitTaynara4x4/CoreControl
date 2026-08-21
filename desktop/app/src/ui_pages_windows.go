//go:build windows

package main

import (
	"fmt"
	"sort"
	"strings"
	"syscall"
	"time"
)

func contentOrigin() (int32, int32) { return 238, 102 }

func health(sys SystemInfo) int {
	score := 100
	if sys.Memory > 90 {
		score -= 25
	} else if sys.Memory > 80 {
		score -= 12
	}
	if sys.Disk > 90 {
		score -= 25
	} else if sys.Disk > 80 {
		score -= 12
	}
	if sys.CPU > 90 {
		score -= 15
	} else if sys.CPU > 75 {
		score -= 7
	}
	if !sys.InternetOK {
		score -= 12
	}
	if !sys.AudioOK || !sys.MicOK {
		score -= 8
	}
	if score < 0 {
		score = 0
	}
	return score
}
func metricColor(v float64) uintptr {
	if v >= 90 {
		return rgb(235, 75, 70)
	}
	if v >= 75 {
		return rgb(242, 153, 39)
	}
	return rgb(30, 165, 89)
}
func healthColor(score int) uintptr {
	if score < 55 {
		return rgb(235, 75, 70)
	}
	if score < 80 {
		return rgb(242, 153, 39)
	}
	return rgb(30, 165, 89)
}
func (a *App) drawDashboard(dc syscall.Handle) {
	x, y := contentOrigin()
	a.mu.RLock()
	s := a.sys
	statusText := a.statusText
	a.mu.RUnlock()
	w := a.width - x - 28
	if w < 760 {
		w = 760
	}

	expired := strings.Contains(strings.ToLower(statusText), "sessão inválida") || strings.Contains(strings.ToLower(statusText), "expirada")
	if expired {
		banner := Rect{x, y, w, 72}
		roundedBox(dc, banner, rgb(255, 249, 240), rgb(245, 215, 174), 12)
		text(dc, "⚠", Rect{banner.X + 18, banner.Y + 12, 42, 46}, a.fonts["h1"], rgb(238, 139, 26), DT_CENTER|DT_VCENTER|DT_SINGLELINE)
		text(dc, "Sessão da Central expirada", Rect{banner.X + 68, banner.Y + 12, 430, 25}, a.fonts["h2"], rgb(83, 57, 28), DT_LEFT|DT_SINGLELINE)
		text(dc, "Entre novamente para manter este computador conectado ao CoreTuner Central.", Rect{banner.X + 68, banner.Y + 38, 600, 22}, a.fonts["small"], rgb(111, 88, 58), DT_LEFT|DT_SINGLELINE|DT_END_ELLIPSIS)
		br := Rect{banner.X + banner.W - 180, banner.Y + 17, 150, 38}
		button(dc, "Entrar novamente", br, true)
		a.hits = append(a.hits, Hit{br, "reauth", 0})
		y += 90
	}

	gap := int32(16)
	leftW := int32(float64(w) * 0.43)
	rightW := w - leftW - gap
	deviceCard := Rect{x, y, leftW, 218}
	healthCard := Rect{x + leftW + gap, y, rightW, 218}
	card(dc, deviceCard)
	card(dc, healthCard)

	roundedBox(dc, Rect{deviceCard.X + 22, deviceCard.Y + 22, 52, 52}, rgb(233, 248, 250), rgb(233, 248, 250), 12)
	monitorIcon(dc, Rect{deviceCard.X + 35, deviceCard.Y + 35, 26, 25}, rgb(0, 139, 158))
	text(dc, nz(s.Hostname, "Este computador"), Rect{deviceCard.X + 92, deviceCard.Y + 22, deviceCard.W - 116, 34}, a.fonts["h1"], rgb(12, 33, 62), DT_LEFT|DT_VCENTER|DT_SINGLELINE|DT_END_ELLIPSIS)
	pairs := []struct{ label, value string }{
		{"Fabricante", nz(s.Manufacturer, "Não identificado")},
		{"Modelo", nz(s.Model, "Não identificado")},
		{"Usuário", nz(s.Username, "Não identificado")},
		{"Sistema", nz(s.OS, "Windows")},
	}
	for i, pair := range pairs {
		py := deviceCard.Y + 83 + int32(i*28)
		text(dc, pair.label, Rect{deviceCard.X + 30, py, 92, 22}, a.fonts["small"], rgb(95, 111, 136), DT_LEFT|DT_SINGLELINE)
		text(dc, pair.value, Rect{deviceCard.X + 126, py, deviceCard.W - 154, 22}, a.fonts["small"], rgb(35, 53, 80), DT_LEFT|DT_SINGLELINE|DT_END_ELLIPSIS)
	}

	score := health(s)
	text(dc, "Saúde do computador", Rect{healthCard.X + 24, healthCard.Y + 20, healthCard.W - 48, 30}, a.fonts["h2"], rgb(15, 37, 68), DT_LEFT|DT_SINGLELINE)
	circle(dc, Rect{healthCard.X + 28, healthCard.Y + 66, 122, 122}, rgb(241, 245, 249))
	text(dc, fmt.Sprintf("%d", score), Rect{healthCard.X + 28, healthCard.Y + 83, 122, 62}, a.fonts["metric"], healthColor(score), DT_CENTER|DT_VCENTER|DT_SINGLELINE)
	text(dc, "/100", Rect{healthCard.X + 28, healthCard.Y + 142, 122, 25}, a.fonts["small"], rgb(97, 112, 137), DT_CENTER|DT_VCENTER|DT_SINGLELINE)
	status := "Excelente"
	if score < 80 {
		status = "Atenção"
	}
	if score < 55 {
		status = "Crítico"
	}
	text(dc, status, Rect{healthCard.X + 180, healthCard.Y + 74, healthCard.W - 205, 38}, a.fonts["h1"], healthColor(score), DT_LEFT|DT_VCENTER|DT_SINGLELINE)
	text(dc, "O CoreTuner encontrou os pontos que precisam da sua atenção.", Rect{healthCard.X + 180, healthCard.Y + 116, healthCard.W - 210, 56}, a.fonts["body"], rgb(77, 95, 121), DT_LEFT|DT_WORDBREAK)

	metricsY := y + 236
	metricW := (w - gap*3) / 4
	type metricInfo struct {
		name, value, detail string
		pct                 float64
		color               uintptr
		internet            bool
	}
	metrics := []metricInfo{
		{"Processador", fmt.Sprintf("%.0f%%", s.CPU), nz(s.CPUName, "Uso atual"), s.CPU, metricColor(s.CPU), false},
		{"Memória RAM", fmt.Sprintf("%.0f%%", s.Memory), fmt.Sprintf("%.1f de %.1f GB usados", s.UsedRAMGB, s.TotalRAMGB), s.Memory, metricColor(s.Memory), false},
		{"Disco", fmt.Sprintf("%.0f%%", s.Disk), fmt.Sprintf("%.0f de %.0f GB usados", s.DiskUsedGB, s.DiskTotalGB), s.Disk, metricColor(s.Disk), false},
		{"Internet", chooseText(s.InternetOK, "Conectado", "Sem conexão"), fmt.Sprintf("Latência %d ms", s.LatencyMS), 0, choose(s.InternetOK, rgb(29, 161, 91), rgb(226, 71, 67)), true},
	}
	for i, m := range metrics {
		r := Rect{x + int32(i)*(metricW+gap), metricsY, metricW, 158}
		card(dc, r)
		text(dc, m.name, Rect{r.X + 18, r.Y + 16, r.W - 36, 25}, a.fonts["h2"], rgb(18, 40, 73), DT_LEFT|DT_SINGLELINE)
		if m.internet {
			text(dc, m.value, Rect{r.X + 18, r.Y + 54, r.W - 36, 38}, a.fonts["h1"], m.color, DT_LEFT|DT_VCENTER|DT_SINGLELINE)
			circle(dc, Rect{r.X + r.W - 35, r.Y + 24, 10, 10}, m.color)
			text(dc, m.detail, Rect{r.X + 18, r.Y + 104, r.W - 36, 22}, a.fonts["small"], rgb(89, 105, 131), DT_LEFT|DT_SINGLELINE)
			text(dc, chooseText(s.InternetOK, "Sem perda de pacotes", "Verifique a rede"), Rect{r.X + 18, r.Y + 128, r.W - 36, 20}, a.fonts["small"], rgb(89, 105, 131), DT_LEFT|DT_SINGLELINE)
		} else {
			text(dc, m.value, Rect{r.X + 18, r.Y + 46, r.W - 36, 42}, a.fonts["metric"], m.color, DT_LEFT|DT_VCENTER|DT_SINGLELINE)
			progress(dc, Rect{r.X + 18, r.Y + 100, r.W - 36, 7}, m.pct, m.color)
			text(dc, m.detail, Rect{r.X + 18, r.Y + 122, r.W - 36, 24}, a.fonts["small"], rgb(89, 105, 131), DT_LEFT|DT_END_ELLIPSIS|DT_SINGLELINE)
		}
	}

	bottomY := metricsY + 176
	attentionCard := Rect{x, bottomY, w, 205}
	card(dc, attentionCard)

	text(dc, "⚠", Rect{attentionCard.X + 18, attentionCard.Y + 17, 28, 28}, a.fonts["h2"], rgb(229, 76, 68), DT_CENTER|DT_VCENTER|DT_SINGLELINE)
	text(dc, "Atenção necessária", Rect{attentionCard.X + 52, attentionCard.Y + 18, attentionCard.W - 72, 28}, a.fonts["h2"], rgb(20, 41, 72), DT_LEFT|DT_SINGLELINE)
	line(dc, attentionCard.X, attentionCard.Y+55, attentionCard.X+attentionCard.W, attentionCard.Y+55, rgb(232, 237, 243))
	recs := recommendations(s)
	for i, recommendation := range recs[:min(3, len(recs))] {
		bulletColor := rgb(231, 76, 68)
		if i > 0 {
			bulletColor = rgb(238, 148, 30)
		}
		circle(dc, Rect{attentionCard.X + 22, attentionCard.Y + 74 + int32(i*42), 8, 8}, bulletColor)
		text(dc, recommendation, Rect{attentionCard.X + 42, attentionCard.Y + 66 + int32(i*42), attentionCard.W - 62, 34}, a.fonts["small"], rgb(62, 79, 106), DT_LEFT|DT_WORDBREAK|DT_END_ELLIPSIS)
	}

}

func chooseText(cond bool, yes, no string) string {
	if cond {
		return yes
	}
	return no
}
func boolPct(v bool) float64 {
	if v {
		return 100
	}
	return 0
}
func nz(s, d string) string {
	if strings.TrimSpace(s) == "" {
		return d
	}
	return strings.TrimSpace(s)
}
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func (a *App) drawDiagnostics(dc syscall.Handle) {
	x, y := contentOrigin()
	a.mu.RLock()
	s := a.sys
	a.mu.RUnlock()
	w := a.width - x - 28
	card(dc, Rect{x, y, w, 170})
	text(dc, "Identificação completa", Rect{x + 24, y + 18, 400, 30}, a.fonts["h2"], rgb(14, 35, 67), DT_LEFT|DT_SINGLELINE)
	cols := [][]string{{"Nome do computador", s.Hostname}, {"Fabricante", nz(s.Manufacturer, "Não identificado")}, {"Modelo", nz(s.Model, "Não identificado")}, {"Número de série", nz(s.Serial, "Não identificado")}, {"Sistema", nz(s.OS, "Windows")}, {"Tempo ligado", formatDuration(s.Uptime)}}
	for i, p := range cols {
		cx := x + 24 + int32(i%3)*(w-48)/3
		cy := y + 58 + int32(i/3)*52
		text(dc, p[0], Rect{cx, cy, (w - 80) / 3, 18}, a.fonts["small"], rgb(103, 118, 142), DT_LEFT|DT_SINGLELINE)
		text(dc, p[1], Rect{cx, cy + 19, (w - 80) / 3, 25}, a.fonts["body"], rgb(25, 43, 71), DT_LEFT|DT_END_ELLIPSIS|DT_SINGLELINE)
	}
	sy := y + 190
	cards := []struct {
		title  string
		value  string
		detail string
		pct    float64
	}{{"Processador", fmt.Sprintf("%.0f%%", s.CPU), nz(s.CPUName, "Modelo não identificado"), s.CPU}, {"Memória RAM", fmt.Sprintf("%.1f GB", s.TotalRAMGB), fmt.Sprintf("%.1f GB em uso (%.0f%%)", s.UsedRAMGB, s.Memory), s.Memory}, {"Armazenamento", nz(s.DiskType, "Disco"), fmt.Sprintf("%s • %.0f GB livres", nz(s.DiskName, "Unidade C:"), s.DiskTotalGB-s.DiskUsedGB), s.Disk}, {"Rede", statusWord(s.InternetOK), fmt.Sprintf("Latência aproximada: %d ms", s.LatencyMS), boolPct(s.InternetOK)}, {"Áudio e microfone", audioWord(s), "Detecção segura pelos dispositivos do Windows", audioPct(s)}}
	gap := int32(12)
	cw := (w - gap) / 2
	ch := int32(145)
	for i, c := range cards {
		col := int32(i % 2)
		row := int32(i / 2)
		rw := cw
		if i == 4 {
			col = 0
			rw = w
		}
		r := Rect{x + col*(cw+gap), sy + row*(ch+gap), rw, ch}
		card(dc, r)
		text(dc, c.title, Rect{r.X + 20, r.Y + 16, 230, 26}, a.fonts["h2"], rgb(17, 38, 70), DT_LEFT|DT_SINGLELINE)
		text(dc, c.value, Rect{r.X + 260, r.Y + 12, r.W - 285, 38}, a.fonts["h1"], metricColor(c.pct), DT_RIGHT|DT_VCENTER|DT_SINGLELINE)
		text(dc, c.detail, Rect{r.X + 20, r.Y + 55, r.W - 40, 34}, a.fonts["body"], rgb(74, 91, 118), DT_LEFT|DT_END_ELLIPSIS|DT_SINGLELINE)
		progress(dc, Rect{r.X + 20, r.Y + 108, r.W - 40, 9}, c.pct, metricColor(c.pct))
	}
}

func statusWord(v bool) string {
	if v {
		return "Conectado"
	}
	return "Sem conexão"
}
func audioWord(s SystemInfo) string {
	if s.AudioOK && s.MicOK {
		return "Funcionando"
	}
	if s.AudioOK || s.MicOK {
		return "Atenção"
	}
	return "Não detectado"
}
func audioPct(s SystemInfo) float64 {
	v := 0.0
	if s.AudioOK {
		v += 50
	}
	if s.MicOK {
		v += 50
	}
	return v
}
func formatDuration(d time.Duration) string {
	days := int(d.Hours()) / 24
	hours := int(d.Hours()) % 24
	if days > 0 {
		return fmt.Sprintf("%d dias e %d horas", days, hours)
	}
	return fmt.Sprintf("%d horas", hours)
}

func (a *App) drawTests(dc syscall.Handle) {
	x, y := contentOrigin()
	w := a.width - x - 28
	a.mu.RLock()
	s := a.sys
	centralOK := a.centralOK
	serverURL := a.serverURL
	a.mu.RUnlock()
	tests := []struct {
		title, detail string
		ok            bool
		action        string
	}{{"Internet", fmt.Sprintf("Conexão %s • latência %d ms", statusWord(s.InternetOK), s.LatencyMS), s.InternetOK, "test-internet"}, {"Áudio", boolText(s.AudioOK, "Saída de áudio detectada", "Saída de áudio não detectada"), s.AudioOK, "test-audio"}, {"Microfone", boolText(s.MicOK, "Microfone detectado", "Microfone não detectado"), s.MicOK, "test-audio"}, {"Acesso ao CoreTuner Central", serverURL, centralOK, "refresh-central"}}
	text(dc, "Testes rápidos", Rect{x, y, w, 35}, a.fonts["h1"], rgb(13, 34, 65), DT_LEFT|DT_SINGLELINE)
	for i, t := range tests {
		r := Rect{x, y + 55 + int32(i)*120, w, 100}
		card(dc, r)
		c := rgb(39, 164, 88)
		sym := "✓"
		if !t.ok {
			c = rgb(235, 91, 73)
			sym = "!"
		}
		circle(dc, Rect{r.X + 22, r.Y + 24, 50, 50}, choose(t.ok, rgb(228, 248, 237), rgb(255, 235, 232)))
		text(dc, sym, Rect{r.X + 22, r.Y + 24, 50, 50}, a.fonts["h1"], c, DT_CENTER|DT_VCENTER|DT_SINGLELINE)
		text(dc, t.title, Rect{r.X + 92, r.Y + 18, 420, 28}, a.fonts["h2"], rgb(17, 38, 70), DT_LEFT|DT_SINGLELINE)
		text(dc, t.detail, Rect{r.X + 92, r.Y + 51, r.W - 360, 28}, a.fonts["body"], rgb(82, 99, 126), DT_LEFT|DT_END_ELLIPSIS|DT_SINGLELINE)
		br := Rect{r.X + r.W - 220, r.Y + 28, 180, 40}
		button(dc, "Executar teste", br, false)
		a.hits = append(a.hits, Hit{br, t.action, 0})
	}
	noteY := y + 55 + int32(len(tests))*120 + 15
	card(dc, Rect{x, noteY, w, 120})
	text(dc, "Privacidade", Rect{x + 22, noteY + 18, 250, 28}, a.fonts["h2"], rgb(18, 101, 246), DT_LEFT|DT_SINGLELINE)
	text(dc, "Os testes verificam somente conexão e dispositivos técnicos. Nenhum áudio é gravado permanentemente e nenhum documento é acessado.", Rect{x + 22, noteY + 54, w - 44, 54}, a.fonts["body"], rgb(68, 86, 115), DT_LEFT|DT_WORDBREAK)
}
func boolText(v bool, a, b string) string {
	if v {
		return a
	}
	return b
}

func (a *App) drawOptimizations(dc syscall.Handle) {
	x, y := contentOrigin()
	w := a.width - x - 28
	a.mu.RLock()
	activeProfile := a.optimizationActive
	activeAt := a.optimizationAppliedAt
	optimizationNote := a.optimizationNote
	optimizationBusy := a.optimizationBusy
	a.mu.RUnlock()

	text(dc, "Perfis de otimização", Rect{x, y, w, 36}, a.fonts["h1"], rgb(13, 34, 65), DT_LEFT|DT_SINGLELINE)
	intro := "Nenhum perfil é aplicado automaticamente. Selecione um perfil para ver exatamente o que será alterado."
	if activeProfile > 0 {
		intro = "Perfil ativo: " + optimizationProfileName(activeProfile)
		if !activeAt.IsZero() {
			intro += " • aplicado em " + activeAt.Format("02/01/2006 15:04")
		}
	}
	text(dc, intro, Rect{x, y + 38, w, 30}, a.fonts["body"], rgb(81, 98, 126), DT_LEFT|DT_SINGLELINE|DT_END_ELLIPSIS)

	cw := (w - 40) / 5
	for i := 1; i <= 5; i++ {
		profileInfo := optimizationProfileExplanation(i)
		r := Rect{x + int32(i-1)*(cw+10), y + 90, cw, 250}
		selected := a.profile == i
		active := activeProfile == i
		if active {
			roundedBox(dc, r, rgb(248, 253, 254), rgb(0, 151, 170), 14)
		} else {
			card(dc, r)
		}
		circle(dc, Rect{r.X + r.W/2 - 32, r.Y + 22, 64, 64}, choose(selected || active, rgb(220, 236, 255), rgb(239, 243, 248)))
		text(dc, fmt.Sprintf("%d", i), Rect{r.X + r.W/2 - 32, r.Y + 22, 64, 64}, a.fonts["h1"], choose(selected || active, rgb(18, 101, 246), rgb(82, 101, 131)), DT_CENTER|DT_VCENTER|DT_SINGLELINE)
		text(dc, profileInfo.Name, Rect{r.X + 10, r.Y + 100, r.W - 20, 34}, a.fonts["h2"], choose(selected || active, rgb(18, 101, 246), rgb(20, 42, 74)), DT_CENTER|DT_VCENTER|DT_SINGLELINE)
		text(dc, profileInfo.Short, Rect{r.X + 16, r.Y + 140, r.W - 32, 62}, a.fonts["small"], rgb(78, 95, 123), DT_CENTER|DT_WORDBREAK)
		br := Rect{r.X + 18, r.Y + 205, r.W - 36, 34}
		label := "Ver e selecionar"
		if selected {
			label = "Selecionado"
		} else if active {
			label = "Ativo • ver detalhes"
		}
		button(dc, label, br, selected)
		if !optimizationBusy {
			a.hits = append(a.hits, Hit{br, "profile", i})
		}
	}

	detailProfile := a.profile
	if detailProfile == 0 {
		detailProfile = activeProfile
	}
	if detailProfile == 0 {
		detailProfile = 1
	}
	detail := optimizationProfileExplanation(detailProfile)

	sy := y + 370
	detailH := int32(265)
	card(dc, Rect{x, sy, w, detailH})
	leftW := w * 64 / 100
	line(dc, x+leftW, sy+18, x+leftW, sy+detailH-18, rgb(226, 233, 241))

	text(dc, "O que o perfil "+detail.Name+" fará", Rect{x + 22, sy + 16, leftW - 44, 30}, a.fonts["h2"], rgb(18, 101, 246), DT_LEFT|DT_SINGLELINE|DT_END_ELLIPSIS)
	text(dc, detail.Summary, Rect{x + 22, sy + 48, leftW - 44, 42}, a.fonts["body"], rgb(74, 92, 121), DT_LEFT|DT_WORDBREAK)
	for i, action := range detail.Actions {
		text(dc, "✓", Rect{x + 24, sy + 96 + int32(i*32), 20, 26}, a.fonts["body"], rgb(31, 151, 83), DT_LEFT|DT_SINGLELINE)
		text(dc, action, Rect{x + 48, sy + 94 + int32(i*32), leftW - 72, 30}, a.fonts["body"], rgb(43, 70, 94), DT_LEFT|DT_WORDBREAK)
	}
	text(dc, detail.Result, Rect{x + 22, sy + 220, leftW - 44, 34}, a.fonts["small"], rgb(58, 79, 107), DT_LEFT|DT_WORDBREAK)

	rightX := x + leftW + 22
	rightW := w - leftW - 44
	text(dc, "Proteções em todos os perfis", Rect{rightX, sy + 16, rightW, 30}, a.fonts["h2"], rgb(31, 151, 83), DT_LEFT|DT_SINGLELINE|DT_END_ELLIPSIS)
	protected := []string{
		"Nenhum arquivo ou pasta é apagado ou movido.",
		"Defender e Firewall não são desativados.",
		"Registro, Lixeira e Downloads não são limpos.",
		"Nenhum programa é encerrado à força.",
		"Sem backup seguro, nenhuma alteração é iniciada.",
	}
	for i, item := range protected {
		text(dc, "✓", Rect{rightX + 2, sy + 58 + int32(i*34), 20, 28}, a.fonts["body"], rgb(31, 151, 83), DT_LEFT|DT_SINGLELINE)
		text(dc, item, Rect{rightX + 25, sy + 56 + int32(i*34), rightW - 25, 32}, a.fonts["body"], rgb(43, 91, 67), DT_LEFT|DT_WORDBREAK)
	}

	buttonsY := sy + detailH + 20
	apply := Rect{x, buttonsY, 260, 42}
	applyLabel := "Aplicar perfil com backup"
	if a.profile == 5 {
		applyLabel = "Restaurar configurações"
	}
	if optimizationBusy {
		applyLabel = "Aplicando com segurança..."
	}
	button(dc, applyLabel, apply, true)
	if !optimizationBusy {
		a.hits = append(a.hits, Hit{apply, "apply-profile", 0})
	}
	reset := Rect{x + 280, buttonsY, 210, 42}
	button(dc, "Cancelar seleção", reset, false)
	if !optimizationBusy {
		a.hits = append(a.hits, Hit{reset, "cancel-profile", 0})
	}
	if optimizationNote == "" {
		optimizationNote = "O backup original nunca é substituído enquanto existir um perfil ativo."
	}
	text(dc, optimizationNote, Rect{x + 520, buttonsY - 2, w - 520, 48}, a.fonts["small"], rgb(91, 107, 132), DT_LEFT|DT_WORDBREAK)
}

func (a *App) refreshOptimizationSummary() {
	active, appliedAt, note := loadOptimizationSummary()
	a.mu.Lock()
	a.optimizationActive = active
	a.optimizationAppliedAt = appliedAt
	a.optimizationNote = note
	a.mu.Unlock()
	a.invalidate()
}

func chooseLabel(c bool, a, b string) string {
	if c {
		return a
	}
	return b
}

func (a *App) drawPrograms(dc syscall.Handle) {
	x, y := contentOrigin()
	w := a.width - x - 28
	a.mu.RLock()
	p := append([]ProcessInfo(nil), a.processes...)
	a.mu.RUnlock()
	text(dc, "Programas e processos", Rect{x, y, w, 36}, a.fonts["h1"], rgb(13, 34, 65), DT_LEFT|DT_SINGLELINE)
	text(dc, "Lista somente para diagnóstico. O CoreTuner não finaliza programas à força.", Rect{x, y + 38, w, 25}, a.fonts["body"], rgb(80, 97, 125), DT_LEFT|DT_SINGLELINE)
	card(dc, Rect{x, y + 80, w, 520})
	headers := []struct {
		s string
		x int32
		w int32
	}{{"Programa", x + 24, w - 420}, {"PID", x + w - 380, 90}, {"CPU acumulada", x + w - 280, 120}, {"Memória", x + w - 145, 110}}
	for _, h := range headers {
		text(dc, h.s, Rect{h.x, y + 98, h.w, 28}, a.fonts["h2"], rgb(31, 52, 83), DT_LEFT|DT_SINGLELINE)
	}
	line(dc, x+20, y+135, x+w-20, y+135, rgb(226, 231, 239))
	for i, v := range p[:min(12, len(p))] {
		ry := y + 145 + int32(i*34)
		if i%2 == 1 {
			fill(dc, Rect{x + 14, ry - 2, w - 28, 32}, rgb(249, 251, 254))
		}
		text(dc, v.Name, Rect{x + 24, ry, w - 430, 28}, a.fonts["body"], rgb(35, 52, 79), DT_LEFT|DT_END_ELLIPSIS|DT_SINGLELINE)
		text(dc, fmt.Sprint(v.PID), Rect{x + w - 380, ry, 90, 28}, a.fonts["body"], rgb(75, 91, 117), DT_LEFT|DT_SINGLELINE)
		text(dc, fmt.Sprintf("%.1f s", v.CPU), Rect{x + w - 280, ry, 120, 28}, a.fonts["body"], rgb(75, 91, 117), DT_LEFT|DT_SINGLELINE)
		text(dc, fmt.Sprintf("%.0f MB", v.MemoryMB), Rect{x + w - 145, ry, 110, 28}, a.fonts["body"], rgb(75, 91, 117), DT_RIGHT|DT_SINGLELINE)
	}
	br := Rect{x, y + 620, 220, 42}
	button(dc, "Atualizar processos", br, false)
	a.hits = append(a.hits, Hit{br, "refresh-local", 0})
}

func (a *App) drawReports(dc syscall.Handle) {
	x, y := contentOrigin()
	w := a.width - x - 28
	text(dc, "Relatórios técnicos", Rect{x, y, w, 36}, a.fonts["h1"], rgb(13, 34, 65), DT_LEFT|DT_SINGLELINE)
	cards := []struct{ title, desc, action string }{{"Relatório de diagnóstico", "Identificação, CPU, memória, disco, rede e recomendações deste computador.", "report"}, {"Comparação antes e depois", "As otimizações já são aplicadas com backup; a comparação automática de métricas será adicionada em uma próxima etapa.", "report"}}
	for i, c := range cards {
		r := Rect{x, y + 70 + int32(i)*160, w, 135}
		card(dc, r)
		text(dc, c.title, Rect{r.X + 24, r.Y + 20, 520, 32}, a.fonts["h2"], rgb(17, 38, 70), DT_LEFT|DT_SINGLELINE)
		text(dc, c.desc, Rect{r.X + 24, r.Y + 58, r.W - 330, 45}, a.fonts["body"], rgb(75, 92, 119), DT_LEFT|DT_WORDBREAK)
		br := Rect{r.X + r.W - 260, r.Y + 44, 220, 44}
		button(dc, "Gerar e abrir", br, true)
		a.hits = append(a.hits, Hit{br, c.action, 0})
	}
	text(dc, "Os relatórios são gerados localmente em HTML e podem ser impressos em PDF pelo navegador.", Rect{x, y + 410, w, 40}, a.fonts["body"], rgb(84, 101, 128), DT_LEFT|DT_WORDBREAK)
}

func (a *App) drawHistory(dc syscall.Handle) {
	x, y := contentOrigin()
	w := a.width - x - 28
	a.mu.RLock()
	h := append([]HistoryItem(nil), a.history...)
	a.mu.RUnlock()
	sort.Slice(h, func(i, j int) bool { return h[i].At.After(h[j].At) })
	text(dc, "Histórico completo", Rect{x, y, w, 36}, a.fonts["h1"], rgb(13, 34, 65), DT_LEFT|DT_SINGLELINE)
	card(dc, Rect{x, y + 55, w, 650})
	for i, v := range h[:min(16, len(h))] {
		ry := y + 75 + int32(i*38)
		text(dc, v.At.Format("02/01/2006 15:04"), Rect{x + 22, ry, 150, 28}, a.fonts["small"], rgb(102, 117, 141), DT_LEFT|DT_SINGLELINE)
		text(dc, v.Title, Rect{x + 180, ry, 270, 28}, a.fonts["body"], rgb(27, 47, 77), DT_LEFT|DT_END_ELLIPSIS|DT_SINGLELINE)
		text(dc, v.Detail, Rect{x + 465, ry, w - 500, 28}, a.fonts["small"], rgb(78, 95, 121), DT_LEFT|DT_END_ELLIPSIS|DT_SINGLELINE)
		line(dc, x+18, ry+31, x+w-18, ry+31, rgb(235, 239, 245))
	}
	if len(h) == 0 {
		text(dc, "Nenhum evento registrado ainda.", Rect{x + 25, y + 100, w - 50, 36}, a.fonts["body"], rgb(95, 111, 137), DT_CENTER|DT_SINGLELINE)
	}
}

func (a *App) drawSettings(dc syscall.Handle) {
	x, y := contentOrigin()
	w := a.width - x - 28
	a.mu.RLock()
	server := a.serverURL
	comp := companyName(a.company)
	a.mu.RUnlock()
	text(dc, "Configurações", Rect{x, y, w, 36}, a.fonts["h1"], rgb(13, 34, 65), DT_LEFT|DT_SINGLELINE)
	items := []struct{ t, d string }{{"Servidor CoreTuner", server}, {"Empresa vinculada", comp}, {"Pasta de dados", dataDir()}, {"Atualização dos indicadores", "Dados locais a cada 2 segundos; conexão com a Central verificada a cada 30 segundos"}, {"Segurança", "Nenhuma limpeza, exclusão ou alteração crítica automática"}}
	for i, v := range items {
		r := Rect{x, y + 60 + int32(i)*100, w, 82}
		card(dc, r)
		text(dc, v.t, Rect{r.X + 22, r.Y + 14, 300, 26}, a.fonts["h2"], rgb(18, 39, 71), DT_LEFT|DT_SINGLELINE)
		text(dc, v.d, Rect{r.X + 22, r.Y + 43, r.W - 44, 26}, a.fonts["body"], rgb(79, 96, 123), DT_LEFT|DT_END_ELLIPSIS|DT_SINGLELINE)
	}
	br := Rect{x, y + 585, 250, 42}
	button(dc, "Abrir painel web", br, true)
	a.hits = append(a.hits, Hit{br, "open-web", 0})
	br2 := Rect{x + 270, y + 585, 230, 42}
	button(dc, "Atualizar tudo", br2, false)
	a.hits = append(a.hits, Hit{br2, "refresh-all", 0})
}

func (a *App) drawSupport(dc syscall.Handle) {
	x, y := contentOrigin()
	w := a.width - x - 28
	text(dc, "Suporte e segurança", Rect{x, y, w, 36}, a.fonts["h1"], rgb(13, 34, 65), DT_LEFT|DT_SINGLELINE)
	card(dc, Rect{x, y + 60, w, 210})
	text(dc, "Antes de solicitar suporte", Rect{x + 24, y + 80, w - 48, 30}, a.fonts["h2"], rgb(17, 39, 71), DT_LEFT|DT_SINGLELINE)
	steps := []string{"Execute o Diagnóstico Profissional", "Teste internet, áudio e microfone", "Confirme a conexão com o CoreTuner Central", "Gere o relatório técnico"}
	for i, v := range steps {
		text(dc, fmt.Sprintf("%d. %s", i+1, v), Rect{x + 28, y + 122 + int32(i*34), w - 56, 28}, a.fonts["body"], rgb(67, 85, 113), DT_LEFT|DT_SINGLELINE)
	}
	card(dc, Rect{x, y + 290, w, 230})
	text(dc, "Proteção máxima", Rect{x + 24, y + 310, w - 48, 30}, a.fonts["h2"], rgb(34, 153, 83), DT_LEFT|DT_SINGLELINE)
	txt := "O CoreTuner não apaga arquivos, não esvazia a Lixeira, não limpa o Registro, não desativa Defender ou Firewall e não fecha programas à força. O monitoramento coleta apenas informações técnicas necessárias."
	text(dc, txt, Rect{x + 26, y + 354, w - 52, 100}, a.fonts["body"], rgb(65, 83, 111), DT_LEFT|DT_WORDBREAK)
	text(dc, "Versão "+appVersion, Rect{x + 26, y + 475, 300, 24}, a.fonts["small"], rgb(103, 118, 142), DT_LEFT|DT_SINGLELINE)
}
