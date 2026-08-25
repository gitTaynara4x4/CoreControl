//go:build windows

package main

import (
	"strconv"
	"strings"
	"syscall"
	"unsafe"
)

const (
	shellSidebarWidth int32 = 226
	shellHeaderHeight int32 = 88
	shellContentLeft  int32 = 248
	shellContentTop   int32 = 104
)

func (a *App) invalidate() { procInvalidateRect.Call(uintptr(a.hwnd), 0, 0) }
func (a *App) paint() {
	var ps PAINTSTRUCT
	dc, _, _ := procBeginPaint.Call(uintptr(a.hwnd), uintptr(unsafe.Pointer(&ps)))
	var rc RECT
	procGetClientRect.Call(uintptr(a.hwnd), uintptr(unsafe.Pointer(&rc)))
	mem, _, _ := procCreateCompatibleDC.Call(dc)
	bmp, _, _ := procCreateCompatibleBitmap.Call(dc, uintptr(rc.Right), uintptr(rc.Bottom))
	old, _, _ := procSelectObject.Call(mem, bmp)
	a.draw(syscall.Handle(mem), rc)
	procBitBlt.Call(dc, 0, 0, uintptr(rc.Right), uintptr(rc.Bottom), mem, 0, 0, SRCCOPY)
	procSelectObject.Call(mem, old)
	procDeleteObject.Call(bmp)
	procDeleteDC.Call(mem)
	procEndPaint.Call(uintptr(a.hwnd), uintptr(unsafe.Pointer(&ps)))
}
func (a *App) draw(dc syscall.Handle, rc RECT) {
	a.hits = nil
	bg := rgb(248, 250, 253)
	fill(dc, Rect{0, 0, rc.Right, rc.Bottom}, bg)
	if a.token == "" {
		a.drawAuth(dc, rc)
		return
	}
	a.drawShell(dc, rc)
	// Sidebar e cabeçalho ficam fixos enquanto páginas longas rolam por baixo.
	saved, _, _ := procSaveDC.Call(uintptr(dc))
	procIntersectClipRect.Call(uintptr(dc), uintptr(shellSidebarWidth), uintptr(shellHeaderHeight), uintptr(rc.Right), uintptr(rc.Bottom))
	switch a.page {
	case 0:
		a.drawDashboard(dc)
	case 1:
		a.drawDiagnostics(dc)
	case 2:
		a.drawTests(dc)
	case 3:
		a.drawOptimizations(dc)
	case 4:
		a.drawPrograms(dc)
	case 5:
		a.drawReports(dc)
	case 6:
		a.drawHistory(dc)
	case 7:
		a.drawSettings(dc)
	case 8:
		a.drawSupport(dc)
	}
	procRestoreDC.Call(uintptr(dc), saved)
	a.drawPageScrollbar(dc)
}

