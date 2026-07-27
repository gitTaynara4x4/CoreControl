//go:build windows

package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"
	"unsafe"
)

const appVersion = "0.4.14"

var defaultServerURL = "http://127.0.0.1:8002"

const (
	WS_OVERLAPPEDWINDOW = 0x00CF0000
	WS_VISIBLE          = 0x10000000
	WS_CHILD            = 0x40000000
	WS_BORDER           = 0x00800000
	WS_TABSTOP          = 0x00010000
	ES_AUTOHSCROLL      = 0x0080
	ES_PASSWORD         = 0x0020
	BS_PUSHBUTTON       = 0
	BS_DEFPUSHBUTTON    = 1
	SW_SHOW             = 5
	SW_HIDE             = 0
	SW_SHOWNORMAL       = 1
	WM_DESTROY          = 0x0002
	WM_SIZE             = 0x0005
	WM_PAINT            = 0x000F
	WM_CLOSE            = 0x0010
	WM_ERASEBKGND       = 0x0014
	WM_SETCURSOR        = 0x0020
	WM_COMMAND          = 0x0111
	WM_TIMER            = 0x0113
	WM_MOUSEMOVE        = 0x0200
	WM_LBUTTONUP        = 0x0202
	WM_SETFONT          = 0x0030
	WM_APP              = 0x8000
	WM_APP_REFRESH      = WM_APP + 1
	WM_APP_MESSAGE      = WM_APP + 2
	COLOR_WINDOW        = 5
	DEFAULT_GUI_FONT    = 17
	TRANSPARENT         = 1
	PS_SOLID            = 0
	DT_LEFT             = 0x0000
	DT_CENTER           = 0x0001
	DT_RIGHT            = 0x0002
	DT_VCENTER          = 0x0004
	DT_SINGLELINE       = 0x0020
	DT_WORDBREAK        = 0x0010
	DT_END_ELLIPSIS     = 0x00008000
	MB_OK               = 0
	MB_ICONINFORMATION  = 0x40
	MB_ICONERROR        = 0x10
	MB_YESNO            = 0x04
	MB_ICONQUESTION     = 0x20
	IDYES               = 6
	SRCCOPY             = 0x00CC0020
	LOGPIXELSY          = 90
	IDC_ARROW           = 32512
	IDC_HAND            = 32649
)

const (
	idServer         = 101
	idEmail          = 102
	idPassword       = 103
	idLogin          = 104
	idShowRegister   = 105
	idForgotPassword = 106
	idCompany        = 111
	idResponsible    = 112
	idRegEmail       = 113
	idRegPassword    = 114
	idRegConfirm     = 115
	idRegister       = 116
	idShowLogin      = 117
)

type POINT struct{ X, Y int32 }
type RECT struct{ Left, Top, Right, Bottom int32 }
type MSG struct {
	HWnd           syscall.Handle
	Message        uint32
	WParam, LParam uintptr
	Time           uint32
	Pt             POINT
	LPrivate       uint32
}
type PAINTSTRUCT struct {
	Hdc         syscall.Handle
	FErase      int32
	RcPaint     RECT
	FRestore    int32
	FIncUpdate  int32
	RgbReserved [32]byte
}
type WNDCLASSEX struct {
	CbSize, Style                            uint32
	LpfnWndProc                              uintptr
	CbClsExtra, CbWndExtra                   int32
	HInstance, HIcon, HCursor, HbrBackground syscall.Handle
	LpszMenuName, LpszClassName              *uint16
	HIconSm                                  syscall.Handle
}
type MEMORYSTATUSEX struct {
	DwLength                                                                                                                  uint32
	DwMemoryLoad                                                                                                              uint32
	UllTotalPhys, UllAvailPhys, UllTotalPageFile, UllAvailPageFile, UllTotalVirtual, UllAvailVirtual, UllAvailExtendedVirtual uint64
}
type FILETIME struct{ Low, High uint32 }

type Rect struct{ X, Y, W, H int32 }

func (r Rect) contains(x, y int32) bool { return x >= r.X && x < r.X+r.W && y >= r.Y && y < r.Y+r.H }

