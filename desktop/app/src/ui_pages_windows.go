//go:build windows

package main

import (
	"fmt"
	"sort"
	"strings"
	"syscall"
	"time"
)

func contentOrigin() (int32, int32) { return shellContentLeft, shellContentTop }

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
		return rgb(232, 77, 73)
	}
	if v >= 75 {
		return rgb(241, 153, 32)
	}
	return rgb(42, 176, 91)
}
func healthColor(score int) uintptr {
	if score < 55 {
		return rgb(232, 77, 73)
	}
	if score < 80 {
		return rgb(241, 153, 32)
	}
	return rgb(42, 176, 91)
}

func clamp32(v, minV, maxV int32) int32 {
	if v < minV {
		return minV
	}
	if v > maxV {
		return maxV
	}
	return v
}

func softCard(dc syscall.Handle, r Rect) {
	roundedBox(dc, Rect{r.X, r.Y + 2, r.W, r.H}, rgb(241, 243, 247), rgb(241, 243, 247), 12)
	roundedBox(dc, r, rgb(255, 255, 255), rgb(229, 233, 239), 12)
}

type dashboardSample struct {
	At                time.Time
	CPU, Memory, Disk float64
	Latency           float64
}

var dashboardSamples []dashboardSample

func recordDashboardSample(s SystemInfo) []dashboardSample {
	stamp := s.Updated
	if stamp.IsZero() {
		stamp = time.Now()
	}
	if len(dashboardSamples) == 0 || !dashboardSamples[len(dashboardSamples)-1].At.Equal(stamp) {
		dashboardSamples = append(dashboardSamples, dashboardSample{
			At: stamp, CPU: s.CPU, Memory: s.Memory, Disk: s.Disk, Latency: float64(s.LatencyMS),
		})
		if len(dashboardSamples) > 60 {
			dashboardSamples = append([]dashboardSample(nil), dashboardSamples[len(dashboardSamples)-60:]...)
		}
	}
	return append([]dashboardSample(nil), dashboardSamples...)
}

func sampleValues(samples []dashboardSample, kind string) []float64 {
	values := make([]float64, 0, len(samples))
	for _, s := range samples {
		switch kind {
		case "cpu":
			values = append(values, s.CPU)
		case "ram":
			values = append(values, s.Memory)
		case "disk":
			values = append(values, s.Disk)
		case "latency":
			values = append(values, s.Latency)
		}
	}
	return values
}

func drawSparkline(dc syscall.Handle, r Rect, values []float64, maxValue float64, color uintptr) {
	if r.W <= 4 || r.H <= 4 || len(values) == 0 {
		return
	}
	if maxValue <= 0 {
		maxValue = 100
	}
	if len(values) == 1 {
		y := r.Y + r.H - int32(values[0]/maxValue*float64(r.H))
		if y < r.Y {
			y = r.Y
		}
		if y > r.Y+r.H {
			y = r.Y + r.H
		}
		line(dc, r.X, y, r.X+r.W, y, color)
		return
	}
	step := float64(r.W) / float64(len(values)-1)
	var px, py int32
	for i, value := range values {
		if value < 0 {
			value = 0
		}
		if value > maxValue {
			value = maxValue
		}
		x := r.X + int32(float64(i)*step)
		y := r.Y + r.H - int32(value/maxValue*float64(r.H))
		if i > 0 {
			line(dc, px, py, x, y, color)
		}
		px, py = x, y
	}
}

type dashboardAlert struct {
	Title, Detail string
	Critical      bool
}

func dashboardAlerts(s SystemInfo) []dashboardAlert {
	alerts := make([]dashboardAlert, 0, 6)
	if s.Disk >= 90 {
		alerts = append(alerts, dashboardAlert{"Espaço em disco crítico", fmt.Sprintf("Disco com %.0f%% de uso", s.Disk), true})
	} else if s.Disk >= 80 {
		alerts = append(alerts, dashboardAlert{"Espaço em disco baixo", fmt.Sprintf("Disco com %.0f%% de uso", s.Disk), false})
	}
	if s.Memory >= 90 {
		alerts = append(alerts, dashboardAlert{"Memória em nível crítico", fmt.Sprintf("RAM em %.0f%% de uso", s.Memory), true})
	} else if s.Memory >= 75 {
		alerts = append(alerts, dashboardAlert{"Memória em uso elevado", fmt.Sprintf("RAM em %.0f%% de uso", s.Memory), false})
	}
	if s.CPU >= 90 {
		alerts = append(alerts, dashboardAlert{"Processador sobrecarregado", fmt.Sprintf("CPU em %.0f%%", s.CPU), true})
	} else if s.CPU >= 75 {
		alerts = append(alerts, dashboardAlert{"Processador em uso elevado", fmt.Sprintf("CPU em %.0f%%", s.CPU), false})
	}
	if !s.InternetOK {
		alerts = append(alerts, dashboardAlert{"Sem acesso à internet", "A conexão precisa ser verificada", true})
	}
	if !s.AudioOK {
		alerts = append(alerts, dashboardAlert{"Áudio não detectado", "Verifique o dispositivo de saída", false})
	}
	if !s.MicOK {
		alerts = append(alerts, dashboardAlert{"Microfone não detectado", "Verifique o dispositivo de entrada", false})
	}
	return alerts
}

func dashboardCheckStats(s SystemInfo, centralOK bool) (healthy, warning, critical int) {
	classify := func(value float64, warn, crit float64) {
		if value >= crit {
			critical++
		} else if value >= warn {
			warning++
		} else {
			healthy++
		}
	}
	classify(s.CPU, 75, 90)
	classify(s.Memory, 75, 90)
	classify(s.Disk, 80, 90)
	if s.InternetOK {
		healthy++
	} else {
		critical++
	}
	if s.AudioOK {
		healthy++
	} else {
		warning++
	}
	if s.MicOK {
		healthy++
	} else {
		warning++
	}
	if centralOK {
		healthy++
	} else {
		critical++
	}
	return
}

func drawMetricIcon(dc syscall.Handle, kind string, r Rect, color uintptr) {
	switch kind {
	case "cpu":
		roundedBox(dc, Rect{r.X + 4, r.Y + 4, r.W - 8, r.H - 8}, rgb(255, 255, 255), color, 3)
		for i := int32(0); i < 3; i++ {
			x := r.X + 7 + i*5
			line(dc, x, r.Y+1, x, r.Y+4, color)
			line(dc, x, r.Y+r.H-4, x, r.Y+r.H-1, color)
			y := r.Y + 7 + i*5
			line(dc, r.X+1, y, r.X+4, y, color)
			line(dc, r.X+r.W-4, y, r.X+r.W-1, y, color)
		}
	case "ram":
		roundedBox(dc, Rect{r.X + 2, r.Y + 5, r.W - 4, r.H - 10}, rgb(255, 255, 255), color, 3)
		for i := int32(0); i < 3; i++ {
			roundedBox(dc, Rect{r.X + 6 + i*6, r.Y + 9, 4, 8}, rgb(255, 255, 255), color, 1)
		}
	case "disk":
		roundedBox(dc, Rect{r.X + 4, r.Y + 3, r.W - 8, r.H - 6}, rgb(255, 255, 255), color, 4)
		line(dc, r.X+7, r.Y+r.H-7, r.X+r.W-7, r.Y+r.H-7, color)
		circle(dc, Rect{r.X + r.W - 9, r.Y + r.H - 10, 3, 3}, color)
	case "internet":
		line(dc, r.X+4, r.Y+9, r.X+r.W/2, r.Y+4, color)
		line(dc, r.X+r.W/2, r.Y+4, r.X+r.W-4, r.Y+9, color)
		line(dc, r.X+7, r.Y+13, r.X+r.W/2, r.Y+9, color)
		line(dc, r.X+r.W/2, r.Y+9, r.X+r.W-7, r.Y+13, color)
		circle(dc, Rect{r.X + r.W/2 - 2, r.Y + r.H - 5, 4, 4}, color)
	}
}

func drawTopMetricCard(dc syscall.Handle, r Rect, title, value, caption, foot1, foot2, kind string, color uintptr, samples []dashboardSample) {
	softCard(dc, r)
	drawMetricIcon(dc, kind, Rect{r.X + 16, r.Y + 15, 23, 23}, color)
	text(dc, title, Rect{r.X + 48, r.Y + 13, r.W - 60, 25}, app.fonts["body"], rgb(40, 52, 72), DT_LEFT|DT_VCENTER|DT_SINGLELINE|DT_END_ELLIPSIS)
	text(dc, value, Rect{r.X + 18, r.Y + 51, r.W - 36, 34}, app.fonts["h1"], rgb(25, 36, 55), DT_LEFT|DT_VCENTER|DT_SINGLELINE)
	text(dc, caption, Rect{r.X + 18, r.Y + 82, r.W - 36, 18}, app.fonts["small"], rgb(120, 131, 151), DT_LEFT|DT_SINGLELINE|DT_END_ELLIPSIS)
	values := sampleValues(samples, kind)
	maxValue := 100.0
	if kind == "internet" {
		values = sampleValues(samples, "latency")
		maxValue = 200
	}
	drawSparkline(dc, Rect{r.X + 18, r.Y + 112, r.W - 36, 35}, values, maxValue, color)
	line(dc, r.X+18, r.Y+r.H-49, r.X+r.W-18, r.Y+r.H-49, rgb(240, 242, 246))
	text(dc, foot1, Rect{r.X + 18, r.Y + r.H - 38, r.W - 36, 17}, app.fonts["small"], rgb(105, 117, 137), DT_LEFT|DT_SINGLELINE|DT_END_ELLIPSIS)
	text(dc, foot2, Rect{r.X + 18, r.Y + r.H - 21, r.W - 36, 17}, app.fonts["small"], rgb(47, 62, 87), DT_LEFT|DT_SINGLELINE|DT_END_ELLIPSIS)
}

func drawActionTile(dc syscall.Handle, r Rect, icon, line1, line2 string, hovered bool) {
	bg := rgb(255, 255, 255)
	border := rgb(229, 233, 239)
	if hovered {
		bg = rgb(247, 250, 255)
		border = rgb(207, 222, 247)
	}
	roundedBox(dc, r, bg, border, 9)
	text(dc, icon, Rect{r.X, r.Y + 9, r.W, 23}, app.fonts["h2"], rgb(47, 124, 246), DT_CENTER|DT_VCENTER|DT_SINGLELINE)
	text(dc, line1, Rect{r.X + 5, r.Y + 37, r.W - 10, 17}, app.fonts["small"], rgb(59, 72, 94), DT_CENTER|DT_SINGLELINE|DT_END_ELLIPSIS)
	if line2 != "" {
		text(dc, line2, Rect{r.X + 5, r.Y + 53, r.W - 10, 16}, app.fonts["small"], rgb(59, 72, 94), DT_CENTER|DT_SINGLELINE|DT_END_ELLIPSIS)
	}
}