func (a *App) drawPageScrollbar(dc syscall.Handle) {
	track, thumb, ok := a.pageScrollbarGeometry()
	if !ok {
		return
	}
	// Área visual discreta, mas a área clicável é maior (calculada em pageScrollbarGeometry).
	trackVisual := Rect{track.X + track.W/2 - 2, track.Y, 4, track.H}
	thumbVisual := Rect{track.X + track.W/2 - 4, thumb.Y, 8, thumb.H}
	trackColor := rgb(232, 237, 244)
	thumbColor := rgb(166, 180, 199)
	if a.scrollDragging || thumb.contains(a.mouseX, a.mouseY) {
		thumbColor = rgb(121, 142, 171)
	}
	roundedBox(dc, trackVisual, trackColor, trackColor, 4)
	roundedBox(dc, thumbVisual, thumbColor, thumbColor, 8)
}
func fill(dc syscall.Handle, r Rect, c uintptr) {
	b, _, _ := procCreateSolidBrush.Call(c)
	rr := RECT{r.X, r.Y, r.X + r.W, r.Y + r.H}
	procFillRect.Call(uintptr(dc), uintptr(unsafe.Pointer(&rr)), b)
	procDeleteObject.Call(b)
}
func card(dc syscall.Handle, r Rect) {
	roundedBox(dc, r, rgb(255, 255, 255), rgb(224, 231, 239), 14)
}
func roundedBox(dc syscall.Handle, r Rect, background, border uintptr, radius int32) {
	brush, _, _ := procCreateSolidBrush.Call(background)
	pen, _, _ := procCreatePen.Call(PS_SOLID, 1, border)
	ob, _, _ := procSelectObject.Call(uintptr(dc), brush)
	op, _, _ := procSelectObject.Call(uintptr(dc), pen)
	procRoundRect.Call(uintptr(dc), uintptr(r.X), uintptr(r.Y), uintptr(r.X+r.W), uintptr(r.Y+r.H), uintptr(radius), uintptr(radius))
	procSelectObject.Call(uintptr(dc), ob)
	procSelectObject.Call(uintptr(dc), op)
	procDeleteObject.Call(brush)
	procDeleteObject.Call(pen)
}
func text(dc syscall.Handle, s string, r Rect, font uintptr, color uintptr, flags uintptr) {
	procSelectObject.Call(uintptr(dc), font)
	procSetBkMode.Call(uintptr(dc), TRANSPARENT)
	procSetTextColor.Call(uintptr(dc), color)
	rr := RECT{r.X, r.Y, r.X + r.W, r.Y + r.H}
	procDrawText.Call(uintptr(dc), uintptr(unsafe.Pointer(utf16(s))), uintptr(len([]rune(s))), uintptr(unsafe.Pointer(&rr)), flags)
}
func line(dc syscall.Handle, x1, y1, x2, y2 int32, c uintptr) {
	p, _, _ := procCreatePen.Call(PS_SOLID, 1, c)
	o, _, _ := procSelectObject.Call(uintptr(dc), p)
	procMoveToEx.Call(uintptr(dc), uintptr(x1), uintptr(y1), 0)
	procLineTo.Call(uintptr(dc), uintptr(x2), uintptr(y2))
	procSelectObject.Call(uintptr(dc), o)
	procDeleteObject.Call(p)
}
func circle(dc syscall.Handle, r Rect, c uintptr) {
	b, _, _ := procCreateSolidBrush.Call(c)
	p, _, _ := procCreatePen.Call(PS_SOLID, 1, c)
	ob, _, _ := procSelectObject.Call(uintptr(dc), b)
	op, _, _ := procSelectObject.Call(uintptr(dc), p)
	procEllipse.Call(uintptr(dc), uintptr(r.X), uintptr(r.Y), uintptr(r.X+r.W), uintptr(r.Y+r.H))
	procSelectObject.Call(uintptr(dc), ob)
	procSelectObject.Call(uintptr(dc), op)
	procDeleteObject.Call(b)
	procDeleteObject.Call(p)
}
func progress(dc syscall.Handle, r Rect, pct float64, color uintptr) {
	fill(dc, r, rgb(232, 237, 244))
	if pct < 0 {
		pct = 0
	}
	if pct > 100 {
		pct = 100
	}
	fill(dc, Rect{r.X, r.Y, int32(float64(r.W) * pct / 100), r.H}, color)
}
func button(dc syscall.Handle, label string, r Rect, primary bool) {
	c := rgb(255, 255, 255)
	tc := rgb(67, 82, 105)
	border := rgb(220, 225, 232)
	hovered := app != nil && app.isHovered(r)
	if primary {
		c = rgb(47, 124, 246)
		tc = rgb(255, 255, 255)
		border = rgb(47, 124, 246)
		if hovered {
			c = rgb(38, 105, 219)
			border = c
		}
	} else if hovered {
		c = rgb(247, 250, 255)
		border = rgb(171, 197, 239)
		tc = rgb(47, 124, 246)
	}
	b, _, _ := procCreateSolidBrush.Call(c)
	p, _, _ := procCreatePen.Call(PS_SOLID, 1, border)
	ob, _, _ := procSelectObject.Call(uintptr(dc), b)
	op, _, _ := procSelectObject.Call(uintptr(dc), p)
	procRoundRect.Call(uintptr(dc), uintptr(r.X), uintptr(r.Y), uintptr(r.X+r.W), uintptr(r.Y+r.H), 10, 10)
	procSelectObject.Call(uintptr(dc), ob)
	procSelectObject.Call(uintptr(dc), op)
	procDeleteObject.Call(b)
	procDeleteObject.Call(p)
	text(dc, label, r, app.fonts["body"], tc, DT_CENTER|DT_VCENTER|DT_SINGLELINE)
}