type Hit struct {
	Rect   Rect
	Action string
	Value  int
}

type AuthUser struct {
	ID                int `json:"id"`
	Name, Email, Role string
	CompanyID         *int `json:"company_id"`
}
type Company struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}
type AuthResponse struct {
	AccessToken string   `json:"access_token"`
	User        AuthUser `json:"user"`
	Company     *Company `json:"company"`
}
type MeResponse struct {
	ID        int      `json:"id"`
	Name      string   `json:"name"`
	Email     string   `json:"email"`
	Role      string   `json:"role"`
	CompanyID *int     `json:"company_id"`
	Company   *Company `json:"company"`
}
type APIError struct {
	Detail any `json:"detail"`
}

type SystemInfo struct {
	Hostname, Manufacturer, Model, Serial, OS, CPUName, DiskName, DiskType, Username string
	CPU, Memory, Disk                                                                float64
	TotalRAMGB, UsedRAMGB, DiskTotalGB, DiskUsedGB                                   float64
	Uptime                                                                           time.Duration
	AudioOK, MicOK                                                                   bool
	InternetOK                                                                       bool
	LatencyMS                                                                        int64
	Updated                                                                          time.Time
}
type ProcessInfo struct {
	Name      string
	PID       int
	ParentPID int
	MemoryMB  float64
	CPU       float64
}
type HistoryItem struct {
	At     time.Time `json:"at"`
	Title  string    `json:"title"`
	Detail string    `json:"detail"`
}
type Session struct {
	ServerURL   string    `json:"server_url"`
	AccessToken string    `json:"access_token"`
	User        AuthUser  `json:"user"`
	Company     *Company  `json:"company"`
	SavedAt     time.Time `json:"saved_at"`
}

type App struct {
	hwnd                            syscall.Handle
	width, height                   int32
	page                            int
	loginMode                       string
	controls                        map[int]syscall.Handle
	loginControls, registerControls []syscall.Handle
	fonts                           map[string]uintptr
	hits                            []Hit
	mu                              sync.RWMutex
	client                          *http.Client
	serverURL, token                string
	user                            AuthUser
	company                         *Company
	sys                             SystemInfo
	processes                       []ProcessInfo
	history                         []HistoryItem
	profile                         int
	statusText                      string
	busy                            bool
	centralOK                       bool
	mouseX, mouseY                  int32
	hoverRect                       Rect
	hoverActive                     bool
	optimizationActive              int
	optimizationAppliedAt           time.Time
	optimizationNote                string
	optimizationBusy                bool
}

var app *App

