//go:build windows

package main

import (
	"strings"
	"syscall"
	"unsafe"
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
	tc := rgb(0, 139, 158)
	border := rgb(180, 210, 216)
	hovered := app != nil && app.isHovered(r)
	if primary {
		c = rgb(0, 151, 170)
		tc = rgb(255, 255, 255)
		border = rgb(0, 151, 170)
		if hovered {
			c = rgb(0, 132, 149)
			border = c
		}
	} else if hovered {
		c = rgb(239, 249, 251)
		border = rgb(0, 151, 170)
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

func logoMark(dc syscall.Handle, r Rect) {
	// Marca vetorial simples: um C azul com detalhe verde-água.
	circle(dc, r, rgb(20, 113, 219))
	inner := Rect{r.X + r.W/4, r.Y + r.H/4, r.W / 2, r.H / 2}
	circle(dc, inner, rgb(255, 255, 255))
	fill(dc, Rect{r.X + r.W/2, r.Y + r.H/5, r.W/2 + 2, r.H * 3 / 5}, rgb(255, 255, 255))
	circle(dc, Rect{r.X + r.W*3/5, r.Y + r.H*3/5, r.W / 4, r.H / 4}, rgb(0, 165, 170))
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
	fill(dc, Rect{0, 0, rc.Right, rc.Bottom}, rgb(249, 251, 254))
	text(dc, "Core", Rect{55, 35, 100, 45}, a.fonts["brand"], rgb(10, 31, 62), DT_LEFT|DT_VCENTER|DT_SINGLELINE)
	text(dc, "Tuner", Rect{125, 35, 120, 45}, a.fonts["brand"], rgb(18, 101, 246), DT_LEFT|DT_VCENTER|DT_SINGLELINE)
	cx := rc.Right/2 - 280
	card(dc, Rect{cx, 110, 560, 620})
	title := "Entrar na sua empresa"
	sub := "Use a mesma conta do site para acessar os computadores."
	if a.loginMode == "register" {
		title = "Criar uma empresa"
		sub = "A empresa será criada no CoreTuner Central."
	}
	text(dc, title, Rect{cx + 40, 145, 480, 45}, a.fonts["title"], rgb(11, 31, 60), DT_CENTER|DT_VCENTER|DT_SINGLELINE)
	text(dc, sub, Rect{cx + 40, 190, 480, 45}, a.fonts["body"], rgb(93, 112, 140), DT_CENTER|DT_WORDBREAK)
	labels := []string{"Servidor CoreTuner", "E-mail", "Senha"}
	ys := []int32{285, 355, 425}
	if a.loginMode == "register" {
		labels = []string{"Nome da empresa", "Responsável", "E-mail", "Senha", "Confirmar senha"}
		ys = []int32{285, 347, 409, 471, 533}
	}
	for i, l := range labels {
		text(dc, l, Rect{cx + 40, ys[i] - 24, 480, 20}, a.fonts["small"], rgb(55, 73, 102), DT_LEFT|DT_SINGLELINE)
	}
	a.mu.RLock()
	st, busy := a.statusText, a.busy
	a.mu.RUnlock()
	if busy {
		text(dc, st, Rect{cx + 40, 680, 480, 24}, a.fonts["small"], rgb(18, 101, 246), DT_CENTER|DT_SINGLELINE)
	}
	text(dc, "Segurança: o CoreTuner não acessa documentos, conversas ou senhas.", Rect{cx + 45, 695, 470, 34}, a.fonts["small"], rgb(37, 153, 87), DT_CENTER|DT_WORDBREAK)
}

var menuLabels = []string{"Painel inicial", "Diagnóstico", "Testes", "Otimizações", "Programas", "Relatórios", "Histórico", "Configurações", "Suporte"}
var menuIcons = []string{"⌂", "◈", "✓", "⌁", "▦", "▤", "↻", "⚙", "?"}

func (a *App) drawShell(dc syscall.Handle, rc RECT) {
	side := int32(212)
	fill(dc, Rect{0, 0, side, rc.Bottom}, rgb(255, 255, 255))
	line(dc, side, 0, side, rc.Bottom, rgb(227, 233, 240))

	logoMark(dc, Rect{24, 22, 38, 38})
	text(dc, "CoreTuner", Rect{72, 20, 125, 42}, a.fonts["brand"], rgb(10, 31, 62), DT_LEFT|DT_VCENTER|DT_SINGLELINE)

	y := int32(92)
	for i, label := range menuLabels {
		r := Rect{14, y + int32(i*48), 184, 40}
		selected := a.page == i
		if selected {
			roundedBox(dc, r, rgb(232, 247, 249), rgb(232, 247, 249), 10)
			fill(dc, Rect{r.X, r.Y + 8, 3, r.H - 16}, rgb(0, 151, 170))
		} else if a.isHovered(r) {
			roundedBox(dc, r, rgb(244, 249, 251), rgb(244, 249, 251), 10)
		}
		iconColor := choose(selected, rgb(0, 139, 158), rgb(91, 107, 130))
		text(dc, menuIcons[i], Rect{r.X + 14, r.Y, 28, r.H}, a.fonts["body"], iconColor, DT_CENTER|DT_VCENTER|DT_SINGLELINE)
		text(dc, label, Rect{r.X + 50, r.Y, r.W - 58, r.H}, a.fonts["body"], choose(selected, rgb(0, 116, 133), rgb(29, 47, 72)), DT_LEFT|DT_VCENTER|DT_SINGLELINE)
		a.hits = append(a.hits, Hit{r, "page", i})
	}

	a.mu.RLock()
	company := companyName(a.company)
	user := a.user.Name
	a.mu.RUnlock()
	profileCard := Rect{18, rc.Bottom - 126, side - 36, 72}
	roundedBox(dc, profileCard, rgb(250, 252, 254), rgb(226, 233, 240), 12)
	initials := "CT"
	if strings.TrimSpace(company) != "" {
		parts := strings.Fields(company)
		if len(parts) == 1 {
			r := []rune(parts[0])
			if len(r) >= 2 {
				initials = strings.ToUpper(string(r[:2]))
			} else if len(r) == 1 {
				initials = strings.ToUpper(string(r))
			}
		} else {
			initials = strings.ToUpper(string([]rune(parts[0])[0]) + string([]rune(parts[len(parts)-1])[0]))
		}
	}
	circle(dc, Rect{profileCard.X + 12, profileCard.Y + 15, 40, 40}, rgb(0, 151, 170))
	text(dc, initials, Rect{profileCard.X + 12, profileCard.Y + 15, 40, 40}, a.fonts["small"], rgb(255, 255, 255), DT_CENTER|DT_VCENTER|DT_SINGLELINE)
	text(dc, company, Rect{profileCard.X + 62, profileCard.Y + 12, profileCard.W - 72, 24}, a.fonts["body"], rgb(17, 38, 70), DT_LEFT|DT_END_ELLIPSIS|DT_SINGLELINE)
	text(dc, user, Rect{profileCard.X + 62, profileCard.Y + 37, profileCard.W - 72, 20}, a.fonts["small"], rgb(98, 113, 137), DT_LEFT|DT_END_ELLIPSIS|DT_SINGLELINE)

	logoutRect := Rect{24, rc.Bottom - 42, 100, 26}
	if a.isHovered(logoutRect) {
		roundedBox(dc, Rect{logoutRect.X - 8, logoutRect.Y - 4, logoutRect.W + 16, logoutRect.H + 8}, rgb(255, 245, 245), rgb(255, 245, 245), 8)
	}
	text(dc, "↪  Sair", logoutRect, a.fonts["small"], rgb(176, 55, 55), DT_LEFT|DT_VCENTER|DT_SINGLELINE)
	a.hits = append(a.hits, Hit{logoutRect, "logout", 0})

	fill(dc, Rect{side, 0, rc.Right - side, 78}, rgb(255, 255, 255))
	line(dc, side, 78, rc.Right, 78, rgb(227, 233, 240))
	text(dc, menuLabels[a.page], Rect{side + 30, 16, 400, 42}, a.fonts["title"], rgb(9, 28, 56), DT_LEFT|DT_VCENTER|DT_SINGLELINE)

	a.mu.RLock()
	sys := a.sys
	a.mu.RUnlock()
	circle(dc, Rect{side + 405, 34, 8, 8}, rgb(31, 171, 102))
	text(dc, "Atualizado agora", Rect{side + 420, 22, 150, 32}, a.fonts["small"], rgb(91, 106, 130), DT_LEFT|DT_VCENTER|DT_SINGLELINE)
	if !sys.Updated.IsZero() {
		text(dc, sys.Updated.Format("15:04:05"), Rect{rc.Right - 110, 23, 82, 30}, a.fonts["small"], rgb(101, 116, 140), DT_RIGHT|DT_VCENTER|DT_SINGLELINE)
	}
}
func choose(cond bool, a, b uintptr) uintptr {
	if cond {
		return a
	}
	return b
}