func max32(a, b int32) int32 {
	if a > b {
		return a
	}
	return b
}

func logoMark(dc syscall.Handle, r Rect) {
	// Símbolo CoreControl: anel segmentado com núcleo e haste central.
	// O favicon e os executáveis usam o asset oficial; esta versão vetorial
	// mantém a mesma leitura visual no desenho nativo da sidebar.
	blue := rgb(19, 102, 246)
	bg := rgb(255, 255, 255)
	circle(dc, r, blue)
	ring := max32(4, r.W/5)
	inner := Rect{r.X + ring, r.Y + ring, r.W - ring*2, r.H - ring*2}
	circle(dc, inner, bg)
	gap := max32(3, r.W/10)
	fill(dc, Rect{r.X + r.W/2 - gap/2, r.Y - 1, gap, ring + 3}, bg)
	fill(dc, Rect{r.X + r.W/2 - gap/2, r.Y + r.H - ring - 2, gap, ring + 4}, bg)
	fill(dc, Rect{r.X - 1, r.Y + r.H/2 - gap/2, ring + 3, gap}, bg)
	fill(dc, Rect{r.X + r.W - ring - 2, r.Y + r.H/2 - gap/2, ring + 4, gap}, bg)
	knob := max32(12, r.W*9/20)
	kx := r.X + (r.W-knob)/2
	ky := r.Y + (r.H-knob)/2 + 2
	circle(dc, Rect{kx, ky, knob, knob}, blue)
	stemW := max32(4, r.W/7)
	stemX := r.X + (r.W-stemW)/2
	fill(dc, Rect{stemX, r.Y + ring/2, stemW, ky - r.Y - ring/2 + 3}, blue)
}

func monitorIcon(dc syscall.Handle, r Rect, color uintptr) {
	pen, _, _ := procCreatePen.Call(PS_SOLID, 2, color)
	op, _, _ := procSelectObject.Call(uintptr(dc), pen)
	procRoundRect.Call(uintptr(dc), uintptr(r.X), uintptr(r.Y), uintptr(r.X+r.W), uintptr(r.Y+r.H-5), 5, 5)
	procMoveToEx.Call(uintptr(dc), uintptr(r.X+r.W/2), uintptr(r.Y+r.H-5), 0)
	procLineTo.Call(uintptr(dc), uintptr(r.X+r.W/2), uintptr(r.Y+r.H))
	procMoveToEx.Call(uintptr(dc), uintptr(r.X+r.W/3), uintptr(r.Y+r.H), 0)
	procLineTo.Call(uintptr(dc), uintptr(r.X+r.W*2/3), uintptr(r.Y+r.H))
	procSelectObject.Call(uintptr(dc), op)
	procDeleteObject.Call(pen)
}

func statusPill(dc syscall.Handle, label string, r Rect, background, foreground uintptr) {
	roundedBox(dc, r, background, background, 10)
	text(dc, label, r, app.fonts["small"], foreground, DT_CENTER|DT_VCENTER|DT_SINGLELINE)
}