var (
	user32                     = syscall.NewLazyDLL("user32.dll")
	kernel32                   = syscall.NewLazyDLL("kernel32.dll")
	gdi32                      = syscall.NewLazyDLL("gdi32.dll")
	shell32                    = syscall.NewLazyDLL("shell32.dll")
	procRegisterClassEx        = user32.NewProc("RegisterClassExW")
	procCreateWindowEx         = user32.NewProc("CreateWindowExW")
	procDefWindowProc          = user32.NewProc("DefWindowProcW")
	procShowWindow             = user32.NewProc("ShowWindow")
	procUpdateWindow           = user32.NewProc("UpdateWindow")
	procGetMessage             = user32.NewProc("GetMessageW")
	procTranslateMessage       = user32.NewProc("TranslateMessage")
	procDispatchMessage        = user32.NewProc("DispatchMessageW")
	procPostQuitMessage        = user32.NewProc("PostQuitMessage")
	procSendMessage            = user32.NewProc("SendMessageW")
	procSetWindowText          = user32.NewProc("SetWindowTextW")
	procGetWindowText          = user32.NewProc("GetWindowTextW")
	procGetWindowTextLength    = user32.NewProc("GetWindowTextLengthW")
	procEnableWindow           = user32.NewProc("EnableWindow")
	procMessageBox             = user32.NewProc("MessageBoxW")
	procBeginPaint             = user32.NewProc("BeginPaint")
	procEndPaint               = user32.NewProc("EndPaint")
	procGetClientRect          = user32.NewProc("GetClientRect")
	procInvalidateRect         = user32.NewProc("InvalidateRect")
	procSetTimer               = user32.NewProc("SetTimer")
	procKillTimer              = user32.NewProc("KillTimer")
	procDestroyWindow          = user32.NewProc("DestroyWindow")
	procLoadCursor             = user32.NewProc("LoadCursorW")
	procSetCursor              = user32.NewProc("SetCursor")
	procGetDlgCtrlID           = user32.NewProc("GetDlgCtrlID")
	procGetDC                  = user32.NewProc("GetDC")
	procReleaseDC              = user32.NewProc("ReleaseDC")
	procFillRect               = user32.NewProc("FillRect")
	procDrawText               = user32.NewProc("DrawTextW")
	procGetStockObject         = gdi32.NewProc("GetStockObject")
	procCreateFont             = gdi32.NewProc("CreateFontW")
	procCreateSolidBrush       = gdi32.NewProc("CreateSolidBrush")
	procCreatePen              = gdi32.NewProc("CreatePen")
	procSelectObject           = gdi32.NewProc("SelectObject")
	procDeleteObject           = gdi32.NewProc("DeleteObject")
	procSetTextColor           = gdi32.NewProc("SetTextColor")
	procSetBkMode              = gdi32.NewProc("SetBkMode")
	procRoundRect              = gdi32.NewProc("RoundRect")
	procEllipse                = gdi32.NewProc("Ellipse")
	procMoveToEx               = gdi32.NewProc("MoveToEx")
	procLineTo                 = gdi32.NewProc("LineTo")
	procCreateCompatibleDC     = gdi32.NewProc("CreateCompatibleDC")
	procCreateCompatibleBitmap = gdi32.NewProc("CreateCompatibleBitmap")
	procBitBlt                 = gdi32.NewProc("BitBlt")
	procDeleteDC               = gdi32.NewProc("DeleteDC")
	procGetDeviceCaps          = gdi32.NewProc("GetDeviceCaps")
	procGetModuleHandle        = kernel32.NewProc("GetModuleHandleW")
	procGlobalMemoryStatusEx   = kernel32.NewProc("GlobalMemoryStatusEx")
	procGetDiskFreeSpaceEx     = kernel32.NewProc("GetDiskFreeSpaceExW")
	procGetSystemTimes         = kernel32.NewProc("GetSystemTimes")
	procGetTickCount64         = kernel32.NewProc("GetTickCount64")
	procShellExecute           = shell32.NewProc("ShellExecuteW")
)

func utf16(s string) *uint16     { return syscall.StringToUTF16Ptr(s) }
func rgb(r, g, b byte) uintptr   { return uintptr(r) | uintptr(g)<<8 | uintptr(b)<<16 }
func loword(v uintptr) int       { return int(v & 0xffff) }
func signedLow(v uintptr) int32  { return int32(int16(v & 0xffff)) }
func signedHigh(v uintptr) int32 { return int32(int16((v >> 16) & 0xffff)) }

func main() { runtime.LockOSThread(); runGUI() }

