//go:build windows

package main

import (
	"syscall"
	"unsafe"
)

func (a *App) createFonts() {
	dc, _, _ := procGetDC.Call(uintptr(a.hwnd))
	dpi, _, _ := procGetDeviceCaps.Call(dc, LOGPIXELSY)
	procReleaseDC.Call(uintptr(a.hwnd), dc)
	mk := func(name string, size int, weight int) uintptr {
		h := -int32(size * int(dpi) / 72)
		f, _, _ := procCreateFont.Call(uintptr(h), 0, 0, 0, uintptr(weight), 0, 0, 0, 1, 0, 0, 5, 0, uintptr(unsafe.Pointer(utf16("Segoe UI"))))
		a.fonts[name] = f
		return f
	}
	mk("title", 24, 600)
	mk("h1", 20, 600)
	mk("h2", 15, 600)
	mk("body", 11, 400)
	mk("small", 9, 400)
	mk("metric", 28, 600)
	mk("brand", 21, 600)
}

func createControl(class, text string, style uint32, x, y, w, h int32, parent syscall.Handle, id int) syscall.Handle {
	r, _, _ := procCreateWindowEx.Call(0, uintptr(unsafe.Pointer(utf16(class))), uintptr(unsafe.Pointer(utf16(text))), uintptr(style), uintptr(x), uintptr(y), uintptr(w), uintptr(h), uintptr(parent), uintptr(id), 0, 0)
	return syscall.Handle(r)
}
func setText(h syscall.Handle, s string) {
	procSetWindowText.Call(uintptr(h), uintptr(unsafe.Pointer(utf16(s))))
}
func getText(h syscall.Handle) string {
	n, _, _ := procGetWindowTextLength.Call(uintptr(h))
	b := make([]uint16, n+1)
	procGetWindowText.Call(uintptr(h), uintptr(unsafe.Pointer(&b[0])), n+1)
	return syscall.UTF16ToString(b)
}
func show(h syscall.Handle, v bool) {
	if v {
		procShowWindow.Call(uintptr(h), SW_SHOW)
	} else {
		procShowWindow.Call(uintptr(h), SW_HIDE)
	}
}
func enable(h syscall.Handle, v bool) {
	x := uintptr(0)
	if v {
		x = 1
	}
	procEnableWindow.Call(uintptr(h), x)
}
func message(title, text string, flags uintptr) int {
	parent := uintptr(0)
	if app != nil {
		parent = uintptr(app.hwnd)
	}
	r, _, _ := procMessageBox.Call(parent, uintptr(unsafe.Pointer(utf16(text))), uintptr(unsafe.Pointer(utf16(title))), flags)
	return int(r)
}

func (a *App) buildLogin() {
	add := func(id int, class, text string, style uint32, reg bool) syscall.Handle {
		h := createControl(class, text, style, 0, 0, 100, 28, a.hwnd, id)
		a.controls[id] = h
		font := a.fonts["body"]
		procSendMessage.Call(uintptr(h), WM_SETFONT, font, 1)
		if reg {
			a.registerControls = append(a.registerControls, h)
		} else {
			a.loginControls = append(a.loginControls, h)
		}
		return h
	}
	add(idServer, "EDIT", a.serverURL, WS_CHILD|WS_VISIBLE|WS_TABSTOP|ES_AUTOHSCROLL, false)
	add(idEmail, "EDIT", "", WS_CHILD|WS_VISIBLE|WS_TABSTOP|ES_AUTOHSCROLL, false)
	add(idPassword, "EDIT", "", WS_CHILD|WS_VISIBLE|WS_TABSTOP|ES_PASSWORD|ES_AUTOHSCROLL, false)
	add(idLogin, "BUTTON", "Entrar", WS_CHILD|WS_VISIBLE|WS_TABSTOP|BS_OWNERDRAW, false)
	add(idShowRegister, "BUTTON", "Criar empresa", WS_CHILD|WS_VISIBLE|WS_TABSTOP|BS_OWNERDRAW, false)
	add(idForgotPassword, "BUTTON", "Esqueci minha senha", WS_CHILD|WS_VISIBLE|WS_TABSTOP|BS_OWNERDRAW, false)
	add(idCompany, "EDIT", "", WS_CHILD|WS_TABSTOP|ES_AUTOHSCROLL, true)
	add(idResponsible, "EDIT", "", WS_CHILD|WS_TABSTOP|ES_AUTOHSCROLL, true)
	add(idRegEmail, "EDIT", "", WS_CHILD|WS_TABSTOP|ES_AUTOHSCROLL, true)
	add(idRegPassword, "EDIT", "", WS_CHILD|WS_TABSTOP|ES_PASSWORD|ES_AUTOHSCROLL, true)
	add(idRegConfirm, "EDIT", "", WS_CHILD|WS_TABSTOP|ES_PASSWORD|ES_AUTOHSCROLL, true)
	add(idRegister, "BUTTON", "Criar empresa e entrar", WS_CHILD|WS_TABSTOP|BS_OWNERDRAW, true)
	add(idShowLogin, "BUTTON", "Já tenho uma conta", WS_CHILD|WS_TABSTOP|BS_OWNERDRAW, true)
	a.layoutLogin()
	a.showLogin("login")
}