func (a *App) drawAuth(dc syscall.Handle, rc RECT) {
	fill(dc, Rect{0, 0, rc.Right, rc.Bottom}, rgb(234, 248, 250))

	// Fundo suave e leve, inspirado na referência, sem interferir no formulário.
	circle(dc, Rect{-140, 105, 520, 520}, rgb(242, 246, 250))
	circle(dc, Rect{-75, rc.Bottom - 300, 260, 260}, rgb(205, 203, 248))
	circle(dc, Rect{rc.Right - 255, -95, 300, 300}, rgb(204, 240, 245))

	logoMark(dc, Rect{54, 35, 28, 28})
	text(dc, "CoreControl", Rect{96, 28, 250, 40}, a.fonts["brand"], rgb(10, 31, 62), DT_LEFT|DT_VCENTER|DT_SINGLELINE)

	layout := makeAuthLayout(rc.Right, rc.Bottom, a.loginMode)
	shadow := Rect{layout.card.X, layout.card.Y + 8, layout.card.W, layout.card.H}
	roundedBox(dc, shadow, rgb(202, 219, 226), rgb(202, 219, 226), 18)
	roundedBox(dc, layout.card, rgb(255, 255, 255), rgb(226, 234, 240), 18)

	logoMark(dc, layout.logo)
	title := "Entrar na sua empresa"
	sub := "Use a mesma conta do site para acessar seus computadores."
	labels := []string{"E-mail", "Senha"}
	if a.loginMode == "register" {
		title = "Criar uma empresa"
		sub = "Cadastre sua empresa e comece a usar o CoreControl."
		labels = []string{"Nome da empresa", "Responsável", "E-mail", "Senha", "Confirmar senha"}
	}
	text(dc, title, layout.title, a.fonts["h1"], rgb(11, 31, 60), DT_CENTER|DT_VCENTER|DT_SINGLELINE)
	text(dc, sub, layout.subtitle, a.fonts["small"], rgb(93, 112, 140), DT_CENTER|DT_WORDBREAK)
	for i, label := range labels {
		text(dc, label, layout.labels[i], a.fonts["small"], rgb(55, 73, 102), DT_LEFT|DT_VCENTER|DT_SINGLELINE)
	}

	a.mu.RLock()
	st, busy := a.statusText, a.busy
	a.mu.RUnlock()
	if busy {
		text(dc, st, layout.status, a.fonts["small"], rgb(38, 113, 208), DT_CENTER|DT_VCENTER|DT_SINGLELINE)
	}
	text(dc, "Privacidade protegida: o CoreControl não acessa documentos, conversas ou senhas.", layout.security, a.fonts["small"], rgb(37, 153, 87), DT_CENTER|DT_VCENTER|DT_WORDBREAK)
}

var menuLabels = []string{"Painel inicial", "Diagnóstico", "Testes", "Otimizações", "Programas", "Relatórios", "Histórico", "Configurações", "Suporte"}

func pageSubtitle(page int) string {
	switch page {
	case 0:
		return "Visão geral do seu sistema"
	case 1:
		return "Hardware, sistema e conectividade"
	case 2:
		return "Validações rápidas deste computador"
	case 3:
		return "Perfis seguros de desempenho"
	case 4:
		return "Processos e consumo em tempo real"
	case 5:
		return "Relatórios técnicos e comparativos"
	case 6:
		return "Atividades realizadas pelo CoreControl"
	case 7:
		return "Preferências e conexão com a Central"
	case 8:
		return "Diagnóstico e suporte técnico"
	default:
		return ""
	}
}

func userInitials(name string) string {
	parts := strings.Fields(strings.TrimSpace(name))
	if len(parts) == 0 {
		return "CC"
	}
	if len(parts) == 1 {
		r := []rune(parts[0])
		if len(r) >= 2 {
			return strings.ToUpper(string(r[:2]))
		}
		return strings.ToUpper(string(r[:1]))
	}
	return strings.ToUpper(string([]rune(parts[0])[0]) + string([]rune(parts[len(parts)-1])[0]))
}