func runGUI() {
	hinst, _, _ := procGetModuleHandle.Call(0)
	cls := utf16("CoreTunerDesktopWindow")
	largeIcon, smallIcon := coreTunerWindowIcons()
	arrowCursor, _, _ := procLoadCursor.Call(0, IDC_ARROW)
	wc := WNDCLASSEX{CbSize: uint32(unsafe.Sizeof(WNDCLASSEX{})), LpfnWndProc: syscall.NewCallback(wndProc), HInstance: syscall.Handle(hinst), HIcon: largeIcon, HCursor: syscall.Handle(arrowCursor), HbrBackground: syscall.Handle(COLOR_WINDOW + 1), LpszClassName: cls, HIconSm: smallIcon}
	procRegisterClassEx.Call(uintptr(unsafe.Pointer(&wc)))
	h, _, _ := procCreateWindowEx.Call(0, uintptr(unsafe.Pointer(cls)), uintptr(unsafe.Pointer(utf16("CoreTuner — Diagnóstico e gestão segura"))), WS_OVERLAPPEDWINDOW, 40, 30, 1420, 900, 0, 0, hinst, 0)
	if h == 0 {
		return
	}
	applyCoreTunerWindowIcons(syscall.Handle(h), largeIcon, smallIcon)
	app = &App{hwnd: syscall.Handle(h), width: 1420, height: 900, page: 0, loginMode: "login", controls: map[int]syscall.Handle{}, fonts: map[string]uintptr{}, client: &http.Client{Timeout: 20 * time.Second}, serverURL: loadServerURL(), statusText: "Preparando diagnóstico seguro..."}
	app.createFonts()
	app.buildLogin()
	app.loadHistory()
	app.loadSession()
	_ = ensureOptimizationDirectories()
	app.refreshOptimizationSummary()
	procShowWindow.Call(h, SW_SHOW)
	procUpdateWindow.Call(h)
	procSetTimer.Call(h, 1, 2500, 0)
	procSetTimer.Call(h, 2, 30000, 0)
	go app.refreshLocal(true)
	var msg MSG
	for {
		r, _, _ := procGetMessage.Call(uintptr(unsafe.Pointer(&msg)), 0, 0, 0)
		if int32(r) <= 0 {
			break
		}
		procTranslateMessage.Call(uintptr(unsafe.Pointer(&msg)))
		procDispatchMessage.Call(uintptr(unsafe.Pointer(&msg)))
	}
}

func wndProc(hwnd uintptr, msg uint32, wParam, lParam uintptr) uintptr {
	switch msg {
	case WM_ERASEBKGND:
		return 1
	case WM_SIZE:
		if app != nil {
			app.width = int32(lParam & 0xffff)
			app.height = int32((lParam >> 16) & 0xffff)
			app.layoutLogin()
			app.invalidate()
		}
		return 0
	case WM_PAINT:
		if app != nil {
			app.paint()
		}
		return 0
	case WM_MOUSEMOVE:
		if app != nil && app.token != "" {
			app.mouseMove(signedLow(lParam), signedHigh(lParam))
		}
		return 0
	case WM_SETCURSOR:
		if app != nil {
			target := syscall.Handle(wParam)
			if target != app.hwnd {
				if app.isClickableControl(target) {
					app.setPointerCursor(true)
					return 1
				}
			} else if app.token != "" {
				app.setPointerCursor(app.hitAt(app.mouseX, app.mouseY) != nil)
				return 1
			}
		}
	case WM_LBUTTONUP:
		if app != nil && app.token != "" {
			app.click(signedLow(lParam), signedHigh(lParam))
		}
		return 0
	case WM_COMMAND:
		if app != nil {
			app.command(loword(wParam))
		}
		return 0
	case WM_TIMER:
		if app != nil {
			if wParam == 1 {
				go app.refreshLocal(false)
			} else if wParam == 2 {
				go app.refreshLocal(true)
				if app.token != "" {
					go app.refreshCentralStatus()
				}
			}
		}
		return 0
	case WM_APP_REFRESH:
		if app != nil {
			app.invalidate()
		}
		return 0
	case WM_CLOSE:
		if app != nil && message("CoreTuner", "Deseja fechar o CoreTuner?", MB_YESNO|MB_ICONQUESTION) != IDYES {
			return 0
		}
		procDestroyWindow.Call(hwnd)
		return 0
	case WM_DESTROY:
		procKillTimer.Call(hwnd, 1)
		procKillTimer.Call(hwnd, 2)
		procPostQuitMessage.Call(0)
		return 0
	}
	r, _, _ := procDefWindowProc.Call(hwnd, uintptr(msg), wParam, lParam)
	return r
}

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
func (a *App) server() (string, error) {
	raw := strings.TrimRight(strings.TrimSpace(getText(a.controls[idServer])), "/")
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") {
		return "", errors.New("Informe um endereço válido do CoreTuner Central")
	}
	host := strings.ToLower(strings.Split(u.Host, ":")[0])
	local := u.Scheme == "http" && (host == "127.0.0.1" || host == "localhost" || host == "::1")
	if u.Scheme != "https" && !local {
		return "", errors.New("HTTPS é obrigatório fora do teste local")
	}
	a.serverURL = raw
	saveServerURL(raw)
	return raw, nil
}
func (a *App) openPasswordRecovery() {
	server, err := a.server()
	if err != nil {
		message("CoreTuner", err.Error(), MB_OK|MB_ICONERROR)
		return
	}
	target := server + "/?forgot=1"
	procShellExecute.Call(uintptr(a.hwnd), uintptr(unsafe.Pointer(utf16("open"))), uintptr(unsafe.Pointer(utf16(target))), 0, 0, SW_SHOWNORMAL)
}