func (a *App) drawDashboard(dc syscall.Handle) {
	x, y := contentOrigin()
	a.mu.RLock()
	s := a.sys
	statusText := a.statusText
	centralOK := a.centralOK
	processes := append([]ProcessInfo(nil), a.processes...)
	history := append([]HistoryItem(nil), a.history...)
	activeProfile := a.optimizationActive
	a.mu.RUnlock()

	w := a.width - x - 24
	if w < 760 {
		w = 760
	}
	availableH := a.height - y - 16
	if availableH < 620 {
		availableH = 620
	}
	samples := recordDashboardSample(s)

	expired := strings.Contains(strings.ToLower(statusText), "sessão inválida") || strings.Contains(strings.ToLower(statusText), "expirada")
	if expired {
		banner := Rect{x, y, w, 52}
		roundedBox(dc, banner, rgb(255, 249, 240), rgb(245, 215, 174), 10)
		text(dc, "!", Rect{banner.X + 14, banner.Y + 10, 30, 30}, a.fonts["h2"], rgb(238, 139, 26), DT_CENTER|DT_VCENTER|DT_SINGLELINE)
		text(dc, "Sessão da Central expirada", Rect{banner.X + 50, banner.Y + 7, 280, 21}, a.fonts["body"], rgb(83, 57, 28), DT_LEFT|DT_SINGLELINE)
		text(dc, "Entre novamente para manter este computador conectado.", Rect{banner.X + 50, banner.Y + 27, 440, 18}, a.fonts["small"], rgb(111, 88, 58), DT_LEFT|DT_SINGLELINE|DT_END_ELLIPSIS)
		br := Rect{banner.X + banner.W - 148, banner.Y + 9, 128, 34}
		button(dc, "Entrar novamente", br, true)
		a.hits = append(a.hits, Hit{br, "reauth", 0})
		y += 62
		availableH -= 62
	}

	gap := int32(12)
	rightW := clamp32(w*24/100, 238, 292)
	mainW := w - rightW - gap
	healthW := clamp32(mainW*25/100, 190, 232)
	metricW := (mainW - healthW - gap*4) / 4
	row1H := int32(220)
	row2H := int32(232)
	footerH := int32(38)
	row3H := availableH - row1H - row2H - footerH - gap*3
	if row3H < 170 {
		footerH = 0
		row2H = 212
		row3H = availableH - row1H - row2H - gap*2
	}
	if row3H < 145 {
		row3H = 145
	}

	// Saúde
	healthCard := Rect{x, y, healthW, row1H}
	softCard(dc, healthCard)
	text(dc, "Saúde do computador", Rect{healthCard.X + 18, healthCard.Y + 14, healthCard.W - 36, 24}, a.fonts["body"], rgb(42, 53, 72), DT_LEFT|DT_VCENTER|DT_SINGLELINE|DT_END_ELLIPSIS)
	score := health(s)
	gaugeSize := clamp32(healthCard.W-72, 104, 132)
	gaugeX := healthCard.X + (healthCard.W-gaugeSize)/2
	gaugeY := healthCard.Y + 50
	circle(dc, Rect{gaugeX, gaugeY, gaugeSize, gaugeSize}, rgb(231, 236, 242))
	circle(dc, Rect{gaugeX + 8, gaugeY + 8, gaugeSize - 16, gaugeSize - 16}, healthColor(score))
	circle(dc, Rect{gaugeX + 15, gaugeY + 15, gaugeSize - 30, gaugeSize - 30}, rgb(255, 255, 255))
	text(dc, fmt.Sprintf("%d", score), Rect{gaugeX + 8, gaugeY + 26, gaugeSize - 16, 47}, a.fonts["metric"], rgb(22, 33, 52), DT_CENTER|DT_VCENTER|DT_SINGLELINE)
	text(dc, "/100", Rect{gaugeX + 8, gaugeY + 73, gaugeSize - 16, 18}, a.fonts["small"], rgb(119, 130, 149), DT_CENTER|DT_SINGLELINE)
	state := "Excelente"
	if score < 80 {
		state = "Atenção"
	}
	if score < 55 {
		state = "Crítico"
	}
	text(dc, state, Rect{healthCard.X + 15, healthCard.Y + 184, healthCard.W - 30, 20}, a.fonts["small"], healthColor(score), DT_CENTER|DT_SINGLELINE)

	// Métricas
	metricX := x + healthW + gap
	cpuCard := Rect{metricX, y, metricW, row1H}
	ramCard := Rect{metricX + metricW + gap, y, metricW, row1H}
	diskCard := Rect{metricX + (metricW+gap)*2, y, metricW, row1H}
	netCard := Rect{metricX + (metricW+gap)*3, y, metricW, row1H}
	drawTopMetricCard(dc, cpuCard, "CPU", fmt.Sprintf("%.0f%%", s.CPU), "Uso atual", "Processador", nz(s.CPUName, "Não identificado"), "cpu", rgb(47, 124, 246), samples)
	drawTopMetricCard(dc, ramCard, "Memória RAM", fmt.Sprintf("%.0f%%", s.Memory), "Uso atual", fmt.Sprintf("Usado %.1f GB", s.UsedRAMGB), fmt.Sprintf("Total %.1f GB", s.TotalRAMGB), "ram", rgb(26, 181, 196), samples)
	drawTopMetricCard(dc, diskCard, "Disco (C:)", fmt.Sprintf("%.0f%%", s.Disk), "Uso atual", fmt.Sprintf("Usado %.0f GB", s.DiskUsedGB), fmt.Sprintf("Total %.0f GB • %s", s.DiskTotalGB, nz(s.DiskType, "disco")), "disk", rgb(42, 176, 91), samples)
	netValue := fmt.Sprintf("%d ms", s.LatencyMS)
	netCaption := "Latência"
	if !s.InternetOK {
		netValue = "Offline"
		netCaption = "Sem conexão"
	}
	drawTopMetricCard(dc, netCard, "Internet", netValue, netCaption, "Estado", chooseText(s.InternetOK, "Conectado", "Verificar rede"), "internet", rgb(134, 93, 255), samples)

	// Alertas - coluna direita topo
	alerts := dashboardAlerts(s)
	alertsCard := Rect{x + mainW + gap, y, rightW, row1H}
	softCard(dc, alertsCard)
	text(dc, "Alertas", Rect{alertsCard.X + 18, alertsCard.Y + 14, alertsCard.W - 74, 25}, a.fonts["body"], rgb(42, 53, 72), DT_LEFT|DT_VCENTER|DT_SINGLELINE)
	badgeText := fmt.Sprintf("%d", len(alerts))
	badgeBg := rgb(231, 242, 255)
	badgeFg := rgb(47, 124, 246)
	if len(alerts) > 0 {
		badgeBg = rgb(255, 235, 235)
		badgeFg = rgb(223, 74, 74)
	}
	statusPill(dc, badgeText, Rect{alertsCard.X + alertsCard.W - 45, alertsCard.Y + 14, 27, 20}, badgeBg, badgeFg)
	if len(alerts) == 0 {
		circle(dc, Rect{alertsCard.X + 18, alertsCard.Y + 62, 22, 22}, rgb(42, 176, 91))
		text(dc, "✓", Rect{alertsCard.X + 18, alertsCard.Y + 62, 22, 22}, a.fonts["small"], rgb(255, 255, 255), DT_CENTER|DT_VCENTER|DT_SINGLELINE)
		text(dc, "Nenhum alerta ativo", Rect{alertsCard.X + 50, alertsCard.Y + 60, alertsCard.W - 68, 22}, a.fonts["body"], rgb(47, 62, 87), DT_LEFT|DT_VCENTER|DT_SINGLELINE)
		text(dc, "O computador está dentro dos limites monitorados.", Rect{alertsCard.X + 50, alertsCard.Y + 83, alertsCard.W - 68, 36}, a.fonts["small"], rgb(118, 129, 149), DT_LEFT|DT_WORDBREAK)
	} else {
		for i, alert := range alerts[:min(2, len(alerts))] {
			ay := alertsCard.Y + 54 + int32(i*68)
			c := rgb(241, 153, 32)
			if alert.Critical {
				c = rgb(232, 77, 73)
			}
			text(dc, "!", Rect{alertsCard.X + 18, ay + 4, 22, 22}, a.fonts["h2"], c, DT_CENTER|DT_VCENTER|DT_SINGLELINE)
			text(dc, alert.Title, Rect{alertsCard.X + 50, ay, alertsCard.W - 68, 20}, a.fonts["small"], rgb(47, 62, 87), DT_LEFT|DT_SINGLELINE|DT_END_ELLIPSIS)
			text(dc, alert.Detail, Rect{alertsCard.X + 50, ay + 23, alertsCard.W - 68, 28}, a.fonts["small"], rgb(119, 130, 149), DT_LEFT|DT_WORDBREAK|DT_END_ELLIPSIS)
		}
	}
	alertLink := Rect{alertsCard.X + 18, alertsCard.Y + alertsCard.H - 33, 145, 22}
	text(dc, "Ver diagnóstico completo", alertLink, a.fonts["small"], rgb(47, 124, 246), DT_LEFT|DT_VCENTER|DT_SINGLELINE)
	a.hits = append(a.hits, Hit{alertLink, "page", 1})

	// Segunda linha
	row2Y := y + row1H + gap
	diagW := clamp32(mainW*28/100, 220, 260)
	perfW := clamp32(mainW*39/100, 300, 365)
	updatesW := mainW - diagW - perfW - gap*2
	if updatesW < 210 {
		perfW -= 210 - updatesW
		updatesW = 210
	}

	diag := Rect{x, row2Y, diagW, row2H}
	softCard(dc, diag)
	text(dc, "Diagnóstico rápido", Rect{diag.X + 18, diag.Y + 14, diag.W - 36, 22}, a.fonts["body"], rgb(42, 53, 72), DT_LEFT|DT_SINGLELINE)
	okCount := 0
	checks := []struct {
		Name string
		OK   bool
	}{{"Internet", s.InternetOK}, {"Áudio", s.AudioOK}, {"Microfone", s.MicOK}, {"Central CoreControl", centralOK}}
	for _, c := range checks {
		if c.OK {
			okCount++
		}
	}
	diagStatus := fmt.Sprintf("%d de %d verificações OK", okCount, len(checks))
	text(dc, diagStatus, Rect{diag.X + 18, diag.Y + 38, diag.W - 36, 18}, a.fonts["small"], choose(okCount == len(checks), rgb(42, 176, 91), rgb(241, 153, 32)), DT_LEFT|DT_SINGLELINE)
	line(dc, diag.X+18, diag.Y+66, diag.X+diag.W-18, diag.Y+66, rgb(239, 241, 245))
	for i, c := range checks {
		cy := diag.Y + 78 + int32(i*27)
		markColor := choose(c.OK, rgb(42, 176, 91), rgb(232, 77, 73))
		mark := "✓"
		stateTxt := "OK"
		if !c.OK {
			mark = "!"
			stateTxt = "Atenção"
		}
		circle(dc, Rect{diag.X + 18, cy + 2, 14, 14}, markColor)
		text(dc, mark, Rect{diag.X + 18, cy + 2, 14, 14}, a.fonts["small"], rgb(255, 255, 255), DT_CENTER|DT_VCENTER|DT_SINGLELINE)
		text(dc, c.Name, Rect{diag.X + 42, cy, diag.W - 105, 18}, a.fonts["small"], rgb(66, 79, 100), DT_LEFT|DT_SINGLELINE|DT_END_ELLIPSIS)
		text(dc, stateTxt, Rect{diag.X + diag.W - 70, cy, 52, 18}, a.fonts["small"], markColor, DT_RIGHT|DT_SINGLELINE)
	}
	diagBtn := Rect{diag.X + 18, diag.Y + diag.H - 42, diag.W - 36, 30}
	button(dc, "Executar diagnóstico", diagBtn, true)
	a.hits = append(a.hits, Hit{diagBtn, "refresh-local", 0})

	perf := Rect{x + diagW + gap, row2Y, perfW, row2H}
	softCard(dc, perf)
	text(dc, "Desempenho em tempo real", Rect{perf.X + 18, perf.Y + 14, perf.W - 36, 22}, a.fonts["body"], rgb(42, 53, 72), DT_LEFT|DT_SINGLELINE)
	legendY := perf.Y + 43
	legend := []struct {
		Name  string
		Color uintptr
	}{{"CPU", rgb(47, 124, 246)}, {"RAM", rgb(26, 181, 196)}, {"Disco", rgb(42, 176, 91)}}
	for i, l := range legend {
		lx := perf.X + 18 + int32(i*68)
		fill(dc, Rect{lx, legendY + 7, 12, 2}, l.Color)
		text(dc, l.Name, Rect{lx + 18, legendY, 45, 17}, a.fonts["small"], rgb(100, 112, 133), DT_LEFT|DT_SINGLELINE)
	}
	graph := Rect{perf.X + 42, perf.Y + 72, perf.W - 62, 103}
	for i := int32(0); i <= 4; i++ {
		gy := graph.Y + i*graph.H/4
		line(dc, graph.X, gy, graph.X+graph.W, gy, rgb(238, 241, 245))
		text(dc, fmt.Sprintf("%d%%", 100-int(i)*25), Rect{perf.X + 8, gy - 6, 30, 14}, a.fonts["small"], rgb(145, 154, 169), DT_RIGHT|DT_SINGLELINE)
	}
	drawSparkline(dc, graph, sampleValues(samples, "cpu"), 100, rgb(47, 124, 246))
	drawSparkline(dc, graph, sampleValues(samples, "ram"), 100, rgb(26, 181, 196))
	drawSparkline(dc, graph, sampleValues(samples, "disk"), 100, rgb(42, 176, 91))
	valsY := perf.Y + perf.H - 38
	text(dc, fmt.Sprintf("%.0f%%  CPU", s.CPU), Rect{perf.X + 18, valsY, 80, 18}, a.fonts["small"], rgb(65, 78, 99), DT_LEFT|DT_SINGLELINE)
	text(dc, fmt.Sprintf("%.0f%%  RAM", s.Memory), Rect{perf.X + 105, valsY, 90, 18}, a.fonts["small"], rgb(65, 78, 99), DT_LEFT|DT_SINGLELINE)
	text(dc, fmt.Sprintf("%.0f%%  Disco", s.Disk), Rect{perf.X + 205, valsY, 95, 18}, a.fonts["small"], rgb(65, 78, 99), DT_LEFT|DT_SINGLELINE)

	updates := Rect{x + diagW + gap + perfW + gap, row2Y, updatesW, row2H}
	softCard(dc, updates)
	text(dc, "Atualizações", Rect{updates.X + 18, updates.Y + 14, updates.W - 36, 22}, a.fonts["body"], rgb(42, 53, 72), DT_LEFT|DT_SINGLELINE)
	text(dc, "Gerenciadas pela Central", Rect{updates.X + 18, updates.Y + 38, updates.W - 36, 18}, a.fonts["small"], rgb(118, 129, 149), DT_LEFT|DT_SINGLELINE|DT_END_ELLIPSIS)
	updateRows := []struct{ Name, Sub string }{{"Windows Update", "Sistema operacional"}, {"Drivers", "Hardware e dispositivos"}, {"Aplicativos", "Pacotes via winget"}}
	for i, row := range updateRows {
		uy := updates.Y + 75 + int32(i*43)
		text(dc, row.Name, Rect{updates.X + 18, uy, updates.W - 88, 18}, a.fonts["small"], rgb(56, 69, 90), DT_LEFT|DT_SINGLELINE|DT_END_ELLIPSIS)
		text(dc, row.Sub, Rect{updates.X + 18, uy + 18, updates.W - 88, 16}, a.fonts["small"], rgb(132, 142, 159), DT_LEFT|DT_SINGLELINE|DT_END_ELLIPSIS)
		statusPill(dc, "Central", Rect{updates.X + updates.W - 66, uy + 2, 49, 18}, rgb(237, 245, 255), rgb(47, 124, 246))
	}
	updatesLink := Rect{updates.X + 18, updates.Y + updates.H - 34, 90, 22}
	text(dc, "Abrir Central", updatesLink, a.fonts["small"], rgb(47, 124, 246), DT_LEFT|DT_VCENTER|DT_SINGLELINE)
	a.hits = append(a.hits, Hit{updatesLink, "open-web", 0})

	quick := Rect{x + mainW + gap, row2Y, rightW, row2H}
	softCard(dc, quick)
	text(dc, "Ações rápidas", Rect{quick.X + 18, quick.Y + 14, quick.W - 36, 22}, a.fonts["body"], rgb(42, 53, 72), DT_LEFT|DT_SINGLELINE)
	actions := []struct {
		Icon, A, B, Action string
		Value              int
	}{{"✓", "Verificar", "sistema", "refresh-local", 0}, {"◈", "Abrir", "diagnóstico", "page", 1}, {"↗", "Abrir", "testes", "page", 2}, {"⚡", "Otimizar", "computador", "page", 3}, {"▤", "Gerar", "relatório", "report", 0}, {"?", "Abrir", "suporte", "page", 8}}
	innerGap := int32(9)
	tileW := (quick.W - 36 - innerGap*2) / 3
	tileH := (quick.H - 59 - innerGap) / 2
	for i, act := range actions {
		col := int32(i % 3)
		row := int32(i / 3)
		r := Rect{quick.X + 18 + col*(tileW+innerGap), quick.Y + 49 + row*(tileH+innerGap), tileW, tileH}
		drawActionTile(dc, r, act.Icon, act.A, act.B, a.isHovered(r))
		a.hits = append(a.hits, Hit{r, act.Action, act.Value})
	}

	// Terceira linha
	row3Y := row2Y + row2H + gap
	processW := clamp32(w*42/100, 340, 500)
	historyW := clamp32(w*31/100, 270, 380)
	summaryW := w - processW - historyW - gap*2
	if summaryW < 230 {
		historyW -= 230 - summaryW
		summaryW = 230
	}

	procCard := Rect{x, row3Y, processW, row3H}
	softCard(dc, procCard)
	text(dc, "Processos em execução", Rect{procCard.X + 18, procCard.Y + 13, procCard.W - 130, 22}, a.fonts["body"], rgb(42, 53, 72), DT_LEFT|DT_SINGLELINE)
	procLink := Rect{procCard.X + procCard.W - 92, procCard.Y + 13, 74, 20}
	text(dc, "Ver todos", procLink, a.fonts["small"], rgb(47, 124, 246), DT_RIGHT|DT_SINGLELINE)
	a.hits = append(a.hits, Hit{procLink, "page", 4})
	line(dc, procCard.X+18, procCard.Y+42, procCard.X+procCard.W-18, procCard.Y+42, rgb(239, 241, 245))
	text(dc, "Processo", Rect{procCard.X + 18, procCard.Y + 50, procCard.W / 2, 17}, a.fonts["small"], rgb(134, 144, 160), DT_LEFT|DT_SINGLELINE)
	text(dc, "CPU", Rect{procCard.X + procCard.W - 130, procCard.Y + 50, 44, 17}, a.fonts["small"], rgb(134, 144, 160), DT_RIGHT|DT_SINGLELINE)
	text(dc, "Memória", Rect{procCard.X + procCard.W - 78, procCard.Y + 50, 60, 17}, a.fonts["small"], rgb(134, 144, 160), DT_RIGHT|DT_SINGLELINE)
	sort.Slice(processes, func(i, j int) bool { return processes[i].MemoryMB > processes[j].MemoryMB })
	rows := min(5, len(processes))
	if row3H < 195 {
		rows = min(3, len(processes))
	}
	for i := 0; i < rows; i++ {
		pr := processes[i]
		py := procCard.Y + 73 + int32(i*29)
		text(dc, nz(pr.Name, "Processo"), Rect{procCard.X + 18, py, procCard.W - 165, 18}, a.fonts["small"], rgb(66, 79, 100), DT_LEFT|DT_SINGLELINE|DT_END_ELLIPSIS)
		text(dc, fmt.Sprintf("%.1f%%", pr.CPU), Rect{procCard.X + procCard.W - 130, py, 44, 18}, a.fonts["small"], rgb(82, 95, 116), DT_RIGHT|DT_SINGLELINE)
		text(dc, fmt.Sprintf("%.0f MB", pr.MemoryMB), Rect{procCard.X + procCard.W - 78, py, 60, 18}, a.fonts["small"], rgb(82, 95, 116), DT_RIGHT|DT_SINGLELINE)
	}
	if rows == 0 {
		text(dc, "Atualize o sistema para carregar os processos.", Rect{procCard.X + 18, procCard.Y + 82, procCard.W - 36, 30}, a.fonts["small"], rgb(120, 131, 150), DT_LEFT|DT_WORDBREAK)
	}

	histCard := Rect{x + processW + gap, row3Y, historyW, row3H}
	softCard(dc, histCard)
	text(dc, "Histórico recente", Rect{histCard.X + 18, histCard.Y + 13, histCard.W - 120, 22}, a.fonts["body"], rgb(42, 53, 72), DT_LEFT|DT_SINGLELINE)
	histLink := Rect{histCard.X + histCard.W - 92, histCard.Y + 13, 74, 20}
	text(dc, "Ver histórico", histLink, a.fonts["small"], rgb(47, 124, 246), DT_RIGHT|DT_SINGLELINE)
	a.hits = append(a.hits, Hit{histLink, "page", 6})
	line(dc, histCard.X+18, histCard.Y+42, histCard.X+histCard.W-18, histCard.Y+42, rgb(239, 241, 245))
	hRows := min(5, len(history))
	if row3H < 195 {
		hRows = min(3, len(history))
	}
	for i := 0; i < hRows; i++ {
		item := history[len(history)-1-i]
		hy := histCard.Y + 57 + int32(i*31)
		circle(dc, Rect{histCard.X + 18, hy + 3, 10, 10}, rgb(42, 176, 91))
		text(dc, item.Title, Rect{histCard.X + 38, hy, histCard.W - 56, 17}, a.fonts["small"], rgb(66, 79, 100), DT_LEFT|DT_SINGLELINE|DT_END_ELLIPSIS)
		text(dc, item.At.Format("02/01 15:04"), Rect{histCard.X + 38, hy + 17, histCard.W - 56, 14}, a.fonts["small"], rgb(139, 148, 164), DT_LEFT|DT_SINGLELINE)
	}
	if hRows == 0 {
		text(dc, "As ações realizadas aparecerão aqui.", Rect{histCard.X + 18, histCard.Y + 72, histCard.W - 36, 28}, a.fonts["small"], rgb(120, 131, 150), DT_LEFT|DT_WORDBREAK)
	}

	summary := Rect{x + processW + gap + historyW + gap, row3Y, summaryW, row3H}
	softCard(dc, summary)
	text(dc, "Resumo do diagnóstico", Rect{summary.X + 18, summary.Y + 13, summary.W - 36, 22}, a.fonts["body"], rgb(42, 53, 72), DT_LEFT|DT_SINGLELINE|DT_END_ELLIPSIS)
	healthy, warn, crit := dashboardCheckStats(s, centralOK)
	total := healthy + warn + crit
	ring := clamp32(summary.W*37/100, 82, 112)
	rx := summary.X + 18
	ry := summary.Y + 55
	circle(dc, Rect{rx, ry, ring, ring}, choose(crit > 0, rgb(232, 77, 73), choose(warn > 0, rgb(241, 153, 32), rgb(42, 176, 91))))
	circle(dc, Rect{rx + 12, ry + 12, ring - 24, ring - 24}, rgb(255, 255, 255))
	text(dc, fmt.Sprintf("%d", total), Rect{rx + 10, ry + 25, ring - 20, 32}, a.fonts["h1"], rgb(31, 43, 63), DT_CENTER|DT_VCENTER|DT_SINGLELINE)
	text(dc, "checks", Rect{rx + 10, ry + 58, ring - 20, 18}, a.fonts["small"], rgb(126, 136, 153), DT_CENTER|DT_SINGLELINE)
	legendX := rx + ring + 18
	legendW := summary.X + summary.W - 18 - legendX
	legendRows := []struct {
		Name  string
		Count int
		Color uintptr
	}{{"Saudável", healthy, rgb(42, 176, 91)}, {"Atenção", warn, rgb(241, 153, 32)}, {"Crítico", crit, rgb(232, 77, 73)}}
	for i, l := range legendRows {
		ly := ry + 8 + int32(i*30)
		circle(dc, Rect{legendX, ly + 3, 8, 8}, l.Color)
		text(dc, l.Name, Rect{legendX + 16, ly, legendW - 52, 17}, a.fonts["small"], rgb(82, 95, 116), DT_LEFT|DT_SINGLELINE|DT_END_ELLIPSIS)
		text(dc, fmt.Sprintf("%d", l.Count), Rect{summary.X + summary.W - 42, ly, 24, 17}, a.fonts["small"], rgb(82, 95, 116), DT_RIGHT|DT_SINGLELINE)
	}
	reportLink := Rect{summary.X + 18, summary.Y + summary.H - 33, 130, 22}
	text(dc, "Gerar relatório completo", reportLink, a.fonts["small"], rgb(47, 124, 246), DT_LEFT|DT_VCENTER|DT_SINGLELINE)
	a.hits = append(a.hits, Hit{reportLink, "report", 0})

	if footerH > 0 {
		footerY := row3Y + row3H + gap
		footer := Rect{x, footerY, w, footerH}
		roundedBox(dc, footer, rgb(255, 255, 255), rgb(229, 233, 239), 10)
		profileName := "Nenhum"
		if activeProfile > 0 {
			profileName = optimizationProfileName(activeProfile)
		}
		footerItems := []string{fmt.Sprintf("Tempo ligado  %s", formatDuration(s.Uptime)), fmt.Sprintf("Latência  %d ms", s.LatencyMS), fmt.Sprintf("Memória  %.0f%%", s.Memory), fmt.Sprintf("Disco  %.0f%%", s.Disk), "Perfil  " + profileName}
		fw := footer.W / int32(len(footerItems))
		for i, item := range footerItems {
			text(dc, item, Rect{footer.X + int32(i)*fw + 12, footer.Y, fw - 24, footer.H}, a.fonts["small"], rgb(102, 114, 135), DT_CENTER|DT_VCENTER|DT_SINGLELINE|DT_END_ELLIPSIS)
		}
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

func drawModuleIntro(dc syscall.Handle, r Rect, title, desc string) {
	softCard(dc, r)
	text(dc, title, Rect{r.X + 20, r.Y + 14, r.W - 40, 25}, app.fonts["h2"], rgb(31, 43, 63), DT_LEFT|DT_VCENTER|DT_SINGLELINE|DT_END_ELLIPSIS)
	text(dc, desc, Rect{r.X + 20, r.Y + 42, r.W - 40, r.H - 52}, app.fonts["small"], rgb(111, 124, 145), DT_LEFT|DT_WORDBREAK)
}

func drawMiniStat(dc syscall.Handle, r Rect, label, value, detail string, color uintptr) {
	softCard(dc, r)
	text(dc, label, Rect{r.X + 16, r.Y + 14, r.W - 32, 20}, app.fonts["small"], rgb(103, 117, 139), DT_LEFT|DT_SINGLELINE|DT_END_ELLIPSIS)
	text(dc, value, Rect{r.X + 16, r.Y + 38, r.W - 32, 34}, app.fonts["h1"], color, DT_LEFT|DT_VCENTER|DT_SINGLELINE|DT_END_ELLIPSIS)
	text(dc, detail, Rect{r.X + 16, r.Y + 76, r.W - 32, 28}, app.fonts["small"], rgb(117, 130, 150), DT_LEFT|DT_WORDBREAK|DT_END_ELLIPSIS)
}

func statusIconKind(title string) string {
	name := strings.ToLower(strings.TrimSpace(title))
	switch {
	case strings.Contains(name, "internet") || strings.Contains(name, "conectividade"):
		return "internet"
	case strings.Contains(name, "áudio") || strings.Contains(name, "audio") || strings.Contains(name, "saída") || strings.Contains(name, "saida"):
		return "audio"
	case strings.Contains(name, "microfone"):
		return "microphone"
	case strings.Contains(name, "central") || strings.Contains(name, "servidor"):
		return "server"
	default:
		return "status"
	}
}

func drawStatusIcon(dc syscall.Handle, kind string, r Rect, color uintptr) {
	// Ícones lineares, sem bloco/caixinha: a cor comunica o estado.
	switch kind {
	case "internet":
		line(dc, r.X+2, r.Y+7, r.X+r.W/2, r.Y+3, color)
		line(dc, r.X+r.W/2, r.Y+3, r.X+r.W-2, r.Y+7, color)
		line(dc, r.X+5, r.Y+11, r.X+r.W/2, r.Y+8, color)
		line(dc, r.X+r.W/2, r.Y+8, r.X+r.W-5, r.Y+11, color)
		line(dc, r.X+8, r.Y+15, r.X+r.W/2, r.Y+13, color)
		line(dc, r.X+r.W/2, r.Y+13, r.X+r.W-8, r.Y+15, color)
		circle(dc, Rect{r.X + r.W/2 - 2, r.Y + r.H - 3, 4, 4}, color)
	case "audio":
		// Speaker outline, sem preenchimento.
		strokeRoundRect(dc, Rect{r.X + 1, r.Y + 7, 5, 8}, color, 2, 1)
		line(dc, r.X+6, r.Y+7, r.X+11, r.Y+3, color)
		line(dc, r.X+11, r.Y+3, r.X+11, r.Y+19, color)
		line(dc, r.X+11, r.Y+19, r.X+6, r.Y+15, color)
		line(dc, r.X+15, r.Y+8, r.X+18, r.Y+11, color)
		line(dc, r.X+18, r.Y+11, r.X+15, r.Y+14, color)
		line(dc, r.X+18, r.Y+5, r.X+21, r.Y+8, color)
		line(dc, r.X+21, r.Y+8, r.X+21, r.Y+14, color)
		line(dc, r.X+21, r.Y+14, r.X+18, r.Y+17, color)
	case "microphone":
		strokeRoundRect(dc, Rect{r.X + 7, r.Y + 2, 9, 13}, color, 7, 1)
		line(dc, r.X+4, r.Y+10, r.X+4, r.Y+12, color)
		line(dc, r.X+4, r.Y+12, r.X+8, r.Y+17, color)
		line(dc, r.X+8, r.Y+17, r.X+15, r.Y+17, color)
		line(dc, r.X+15, r.Y+17, r.X+19, r.Y+12, color)
		line(dc, r.X+19, r.Y+12, r.X+19, r.Y+10, color)
		line(dc, r.X+12, r.Y+17, r.X+12, r.Y+21, color)
		line(dc, r.X+8, r.Y+21, r.X+16, r.Y+21, color)
	case "server":
		strokeRoundRect(dc, Rect{r.X + 2, r.Y + 3, r.W - 4, 7}, color, 3, 1)
		strokeRoundRect(dc, Rect{r.X + 2, r.Y + 14, r.W - 4, 7}, color, 3, 1)
		circle(dc, Rect{r.X + 5, r.Y + 6, 2, 2}, color)
		circle(dc, Rect{r.X + 5, r.Y + 17, 2, 2}, color)
		line(dc, r.X+10, r.Y+6, r.X+r.W-5, r.Y+6, color)
		line(dc, r.X+10, r.Y+17, r.X+r.W-5, r.Y+17, color)
	default:
		strokeCircle(dc, Rect{r.X + 5, r.Y + 5, r.W - 10, r.H - 10}, color, 1)
	}
}

func drawStatusRow(dc syscall.Handle, r Rect, title, detail, status string, ok bool, actionLabel, action string) {
	fill(dc, Rect{r.X, r.Y + r.H - 1, r.W, 1}, rgb(237, 239, 243))
	iconColor := choose(ok, rgb(42, 176, 91), rgb(241, 153, 32))
	drawStatusIcon(dc, statusIconKind(title), Rect{r.X + 3, r.Y + 14, 22, 22}, iconColor)
	text(dc, title, Rect{r.X + 38, r.Y + 8, 250, 24}, app.fonts["body"], rgb(37, 50, 71), DT_LEFT|DT_VCENTER|DT_SINGLELINE|DT_END_ELLIPSIS)
	text(dc, detail, Rect{r.X + 38, r.Y + 32, r.W - 314, 22}, app.fonts["small"], rgb(112, 125, 146), DT_LEFT|DT_SINGLELINE|DT_END_ELLIPSIS)
	text(dc, status, Rect{r.X + r.W - 285, r.Y + 12, 110, 28}, app.fonts["small"], iconColor, DT_RIGHT|DT_VCENTER|DT_SINGLELINE)
	if action != "" {
		br := Rect{r.X + r.W - 155, r.Y + 9, 145, 36}
		button(dc, actionLabel, br, false)
		app.hits = append(app.hits, Hit{br, action, 0})
	}
}

func (a *App) drawDiagnostics(dc syscall.Handle) {
	x, y := contentOrigin()
	a.mu.RLock()
	s := a.sys
	centralOK := a.centralOK
	a.mu.RUnlock()
	w := a.width - x - 28
	gap := int32(12)

	// Identificação em uma faixa compacta, no mesmo padrão do Painel.
	info := Rect{x, y, w, 120}
	softCard(dc, info)
	text(dc, "Identificação do computador", Rect{x + 20, y + 14, 310, 26}, a.fonts["h2"], rgb(31, 43, 63), DT_LEFT|DT_VCENTER|DT_SINGLELINE)
	items := []struct{ label, value string }{
		{"Computador", nz(s.Hostname, "Não identificado")},
		{"Sistema", nz(s.OS, "Windows")},
		{"Fabricante / modelo", strings.TrimSpace(nz(s.Manufacturer, "Não identificado") + " • " + nz(s.Model, "Modelo não identificado"))},
		{"Tempo ligado", formatDuration(s.Uptime)},
	}
	colW := (w - 40) / 4
	for i, it := range items {
		cx := x + 20 + int32(i)*colW
		text(dc, it.label, Rect{cx, y + 52, colW - 16, 18}, a.fonts["small"], rgb(119, 131, 150), DT_LEFT|DT_SINGLELINE)
		text(dc, it.value, Rect{cx, y + 72, colW - 16, 28}, a.fonts["body"], rgb(34, 47, 68), DT_LEFT|DT_VCENTER|DT_SINGLELINE|DT_END_ELLIPSIS)
	}

	metricY := y + 134
	mw := (w - gap*3) / 4
	metrics := []struct {
		label, value, detail string
		color                uintptr
	}{
		{"Processador", fmt.Sprintf("%.0f%%", s.CPU), nz(s.CPUName, "Processador"), metricColor(s.CPU)},
		{"Memória RAM", fmt.Sprintf("%.0f%%", s.Memory), fmt.Sprintf("%.1f de %.1f GB em uso", s.UsedRAMGB, s.TotalRAMGB), metricColor(s.Memory)},
		{"Armazenamento", fmt.Sprintf("%.0f%%", s.Disk), fmt.Sprintf("%.0f GB livres", s.DiskTotalGB-s.DiskUsedGB), metricColor(s.Disk)},
		{"Internet", chooseText(s.InternetOK, "Online", "Offline"), fmt.Sprintf("Latência %d ms", s.LatencyMS), choose(s.InternetOK, rgb(42, 176, 91), rgb(232, 77, 73))},
	}
	for i, m := range metrics {
		drawMiniStat(dc, Rect{x + int32(i)*(mw+gap), metricY, mw, 112}, m.label, m.value, m.detail, m.color)
	}

	bottomY := metricY + 126
	leftW := (w - gap) * 58 / 100
	rightW := w - gap - leftW
	left := Rect{x, bottomY, leftW, 310}
	right := Rect{x + leftW + gap, bottomY, rightW, 310}
	softCard(dc, left)
	softCard(dc, right)

	text(dc, "Hardware e sistema", Rect{left.X + 20, left.Y + 15, left.W - 40, 26}, a.fonts["h2"], rgb(31, 43, 63), DT_LEFT|DT_VCENTER|DT_SINGLELINE)
	hardware := []struct{ k, v string }{
		{"Processador", nz(s.CPUName, "Não identificado")},
		{"Memória instalada", fmt.Sprintf("%.1f GB", s.TotalRAMGB)},
		{"Disco principal", strings.TrimSpace(nz(s.DiskName, "Unidade C:") + " • " + nz(s.DiskType, "Armazenamento"))},
		{"Capacidade", fmt.Sprintf("%.0f GB total • %.0f GB livres", s.DiskTotalGB, s.DiskTotalGB-s.DiskUsedGB)},
		{"Número de série", nz(s.Serial, "Não identificado")},
	}
	for i, it := range hardware {
		ry := left.Y + 56 + int32(i)*47
		if i > 0 {
			line(dc, left.X+20, ry-8, left.X+left.W-20, ry-8, rgb(238, 240, 244))
		}
		text(dc, it.k, Rect{left.X + 20, ry, 155, 20}, a.fonts["small"], rgb(118, 130, 149), DT_LEFT|DT_SINGLELINE)
		text(dc, it.v, Rect{left.X + 185, ry - 2, left.W - 205, 24}, a.fonts["body"], rgb(41, 54, 75), DT_RIGHT|DT_VCENTER|DT_SINGLELINE|DT_END_ELLIPSIS)
	}

	text(dc, "Conectividade e dispositivos", Rect{right.X + 20, right.Y + 15, right.W - 40, 26}, a.fonts["h2"], rgb(31, 43, 63), DT_LEFT|DT_VCENTER|DT_SINGLELINE)
	statuses := []struct {
		title, detail string
		ok            bool
	}{
		{"Internet", fmt.Sprintf("Latência aproximada de %d ms", s.LatencyMS), s.InternetOK},
		{"Áudio", chooseText(s.AudioOK, "Dispositivo de saída detectado", "Saída de áudio não detectada"), s.AudioOK},
		{"Microfone", chooseText(s.MicOK, "Dispositivo de entrada detectado", "Microfone não detectado"), s.MicOK},
		{"Central CoreControl", chooseText(centralOK, "Comunicação confirmada", "Conexão não confirmada"), centralOK},
	}
	for i, st := range statuses {
		ry := right.Y + 51 + int32(i)*53
		iconColor := choose(st.ok, rgb(42, 176, 91), rgb(241, 153, 32))
		drawStatusIcon(dc, statusIconKind(st.title), Rect{right.X + 20, ry + 5, 22, 22}, iconColor)
		text(dc, st.title, Rect{right.X + 56, ry, 145, 22}, a.fonts["body"], rgb(38, 51, 72), DT_LEFT|DT_SINGLELINE)
		text(dc, st.detail, Rect{right.X + 56, ry + 23, right.W - 159, 20}, a.fonts["small"], rgb(115, 128, 148), DT_LEFT|DT_SINGLELINE|DT_END_ELLIPSIS)
		text(dc, chooseText(st.ok, "OK", "Atenção"), Rect{right.X + right.W - 90, ry + 4, 65, 22}, a.fonts["small"], iconColor, DT_RIGHT|DT_SINGLELINE)
	}
	br := Rect{right.X + 20, right.Y + right.H - 52, right.W - 40, 36}
	button(dc, "Atualizar diagnóstico", br, true)
	a.hits = append(a.hits, Hit{br, "refresh-all", 0})
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
	testRunning := a.testRunning
	testLastAction := a.testLastAction
	testLastAt := a.testLastAt
	testLastOK := a.testLastOK
	testLastMessage := a.testLastMessage
	a.mu.RUnlock()
	gap := int32(12)
	mw := (w - gap*3) / 4
	cards := []struct {
		label, value, detail string
		ok                   bool
	}{
		{"Internet", chooseText(s.InternetOK, "Online", "Offline"), fmt.Sprintf("Latência %d ms", s.LatencyMS), s.InternetOK},
		{"Áudio", chooseText(s.AudioOK, "Detectado", "Atenção"), "Saída de áudio do Windows", s.AudioOK},
		{"Microfone", chooseText(s.MicOK, "Detectado", "Atenção"), "Entrada de áudio do Windows", s.MicOK},
		{"CoreControl", chooseText(centralOK, "Conectado", "Atenção"), nz(serverURL, "Servidor não definido"), centralOK},
	}
	for i, c := range cards {
		color := choose(c.ok, rgb(42, 176, 91), rgb(241, 153, 32))
		drawMiniStat(dc, Rect{x + int32(i)*(mw+gap), y, mw, 112}, c.label, c.value, c.detail, color)
	}

	panelY := y + 126
	panel := Rect{x, panelY, w, 360}
	softCard(dc, panel)
	text(dc, "Central de testes", Rect{x + 20, panelY + 14, 260, 26}, a.fonts["h2"], rgb(31, 43, 63), DT_LEFT|DT_VCENTER|DT_SINGLELINE)
	text(dc, "Execute novamente qualquer verificação sem alterar configurações do computador.", Rect{x + 20, panelY + 42, w - 40, 22}, a.fonts["small"], rgb(112, 125, 146), DT_LEFT|DT_SINGLELINE|DT_END_ELLIPSIS)
	tests := []struct {
		title, detail, status, action string
		ok                            bool
	}{
		{"Conectividade com a internet", fmt.Sprintf("Verifica acesso e atualiza a latência atual (%d ms).", s.LatencyMS), chooseText(s.InternetOK, "Funcionando", "Verificar"), "test-internet", s.InternetOK},
		{"Saída de áudio", chooseText(s.AudioOK, "Dispositivo de saída detectado pelo Windows.", "Nenhuma saída de áudio foi confirmada."), chooseText(s.AudioOK, "Detectado", "Atenção"), "test-audio", s.AudioOK},
		{"Microfone", chooseText(s.MicOK, "Dispositivo de entrada detectado pelo Windows.", "Nenhum microfone foi confirmado."), chooseText(s.MicOK, "Detectado", "Atenção"), "test-mic", s.MicOK},
		{"Conexão com a Central", nz(serverURL, "Servidor não configurado"), chooseText(centralOK, "Conectado", "Verificar"), "test-central", centralOK},
	}
	for i, t := range tests {
		detail := t.detail
		status := t.status
		ok := t.ok
		actionLabel := "Executar teste"
		if testRunning == t.action {
			detail = "Executando verificação agora..."
			status = "Testando..."
			ok = false
			actionLabel = "Testando..."
		} else if testLastAction == t.action && !testLastAt.IsZero() {
			detail = testLastMessage
			ok = testLastOK
			if testLastOK {
				status = "Concluído • " + testLastAt.Format("15:04:05")
			} else {
				status = "Falhou • " + testLastAt.Format("15:04:05")
			}
			actionLabel = "Testar novamente"
		}
		r := Rect{panel.X + 20, panel.Y + 73 + int32(i)*67, panel.W - 40, 58}
		drawStatusRow(dc, r, t.title, detail, status, ok, actionLabel, t.action)
	}

	note := Rect{x, panelY + 374, w, 112}
	drawModuleIntro(dc, note, "Privacidade durante os testes", "As verificações usam apenas informações técnicas do Windows e conectividade. O CoreControl não grava conversas, não lê documentos e não acessa senhas para executar estes testes.")
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
	verification, _ := loadOptimizationVerification()
	gap := int32(10)

	intro := "Nenhum perfil é aplicado automaticamente. Escolha um perfil para revisar as alterações antes de confirmar."
	if activeProfile > 0 {
		intro = "Perfil ativo: " + optimizationProfileName(activeProfile)
		if !activeAt.IsZero() {
			intro += " • " + activeAt.Format("02/01/2006 15:04")
		}
	}
	drawModuleIntro(dc, Rect{x, y, w, 76}, "Otimização segura", intro)

	cardY := y + 90
	cw := (w - gap*4) / 5
	for i := 1; i <= 5; i++ {
		p := optimizationProfileExplanation(i)
		r := Rect{x + int32(i-1)*(cw+gap), cardY, cw, 166}
		selected := a.profile == i
		active := activeProfile == i
		softCard(dc, r)
		if selected || active {
			roundedBox(dc, Rect{r.X + 1, r.Y + 1, r.W - 2, r.H - 2}, rgb(250, 252, 255), rgb(92, 151, 246), 11)
		}
		circle(dc, Rect{r.X + 16, r.Y + 16, 32, 32}, choose(selected || active, rgb(234, 243, 255), rgb(244, 246, 249)))
		text(dc, fmt.Sprintf("%d", i), Rect{r.X + 16, r.Y + 16, 32, 32}, a.fonts["small"], choose(selected || active, rgb(47, 124, 246), rgb(103, 116, 138)), DT_CENTER|DT_VCENTER|DT_SINGLELINE)
		text(dc, p.Name, Rect{r.X + 58, r.Y + 15, r.W - 72, 26}, a.fonts["body"], rgb(34, 47, 68), DT_LEFT|DT_VCENTER|DT_SINGLELINE|DT_END_ELLIPSIS)
		text(dc, p.Short, Rect{r.X + 16, r.Y + 58, r.W - 32, 48}, a.fonts["small"], rgb(112, 125, 146), DT_LEFT|DT_WORDBREAK)
		status := "Ver e selecionar"
		if active {
			status = "Ativo"
		} else if selected {
			status = "Selecionado"
		}
		br := Rect{r.X + 16, r.Y + r.H - 46, r.W - 32, 32}
		button(dc, status, br, selected)
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
	bottomY := cardY + 180
	leftW := (w - gap) * 62 / 100
	left := Rect{x, bottomY, leftW, 300}
	right := Rect{x + leftW + gap, bottomY, w - leftW - gap, 300}
	softCard(dc, left)
	softCard(dc, right)
	text(dc, "O que o perfil "+detail.Name+" fará", Rect{left.X + 20, left.Y + 15, left.W - 40, 26}, a.fonts["h2"], rgb(31, 43, 63), DT_LEFT|DT_VCENTER|DT_SINGLELINE|DT_END_ELLIPSIS)
	text(dc, detail.Name+" — "+detail.Summary, Rect{left.X + 20, left.Y + 46, left.W - 40, 42}, a.fonts["small"], rgb(108, 122, 143), DT_LEFT|DT_WORDBREAK)
	for i, action := range detail.Actions {
		if i >= 4 {
			break
		}
		ry := left.Y + 98 + int32(i)*38
		circle(dc, Rect{left.X + 21, ry + 5, 8, 8}, rgb(42, 176, 91))
		text(dc, action, Rect{left.X + 42, ry, left.W - 62, 30}, a.fonts["body"], rgb(48, 62, 83), DT_LEFT|DT_WORDBREAK|DT_END_ELLIPSIS)
	}
	text(dc, detail.Result, Rect{left.X + 20, left.Y + 250, left.W - 40, 34}, a.fonts["small"], rgb(104, 118, 139), DT_LEFT|DT_WORDBREAK)

	if verification != nil {
		verifiedAll := verification.Total > 0 && verification.Confirmed == verification.Total && verification.Error == ""
		title := "Aplicado e verificado"
		accent := rgb(42, 176, 91)
		if verification.Profile == 5 {
			title = "Otimização desativada"
			if !verifiedAll {
				title = "Restauração com observações"
				accent = rgb(244, 148, 35)
			}
		} else if !verifiedAll {
			title = "Aplicado com observações"
			accent = rgb(244, 148, 35)
		}
		circle(dc, Rect{right.X + 20, right.Y + 18, 11, 11}, accent)
		text(dc, title, Rect{right.X + 42, right.Y + 10, right.W - 62, 28}, a.fonts["h2"], rgb(31, 43, 63), DT_LEFT|DT_VCENTER|DT_SINGLELINE|DT_END_ELLIPSIS)
		summary := fmt.Sprintf("%d de %d alterações confirmadas pelo Windows • %s", verification.Confirmed, verification.Total, verification.At.Format("15:04"))
		if verification.Profile == 5 && verifiedAll {
			summary = "Nenhum perfil ativo • configurações originais restauradas • " + verification.At.Format("15:04")
		} else if verification.Total == 0 {
			summary = "Operação concluída e registrada • " + verification.At.Format("15:04")
		}
		text(dc, summary, Rect{right.X + 20, right.Y + 42, right.W - 40, 22}, a.fonts["small"], rgb(106, 120, 141), DT_LEFT|DT_SINGLELINE|DT_END_ELLIPSIS)
		shown := 0
		for _, item := range verification.Items {
			if shown >= 4 {
				break
			}
			ry := right.Y + 76 + int32(shown)*34
			dot := rgb(113, 126, 146)
			if item.Status == "verified" {
				dot = rgb(42, 176, 91)
			} else if item.Status == "warning" {
				dot = rgb(244, 148, 35)
			}
			circle(dc, Rect{right.X + 20, ry + 5, 9, 9}, dot)
			value := item.After
			if value == "" {
				value = item.Note
			}
			text(dc, item.Label, Rect{right.X + 42, ry - 2, right.W * 48 / 100, 22}, a.fonts["body"], rgb(48, 62, 83), DT_LEFT|DT_SINGLELINE|DT_END_ELLIPSIS)
			text(dc, value, Rect{right.X + right.W*52/100, ry - 2, right.W*48/100 - 20, 22}, a.fonts["small"], dot, DT_RIGHT|DT_SINGLELINE|DT_END_ELLIPSIS)
			shown++
		}
		compare := Rect{right.X + 20, right.Y + right.H - 96, right.W - 40, 32}
		button(dc, "Ver antes × depois", compare, false)
		a.hits = append(a.hits, Hit{compare, "comparison-report", 0})
	} else {
		text(dc, "Proteções em todos os perfis", Rect{right.X + 20, right.Y + 15, right.W - 40, 26}, a.fonts["h2"], rgb(31, 43, 63), DT_LEFT|DT_VCENTER|DT_SINGLELINE)
		protected := []string{"Sem backup seguro, nenhuma alteração é iniciada.", "Nenhum arquivo ou pasta é apagado ou movido.", "Defender e Firewall preservados", "Nenhum programa encerrado à força", "Restauração disponível"}
		for i, item := range protected {
			ry := right.Y + 55 + int32(i)*38
			circle(dc, Rect{right.X + 20, ry + 4, 9, 9}, rgb(42, 176, 91))
			text(dc, item, Rect{right.X + 42, ry - 2, right.W - 62, 24}, a.fonts["body"], rgb(48, 62, 83), DT_LEFT|DT_SINGLELINE|DT_END_ELLIPSIS)
		}
	}
	apply := Rect{right.X + 20, right.Y + right.H - 56, right.W - 40, 38}
	label := "Aplicar perfil com backup"
	if a.profile == 5 {
		label = "Desativar perfil e restaurar"
	} else if verification != nil && a.profile != 0 && verification.Profile == a.profile {
		label = "Aplicar novamente e verificar"
	}
	if optimizationBusy {
		label = "Aplicando e verificando..."
	}
	if activeProfile > 0 && a.profile != 5 && !optimizationBusy {
		disableW := (apply.W - 10) * 38 / 100
		disable := Rect{apply.X, apply.Y, disableW, apply.H}
		primary := Rect{apply.X + disableW + 10, apply.Y, apply.W - disableW - 10, apply.H}
		button(dc, "Desativar perfil", disable, false)
		button(dc, label, primary, true)
		a.hits = append(a.hits, Hit{disable, "disable-profile", 0})
		a.hits = append(a.hits, Hit{primary, "apply-profile", 0})
	} else {
		button(dc, label, apply, true)
		if !optimizationBusy {
			a.hits = append(a.hits, Hit{apply, "apply-profile", 0})
		}
	}
	if optimizationNote != "" {
		text(dc, optimizationNote, Rect{x, bottomY + 312, w, 28}, a.fonts["small"], rgb(109, 122, 143), DT_LEFT|DT_SINGLELINE|DT_END_ELLIPSIS)
	}
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
	y -= a.pageScrollY
	w := a.width - x - 28
	if w < 360 {
		w = 360
	}
	a.mu.RLock()
	apps := append([]ActivityApp(nil), a.activityApps...)
	processes := append([]ProcessInfo(nil), a.processes...)
	browserTabs := append([]BrowserTab(nil), a.browserTabs...)
	sessions := append([]ActivitySession(nil), a.activitySessions...)
	var current *ActivitySession
	if a.activityCurrent != nil {
		copyCurrent := *a.activityCurrent
		current = &copyCurrent
	}
	s := a.sys
	expanded := make(map[string]bool, len(a.activityExpanded))
	for k, v := range a.activityExpanded {
		expanded[k] = v
	}
	a.mu.RUnlock()

	now := time.Now()
	groups := buildActivityGroups(apps, processes)
	focusName := "Nenhum aplicativo"
	focusDetail := "Aguardando uma janela em primeiro plano"
	if current != nil {
		focusName = friendlyProcessName(nz(current.ProcessName, "Aplicativo"))
		focusDetail = nz(current.WindowTitle, "Janela em primeiro plano")
	}

	// Layout adaptativo: prioriza o que já é medido de verdade por processo.
	compactHeader := w < 760
	showStatus := w >= 560
	showTime := w >= 700
	rowH := int32(44)
	childH := int32(48)
	groupGap := int32(7)
	expandedGap := int32(9)
	headOffset := int32(126)
	if compactHeader {
		headOffset = 166
	}
	rowsHeight := int32(len(groups)) * (rowH + groupGap)
	for _, g := range groups {
		key := strings.ToLower(strings.TrimSpace(g.Name))
		if expanded[key] {
			childrenCount := len(activityChildrenForGroup(g.Name, apps, processes, browserTabs))
			if childrenCount > 0 {
				rowsHeight += expandedGap*2 + int32(childrenCount)*childH
			}
		}
	}
	panelH := headOffset + 54 + rowsHeight
	minPanelH := int32(520)
	if compactHeader {
		minPanelH = 560
	}
	if panelH < minPanelH {
		panelH = minPanelH
	}

	panel := Rect{x, y, w, panelH}
	softCard(dc, panel)
	text(dc, "Processos e aplicativos", Rect{panel.X + 20, panel.Y + 15, panel.W - 40, 28}, a.fonts["h2"], rgb(31, 43, 63), DT_LEFT|DT_VCENTER|DT_SINGLELINE|DT_END_ELLIPSIS)

	stats := []struct {
		v, l string
		c    uintptr
	}{
		{fmt.Sprintf("%.0f%%", s.CPU), "CPU", metricColor(s.CPU)},
		{fmt.Sprintf("%.0f%%", s.Memory), "RAM", metricColor(s.Memory)},
		{fmt.Sprintf("%.0f%%", s.Disk), "Disco", metricColor(s.Disk)},
		{chooseLabel(s.InternetOK, "Online", "Offline"), "Rede", choose(s.InternetOK, rgb(42, 176, 91), rgb(232, 77, 73))},
	}

	if !compactHeader {
		statW := int32(68)
		statGap := int32(7)
		statsX := panel.X + panel.W - 20 - int32(len(stats))*statW - int32(len(stats)-1)*statGap
		leftInfoW := statsX - (panel.X + 20) - 16
		if leftInfoW < 160 {
			leftInfoW = 160
		}
		text(dc, "Tudo que está rodando neste computador, com uso de recursos e janela atual.", Rect{panel.X + 20, panel.Y + 43, leftInfoW, 20}, a.fonts["small"], rgb(108, 122, 143), DT_LEFT|DT_SINGLELINE|DT_END_ELLIPSIS)
		text(dc, "Em foco: "+focusName+" • "+focusDetail, Rect{panel.X + 20, panel.Y + 68, leftInfoW, 20}, a.fonts["small"], rgb(47, 124, 246), DT_LEFT|DT_SINGLELINE|DT_END_ELLIPSIS)
		for i, st := range stats {
			sx := statsX + int32(i)*(statW+statGap)
			r := Rect{sx, panel.Y + 14, statW, 58}
			roundedBox(dc, r, rgb(248, 250, 253), rgb(226, 232, 240), 8)
			text(dc, st.v, Rect{r.X + 2, r.Y + 8, r.W - 4, 20}, a.fonts["body"], st.c, DT_CENTER|DT_SINGLELINE|DT_END_ELLIPSIS)
			text(dc, st.l, Rect{r.X + 2, r.Y + 32, r.W - 4, 16}, a.fonts["small"], rgb(109, 122, 143), DT_CENTER|DT_SINGLELINE)
		}
		refresh := Rect{panel.X + panel.W - 130, panel.Y + 82, 108, 32}
		button(dc, "Atualizar", refresh, false)
		a.hits = append(a.hits, Hit{refresh, "refresh-activity", 0})
	} else {
		text(dc, "Em foco: "+focusName+" • "+focusDetail, Rect{panel.X + 20, panel.Y + 45, panel.W - 40, 20}, a.fonts["small"], rgb(47, 124, 246), DT_LEFT|DT_SINGLELINE|DT_END_ELLIPSIS)
		statGap := int32(6)
		statW := (panel.W - 40 - statGap*3) / 4
		for i, st := range stats {
			sx := panel.X + 20 + int32(i)*(statW+statGap)
			r := Rect{sx, panel.Y + 76, statW, 50}
			roundedBox(dc, r, rgb(248, 250, 253), rgb(226, 232, 240), 8)
			text(dc, st.v, Rect{r.X + 2, r.Y + 6, r.W - 4, 18}, a.fonts["small"], st.c, DT_CENTER|DT_SINGLELINE|DT_END_ELLIPSIS)
			text(dc, st.l, Rect{r.X + 2, r.Y + 28, r.W - 4, 15}, a.fonts["small"], rgb(109, 122, 143), DT_CENTER|DT_SINGLELINE)
		}
		refreshW := clamp32(panel.W/4, 88, 118)
		refresh := Rect{panel.X + panel.W - 20 - refreshW, panel.Y + 132, refreshW, 28}
		button(dc, "Atualizar", refresh, false)
		a.hits = append(a.hits, Hit{refresh, "refresh-activity", 0})
	}

	headY := panel.Y + headOffset
	innerX := panel.X + 14
	innerW := panel.W - 28
	fill(dc, Rect{innerX, headY, innerW, 38}, rgb(247, 249, 252))
	line(dc, innerX, headY+38, innerX+innerW, headY+38, rgb(226, 231, 238))

	// Distribuição proporcional: sem colunas vazias. Nome recebe a maior parte do espaço.
	nameX := innerX + 10
	usableW := innerW - 20
	statusX, cpuX, memX, timeX := int32(0), int32(0), int32(0), int32(0)
	statusW, cpuW, memW, timeW := int32(0), int32(0), int32(0), int32(0)
	var nameW int32
	if showStatus && showTime {
		nameW = usableW * 50 / 100
		statusW = usableW * 13 / 100
		cpuW = usableW * 10 / 100
		memW = usableW * 14 / 100
		timeW = usableW - nameW - statusW - cpuW - memW
	} else if showStatus {
		nameW = usableW * 59 / 100
		statusW = usableW * 15 / 100
		cpuW = usableW * 11 / 100
		memW = usableW - nameW - statusW - cpuW
	} else {
		nameW = usableW * 64 / 100
		cpuW = usableW * 15 / 100
		memW = usableW - nameW - cpuW
	}
	cursorX := nameX + nameW
	if showStatus {
		statusX = cursorX
		cursorX += statusW
	}
	cpuX = cursorX
	cursorX += cpuW
	memX = cursorX
	cursorX += memW
	if showTime {
		timeX = cursorX
	}
	showSubTitle := nameW >= 300

	text(dc, "Nome", Rect{nameX, headY + 10, nameW, 20}, a.fonts["small"], rgb(101, 115, 136), DT_LEFT|DT_SINGLELINE|DT_END_ELLIPSIS)
	if showStatus {
		text(dc, "Status", Rect{statusX, headY + 10, statusW, 20}, a.fonts["small"], rgb(101, 115, 136), DT_LEFT|DT_SINGLELINE|DT_END_ELLIPSIS)
	}
	text(dc, fmt.Sprintf("%.0f%% CPU", s.CPU), Rect{cpuX, headY + 10, cpuW, 20}, a.fonts["small"], rgb(64, 78, 100), DT_CENTER|DT_SINGLELINE|DT_END_ELLIPSIS)
	text(dc, fmt.Sprintf("%.0f%% RAM", s.Memory), Rect{memX, headY + 10, memW, 20}, a.fonts["small"], rgb(64, 78, 100), DT_CENTER|DT_SINGLELINE|DT_END_ELLIPSIS)
	if showTime {
		text(dc, "Tempo em foco", Rect{timeX, headY + 10, timeW, 20}, a.fonts["small"], rgb(64, 78, 100), DT_CENTER|DT_SINGLELINE|DT_END_ELLIPSIS)
	}
	// Divisórias verticais sutis ajudam o olho a acompanhar a coluna ao redimensionar.
	dividers := []int32{cpuX, memX}
	if showStatus {
		dividers = append([]int32{statusX}, dividers...)
	}
	if showTime {
		dividers = append(dividers, timeX)
	}
	for _, dx := range dividers {
		line(dc, dx, headY, dx, panel.Y+panel.H-16, rgb(235, 239, 244))
	}

	rows := len(groups)
	if rows == 0 {
		text(dc, "Nenhum processo foi encontrado.", Rect{panel.X + 20, headY + 80, panel.W - 40, 30}, a.fonts["body"], rgb(109, 122, 143), DT_CENTER|DT_SINGLELINE)
	} else {
		maxMem := 1.0
		maxCPU := 1.0
		for _, g := range groups {
			if g.MemoryMB > maxMem {
				maxMem = g.MemoryMB
			}
			if g.CPU > maxCPU {
				maxCPU = g.CPU
			}
		}
		rowY := headY + 43
		for i, g := range groups {
			ry := rowY
			row := Rect{innerX, ry, innerW, rowH - 3}
			if i%2 == 1 {
				fill(dc, row, rgb(251, 252, 254))
			}
			if g.Focused {
				roundedBox(dc, row, rgb(237, 245, 255), rgb(160, 196, 246), 5)
			}
			fill(dc, Rect{cpuX, ry + 2, cpuW, rowH - 7}, activityHeatColorLight(g.CPU, maxCPU))
			fill(dc, Rect{memX, ry + 2, memW, rowH - 7}, activityHeatColorLight(g.MemoryMB, maxMem))

			key := strings.ToLower(strings.TrimSpace(g.Name))
			children := activityChildrenForGroup(g.Name, apps, processes, browserTabs)
			isExpanded := expanded[key]
			arrowW := int32(14)
			dotX := nameX + arrowW + 6
			labelX := dotX + 22
			labelW := nameW - (labelX - nameX)
			if labelW < 70 {
				labelW = 70
			}
			arrow := "›"
			if isExpanded {
				arrow = "⌄"
			}
			if len(children) > 0 {
				toggleRect := Rect{nameX - 4, ry + 1, nameW, rowH - 5}
				a.hits = append(a.hits, Hit{toggleRect, "toggle-activity-group", i})
				text(dc, arrow, Rect{nameX, ry + 9, arrowW, 20}, a.fonts["body"], rgb(104, 121, 148), DT_LEFT|DT_SINGLELINE)
			}
			iconRect := Rect{dotX - 5, ry + 9, 18, 18}
			if !drawProcessIcon(dc, g.ExePath, iconRect) {
				circle(dc, Rect{dotX, ry + 14, 7, 7}, choose(g.Focused, rgb(47, 124, 246), rgb(160, 174, 195)))
			}
			label := friendlyProcessName(g.Name)
			browserGroupTabs := browserTabsForProcess(g.Name, browserTabs)
			if len(browserGroupTabs) > 0 {
				label = fmt.Sprintf("%s (%d abas)", label, len(browserGroupTabs))
			} else if g.Count > 1 {
				label = fmt.Sprintf("%s (%d)", label, g.Count)
			}
			labelY := ry + 10
			if showSubTitle {
				labelY = ry + 5
			}
			text(dc, label, Rect{labelX, labelY, labelW, 20}, a.fonts["body"], rgb(40, 54, 75), DT_LEFT|DT_SINGLELINE|DT_END_ELLIPSIS)
			if showSubTitle {
				sub := ""
				if g.Title != "" {
					pageName, pageDetail := friendlyActivityWindow(g.Title, g.Name)
					sub = pageName
					if pageDetail != "" && !strings.EqualFold(pageDetail, pageName) && !strings.EqualFold(pageDetail, friendlyProcessName(g.Name)) {
						sub += " • " + pageDetail
					}
				}
				if sub == "" {
					sub = chooseText(g.Visible, "Janela aberta", "Em segundo plano")
				}
				text(dc, sub, Rect{labelX, ry + 22, labelW, 15}, a.fonts["small"], rgb(115, 128, 148), DT_LEFT|DT_SINGLELINE|DT_END_ELLIPSIS)
			}

			status := "Segundo plano"
			statusColor := rgb(117, 130, 150)
			if g.Focused {
				status, statusColor = "Em uso", rgb(39, 151, 91)
			} else if g.Visible {
				status = "Aberto"
			}
			if showStatus {
				text(dc, status, Rect{statusX, ry + 11, statusW, 18}, a.fonts["small"], statusColor, DT_LEFT|DT_SINGLELINE|DT_END_ELLIPSIS)
			}
			text(dc, fmt.Sprintf("%.1f%%", g.CPU), Rect{cpuX + 4, ry + 11, cpuW - 8, 18}, a.fonts["small"], rgb(54, 67, 88), DT_RIGHT|DT_SINGLELINE)
			text(dc, formatMemoryMB(g.MemoryMB), Rect{memX + 4, ry + 11, memW - 8, 18}, a.fonts["small"], rgb(54, 67, 88), DT_RIGHT|DT_SINGLELINE|DT_END_ELLIPSIS)
			if showTime {
				text(dc, formatActivitySeconds(g.ActiveSeconds), Rect{timeX + 4, ry + 11, timeW - 8, 18}, a.fonts["small"], rgb(86, 100, 121), DT_RIGHT|DT_SINGLELINE|DT_END_ELLIPSIS)
			}
			rowY += rowH

			if isExpanded && len(children) > 0 {
				rowY += expandedGap
				for ci, child := range children {
					cy := rowY
					childRow := Rect{innerX + 18, cy, innerW - 24, childH - 5}
					childBg := rgb(249, 250, 252)
					if ci%2 == 1 {
						childBg = rgb(252, 253, 254)
					}
					if child.BrowserTab && child.Focused {
						childBg = rgb(239, 246, 255)
					}
					roundedBox(dc, childRow, childBg, rgb(235, 239, 244), 5)
					line(dc, nameX+10, cy-6, nameX+10, cy+childH-4, rgb(214, 223, 234))
					line(dc, nameX+10, cy+childH/2-1, nameX+27, cy+childH/2-1, rgb(214, 223, 234))
					childIcon := Rect{nameX + 30, cy + 14, 18, 18}
					childLabelX := nameX + 56
					childLabelW := nameW - 64
					if childLabelW < 90 {
						childLabelW = 90
					}
					childLabel := ""
					childSub := ""
					if child.Summary {
						childLabel = fmt.Sprintf("Outros componentes do %s (%d)", friendlyProcessName(g.Name), child.ProcessCount)
						childSub = "Componentes internos usados pelo aplicativo"
					} else if child.BrowserTab {
						childLabel, childSub = friendlyBrowserTab(child.Title, child.Domain, child.URL)
					} else {
						childLabel, childSub = friendlyActivityWindow(child.Title, g.Name)
					}

					if child.BrowserTab {
						drawBrowserFavicon(dc, child.FavIconURL, child.Domain, childIcon)
					} else if child.ExePath != "" {
						_ = drawProcessIcon(dc, child.ExePath, childIcon)
					}

					// Abas do navegador têm mais respiro: título e contexto em linhas separadas.
					if child.BrowserTab {
						text(dc, childLabel, Rect{childLabelX, cy + 7, childLabelW, 18}, a.fonts["body"], rgb(55, 70, 92), DT_LEFT|DT_SINGLELINE|DT_END_ELLIPSIS)
						text(dc, childSub, Rect{childLabelX, cy + 27, childLabelW, 15}, a.fonts["small"], rgb(119, 132, 152), DT_LEFT|DT_SINGLELINE|DT_END_ELLIPSIS)
					} else if showSubTitle && childLabelW > 160 {
						text(dc, childLabel, Rect{childLabelX, cy + 7, childLabelW, 17}, a.fonts["small"], rgb(67, 82, 104), DT_LEFT|DT_SINGLELINE|DT_END_ELLIPSIS)
						text(dc, childSub, Rect{childLabelX, cy + 26, childLabelW, 14}, a.fonts["small"], rgb(126, 139, 158), DT_LEFT|DT_SINGLELINE|DT_END_ELLIPSIS)
					} else {
						text(dc, childLabel, Rect{childLabelX, cy + 14, childLabelW, 18}, a.fonts["small"], rgb(91, 105, 126), DT_LEFT|DT_SINGLELINE|DT_END_ELLIPSIS)
					}

					if showStatus {
						childStatus := "Aberto"
						childStatusColor := rgb(121, 134, 153)
						if child.Focused {
							childStatus = "Em uso"
							childStatusColor = rgb(39, 151, 91)
						} else if child.BrowserTab {
							childStatus = "Aba aberta"
						} else if child.Summary {
							childStatus = "Segundo plano"
						}
						text(dc, childStatus, Rect{statusX, cy + 15, statusW, 18}, a.fonts["small"], childStatusColor, DT_LEFT|DT_SINGLELINE|DT_END_ELLIPSIS)
					}

					// Uma aba não possui CPU/RAM/tempo próprios confiáveis. Não desenha traços vazios.
					if !child.BrowserTab {
						text(dc, fmt.Sprintf("%.1f%%", child.CPU), Rect{cpuX + 4, cy + 15, cpuW - 8, 18}, a.fonts["small"], rgb(84, 98, 119), DT_RIGHT|DT_SINGLELINE)
						text(dc, formatMemoryMB(child.MemoryMB), Rect{memX + 4, cy + 15, memW - 8, 18}, a.fonts["small"], rgb(84, 98, 119), DT_RIGHT|DT_SINGLELINE|DT_END_ELLIPSIS)
					}
					rowY += childH
				}
				rowY += expandedGap
			}
			rowY += groupGap
		}
	}

	bottomY := y + panelH + 12
	stackBottom := w < 760
	var recent, summary Rect
	if stackBottom {
		recent = Rect{x, bottomY, w, 172}
		summary = Rect{x, bottomY + 184, w, 172}
	} else {
		leftW := (w - 12) * 58 / 100
		recent = Rect{x, bottomY, leftW, 172}
		summary = Rect{x + leftW + 12, bottomY, w - leftW - 12, 172}
	}
	softCard(dc, recent)
	softCard(dc, summary)
	text(dc, "Trocas recentes", Rect{recent.X + 20, recent.Y + 14, recent.W - 40, 26}, a.fonts["h2"], rgb(31, 43, 63), DT_LEFT|DT_VCENTER|DT_SINGLELINE|DT_END_ELLIPSIS)
	text(dc, "Aplicativos e janelas usados mais recentemente", Rect{recent.X + 20, recent.Y + 42, recent.W - 40, 18}, a.fonts["small"], rgb(113, 126, 147), DT_LEFT|DT_SINGLELINE|DT_END_ELLIPSIS)
	recentItems := activityRecentSessions(sessions, current, 4)
	if len(recentItems) == 0 {
		text(dc, "A linha do tempo começa assim que você alterna entre aplicativos.", Rect{recent.X + 20, recent.Y + 90, recent.W - 40, 24}, a.fonts["small"], rgb(110, 123, 144), DT_LEFT|DT_WORDBREAK)
	} else {
		for i, item := range recentItems {
			ry := recent.Y + 72 + int32(i)*24
			circle(dc, Rect{recent.X + 22, ry + 6, 7, 7}, choose(i == 0 && current != nil && item.EndedAt.IsZero(), rgb(39, 179, 99), rgb(132, 150, 178)))
			nameWRecent := clamp32(recent.W*24/100, 90, 145)
			text(dc, friendlyProcessName(item.ProcessName), Rect{recent.X + 40, ry, nameWRecent, 20}, a.fonts["small"], rgb(45, 58, 79), DT_LEFT|DT_SINGLELINE|DT_END_ELLIPSIS)
			titleX := recent.X + 48 + nameWRecent
			timeW := int32(72)
			titleW := recent.X + recent.W - 24 - timeW - titleX - 8
			if titleW > 40 {
				text(dc, item.WindowTitle, Rect{titleX, ry, titleW, 20}, a.fonts["small"], rgb(105, 119, 140), DT_LEFT|DT_SINGLELINE|DT_END_ELLIPSIS)
			}
			text(dc, formatActivitySeconds(item.ActiveSeconds), Rect{recent.X + recent.W - 24 - timeW, ry, timeW, 20}, a.fonts["small"], rgb(105, 119, 140), DT_RIGHT|DT_SINGLELINE)
		}
	}

	text(dc, "Resumo técnico", Rect{summary.X + 20, summary.Y + 14, summary.W - 40, 26}, a.fonts["h2"], rgb(31, 43, 63), DT_LEFT|DT_VCENTER|DT_SINGLELINE|DT_END_ELLIPSIS)
	items := []string{
		"Processos/grupos listados: " + fmt.Sprint(len(groups)),
		"Janelas visíveis: " + fmt.Sprint(len(apps)),
		"Tempo em foco hoje: " + formatActivitySeconds(activitySecondsToday(sessions, current, now)),
		"Não registra teclas nem senhas.",
	}
	for i, item := range items {
		ry := summary.Y + 52 + int32(i)*26
		circle(dc, Rect{summary.X + 20, ry + 6, 7, 7}, rgb(42, 176, 91))
		text(dc, item, Rect{summary.X + 38, ry, summary.W - 58, 20}, a.fonts["small"], rgb(76, 91, 113), DT_LEFT|DT_SINGLELINE|DT_END_ELLIPSIS)
	}
}

type activityGroup struct {
	Name          string
	Count         int
	CPU           float64
	MemoryMB      float64
	Focused       bool
	Visible       bool
	ActiveSeconds int64
	Title         string
	ExePath       string
}

func buildActivityGroups(apps []ActivityApp, processes []ProcessInfo) []activityGroup {
	groups := map[string]*activityGroup{}
	ensure := func(name string) *activityGroup {
		key := strings.ToLower(strings.TrimSpace(name))
		if key == "" {
			key = "processo"
			name = "Processo"
		}
		if existing, ok := groups[key]; ok {
			return existing
		}
		item := &activityGroup{Name: name}
		groups[key] = item
		return item
	}
	for _, proc := range processes {
		g := ensure(proc.Name)
		g.Count++
		g.CPU += proc.CPU
		g.MemoryMB += proc.MemoryMB
		if g.ExePath == "" && proc.ExePath != "" {
			g.ExePath = proc.ExePath
		}
	}
	for _, app := range apps {
		g := ensure(app.Name)
		if g.Count == 0 {
			g.Count = 1
			g.MemoryMB += app.MemoryMB
			g.CPU += app.CPU
		}
		g.Visible = true
		if app.Focused {
			g.Focused = true
		}
		if app.ActiveSeconds > g.ActiveSeconds {
			g.ActiveSeconds = app.ActiveSeconds
		}
		if g.Title == "" || app.Focused {
			g.Title = app.WindowTitle
		}
	}
	out := make([]activityGroup, 0, len(groups))
	for _, item := range groups {
		out = append(out, *item)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Focused != out[j].Focused {
			return out[i].Focused
		}
		if out[i].Visible != out[j].Visible {
			return out[i].Visible
		}
		if out[i].MemoryMB != out[j].MemoryMB {
			return out[i].MemoryMB > out[j].MemoryMB
		}
		return strings.ToLower(out[i].Name) < strings.ToLower(out[j].Name)
	})
	return out
}

type activityChild struct {
	PID          int
	Title        string
	URL          string
	Domain       string
	CPU          float64
	MemoryMB     float64
	Visible      bool
	Focused      bool
	Summary      bool
	BrowserTab   bool
	FavIconURL   string
	ExePath      string
	ProcessCount int
}

func activityChildrenForGroup(groupName string, apps []ActivityApp, processes []ProcessInfo, browserTabs []BrowserTab) []activityChild {
	key := strings.ToLower(strings.TrimSpace(groupName))
	children := make([]activityChild, 0, 12)
	visiblePID := map[int]bool{}

	// Quando a extensão do navegador está conectada, as abas são a fonte mais útil.
	// O Windows continua sendo usado para CPU/memória do navegador como um todo.
	browserItems := browserTabsForProcess(groupName, browserTabs)
	if len(browserItems) > 0 {
		for _, tab := range browserItems {
			children = append(children, activityChild{
				Title: tab.Title, URL: tab.URL, Domain: tab.Domain, FavIconURL: tab.FavIconURL,
				Visible: true, Focused: tab.Active, BrowserTab: true,
			})
		}
		sort.SliceStable(children, func(i, j int) bool {
			if children[i].Summary != children[j].Summary {
				return !children[i].Summary
			}
			if children[i].Focused != children[j].Focused {
				return children[i].Focused
			}
			return strings.ToLower(children[i].Title) < strings.ToLower(children[j].Title)
		})
		return children
	}

	seenWindow := map[string]bool{}
	for _, app := range apps {
		if strings.ToLower(strings.TrimSpace(app.Name)) != key {
			continue
		}
		wkey := fmt.Sprintf("%d\x00%s", app.PID, app.WindowTitle)
		if seenWindow[wkey] {
			continue
		}
		seenWindow[wkey] = true
		visiblePID[app.PID] = true
		childPath := ""
		for _, proc := range processes {
			if proc.PID == app.PID {
				childPath = proc.ExePath
				break
			}
		}
		children = append(children, activityChild{
			PID: app.PID, Title: app.WindowTitle, CPU: app.CPU, MemoryMB: app.MemoryMB, ExePath: childPath,
			Visible: true, Focused: app.Focused, ProcessCount: 1,
		})
	}

	background := activityChild{Summary: true}
	for _, proc := range processes {
		if strings.ToLower(strings.TrimSpace(proc.Name)) != key || visiblePID[proc.PID] {
			continue
		}
		background.ProcessCount++
		background.CPU += proc.CPU
		background.MemoryMB += proc.MemoryMB
	}
	if background.ProcessCount > 0 {
		background.Title = fmt.Sprintf("Processos em segundo plano (%d)", background.ProcessCount)
		children = append(children, background)
	}

	sort.SliceStable(children, func(i, j int) bool {
		if children[i].Summary != children[j].Summary {
			return !children[i].Summary
		}
		if children[i].Focused != children[j].Focused {
			return children[i].Focused
		}
		if children[i].MemoryMB != children[j].MemoryMB {
			return children[i].MemoryMB > children[j].MemoryMB
		}
		return children[i].PID < children[j].PID
	})
	return children
}

func activityHeatColor(value, maxValue float64) uintptr {
	if maxValue <= 0 {
		maxValue = 1
	}
	ratio := value / maxValue
	if ratio < 0 {
		ratio = 0
	}
	if ratio > 1 {
		ratio = 1
	}
	if ratio < 0.01 {
		return rgb(28, 24, 38)
	}
	if ratio < 0.20 {
		return rgb(49, 29, 76)
	}
	if ratio < 0.45 {
		return rgb(74, 35, 113)
	}
	if ratio < 0.70 {
		return rgb(96, 42, 136)
	}
	return rgb(120, 50, 160)
}

func activityHeatColorLight(value, maxValue float64) uintptr {
	if maxValue <= 0 {
		maxValue = 1
	}
	ratio := value / maxValue
	if ratio < 0 {
		ratio = 0
	}
	if ratio > 1 {
		ratio = 1
	}
	if ratio < 0.01 {
		return rgb(249, 250, 252)
	}
	if ratio < 0.20 {
		return rgb(244, 248, 255)
	}
	if ratio < 0.45 {
		return rgb(235, 244, 255)
	}
	if ratio < 0.70 {
		return rgb(222, 238, 255)
	}
	return rgb(208, 229, 255)
}

func friendlyBrowserTab(title, domain, rawURL string) (string, string) {
	title = strings.TrimSpace(title)
	domain = strings.TrimSpace(strings.TrimPrefix(strings.ToLower(domain), "www."))
	lowerTitle := strings.ToLower(title)
	label := title
	sub := domain

	switch {
	case strings.Contains(domain, "youtube.com") || strings.Contains(lowerTitle, "youtube"):
		label = "YouTube"
		sub = strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(title), "- YouTube"))
		if sub == "" || strings.EqualFold(sub, "YouTube") {
			sub = domain
		}
	case strings.Contains(domain, "chatgpt.com") || strings.Contains(lowerTitle, "chatgpt"):
		label = "ChatGPT"
		sub = strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(title), "- ChatGPT"))
		if sub == "" || strings.EqualFold(sub, "ChatGPT") {
			sub = domain
		}
	case strings.Contains(domain, "mail.google.com") || strings.Contains(lowerTitle, "gmail"):
		label = "Gmail"
		sub = title
	case strings.Contains(lowerTitle, "valora crm"):
		label = "Valora CRM"
		if pos := strings.Index(title, "|"); pos > 0 {
			sub = strings.TrimSpace(title[:pos])
		} else if pos := strings.Index(title, " - "); pos > 0 {
			sub = strings.TrimSpace(title[:pos])
		} else {
			sub = title
		}
	default:
		if label == "" {
			label = domain
		}
		if sub == "" {
			sub = rawURL
		}
	}
	if strings.TrimSpace(label) == "" {
		label = "Página do navegador"
	}
	if strings.EqualFold(strings.TrimSpace(sub), strings.TrimSpace(label)) {
		sub = domain
	}
	return label, sub
}

