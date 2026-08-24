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
	add(idServer, "EDIT", a.serverURL, WS_CHILD|WS_VISIBLE|WS_BORDER|WS_TABSTOP|ES_AUTOHSCROLL, false)
	add(idEmail, "EDIT", "", WS_CHILD|WS_VISIBLE|WS_BORDER|WS_TABSTOP|ES_AUTOHSCROLL, false)
	add(idPassword, "EDIT", "", WS_CHILD|WS_VISIBLE|WS_BORDER|WS_TABSTOP|ES_PASSWORD|ES_AUTOHSCROLL, false)
	add(idLogin, "BUTTON", "Entrar", WS_CHILD|WS_VISIBLE|WS_TABSTOP|BS_DEFPUSHBUTTON, false)
	add(idShowRegister, "BUTTON", "Criar empresa", WS_CHILD|WS_VISIBLE|WS_TABSTOP|BS_PUSHBUTTON, false)
	add(idForgotPassword, "BUTTON", "Esqueci minha senha", WS_CHILD|WS_VISIBLE|WS_TABSTOP|BS_PUSHBUTTON, false)
	add(idCompany, "EDIT", "", WS_CHILD|WS_BORDER|WS_TABSTOP|ES_AUTOHSCROLL, true)
	add(idResponsible, "EDIT", "", WS_CHILD|WS_BORDER|WS_TABSTOP|ES_AUTOHSCROLL, true)
	add(idRegEmail, "EDIT", "", WS_CHILD|WS_BORDER|WS_TABSTOP|ES_AUTOHSCROLL, true)
	add(idRegPassword, "EDIT", "", WS_CHILD|WS_BORDER|WS_TABSTOP|ES_PASSWORD|ES_AUTOHSCROLL, true)
	add(idRegConfirm, "EDIT", "", WS_CHILD|WS_BORDER|WS_TABSTOP|ES_PASSWORD|ES_AUTOHSCROLL, true)
	add(idRegister, "BUTTON", "Criar empresa e entrar", WS_CHILD|WS_TABSTOP|BS_DEFPUSHBUTTON, true)
	add(idShowLogin, "BUTTON", "Já tenho uma conta", WS_CHILD|WS_TABSTOP|BS_PUSHBUTTON, true)
	a.layoutLogin()
	a.showLogin("login")
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
	cardW := width * 31 / 100
	if cardW < 460 {
		cardW = 460
	}
	if cardW > 530 {
		cardW = 530
	}
	cardH := int32(570)
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

	contentPad := int32(48)
	if cardW < 430 {
		contentPad = 32
	}
	contentX := cardX + contentPad
	contentW := cardW - contentPad*2

	l := authLayout{
		card:     Rect{cardX, cardY, cardW, cardH},
		logo:     Rect{cardX + cardW/2 - 24, cardY + 28, 48, 48},
		title:    Rect{cardX + 30, cardY + 88, cardW - 60, 38},
		subtitle: Rect{cardX + 42, cardY + 124, cardW - 84, 40},
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

	fieldStart := cardY + 184
	for i := int32(0); i < 2; i++ {
		y := fieldStart + i*88
		l.labels = append(l.labels, Rect{contentX, y, contentW, 18})
		l.fields = append(l.fields, Rect{contentX, y + 24, contentW, 40})
	}
	l.primary = Rect{contentX, cardY + 368, contentW, 46}
	l.secondary = Rect{contentX, cardY + 426, contentW, 40}
	l.forgot = Rect{contentX + contentW/2 - 115, cardY + 480, 230, 30}
	l.status = Rect{contentX, cardY + cardH - 62, contentW, 18}
	l.security = Rect{contentX, cardY + cardH - 40, contentW, 30}
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
			setpos(id, layout.fields[i])
		}
		setpos(idRegister, layout.primary)
		setpos(idShowLogin, layout.secondary)
		return
	}

	setpos(idEmail, layout.fields[0])
	setpos(idPassword, layout.fields[1])
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