func (a *App) login() {
	a.setBusy(true, "Entrando na empresa...")
	defer a.setBusy(false, "")
	server, err := a.server()
	if err != nil {
		message("CoreTuner", err.Error(), MB_OK|MB_ICONERROR)
		return
	}
	email := strings.TrimSpace(getText(a.controls[idEmail]))
	pass := getText(a.controls[idPassword])
	if email == "" || pass == "" {
		message("CoreTuner", "Preencha e-mail e senha.", MB_OK|MB_ICONERROR)
		return
	}
	var resp AuthResponse
	if err = a.request("POST", server+"/api/auth/login", map[string]string{"email": email, "password": pass}, "", &resp); err != nil {
		message("Falha no login", err.Error(), MB_OK|MB_ICONERROR)
		return
	}
	a.applyAuth(resp)
	a.addHistory("Login realizado", fmt.Sprintf("Empresa: %s", companyName(resp.Company)))
}
func (a *App) register() {
	a.setBusy(true, "Criando empresa...")
	defer a.setBusy(false, "")
	server, err := a.server()
	if err != nil {
		message("CoreTuner", err.Error(), MB_OK|MB_ICONERROR)
		return
	}
	p := map[string]string{"company_name": strings.TrimSpace(getText(a.controls[idCompany])), "responsible_name": strings.TrimSpace(getText(a.controls[idResponsible])), "email": strings.TrimSpace(getText(a.controls[idRegEmail])), "password": getText(a.controls[idRegPassword]), "password_confirmation": getText(a.controls[idRegConfirm])}
	if p["company_name"] == "" || p["responsible_name"] == "" || p["email"] == "" {
		message("CoreTuner", "Preencha todos os campos.", MB_OK|MB_ICONERROR)
		return
	}
	var resp AuthResponse
	if err = a.request("POST", server+"/api/auth/register-company", p, "", &resp); err != nil {
		message("Cadastro não concluído", err.Error(), MB_OK|MB_ICONERROR)
		return
	}
	a.applyAuth(resp)
	a.addHistory("Empresa criada", companyName(resp.Company))
	message("CoreTuner", "Empresa criada com sucesso.", MB_OK|MB_ICONINFORMATION)
}
func (a *App) applyAuth(resp AuthResponse) {
	a.mu.Lock()
	a.token = resp.AccessToken
	a.user = resp.User
	a.company = resp.Company
	a.centralOK = true
	a.statusText = "Conectado ao CoreTuner Central"
	a.mu.Unlock()
	a.hideAuth()
	a.saveSession()
	go a.refreshCentralStatus()
	a.invalidate()
}
func (a *App) logout() {
	a.mu.Lock()
	a.token = ""
	a.centralOK = false
	a.company = nil
	a.page = 0
	a.statusText = "Sessão encerrada"
	a.mu.Unlock()
	os.Remove(sessionPath())
	setText(a.controls[idPassword], "")
	a.showLogin("login")
}