func friendlyActivityWindow(title, processName string) (string, string) {
	clean := strings.TrimSpace(title)
	if clean == "" {
		return friendlyProcessName(processName), "Janela do aplicativo"
	}

	suffixes := []string{
		" - Opera", " — Opera",
		" - Google Chrome", " — Google Chrome",
		" - Microsoft Edge", " — Microsoft Edge",
		" - Visual Studio Code", " — Visual Studio Code",
		" - Explorador de Arquivos", " — Explorador de Arquivos",
		" - File Explorer", " — File Explorer",
	}
	for _, suffix := range suffixes {
		if len(clean) >= len(suffix) && strings.EqualFold(clean[len(clean)-len(suffix):], suffix) {
			clean = strings.TrimSpace(clean[:len(clean)-len(suffix)])
			break
		}
	}

	label := clean
	sub := friendlyProcessName(processName)
	lower := strings.ToLower(clean)
	switch {
	case strings.Contains(lower, "youtube"):
		label = "YouTube"
		trimmed := strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(clean), "- YouTube"))
		if trimmed != "" && !strings.EqualFold(trimmed, "YouTube") {
			sub = trimmed
		} else {
			sub = "YouTube"
		}
	case strings.Contains(lower, "valora crm"):
		label = "Valora CRM"
		if idx := strings.Index(clean, "|"); idx > 0 {
			sub = strings.TrimSpace(clean[:idx])
		} else {
			sub = clean
		}
	case strings.Contains(lower, "chatgpt"):
		label = "ChatGPT"
		sub = clean
	case strings.Contains(lower, "gmail"):
		label = "Gmail"
		sub = clean
	default:
		label = clean
	}

	if strings.TrimSpace(label) == "" {
		label = friendlyProcessName(processName)
	}
	if strings.EqualFold(strings.TrimSpace(sub), strings.TrimSpace(label)) {
		sub = friendlyProcessName(processName)
	}
	return label, sub
}