func drawSidebarIcon(dc syscall.Handle, index int, r Rect, color uintptr) {
	cx := r.X + r.W/2
	cy := r.Y + r.H/2
	switch index {
	case 0: // casa
		line(dc, r.X+3, cy, cx, r.Y+4, color)
		line(dc, cx, r.Y+4, r.X+r.W-3, cy, color)
		line(dc, r.X+5, cy-1, r.X+5, r.Y+r.H-4, color)
		line(dc, r.X+r.W-5, cy-1, r.X+r.W-5, r.Y+r.H-4, color)
		line(dc, r.X+5, r.Y+r.H-4, r.X+r.W-5, r.Y+r.H-4, color)
	case 1: // pulso
		line(dc, r.X+2, cy, r.X+6, cy, color)
		line(dc, r.X+6, cy, r.X+9, cy-6, color)
		line(dc, r.X+9, cy-6, r.X+13, cy+7, color)
		line(dc, r.X+13, cy+7, r.X+17, cy-3, color)
		line(dc, r.X+17, cy-3, r.X+r.W-2, cy-3, color)
	case 2: // checklist
		roundedBox(dc, Rect{r.X + 4, r.Y + 3, r.W - 8, r.H - 6}, rgb(255, 255, 255), color, 4)
		line(dc, r.X+8, cy, r.X+11, cy+3, color)
		line(dc, r.X+11, cy+3, r.X+16, cy-4, color)
	case 3: // raio
		line(dc, cx+2, r.Y+2, r.X+7, cy+2, color)
		line(dc, r.X+7, cy+2, cx-1, cy+2, color)
		line(dc, cx-1, cy+2, cx-3, r.Y+r.H-2, color)
		line(dc, cx-3, r.Y+r.H-2, r.X+r.W-5, cy-2, color)
		line(dc, r.X+r.W-5, cy-2, cx+2, cy-2, color)
	case 4: // grade
		for yy := int32(0); yy < 2; yy++ {
			for xx := int32(0); xx < 2; xx++ {
				roundedBox(dc, Rect{r.X + 3 + xx*9, r.Y + 3 + yy*9, 6, 6}, rgb(255, 255, 255), color, 2)
			}
		}
	case 5: // documento
		roundedBox(dc, Rect{r.X + 5, r.Y + 2, r.W - 10, r.H - 4}, rgb(255, 255, 255), color, 3)
		line(dc, r.X+8, r.Y+8, r.X+r.W-8, r.Y+8, color)
		line(dc, r.X+8, r.Y+12, r.X+r.W-8, r.Y+12, color)
		line(dc, r.X+8, r.Y+16, r.X+r.W-10, r.Y+16, color)
	case 6: // histórico
		circle(dc, Rect{r.X + 3, r.Y + 3, r.W - 6, r.H - 6}, color)
		circle(dc, Rect{r.X + 5, r.Y + 5, r.W - 10, r.H - 10}, rgb(255, 255, 255))
		line(dc, cx, cy, cx, r.Y+7, color)
		line(dc, cx, cy, r.X+7, cy, color)
	case 7: // configurações simplificadas
		circle(dc, Rect{cx - 5, cy - 5, 10, 10}, color)
		circle(dc, Rect{cx - 2, cy - 2, 4, 4}, rgb(255, 255, 255))
		line(dc, cx, r.Y+2, cx, r.Y+6, color)
		line(dc, cx, r.Y+r.H-6, cx, r.Y+r.H-2, color)
		line(dc, r.X+2, cy, r.X+6, cy, color)
		line(dc, r.X+r.W-6, cy, r.X+r.W-2, cy, color)
	case 8:
		circle(dc, Rect{r.X + 3, r.Y + 3, r.W - 6, r.H - 6}, color)
		circle(dc, Rect{r.X + 5, r.Y + 5, r.W - 10, r.H - 10}, rgb(255, 255, 255))
		text(dc, "?", r, app.fonts["small"], color, DT_CENTER|DT_VCENTER|DT_SINGLELINE)
	}
}

