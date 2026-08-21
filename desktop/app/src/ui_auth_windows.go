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
func (a *App) layoutLogin() {
	if a == nil {
		return
	}
	cx := a.width/2 - 240
	if cx < 20 {
		cx = 20
	}
	top := 220
	w := int32(480)
	setpos := func(id int, x, y, w, h int32) {
		user32.NewProc("SetWindowPos").Call(uintptr(a.controls[id]), 0, uintptr(x), uintptr(y), uintptr(w), uintptr(h), 0x0004)
	}
	setpos(idServer, cx, int32(top), w, 34)
	setpos(idEmail, cx, int32(top+70), w, 34)
	setpos(idPassword, cx, int32(top+140), w, 34)
	setpos(idLogin, cx, int32(top+200), 230, 42)
	setpos(idShowRegister, cx+250, int32(top+200), 230, 42)
	setpos(idForgotPassword, cx, int32(top+255), w, 36)
	ys := []int{top, top + 62, top + 124, top + 186, top + 248}
	ids := []int{idCompany, idResponsible, idRegEmail, idRegPassword, idRegConfirm}
	for i, id := range ids {
		setpos(id, cx, int32(ys[i]), w, 34)
	}
	setpos(idRegister, cx, int32(top+315), 250, 42)
	setpos(idShowLogin, cx+270, int32(top+315), 210, 42)
}
func (a *App) showLogin(mode string) {
	a.loginMode = mode
	for _, h := range a.loginControls {
		show(h, mode == "login" && a.token == "")
	}
	for _, h := range a.registerControls {
		show(h, mode == "register" && a.token == "")
	}
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