func friendlyProcessName(name string) string {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "coretuner-desktop", "corecontrol", "corecontrol.exe":
		return "CoreControl"
	case "opera", "opera.exe":
		return "Opera"
	case "chrome", "chrome.exe":
		return "Google Chrome"
	case "msedge", "msedge.exe":
		return "Microsoft Edge"
	case "explorer", "explorer.exe":
		return "Explorador do Windows"
	case "taskmgr", "taskmgr.exe":
		return "Gerenciador de Tarefas"
	case "code", "code.exe":
		return "Visual Studio Code"
	case "snippingtool", "snippingtool.exe":
		return "Ferramenta de Captura"
	case "systemsettings", "systemsettings.exe":
		return "Configurações do Windows"
	}
	return name
}

func activitySecondsToday(sessions []ActivitySession, current *ActivitySession, now time.Time) int64 {
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	var total int64
	for _, item := range sessions {
		if !item.StartedAt.Before(today) {
			total += item.ActiveSeconds
		}
	}
	if current != nil && !current.StartedAt.Before(today) {
		total += maxInt64(0, int64(now.Sub(current.StartedAt).Seconds()))
	}
	return total
}

func activityRecentSessions(sessions []ActivitySession, current *ActivitySession, limit int) []ActivitySession {
	result := make([]ActivitySession, 0, limit)
	if current != nil {
		copyCurrent := *current
		copyCurrent.ActiveSeconds = maxInt64(0, int64(time.Since(copyCurrent.StartedAt).Seconds()))
		result = append(result, copyCurrent)
	}
	for i := len(sessions) - 1; i >= 0 && len(result) < limit; i-- {
		result = append(result, sessions[i])
	}
	return result
}