type authPalette struct {
	page         uintptr
	orb          uintptr
	orbAccent    uintptr
	card         uintptr
	shadow       uintptr
	border       uintptr
	field        uintptr
	fieldBorder  uintptr
	title        uintptr
	body         uintptr
	label        uintptr
	primary      uintptr
	primaryHot   uintptr
	secondary    uintptr
	secondaryHot uintptr
	buttonText   uintptr
	link         uintptr
	success      uintptr
}

var (
	authLightEditBrush uintptr
	authDarkEditBrush  uintptr
)

func (a *App) authPalette() authPalette {
	if a != nil && a.isDarkTheme() {
		return authPalette{
			page:         rawRGB(23, 23, 23),
			orb:          rawRGB(28, 28, 28),
			orbAccent:    rawRGB(31, 31, 31),
			card:         rawRGB(33, 33, 33),
			shadow:       rawRGB(13, 13, 13),
			border:       rawRGB(55, 55, 55),
			field:        rawRGB(43, 43, 43),
			fieldBorder:  rawRGB(65, 65, 65),
			title:        rawRGB(245, 245, 245),
			body:         rawRGB(166, 166, 174),
			label:        rawRGB(205, 205, 211),
			primary:      rawRGB(37, 99, 235),
			primaryHot:   rawRGB(29, 78, 216),
			secondary:    rawRGB(42, 42, 42),
			secondaryHot: rawRGB(50, 50, 50),
			buttonText:   rawRGB(245, 245, 245),
			link:         rawRGB(112, 166, 255),
			success:      rawRGB(74, 222, 128),
		}
	}
	return authPalette{
		page:         rawRGB(247, 249, 252),
		orb:          rawRGB(238, 243, 249),
		orbAccent:    rawRGB(232, 239, 250),
		card:         rawRGB(255, 255, 255),
		shadow:       rawRGB(220, 227, 235),
		border:       rawRGB(226, 232, 240),
		field:        rawRGB(250, 252, 255),
		fieldBorder:  rawRGB(210, 220, 232),
		title:        rawRGB(15, 31, 55),
		body:         rawRGB(96, 112, 136),
		label:        rawRGB(52, 68, 91),
		primary:      rawRGB(37, 99, 235),
		primaryHot:   rawRGB(29, 78, 216),
		secondary:    rawRGB(255, 255, 255),
		secondaryHot: rawRGB(246, 249, 253),
		buttonText:   rawRGB(30, 45, 66),
		link:         rawRGB(37, 99, 235),
		success:      rawRGB(22, 163, 74),
	}
}