func (a *App) drawShell(dc syscall.Handle, rc RECT) {
	side := shellSidebarWidth
	fill(dc, Rect{0, 0, side, rc.Bottom}, rgb(255, 255, 255))
	line(dc, side, 0, side, rc.Bottom, rgb(232, 235, 240))

	logoMark(dc, Rect{20, 24, 34, 34})
	text(dc, "CoreControl", Rect{64, 19, side - 68, 44}, a.fonts["brand"], rgb(17, 31, 55), DT_LEFT|DT_VCENTER|DT_SINGLELINE)

	y := int32(94)
	for i, label := range menuLabels {
		r := Rect{14, y + int32(i*44), side - 28, 38}
		selected := a.page == i
		if selected {
			roundedBox(dc, r, rgb(237, 245, 255), rgb(237, 245, 255), 9)
		} else if a.isHovered(r) {
			roundedBox(dc, r, rgb(247, 249, 252), rgb(247, 249, 252), 9)
		}
		iconColor := choose(selected, rgb(47, 124, 246), rgb(111, 128, 155))
		drawSidebarIcon(dc, i, Rect{r.X + 10, r.Y + 9, 20, 20}, iconColor)
		text(dc, label, Rect{r.X + 42, r.Y, r.W - 50, r.H}, a.fonts["body"], choose(selected, rgb(47, 124, 246), rgb(47, 62, 87)), DT_LEFT|DT_VCENTER|DT_SINGLELINE|DT_END_ELLIPSIS)
		a.hits = append(a.hits, Hit{r, "page", i})
	}

	a.mu.RLock()
	company := companyName(a.company)
	user := a.user.Name
	sys := a.sys
	a.mu.RUnlock()

	if rc.Bottom >= 720 {
		deviceY := rc.Bottom - 216
		device := Rect{14, deviceY, side - 28, 108}
		roundedBox(dc, device, rgb(250, 251, 253), rgb(232, 235, 240), 10)
		roundedBox(dc, Rect{device.X + 12, device.Y + 14, 38, 32}, rgb(240, 246, 255), rgb(226, 233, 243), 7)
		monitorIcon(dc, Rect{device.X + 20, device.Y + 20, 22, 20}, rgb(47, 124, 246))
		text(dc, nz(sys.Hostname, "Este computador"), Rect{device.X + 58, device.Y + 12, device.W - 68, 22}, a.fonts["small"], rgb(35, 48, 69), DT_LEFT|DT_VCENTER|DT_SINGLELINE|DT_END_ELLIPSIS)
		text(dc, nz(sys.OS, "Windows"), Rect{device.X + 58, device.Y + 34, device.W - 68, 18}, a.fonts["small"], rgb(111, 124, 145), DT_LEFT|DT_SINGLELINE|DT_END_ELLIPSIS)
		text(dc, nz(sys.CPUName, "Processador"), Rect{device.X + 12, device.Y + 61, device.W - 24, 18}, a.fonts["small"], rgb(86, 101, 124), DT_LEFT|DT_SINGLELINE|DT_END_ELLIPSIS)
		text(dc, strings.TrimSpace(nz(formatRAMDisk(sys), "Informações do sistema")), Rect{device.X + 12, device.Y + 82, device.W - 24, 18}, a.fonts["small"], rgb(86, 101, 124), DT_LEFT|DT_SINGLELINE|DT_END_ELLIPSIS)

		profile := Rect{14, rc.Bottom - 98, side - 28, 58}
		roundedBox(dc, profile, rgb(255, 255, 255), rgb(232, 235, 240), 10)
		circle(dc, Rect{profile.X + 12, profile.Y + 11, 36, 36}, rgb(47, 124, 246))
		text(dc, userInitials(user), Rect{profile.X + 12, profile.Y + 11, 36, 36}, a.fonts["small"], rgb(255, 255, 255), DT_CENTER|DT_VCENTER|DT_SINGLELINE)
		text(dc, nz(company, "CoreControl"), Rect{profile.X + 57, profile.Y + 9, profile.W - 68, 21}, a.fonts["small"], rgb(31, 43, 63), DT_LEFT|DT_VCENTER|DT_SINGLELINE|DT_END_ELLIPSIS)
		text(dc, nz(user, "Usuário"), Rect{profile.X + 57, profile.Y + 29, profile.W - 68, 18}, a.fonts["small"], rgb(111, 124, 145), DT_LEFT|DT_SINGLELINE|DT_END_ELLIPSIS)
	}

	logoutRect := Rect{20, rc.Bottom - 32, 86, 22}
	if a.isHovered(logoutRect) {
		roundedBox(dc, Rect{logoutRect.X - 6, logoutRect.Y - 3, logoutRect.W + 12, logoutRect.H + 6}, rgb(255, 246, 246), rgb(255, 246, 246), 7)
	}
	text(dc, "↪  Sair", logoutRect, a.fonts["small"], rgb(194, 70, 70), DT_LEFT|DT_VCENTER|DT_SINGLELINE)
	a.hits = append(a.hits, Hit{logoutRect, "logout", 0})

	fill(dc, Rect{side, 0, rc.Right - side, shellHeaderHeight}, rgb(255, 255, 255))
	line(dc, side, shellHeaderHeight, rc.Right, shellHeaderHeight, rgb(232, 235, 240))
	text(dc, menuLabels[a.page], Rect{side + 32, 14, 350, 34}, a.fonts["title"], rgb(23, 33, 50), DT_LEFT|DT_VCENTER|DT_SINGLELINE)
	text(dc, pageSubtitle(a.page), Rect{side + 32, 48, 360, 22}, a.fonts["small"], rgb(112, 124, 145), DT_LEFT|DT_SINGLELINE|DT_END_ELLIPSIS)

	statusX := side + 392
	circle(dc, Rect{statusX, 31, 7, 7}, rgb(39, 179, 99))
	text(dc, "Online", Rect{statusX + 14, 22, 60, 24}, a.fonts["small"], rgb(39, 179, 99), DT_LEFT|DT_VCENTER|DT_SINGLELINE)
	text(dc, "↻  Atualizado agora", Rect{statusX + 78, 22, 130, 24}, a.fonts["small"], rgb(113, 126, 148), DT_LEFT|DT_VCENTER|DT_SINGLELINE)

	profileW := int32(205)
	profileX := rc.Right - profileW - 24
	if profileX > statusX+220 {
		circle(dc, Rect{profileX, 23, 40, 40}, rgb(238, 244, 255))
		text(dc, userInitials(user), Rect{profileX, 23, 40, 40}, a.fonts["small"], rgb(47, 124, 246), DT_CENTER|DT_VCENTER|DT_SINGLELINE)
		text(dc, nz(user, "Usuário"), Rect{profileX + 50, 19, profileW - 50, 22}, a.fonts["body"], rgb(31, 43, 63), DT_LEFT|DT_VCENTER|DT_SINGLELINE|DT_END_ELLIPSIS)
		text(dc, nz(company, "Conta local"), Rect{profileX + 50, 42, profileW - 50, 18}, a.fonts["small"], rgb(114, 126, 147), DT_LEFT|DT_SINGLELINE|DT_END_ELLIPSIS)
	}
}

func formatRAMDisk(s SystemInfo) string {
	ram := ""
	disk := ""
	if s.TotalRAMGB > 0 {
		ram = strings.TrimSpace(formatOneDecimal(s.TotalRAMGB)) + " GB RAM"
	}
	if s.DiskTotalGB > 0 {
		disk = strings.TrimSpace(formatNoDecimal(s.DiskTotalGB)) + " GB " + nz(s.DiskType, "disco")
	}
	if ram != "" && disk != "" {
		return ram + " • " + disk
	}
	return ram + disk
}

func formatOneDecimal(v float64) string {
	s := strings.TrimRight(strings.TrimRight(strconv.FormatFloat(v, 'f', 1, 64), "0"), ".")
	return s
}

func formatNoDecimal(v float64) string {
	return strconv.FormatFloat(v, 'f', 0, 64)
}

func choose(cond bool, a, b uintptr) uintptr {
	if cond {
		return a
	}
	return b
}