func formatActivitySeconds(seconds int64) string {
	if seconds < 0 {
		seconds = 0
	}
	if seconds < 60 {
		return fmt.Sprintf("%ds", seconds)
	}
	minutes := seconds / 60
	if minutes < 60 {
		return fmt.Sprintf("%dm", minutes)
	}
	hours := minutes / 60
	minutes %= 60
	return fmt.Sprintf("%dh %02dm", hours, minutes)
}

func formatMemoryMB(value float64) string {
	if value >= 1024 {
		return fmt.Sprintf("%.1f GB", value/1024)
	}
	return fmt.Sprintf("%.0f MB", value)
}

func maxFloat(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}

func (a *App) drawReports(dc syscall.Handle) {
	x, y := contentOrigin()
	w := a.width - x - 28
	gap := int32(12)
	cw := (w - gap) / 2
	cards := []struct{ title, desc, meta, action string }{
		{"Relatório de diagnóstico", "Identificação, saúde, CPU, memória, disco, rede e recomendações do computador.", "HTML local • pronto para imprimir em PDF", "report"},
		{"Comparação antes e depois", "Compara as amostras registradas antes e depois da última otimização executada.", "Saúde • CPU • RAM • Disco • Latência", "comparison-report"},
	}
	for i, c := range cards {
		r := Rect{x + int32(i)*(cw+gap), y, cw, 224}
		softCard(dc, r)
		circle(dc, Rect{r.X + 20, r.Y + 18, 34, 34}, rgb(239, 245, 255))
		text(dc, chooseText(i == 0, "▤", "↔"), Rect{r.X + 20, r.Y + 18, 34, 34}, a.fonts["body"], rgb(47, 124, 246), DT_CENTER|DT_VCENTER|DT_SINGLELINE)
		text(dc, c.title, Rect{r.X + 66, r.Y + 18, r.W - 86, 28}, a.fonts["h2"], rgb(31, 43, 63), DT_LEFT|DT_VCENTER|DT_SINGLELINE|DT_END_ELLIPSIS)
		text(dc, c.desc, Rect{r.X + 20, r.Y + 70, r.W - 40, 54}, a.fonts["body"], rgb(94, 108, 130), DT_LEFT|DT_WORDBREAK)
		text(dc, c.meta, Rect{r.X + 20, r.Y + 132, r.W - 40, 24}, a.fonts["small"], rgb(118, 130, 149), DT_LEFT|DT_SINGLELINE|DT_END_ELLIPSIS)
		br := Rect{r.X + 20, r.Y + r.H - 54, r.W - 40, 38}
		button(dc, "Gerar e abrir", br, true)
		a.hits = append(a.hits, Hit{br, c.action, 0})
	}
	bottom := Rect{x, y + 238, w, 220}
	softCard(dc, bottom)
	text(dc, "Como os relatórios funcionam", Rect{bottom.X + 20, bottom.Y + 15, 340, 26}, a.fonts["h2"], rgb(31, 43, 63), DT_LEFT|DT_VCENTER|DT_SINGLELINE)
	steps := []struct{ n, title, desc string }{{"1", "Coleta local", "Usa somente os dados técnicos já exibidos pelo CoreControl."}, {"2", "Arquivo HTML", "O relatório é salvo na pasta local de dados do aplicativo."}, {"3", "Compartilhamento", "Abra no navegador e use a impressão do Windows para salvar em PDF."}}
	stepW := (bottom.W - 60) / 3
	for i, s := range steps {
		cx := bottom.X + 20 + int32(i)*(stepW+10)
		circle(dc, Rect{cx, bottom.Y + 62, 28, 28}, rgb(239, 245, 255))
		text(dc, s.n, Rect{cx, bottom.Y + 62, 28, 28}, a.fonts["small"], rgb(47, 124, 246), DT_CENTER|DT_VCENTER|DT_SINGLELINE)
		text(dc, s.title, Rect{cx + 40, bottom.Y + 60, stepW - 40, 24}, a.fonts["body"], rgb(42, 55, 76), DT_LEFT|DT_SINGLELINE|DT_END_ELLIPSIS)
		text(dc, s.desc, Rect{cx, bottom.Y + 104, stepW, 64}, a.fonts["small"], rgb(112, 125, 146), DT_LEFT|DT_WORDBREAK)
	}
	open := Rect{x, y + 474, 190, 36}
	button(dc, "Abrir pasta de relatórios", open, false)
	a.hits = append(a.hits, Hit{open, "open-reports-folder", 0})
}