func (a *App) setBusy(v bool, text string) {
	a.mu.Lock()
	a.busy = v
	if text != "" {
		a.statusText = text
	}
	a.mu.Unlock()
	enable(a.controls[idLogin], !v)
	enable(a.controls[idRegister], !v)
	a.invalidate()
}
func (a *App) request(method, endpoint string, payload any, token string, out any) error {
	var body io.Reader
	if payload != nil {
		b, _ := json.Marshal(payload)
		body = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, endpoint, body)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := a.client.Do(req)
	if err != nil {
		return fmt.Errorf("não foi possível conectar ao servidor: %w", err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var ae APIError
		if json.Unmarshal(b, &ae) == nil && ae.Detail != nil {
			return fmt.Errorf("%v", ae.Detail)
		}
		return fmt.Errorf("servidor retornou HTTP %d", resp.StatusCode)
	}
	if out != nil && len(b) > 0 {
		if err = json.Unmarshal(b, out); err != nil {
			return fmt.Errorf("resposta inválida do servidor")
		}
	}
	return nil
}
func (a *App) refreshCentralStatus() {
	a.mu.RLock()
	token, server := a.token, a.serverURL
	a.mu.RUnlock()
	if token == "" {
		return
	}
	var me MeResponse
	if err := a.request("GET", server+"/api/auth/me", nil, token, &me); err != nil {
		a.mu.Lock()
		a.centralOK = false
		a.statusText = "Falha ao verificar a sessão da Central: " + err.Error()
		a.mu.Unlock()
		a.invalidate()
		return
	}
	a.mu.Lock()
	a.centralOK = true
	a.user = AuthUser{ID: me.ID, Name: me.Name, Email: me.Email, Role: me.Role, CompanyID: me.CompanyID}
	a.company = me.Company
	a.statusText = "Conectado ao CoreTuner Central"
	a.mu.Unlock()
	a.saveSession()
	a.invalidate()
}

func (a *App) refreshLocal(full bool) {
	a.mu.RLock()
	base := a.sys
	currentProcesses := append([]ProcessInfo(nil), a.processes...)
	page := a.page
	a.mu.RUnlock()
	var sys SystemInfo
	if full || base.Hostname == "" {
		sys = collectSystemFull(a.client, a.serverURL)
	} else {
		sys = collectSystemDynamic(base)
	}
	procs := currentProcesses
	if full || page == 4 || len(procs) == 0 {
		procs = collectProcesses()
	}
	a.mu.Lock()
	a.sys = sys
	a.processes = procs
	if full && base.Hostname == "" {
		a.statusText = "Diagnóstico local concluído"
	}
	a.mu.Unlock()
	a.invalidate()
}

func collectSystemDynamic(s SystemInfo) SystemInfo {
	s.Updated = time.Now()
	var mem MEMORYSTATUSEX
	mem.DwLength = uint32(unsafe.Sizeof(mem))
	if r, _, _ := procGlobalMemoryStatusEx.Call(uintptr(unsafe.Pointer(&mem))); r != 0 {
		s.Memory = float64(mem.DwMemoryLoad)
		s.TotalRAMGB = float64(mem.UllTotalPhys) / (1 << 30)
		s.UsedRAMGB = float64(mem.UllTotalPhys-mem.UllAvailPhys) / (1 << 30)
	}
	root := utf16("C:\\")
	var free, total, totalFree uint64
	if r, _, _ := procGetDiskFreeSpaceEx.Call(uintptr(unsafe.Pointer(root)), uintptr(unsafe.Pointer(&free)), uintptr(unsafe.Pointer(&total)), uintptr(unsafe.Pointer(&totalFree))); r != 0 {
		s.DiskTotalGB = float64(total) / (1 << 30)
		s.DiskUsedGB = float64(total-free) / (1 << 30)
		if total > 0 {
			s.Disk = s.DiskUsedGB / s.DiskTotalGB * 100
		}
	}
	tick, _, _ := procGetTickCount64.Call()
	s.Uptime = time.Duration(tick) * time.Millisecond
	s.CPU = cpuUsage()
	return s
}

func collectSystemFull(client *http.Client, server string) SystemInfo {
	h, _ := os.Hostname()
	u := os.Getenv("USERNAME")
	s := collectSystemDynamic(SystemInfo{Hostname: h, Username: u, Updated: time.Now()})
	details := nativeSystemDetails()
	for k, v := range details {
		switch k {
		case "Manufacturer":
			s.Manufacturer = v
		case "Model":
			s.Model = v
		case "Serial":
			s.Serial = v
		case "CPU":
			s.CPUName = v
		case "OS":
			s.OS = v
		case "Disk":
			s.DiskName = v
		case "DiskType":
			s.DiskType = v
		}
	}
	s.AudioOK, s.MicOK = nativeAudioStatus()
	start := time.Now()
	conn, err := net.DialTimeout("tcp", "1.1.1.1:443", 3*time.Second)
	if err == nil {
		s.InternetOK = true
		s.LatencyMS = time.Since(start).Milliseconds()
		conn.Close()
	} else {
		s.InternetOK = false
		s.LatencyMS = 0
	}
	return s
}

var cpuMu sync.Mutex
var prevIdle, prevKernel, prevUser uint64

func cpuUsage() float64 {
	cpuMu.Lock()
	defer cpuMu.Unlock()
	var idle, kernel, user FILETIME
	if r, _, _ := procGetSystemTimes.Call(uintptr(unsafe.Pointer(&idle)), uintptr(unsafe.Pointer(&kernel)), uintptr(unsafe.Pointer(&user))); r == 0 {
		return 0
	}
	cv := func(f FILETIME) uint64 { return uint64(f.High)<<32 | uint64(f.Low) }
	i, k, u := cv(idle), cv(kernel), cv(user)
	if prevKernel == 0 {
		prevIdle = i
		prevKernel = k
		prevUser = u
		return 0
	}
	sys := (k - prevKernel) + (u - prevUser)
	id := i - prevIdle
	prevIdle = i
	prevKernel = k
	prevUser = u
	if sys == 0 {
		return 0
	}
	v := 100 * (float64(sys-id) / float64(sys))
	if v < 0 {
		v = 0
	}
	if v > 100 {
		v = 100
	}
	return v
}
func collectProcesses() []ProcessInfo {
	out := nativeProcesses()
	sort.Slice(out, func(i, j int) bool { return out[i].MemoryMB > out[j].MemoryMB })
	if len(out) > 15 {
		out = out[:15]
	}
	return out
}

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
func (a *App) loadHistory() {
	b, err := os.ReadFile(filepath.Join(dataDir(), "history.json"))
	if err == nil {
		json.Unmarshal(b, &a.history)
	}
}
func dataDir() string {
	if p := os.Getenv("LOCALAPPDATA"); p != "" {
		return filepath.Join(p, "CoreTuner")
	}
	home, _ := os.UserHomeDir()
	if home != "" {
		return filepath.Join(home, "AppData", "Local", "CoreTuner")
	}
	return filepath.Join(os.TempDir(), "CoreTuner")
}

func sessionPath() string { return filepath.Join(dataDir(), "session.json") }
func (a *App) saveSession() {
	os.MkdirAll(dataDir(), 0755)
	a.mu.RLock()
	s := Session{ServerURL: a.serverURL, AccessToken: a.token, User: a.user, Company: a.company, SavedAt: time.Now()}
	a.mu.RUnlock()
	b, _ := json.MarshalIndent(s, "", "  ")
	_ = os.WriteFile(sessionPath(), b, 0600)
}
func (a *App) loadSession() {
	b, err := os.ReadFile(sessionPath())
	if err != nil {
		return
	}
	var s Session
	if json.Unmarshal(b, &s) != nil || s.AccessToken == "" {
		return
	}
	a.serverURL = s.ServerURL
	a.token = s.AccessToken
	a.user = s.User
	a.company = s.Company
	a.centralOK = false
	setText(a.controls[idServer], a.serverURL)
	a.hideAuth()
	go a.refreshCentralStatus()
}
func loadServerURL() string {
	b, err := os.ReadFile(filepath.Join(dataDir(), "server-url.txt"))
	if err == nil && strings.TrimSpace(string(b)) != "" {
		return strings.TrimRight(strings.TrimSpace(string(b)), "/")
	}
	if v := strings.TrimSpace(os.Getenv("CORETUNER_SERVER_URL")); v != "" {
		return strings.TrimRight(v, "/")
	}
	return defaultServerURL
}
func saveServerURL(v string) {
	os.MkdirAll(dataDir(), 0755)
	_ = os.WriteFile(filepath.Join(dataDir(), "server-url.txt"), []byte(v), 0600)
}
func companyName(c *Company) string {
	if c == nil || strings.TrimSpace(c.Name) == "" {
		return "Empresa"
	}
	return c.Name
}