func authFillRaw(dc syscall.Handle, r Rect, c uintptr) {
	b, _, _ := procCreateSolidBrush.Call(c)
	rr := RECT{r.X, r.Y, r.X + r.W, r.Y + r.H}
	procFillRect.Call(uintptr(dc), uintptr(unsafe.Pointer(&rr)), b)
	procDeleteObject.Call(b)
}

func authRoundRaw(dc syscall.Handle, r Rect, background, border uintptr, radius int32) {
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

func authTextRaw(dc syscall.Handle, s string, r Rect, font uintptr, color uintptr, flags uintptr) {
	procSelectObject.Call(uintptr(dc), font)
	procSetBkMode.Call(uintptr(dc), TRANSPARENT)
	procSetTextColor.Call(uintptr(dc), color)
	rr := RECT{r.X, r.Y, r.X + r.W, r.Y + r.H}
	procDrawText.Call(uintptr(dc), uintptr(unsafe.Pointer(utf16(s))), uintptr(len([]rune(s))), uintptr(unsafe.Pointer(&rr)), flags)
}

func authCircleRaw(dc syscall.Handle, r Rect, c uintptr) {
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

func authLogoMark(dc syscall.Handle, r Rect, background uintptr) {
	blue := rawRGB(19, 102, 246)
	authCircleRaw(dc, r, blue)
	ring := max32(4, r.W/5)
	inner := Rect{r.X + ring, r.Y + ring, r.W - ring*2, r.H - ring*2}
	authCircleRaw(dc, inner, background)
	gap := max32(3, r.W/10)
	authFillRaw(dc, Rect{r.X + r.W/2 - gap/2, r.Y - 1, gap, ring + 3}, background)
	authFillRaw(dc, Rect{r.X + r.W/2 - gap/2, r.Y + r.H - ring - 2, gap, ring + 4}, background)
	authFillRaw(dc, Rect{r.X - 1, r.Y + r.H/2 - gap/2, ring + 3, gap}, background)
	authFillRaw(dc, Rect{r.X + r.W - ring - 2, r.Y + r.H/2 - gap/2, ring + 4, gap}, background)
	knob := max32(12, r.W*9/20)
	kx := r.X + (r.W-knob)/2
	ky := r.Y + (r.H-knob)/2 + 2
	authCircleRaw(dc, Rect{kx, ky, knob, knob}, blue)
	stemW := max32(4, r.W/7)
	stemX := r.X + (r.W-stemW)/2
	authFillRaw(dc, Rect{stemX, r.Y + ring/2, stemW, ky - r.Y - ring/2 + 3}, blue)
}

func authFieldInner(r Rect) Rect {
	return Rect{r.X + 13, r.Y + 7, r.W - 26, r.H - 14}
}

func (a *App) isAuthEditControl(h syscall.Handle) bool {
	for _, id := range []int{idEmail, idPassword, idCompany, idResponsible, idRegEmail, idRegPassword, idRegConfirm} {
		if a.controls[id] == h {
			return true
		}
	}
	return false
}

func (a *App) authEditColor(dc syscall.Handle, control syscall.Handle) uintptr {
	if a == nil || !a.isAuthEditControl(control) {
		return 0
	}
	pal := a.authPalette()
	procSetTextColor.Call(uintptr(dc), pal.title)
	procSetBkColor.Call(uintptr(dc), pal.field)
	if a.isDarkTheme() {
		if authDarkEditBrush == 0 {
			authDarkEditBrush, _, _ = procCreateSolidBrush.Call(pal.field)
		}
		return authDarkEditBrush
	}
	if authLightEditBrush == 0 {
		authLightEditBrush, _, _ = procCreateSolidBrush.Call(pal.field)
	}
	return authLightEditBrush
}

const (
	ODS_SELECTED = 0x0001
	ODS_DISABLED = 0x0004
	ODS_FOCUS    = 0x0010
)

func (a *App) drawAuthButton(lParam uintptr) bool {
	if lParam == 0 {
		return false
	}
	dis := (*DRAWITEMSTRUCT)(unsafe.Pointer(lParam))
	id := int(dis.CtlID)
	label := ""
	kind := "secondary"
	switch id {
	case idLogin:
		label, kind = "Entrar", "primary"
	case idRegister:
		label, kind = "Criar empresa e entrar", "primary"
	case idShowRegister:
		label = "Criar empresa"
	case idShowLogin:
		label = "Já tenho uma conta"
	case idForgotPassword:
		label, kind = "Esqueci minha senha", "link"
	default:
		return false
	}

	pal := a.authPalette()
	r := Rect{dis.RcItem.Left, dis.RcItem.Top, dis.RcItem.Right - dis.RcItem.Left, dis.RcItem.Bottom - dis.RcItem.Top}
	authFillRaw(dis.HDC, r, pal.card)
	pressed := dis.ItemState&ODS_SELECTED != 0
	disabled := dis.ItemState&ODS_DISABLED != 0

	if kind == "link" {
		fg := pal.link
		if disabled {
			fg = pal.body
		}
		if pressed {
			r.Y++
		}
		authTextRaw(dis.HDC, label, r, a.fonts["small"], fg, DT_CENTER|DT_VCENTER|DT_SINGLELINE)
		return true
	}

	bg := pal.secondary
	border := pal.fieldBorder
	fg := pal.buttonText
	if kind == "primary" {
		bg = pal.primary
		border = pal.primary
		fg = rawRGB(255, 255, 255)
	}
	if pressed {
		if kind == "primary" {
			bg, border = pal.primaryHot, pal.primaryHot
		} else {
			bg = pal.secondaryHot
		}
	}
	if disabled {
		fg = pal.body
	}
	authRoundRaw(dis.HDC, r, bg, border, 10)
	if dis.ItemState&ODS_FOCUS != 0 {
		focus := Rect{r.X + 3, r.Y + 3, r.W - 6, r.H - 6}
		authRoundRaw(dis.HDC, focus, bg, pal.link, 8)
	}
	authTextRaw(dis.HDC, label, r, a.fonts["body"], fg, DT_CENTER|DT_VCENTER|DT_SINGLELINE)
	return true
}

type authLayout struct {
	card      Rect
	logo      Rect
	title     Rect
	subtitle  Rect
	labels    []Rect
	fields    []Rect
	primary   Rect
	secondary Rect
	forgot    Rect
	status    Rect
	security  Rect
}

func makeAuthLayout(width, height int32, mode string) authLayout {
	margin := int32(28)

	// O card acompanha o tamanho da janela, mas mantém limites confortáveis.
	// Em monitores grandes ele não fica minúsculo; em janelas menores não estoura.
	cardW := width * 30 / 100
	if cardW < 440 {
		cardW = 440
	}
	if cardW > 500 {
		cardW = 500
	}
	cardH := int32(548)
	if mode == "register" {
		cardW = width * 34 / 100
		if cardW < 500 {
			cardW = 500
		}
		if cardW > 570 {
			cardW = 570
		}
		cardH = 680
	}

	if maxW := width - margin*2; maxW < cardW {
		cardW = maxW
	}
	if maxH := height - margin*2; maxH < cardH {
		cardH = maxH
	}
	if cardW < 340 {
		cardW = 340
	}
	if cardH < 500 && mode == "login" {
		cardH = 500
	}
	if cardH < 600 && mode == "register" {
		cardH = 600
	}

	cardX := (width - cardW) / 2
	cardY := (height - cardH) / 2
	if cardX < margin {
		cardX = margin
	}
	if cardY < margin {
		cardY = margin
	}

	contentPad := int32(44)
	if cardW < 420 {
		contentPad = 30
	}
	contentX := cardX + contentPad
	contentW := cardW - contentPad*2

	l := authLayout{
		card:     Rect{cardX, cardY, cardW, cardH},
		logo:     Rect{cardX + cardW/2 - 22, cardY + 28, 44, 44},
		title:    Rect{cardX + 30, cardY + 82, cardW - 60, 36},
		subtitle: Rect{cardX + 42, cardY + 116, cardW - 84, 38},
	}

	if mode == "register" {
		fieldStart := cardY + 174
		for i := int32(0); i < 5; i++ {
			y := fieldStart + i*68
			l.labels = append(l.labels, Rect{contentX, y, contentW, 18})
			l.fields = append(l.fields, Rect{contentX, y + 23, contentW, 40})
		}
		l.primary = Rect{contentX, cardY + cardH - 144, contentW, 46}
		l.secondary = Rect{contentX, cardY + cardH - 88, contentW, 38}
		l.status = Rect{contentX, cardY + cardH - 46, contentW, 18}
		l.security = Rect{contentX, cardY + cardH - 30, contentW, 22}
		return l
	}

	fieldStart := cardY + 174
	for i := int32(0); i < 2; i++ {
		y := fieldStart + i*82
		l.labels = append(l.labels, Rect{contentX, y, contentW, 18})
		l.fields = append(l.fields, Rect{contentX, y + 23, contentW, 46})
	}
	l.primary = Rect{contentX, cardY + 350, contentW, 46}
	l.secondary = Rect{contentX, cardY + 407, contentW, 42}
	l.forgot = Rect{contentX + contentW/2 - 115, cardY + 458, 230, 28}
	l.status = Rect{contentX, cardY + cardH - 66, contentW, 18}
	l.security = Rect{contentX, cardY + cardH - 44, contentW, 34}
	return l
}

func (a *App) layoutLogin() {
	if a == nil {
		return
	}
	if a.token != "" {
		show(a.controls[idServer], a.page == 7)
		if a.page == 7 {
			x, y := contentOrigin()
			w := a.width - x - 28
			r := Rect{x + 20, y + 76, w - 240, 38}
			user32.NewProc("SetWindowPos").Call(uintptr(a.controls[idServer]), 0, uintptr(r.X), uintptr(r.Y), uintptr(r.W), uintptr(r.H), 0x0004)
		}
		return
	}
	layout := makeAuthLayout(a.width, a.height, a.loginMode)
	setpos := func(id int, r Rect) {
		h := a.controls[id]
		if h == 0 {
			return
		}
		user32.NewProc("SetWindowPos").Call(uintptr(h), 0, uintptr(r.X), uintptr(r.Y), uintptr(r.W), uintptr(r.H), 0x0004)
	}

	// O servidor continua configurado e funcional, mas fica fora da tela de login.
	show(a.controls[idServer], false)

	if a.loginMode == "register" {
		ids := []int{idCompany, idResponsible, idRegEmail, idRegPassword, idRegConfirm}
		for i, id := range ids {
			setpos(id, authFieldInner(layout.fields[i]))
		}
		setpos(idRegister, layout.primary)
		setpos(idShowLogin, layout.secondary)
		return
	}

	setpos(idEmail, authFieldInner(layout.fields[0]))
	setpos(idPassword, authFieldInner(layout.fields[1]))
	setpos(idLogin, layout.primary)
	setpos(idShowRegister, layout.secondary)
	setpos(idForgotPassword, layout.forgot)
}

func (a *App) showLogin(mode string) {
	a.loginMode = mode
	for _, h := range a.loginControls {
		show(h, mode == "login" && a.token == "")
	}
	for _, h := range a.registerControls {
		show(h, mode == "register" && a.token == "")
	}
	show(a.controls[idServer], false)
	a.layoutLogin()
	a.invalidate()
}
func (a *App) hideAuth() {
	for _, h := range a.loginControls {
		show(h, false)
	}
	for _, h := range a.registerControls {
		show(h, false)
	}
}

func (a *App) command(id int) {
	switch id {
	case idShowRegister:
		a.showLogin("register")
	case idShowLogin:
		a.showLogin("login")
	case idLogin:
		go a.login()
	case idForgotPassword:
		a.openPasswordRecovery()
	case idRegister:
		go a.register()
	}
}