func (a *App) drawHistory(dc syscall.Handle) {
	x, y := contentOrigin()
	w := a.width - x - 28
	a.mu.RLock()
	h := append([]HistoryItem(nil), a.history...)
	a.mu.RUnlock()
	sort.Slice(h, func(i, j int) bool { return h[i].At.After(h[j].At) })
	today := 0
	reports := 0
	opts := 0
	for _, v := range h {
		if time.Since(v.At) < 24*time.Hour {
			today++
		}
		low := strings.ToLower(v.Title)
		if strings.Contains(low, "relatório") {
			reports++
		}
		if strings.Contains(low, "otimiza") || strings.Contains(low, "perfil") {
			opts++
		}
	}
	gap := int32(12)
	sw := (w - gap*2) / 3
	drawMiniStat(dc, Rect{x, y, sw, 104}, "Eventos nas últimas 24h", fmt.Sprint(today), "Atividades registradas localmente", rgb(47, 124, 246))
	drawMiniStat(dc, Rect{x + sw + gap, y, sw, 104}, "Relatórios", fmt.Sprint(reports), "Gerações registradas", rgb(42, 176, 91))
	drawMiniStat(dc, Rect{x + (sw+gap)*2, y, sw, 104}, "Otimizações", fmt.Sprint(opts), "Ações e perfis registrados", rgb(139, 92, 246))
	panel := Rect{x, y + 118, w, 530}
	softCard(dc, panel)
	text(dc, "Atividade recente", Rect{panel.X + 20, panel.Y + 14, 300, 28}, a.fonts["h2"], rgb(31, 43, 63), DT_LEFT|DT_VCENTER|DT_SINGLELINE)
	text(dc, "Histórico local das ações realizadas no CoreControl", Rect{panel.X + 20, panel.Y + 43, 360, 20}, a.fonts["small"], rgb(113, 126, 147), DT_LEFT|DT_SINGLELINE)
	if len(h) == 0 {
		text(dc, "Nenhuma atividade registrada ainda.", Rect{panel.X + 20, panel.Y + 110, panel.W - 40, 32}, a.fonts["body"], rgb(109, 122, 143), DT_CENTER|DT_SINGLELINE)
		return
	}
	for i, v := range h[:min(11, len(h))] {
		ry := panel.Y + 80 + int32(i)*39
		circle(dc, Rect{panel.X + 22, ry + 7, 8, 8}, rgb(47, 124, 246))
		text(dc, v.Title, Rect{panel.X + 44, ry, 260, 24}, a.fonts["body"], rgb(42, 55, 76), DT_LEFT|DT_SINGLELINE|DT_END_ELLIPSIS)
		text(dc, v.Detail, Rect{panel.X + 320, ry, panel.W - 505, 24}, a.fonts["small"], rgb(108, 121, 142), DT_LEFT|DT_SINGLELINE|DT_END_ELLIPSIS)
		text(dc, v.At.Format("02/01 15:04"), Rect{panel.X + panel.W - 150, ry, 120, 24}, a.fonts["small"], rgb(119, 131, 150), DT_RIGHT|DT_SINGLELINE)
		if i < min(11, len(h))-1 {
			line(dc, panel.X+44, ry+31, panel.X+panel.W-22, ry+31, rgb(239, 241, 244))
		}
	}
}

func (a *App) drawSettings(dc syscall.Handle) {
	x, y := contentOrigin()
	w := a.width - x - 28
	gap := int32(12)
	a.mu.RLock()
	server := a.serverURL
	comp := companyName(a.company)
	centralOK := a.centralOK
	a.mu.RUnlock()
	serverCard := Rect{x, y, w, 170}
	softCard(dc, serverCard)
	text(dc, "Conexão com a Central", Rect{x + 20, y + 14, 310, 26}, a.fonts["h2"], rgb(31, 43, 63), DT_LEFT|DT_VCENTER|DT_SINGLELINE)
	circle(dc, Rect{x + w - 154, y + 22, 8, 8}, choose(centralOK, rgb(42, 176, 91), rgb(241, 153, 32)))
	text(dc, chooseText(centralOK, "Conectado", "Verificar conexão"), Rect{x + w - 138, y + 13, 118, 25}, a.fonts["small"], choose(centralOK, rgb(42, 176, 91), rgb(241, 153, 32)), DT_RIGHT|DT_VCENTER|DT_SINGLELINE)
	text(dc, "Endereço do servidor", Rect{x + 20, y + 50, 250, 20}, a.fonts["small"], rgb(113, 126, 147), DT_LEFT|DT_SINGLELINE)
	text(dc, "Fora do ambiente local, use sempre HTTPS.", Rect{x + 20, y + 126, w - 250, 22}, a.fonts["small"], rgb(118, 130, 149), DT_LEFT|DT_SINGLELINE)
	br := Rect{x + w - 190, y + 112, 170, 36}
	button(dc, "Salvar e testar", br, true)
	a.hits = append(a.hits, Hit{br, "save-server", 0})

	bottomY := y + 184
	cw := (w - gap) / 2
	left := Rect{x, bottomY, cw, 328}
	right := Rect{x + cw + gap, bottomY, cw, 328}
	softCard(dc, left)
	softCard(dc, right)
	text(dc, "Este computador", Rect{left.X + 20, left.Y + 15, left.W - 40, 26}, a.fonts["h2"], rgb(31, 43, 63), DT_LEFT|DT_VCENTER|DT_SINGLELINE)
	local := []struct{ k, v string }{{"Empresa vinculada", nz(comp, "Não vinculada")}, {"Pasta de dados", dataDir()}, {"Indicadores", "Atualização local a cada 2 segundos"}, {"Segurança", "Sem exclusões ou alterações críticas automáticas"}}
	for i, it := range local {
		ry := left.Y + 58 + int32(i)*55
		text(dc, it.k, Rect{left.X + 20, ry, 150, 20}, a.fonts["small"], rgb(116, 129, 148), DT_LEFT|DT_SINGLELINE)
		text(dc, it.v, Rect{left.X + 178, ry - 2, left.W - 198, 24}, a.fonts["body"], rgb(43, 56, 77), DT_RIGHT|DT_VCENTER|DT_SINGLELINE|DT_END_ELLIPSIS)
		if i < 3 {
			line(dc, left.X+20, ry+34, left.X+left.W-20, ry+34, rgb(239, 241, 244))
		}
	}
	openData := Rect{left.X + 20, left.Y + left.H - 52, left.W - 40, 36}
	button(dc, "Abrir pasta de dados", openData, false)
	a.hits = append(a.hits, Hit{openData, "open-data-folder", 0})

	text(dc, "Ações e manutenção", Rect{right.X + 20, right.Y + 15, right.W - 40, 26}, a.fonts["h2"], rgb(31, 43, 63), DT_LEFT|DT_VCENTER|DT_SINGLELINE)
	actions := []struct {
		title, desc, label, action string
		primary                    bool
	}{{"Painel web", "Abra a Central da sua empresa no navegador.", "Abrir painel", "open-web", true}, {"Atualizar informações", "Atualiza dados locais e testa a conexão com o servidor.", "Atualizar tudo", "refresh-all", false}, {"Relatórios", "Acesse a pasta onde os relatórios locais são armazenados.", "Abrir relatórios", "open-reports-folder", false}}
	for i, it := range actions {
		ry := right.Y + 58 + int32(i)*78
		text(dc, it.title, Rect{right.X + 20, ry, right.W - 200, 22}, a.fonts["body"], rgb(43, 56, 77), DT_LEFT|DT_SINGLELINE)
		text(dc, it.desc, Rect{right.X + 20, ry + 25, right.W - 200, 36}, a.fonts["small"], rgb(112, 125, 146), DT_LEFT|DT_WORDBREAK)
		b := Rect{right.X + right.W - 158, ry + 8, 138, 34}
		button(dc, it.label, b, it.primary)
		a.hits = append(a.hits, Hit{b, it.action, 0})
	}
	_ = server
}

func (a *App) drawSupport(dc syscall.Handle) {
	x, y := contentOrigin()
	w := a.width - x - 28
	gap := int32(12)
	cw := (w - gap*2) / 3
	actions := []struct {
		title, desc, button, action string
		primary                     bool
	}{{"Pacote técnico", "Gera um ZIP com diagnóstico, versão, conexão e histórico recente. Não inclui arquivos pessoais.", "Gerar pacote", "support-package", true}, {"Relatório do computador", "Cria um relatório técnico em HTML que pode ser impresso ou enviado ao suporte.", "Gerar relatório", "report", false}, {"Atualizar diagnóstico", "Coleta novamente os dados locais e verifica a comunicação com a Central.", "Atualizar agora", "refresh-all", false}}
	for i, it := range actions {
		r := Rect{x + int32(i)*(cw+gap), y, cw, 210}
		softCard(dc, r)
		circle(dc, Rect{r.X + 20, r.Y + 18, 34, 34}, rgb(239, 245, 255))
		text(dc, []string{"↥", "▤", "↻"}[i], Rect{r.X + 20, r.Y + 18, 34, 34}, a.fonts["body"], rgb(47, 124, 246), DT_CENTER|DT_VCENTER|DT_SINGLELINE)
		text(dc, it.title, Rect{r.X + 66, r.Y + 18, r.W - 86, 28}, a.fonts["h2"], rgb(31, 43, 63), DT_LEFT|DT_VCENTER|DT_SINGLELINE|DT_END_ELLIPSIS)
		text(dc, it.desc, Rect{r.X + 20, r.Y + 68, r.W - 40, 76}, a.fonts["small"], rgb(105, 119, 140), DT_LEFT|DT_WORDBREAK)
		b := Rect{r.X + 20, r.Y + r.H - 54, r.W - 40, 38}
		button(dc, it.button, b, it.primary)
		a.hits = append(a.hits, Hit{b, it.action, 0})
	}
	privacy := Rect{x, y + 224, w, 190}
	softCard(dc, privacy)
	text(dc, "Privacidade do suporte", Rect{privacy.X + 20, privacy.Y + 15, 320, 26}, a.fonts["h2"], rgb(31, 43, 63), DT_LEFT|DT_VCENTER|DT_SINGLELINE)
	text(dc, "O CoreControl envia apenas o que você decidir compartilhar. O pacote técnico usa dados de hardware, sistema, conectividade e o histórico gerado pelo próprio aplicativo.", Rect{privacy.X + 20, privacy.Y + 52, privacy.W - 40, 44}, a.fonts["body"], rgb(84, 99, 121), DT_LEFT|DT_WORDBREAK)
	checks := []string{"Sem documentos pessoais", "Sem conversas ou áudio gravado", "Sem senhas ou token da sessão", "Arquivo gerado localmente"}
	for i, item := range checks {
		cx := privacy.X + 20 + int32(i%2)*(privacy.W/2)
		cy := privacy.Y + 116 + int32(i/2)*34
		circle(dc, Rect{cx, cy + 5, 9, 9}, rgb(42, 176, 91))
		text(dc, item, Rect{cx + 20, cy, privacy.W/2 - 36, 24}, a.fonts["small"], rgb(72, 89, 112), DT_LEFT|DT_SINGLELINE|DT_END_ELLIPSIS)
	}
	footer := Rect{x, y + 428, w, 88}
	softCard(dc, footer)
	text(dc, "Arquivos de suporte", Rect{footer.X + 20, footer.Y + 16, 250, 24}, a.fonts["body"], rgb(42, 55, 76), DT_LEFT|DT_SINGLELINE)
	text(dc, "Os pacotes ficam na pasta de dados local do CoreControl.", Rect{footer.X + 20, footer.Y + 43, 420, 20}, a.fonts["small"], rgb(112, 125, 146), DT_LEFT|DT_SINGLELINE)
	open := Rect{footer.X + footer.W - 214, footer.Y + 25, 194, 36}
	button(dc, "Abrir pasta de suporte", open, false)
	a.hits = append(a.hits, Hit{open, "open-support-folder", 0})
	text(dc, "Versão "+appVersion, Rect{footer.X + footer.W - 330, footer.Y + 33, 100, 20}, a.fonts["small"], rgb(119, 131, 150), DT_RIGHT|DT_SINGLELINE)
}
