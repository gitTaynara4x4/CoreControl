//go:build windows

package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"
	"unsafe"
)

const appVersion = "0.4.9"

var defaultServerURL = "http://127.0.0.1:8002"

const (
	WS_OVERLAPPEDWINDOW  = 0x00CF0000
	WS_VISIBLE           = 0x10000000
	WS_CHILD             = 0x40000000
	WS_BORDER            = 0x00800000
	WS_VSCROLL           = 0x00200000
	WS_TABSTOP           = 0x00010000
	ES_AUTOHSCROLL       = 0x0080
	ES_PASSWORD          = 0x0020
	BS_PUSHBUTTON        = 0x00000000
	BS_DEFPUSHBUTTON     = 0x00000001
	LBS_NOTIFY           = 0x0001
	LBS_NOINTEGRALHEIGHT = 0x0100
	SW_SHOW              = 5
	SW_HIDE              = 0
	SW_SHOWNORMAL        = 1
	CW_USEDEFAULT        = ^uintptr(0x7fffffff)
	WM_DESTROY           = 0x0002
	WM_COMMAND           = 0x0111
	WM_SETFONT           = 0x0030
	WM_CLOSE             = 0x0010
	WM_APP               = 0x8000
	LB_ADDSTRING         = 0x0180
	LB_RESETCONTENT      = 0x0184
	MB_OK                = 0x00000000
	MB_ICONINFORMATION   = 0x00000040
	MB_ICONERROR         = 0x00000010
	MB_YESNO             = 0x00000004
	MB_ICONQUESTION      = 0x00000020
	IDYES                = 6
	COLOR_WINDOW         = 5
	DEFAULT_GUI_FONT     = 17
)

const (
	idServer           = 101
	idLoginEmail       = 102
	idLoginPassword    = 103
	idLoginButton      = 104
	idShowRegister     = 105
	idForgotPassword   = 106
	idRegisterCompany  = 110
	idRegisterName     = 111
	idRegisterEmail    = 112
	idRegisterPassword = 113
	idRegisterConfirm  = 114
	idRegisterButton   = 115
	idShowLogin        = 116
	idDeviceName       = 201
	idSector           = 202
	idLocation         = 203
	idInstall          = 204
	idRefresh          = 205
	idOpenCentral      = 206
	idLogout           = 207
	idDevicesList      = 208
)

var (
	user32                  = syscall.NewLazyDLL("user32.dll")
	kernel32                = syscall.NewLazyDLL("kernel32.dll")
	gdi32                   = syscall.NewLazyDLL("gdi32.dll")
	shell32                 = syscall.NewLazyDLL("shell32.dll")
	procRegisterClassEx     = user32.NewProc("RegisterClassExW")
	procCreateWindowEx      = user32.NewProc("CreateWindowExW")
	procDefWindowProc       = user32.NewProc("DefWindowProcW")
	procShowWindow          = user32.NewProc("ShowWindow")
	procUpdateWindow        = user32.NewProc("UpdateWindow")
	procGetMessage          = user32.NewProc("GetMessageW")
	procTranslateMessage    = user32.NewProc("TranslateMessage")
	procDispatchMessage     = user32.NewProc("DispatchMessageW")
	procPostQuitMessage     = user32.NewProc("PostQuitMessage")
	procSendMessage         = user32.NewProc("SendMessageW")
	procSetWindowText       = user32.NewProc("SetWindowTextW")
	procGetWindowText       = user32.NewProc("GetWindowTextW")
	procGetWindowTextLength = user32.NewProc("GetWindowTextLengthW")
	procEnableWindow        = user32.NewProc("EnableWindow")
	procSetFocus            = user32.NewProc("SetFocus")
	procMessageBox          = user32.NewProc("MessageBoxW")
	procDestroyWindow       = user32.NewProc("DestroyWindow")
	procGetStockObject      = gdi32.NewProc("GetStockObject")
	procCreateFont          = gdi32.NewProc("CreateFontW")
	procGetModuleHandle     = kernel32.NewProc("GetModuleHandleW")
	procShellExecute        = shell32.NewProc("ShellExecuteW")
	procIsUserAnAdmin       = shell32.NewProc("IsUserAnAdmin")
)

type WNDCLASSEX struct {
	CbSize        uint32
	Style         uint32
	LpfnWndProc   uintptr
	CbClsExtra    int32
	CbWndExtra    int32
	HInstance     syscall.Handle
	HIcon         syscall.Handle
	HCursor       syscall.Handle
	HbrBackground syscall.Handle
	LpszMenuName  *uint16
	LpszClassName *uint16
	HIconSm       syscall.Handle
}

type POINT struct{ X, Y int32 }
type MSG struct {
	HWnd     syscall.Handle
	Message  uint32
	WParam   uintptr
	LParam   uintptr
	Time     uint32
	Pt       POINT
	LPrivate uint32
}

type AuthUser struct {
	ID        int    `json:"id"`
	Name      string `json:"name"`
	Email     string `json:"email"`
	Role      string `json:"role"`
	CompanyID *int   `json:"company_id"`
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

type Telemetry struct {
	CPUPercent    *float64 `json:"cpu_percent"`
	MemoryPercent *float64 `json:"memory_percent"`
	DiskPercent   *float64 `json:"disk_percent"`
	TemperatureC  *float64 `json:"temperature_c"`
}

type Device struct {
	ID          int        `json:"id"`
	Name        string     `json:"name"`
	Hostname    string     `json:"hostname"`
	Online      bool       `json:"online"`
	HealthScore int        `json:"health_score"`
	AlertsOpen  int        `json:"alerts_open"`
	Telemetry   *Telemetry `json:"telemetry"`
}

type Machine struct {
	DeviceUID    string `json:"device_uid"`
	Hostname     string `json:"hostname"`
	Manufacturer string `json:"manufacturer"`
	Model        string `json:"model"`
	SerialNumber string `json:"serial_number"`
	OSName       string `json:"os_name"`
	OSVersion    string `json:"os_version"`
}

type InstallResponse struct {
	DeviceID      int              `json:"device_id"`
	CompanyID     int              `json:"company_id"`
	CompanyName   string           `json:"company_name"`
	AgentSecret   string           `json:"agent_secret"`
	RemoteAgent   *RemoteAgentInfo `json:"remote_agent"`
	RemoteWarning string           `json:"remote_warning"`
}

type ComponentInfo struct {
	Filename string `json:"filename"`
	URL      string `json:"url"`
	SHA256   string `json:"sha256"`
	Size     int64  `json:"size"`
}

type RemoteAgentInfo struct {
	Filename      string `json:"filename"`
	URL           string `json:"url"`
	SHA256        string `json:"sha256"`
	Size          int64  `json:"size"`
	MeshGroupID   string `json:"mesh_group_id"`
	MeshGroupHex  string `json:"mesh_group_hex"`
	MeshGroupName string `json:"mesh_group_name"`
	ServerURL     string `json:"server_url"`
}

type RemoteStatusResponse struct {
	MeshConnected  bool   `json:"mesh_connected"`
	MeshNodeID     string `json:"mesh_node_id"`
	ServiceRunning bool   `json:"service_running"`
	Available      bool   `json:"available"`
	Warning        string `json:"warning"`
}

type ComponentManifest struct {
	Version string                   `json:"version"`
	Files   map[string]ComponentInfo `json:"files"`
}

type APIError struct {
	Detail any `json:"detail"`
}

type App struct {
	hwnd           syscall.Handle
	font           uintptr
	titleFont      uintptr
	sectionFont    uintptr
	smallFont      uintptr
	buttonFont     uintptr
	controls       map[int]syscall.Handle
	loginGroup     []syscall.Handle
	registerGroup  []syscall.Handle
	dashboardGroup []syscall.Handle
	client         *http.Client
	serverURL      string
	token          string
	user           AuthUser
	company        *Company
	devices        []Device
	status         syscall.Handle
	title          syscall.Handle
	subtitle       syscall.Handle
	installNotice  string
}

var app *App

func utf16(s string) *uint16 { return syscall.StringToUTF16Ptr(s) }

func loword(v uintptr) int { return int(v & 0xffff) }

func createControl(class, text string, style uint32, x, y, w, height int32, parent syscall.Handle, id int) syscall.Handle {
	hwnd, _, _ := procCreateWindowEx.Call(
		0,
		uintptr(unsafe.Pointer(utf16(class))),
		uintptr(unsafe.Pointer(utf16(text))),
		uintptr(style),
		uintptr(x), uintptr(y), uintptr(w), uintptr(height),
		uintptr(parent), uintptr(id), 0, 0,
	)
	if hwnd != 0 && app != nil && app.font != 0 {
		procSendMessage.Call(hwnd, WM_SETFONT, app.font, 1)
	}
	return syscall.Handle(hwnd)
}

func setText(h syscall.Handle, text string) {
	procSetWindowText.Call(uintptr(h), uintptr(unsafe.Pointer(utf16(text))))
}
func getText(h syscall.Handle) string {
	ln, _, _ := procGetWindowTextLength.Call(uintptr(h))
	buf := make([]uint16, ln+1)
	procGetWindowText.Call(uintptr(h), uintptr(unsafe.Pointer(&buf[0])), ln+1)
	return syscall.UTF16ToString(buf)
}
func show(h syscall.Handle, visible bool) {
	if visible {
		procShowWindow.Call(uintptr(h), SW_SHOW)
	} else {
		procShowWindow.Call(uintptr(h), SW_HIDE)
	}
}
func enable(h syscall.Handle, enabled bool) {
	v := uintptr(0)
	if enabled {
		v = 1
	}
	procEnableWindow.Call(uintptr(h), v)
}
func message(title, text string, flags uintptr) int {
	r, _, _ := procMessageBox.Call(uintptr(app.hwnd), uintptr(unsafe.Pointer(utf16(text))), uintptr(unsafe.Pointer(utf16(title))), flags)
	return int(r)
}

func main() {
	// Instalação por usuário: não solicita administrador, não cria tarefa como SYSTEM
	// e não desativa nenhuma proteção do Windows.
	runtime.LockOSThread()
	runGUI()
}

func runGUI() {
	hinst, _, _ := procGetModuleHandle.Call(0)
	className := utf16("CoreTunerSetupWindow")
	wc := WNDCLASSEX{CbSize: uint32(unsafe.Sizeof(WNDCLASSEX{})), LpfnWndProc: syscall.NewCallback(wndProc), HInstance: syscall.Handle(hinst), HbrBackground: syscall.Handle(COLOR_WINDOW + 1), LpszClassName: className}
	procRegisterClassEx.Call(uintptr(unsafe.Pointer(&wc)))
	h, _, _ := procCreateWindowEx.Call(0, uintptr(unsafe.Pointer(className)), uintptr(unsafe.Pointer(utf16("CoreTuner — Instalação segura"))), WS_OVERLAPPEDWINDOW, 180, 55, 620, 780, 0, 0, hinst, 0)
	if h == 0 {
		return
	}
	font, _, _ := procGetStockObject.Call(DEFAULT_GUI_FONT)
	app = &App{hwnd: syscall.Handle(h), font: font, controls: map[int]syscall.Handle{}, client: &http.Client{Timeout: 25 * time.Second}, serverURL: loadServerURL()}
	app.createFonts()
	buildUI()
	procShowWindow.Call(h, SW_SHOW)
	procUpdateWindow.Call(h)
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
	case WM_COMMAND:
		if app != nil {
			app.handleCommand(loword(wParam))
		}
		return 0
	case WM_CLOSE:
		procDestroyWindow.Call(hwnd)
		return 0
	case WM_DESTROY:
		procPostQuitMessage.Call(0)
		return 0
	}
	r, _, _ := procDefWindowProc.Call(hwnd, uintptr(msg), wParam, lParam)
	return r
}

func (a *App) createFonts() {
	create := func(size, weight int) uintptr {
		font, _, _ := procCreateFont.Call(
			uintptr(-size), 0, 0, 0, uintptr(weight), 0, 0, 0,
			1, 0, 0, 5, 0,
			uintptr(unsafe.Pointer(utf16("Segoe UI"))),
		)
		return font
	}
	a.font = create(16, 400)
	a.titleFont = create(31, 600)
	a.sectionFont = create(20, 600)
	a.smallFont = create(14, 400)
	a.buttonFont = create(16, 600)
}

func applyFont(h syscall.Handle, font uintptr) {
	if h != 0 && font != 0 {
		procSendMessage.Call(uintptr(h), WM_SETFONT, font, 1)
	}
}

func (a *App) add(id int, h syscall.Handle, group *[]syscall.Handle) syscall.Handle {
	a.controls[id] = h
	if group != nil {
		*group = append(*group, h)
	}
	return h
}

func buildUI() {
	a := app
	a.title = createControl("STATIC", "CoreTuner", WS_CHILD|WS_VISIBLE, 60, 24, 500, 42, a.hwnd, 0)
	applyFont(a.title, a.titleFont)
	a.subtitle = createControl("STATIC", "Instalação segura do computador", WS_CHILD|WS_VISIBLE, 60, 70, 500, 25, a.hwnd, 0)
	applyFont(a.subtitle, a.smallFont)
	serverLabel := createControl("STATIC", "Servidor CoreTuner", WS_CHILD|WS_VISIBLE, 60, 112, 180, 20, a.hwnd, 0)
	applyFont(serverLabel, a.smallFont)
	a.add(idServer, createControl("EDIT", a.serverURL, WS_CHILD|WS_VISIBLE|WS_BORDER|WS_TABSTOP|ES_AUTOHSCROLL, 60, 138, 500, 34, a.hwnd, idServer), nil)

	// Login
	l1 := createControl("STATIC", "Entrar na empresa", WS_CHILD|WS_VISIBLE, 60, 210, 500, 32, a.hwnd, 0)
	applyFont(l1, a.sectionFont)
	a.loginGroup = append(a.loginGroup, l1)
	l2 := createControl("STATIC", "E-mail", WS_CHILD|WS_VISIBLE, 60, 264, 120, 20, a.hwnd, 0)
	applyFont(l2, a.smallFont)
	a.loginGroup = append(a.loginGroup, l2)
	a.add(idLoginEmail, createControl("EDIT", "", WS_CHILD|WS_VISIBLE|WS_BORDER|WS_TABSTOP|ES_AUTOHSCROLL, 60, 290, 500, 36, a.hwnd, idLoginEmail), &a.loginGroup)
	l3 := createControl("STATIC", "Senha", WS_CHILD|WS_VISIBLE, 60, 344, 120, 20, a.hwnd, 0)
	applyFont(l3, a.smallFont)
	a.loginGroup = append(a.loginGroup, l3)
	a.add(idLoginPassword, createControl("EDIT", "", WS_CHILD|WS_VISIBLE|WS_BORDER|WS_TABSTOP|ES_PASSWORD|ES_AUTOHSCROLL, 60, 370, 500, 36, a.hwnd, idLoginPassword), &a.loginGroup)
	a.add(idLoginButton, createControl("BUTTON", "Entrar", WS_CHILD|WS_VISIBLE|WS_TABSTOP|BS_DEFPUSHBUTTON, 60, 432, 240, 44, a.hwnd, idLoginButton), &a.loginGroup)
	a.add(idShowRegister, createControl("BUTTON", "Criar uma empresa", WS_CHILD|WS_VISIBLE|WS_TABSTOP|BS_PUSHBUTTON, 320, 432, 240, 44, a.hwnd, idShowRegister), &a.loginGroup)
	applyFont(a.controls[idLoginButton], a.buttonFont)
	applyFont(a.controls[idShowRegister], a.buttonFont)
	a.add(idForgotPassword, createControl("BUTTON", "Esqueci minha senha", WS_CHILD|WS_VISIBLE|WS_TABSTOP|BS_PUSHBUTTON, 60, 492, 500, 38, a.hwnd, idForgotPassword), &a.loginGroup)
	loginNote := createControl("STATIC", "Sua conta e os dados técnicos são enviados por conexão segura.", WS_CHILD|WS_VISIBLE, 60, 552, 500, 30, a.hwnd, 0)
	applyFont(loginNote, a.smallFont)
	a.loginGroup = append(a.loginGroup, loginNote)

	// Register
	r1 := createControl("STATIC", "Criar nova empresa", WS_CHILD, 60, 188, 500, 32, a.hwnd, 0)
	applyFont(r1, a.sectionFont)
	a.registerGroup = append(a.registerGroup, r1)
	fields := []struct {
		id    int
		label string
		y     int32
		pass  bool
	}{
		{idRegisterCompany, "Nome da empresa", 236, false}, {idRegisterName, "Nome do responsável", 306, false}, {idRegisterEmail, "E-mail", 376, false}, {idRegisterPassword, "Senha (mínimo 10 caracteres)", 446, true}, {idRegisterConfirm, "Confirmar senha", 516, true},
	}
	for _, f := range fields {
		lab := createControl("STATIC", f.label, WS_CHILD, 60, f.y, 300, 20, a.hwnd, 0)
		applyFont(lab, a.smallFont)
		a.registerGroup = append(a.registerGroup, lab)
		style := uint32(WS_CHILD | WS_BORDER | WS_TABSTOP | ES_AUTOHSCROLL)
		if f.pass {
			style |= ES_PASSWORD
		}
		a.add(f.id, createControl("EDIT", "", style, 60, f.y+24, 500, 34, a.hwnd, f.id), &a.registerGroup)
	}
	a.add(idRegisterButton, createControl("BUTTON", "Criar empresa e entrar", WS_CHILD|WS_TABSTOP|BS_DEFPUSHBUTTON, 60, 598, 280, 44, a.hwnd, idRegisterButton), &a.registerGroup)
	a.add(idShowLogin, createControl("BUTTON", "Já tenho uma conta", WS_CHILD|WS_TABSTOP|BS_PUSHBUTTON, 360, 598, 200, 44, a.hwnd, idShowLogin), &a.registerGroup)
	applyFont(a.controls[idRegisterButton], a.buttonFont)
	applyFont(a.controls[idShowLogin], a.buttonFont)

	// Assistente de instalação. Depois da instalação, o Setup abre o painel e fecha.
	statusCard := createControl("STATIC", "", WS_CHILD|WS_BORDER, 60, 188, 500, 58, a.hwnd, 0)
	a.dashboardGroup = append(a.dashboardGroup, statusCard)
	a.status = createControl("STATIC", "Aguardando login", WS_CHILD, 78, 207, 464, 24, a.hwnd, 0)
	applyFont(a.status, a.smallFont)
	a.dashboardGroup = append(a.dashboardGroup, a.status)
	steps := createControl("STATIC", "1  Conta     2  Computador     3  Instalação     4  Concluído", WS_CHILD, 60, 266, 500, 26, a.hwnd, 0)
	applyFont(steps, a.smallFont)
	a.dashboardGroup = append(a.dashboardGroup, steps)
	section := createControl("STATIC", "Identifique este computador", WS_CHILD, 60, 310, 500, 30, a.hwnd, 0)
	applyFont(section, a.sectionFont)
	a.dashboardGroup = append(a.dashboardGroup, section)
	help := createControl("STATIC", "Preencha as informações para instalar o monitoramento seguro.", WS_CHILD, 60, 344, 500, 24, a.hwnd, 0)
	applyFont(help, a.smallFont)
	a.dashboardGroup = append(a.dashboardGroup, help)
	labs := []struct {
		text string
		x, y int32
	}{
		{"Nome deste computador", 60, 388}, {"Setor", 60, 466}, {"Local / unidade", 320, 466},
	}
	for _, v := range labs {
		h := createControl("STATIC", v.text, WS_CHILD, v.x, v.y, 240, 20, a.hwnd, 0)
		applyFont(h, a.smallFont)
		a.dashboardGroup = append(a.dashboardGroup, h)
	}
	host, _ := os.Hostname()
	a.add(idDeviceName, createControl("EDIT", host, WS_CHILD|WS_BORDER|WS_TABSTOP|ES_AUTOHSCROLL, 60, 414, 500, 36, a.hwnd, idDeviceName), &a.dashboardGroup)
	a.add(idSector, createControl("EDIT", "", WS_CHILD|WS_BORDER|WS_TABSTOP|ES_AUTOHSCROLL, 60, 492, 240, 36, a.hwnd, idSector), &a.dashboardGroup)
	a.add(idLocation, createControl("EDIT", "", WS_CHILD|WS_BORDER|WS_TABSTOP|ES_AUTOHSCROLL, 320, 492, 240, 36, a.hwnd, idLocation), &a.dashboardGroup)
	a.add(idInstall, createControl("BUTTON", "Instalar e continuar", WS_CHILD|WS_TABSTOP|BS_DEFPUSHBUTTON, 60, 554, 500, 48, a.hwnd, idInstall), &a.dashboardGroup)
	applyFont(a.controls[idInstall], a.buttonFont)
	a.add(idOpenCentral, createControl("BUTTON", "Abrir painel web", WS_CHILD|WS_TABSTOP, 60, 622, 240, 40, a.hwnd, idOpenCentral), &a.dashboardGroup)
	a.add(idLogout, createControl("BUTTON", "Trocar conta", WS_CHILD|WS_TABSTOP, 320, 622, 240, 40, a.hwnd, idLogout), &a.dashboardGroup)
	footer := createControl("STATIC", "Instalação segura e verificada. O CoreTuner não acessa documentos, conversas ou senhas.", WS_CHILD, 60, 690, 500, 34, a.hwnd, 0)
	applyFont(footer, a.smallFont)
	a.dashboardGroup = append(a.dashboardGroup, footer)

	a.showMode("login")
}

func (a *App) showMode(mode string) {
	for _, h := range a.loginGroup {
		show(h, mode == "login")
	}
	for _, h := range a.registerGroup {
		show(h, mode == "register")
	}
	for _, h := range a.dashboardGroup {
		show(h, mode == "dashboard")
	}
	if mode == "dashboard" {
		setText(a.title, "CoreTuner")
		setText(a.subtitle, "Instalação segura do computador — "+companyName(a.company))
	} else {
		setText(a.title, "CoreTuner")
		setText(a.subtitle, "Instalação segura do computador")
	}
}

func companyName(c *Company) string {
	if c == nil {
		return "Empresa"
	}
	return c.Name
}

func (a *App) handleCommand(id int) {
	switch id {
	case idShowRegister:
		a.showMode("register")
	case idShowLogin:
		a.showMode("login")
	case idLoginButton:
		a.login()
	case idForgotPassword:
		a.openPasswordRecovery()
	case idRegisterButton:
		a.register()
	case idRefresh:
		a.refreshDevices()
	case idInstall:
		a.installCurrent()
	case idOpenCentral:
		a.openCentral()
	case idLogout:
		a.logout()
	}
}

func (a *App) server() (string, error) {
	raw := strings.TrimRight(strings.TrimSpace(getText(a.controls[idServer])), "/")
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return "", errors.New("Informe um endereço válido do CoreTuner Central")
	}
	host := strings.ToLower(strings.Split(parsed.Host, ":")[0])
	localHTTP := parsed.Scheme == "http" && (host == "127.0.0.1" || host == "localhost" || host == "::1")
	if parsed.Scheme != "https" && !localHTTP {
		return "", errors.New("HTTPS é obrigatório fora do teste local")
	}
	a.serverURL = raw
	saveServerURL(raw)
	return raw, nil
}

func (a *App) login() {
	server, err := a.server()
	if err != nil {
		message("CoreTuner", err.Error(), MB_OK|MB_ICONERROR)
		return
	}
	email := strings.TrimSpace(getText(a.controls[idLoginEmail]))
	password := getText(a.controls[idLoginPassword])
	if email == "" || password == "" {
		message("CoreTuner", "Preencha e-mail e senha.", MB_OK|MB_ICONERROR)
		return
	}
	enable(a.controls[idLoginButton], false)
	setText(a.subtitle, "Conectando ao CoreTuner Central...")
	var resp AuthResponse
	err = a.request("POST", server+"/api/auth/login", map[string]string{"email": email, "password": password}, "", &resp)
	enable(a.controls[idLoginButton], true)
	if err != nil {
		setText(a.subtitle, "Não foi possível entrar.")
		message("Falha no login", err.Error(), MB_OK|MB_ICONERROR)
		return
	}
	a.applyAuth(resp)
}

func (a *App) register() {
	server, err := a.server()
	if err != nil {
		message("CoreTuner", err.Error(), MB_OK|MB_ICONERROR)
		return
	}
	payload := map[string]string{
		"company_name":          strings.TrimSpace(getText(a.controls[idRegisterCompany])),
		"responsible_name":      strings.TrimSpace(getText(a.controls[idRegisterName])),
		"email":                 strings.TrimSpace(getText(a.controls[idRegisterEmail])),
		"password":              getText(a.controls[idRegisterPassword]),
		"password_confirmation": getText(a.controls[idRegisterConfirm]),
	}
	if payload["company_name"] == "" || payload["responsible_name"] == "" || payload["email"] == "" {
		message("CoreTuner", "Preencha todos os campos.", MB_OK|MB_ICONERROR)
		return
	}
	enable(a.controls[idRegisterButton], false)
	setText(a.subtitle, "Criando empresa com segurança...")
	var resp AuthResponse
	err = a.request("POST", server+"/api/auth/register-company", payload, "", &resp)
	enable(a.controls[idRegisterButton], true)
	if err != nil {
		setText(a.subtitle, "Não foi possível criar a empresa.")
		message("Cadastro não concluído", err.Error(), MB_OK|MB_ICONERROR)
		return
	}
	a.applyAuth(resp)
	message("CoreTuner", "Empresa criada com sucesso. Agora você pode instalar este computador.", MB_OK|MB_ICONINFORMATION)
}

func (a *App) applyAuth(resp AuthResponse) {
	a.token = resp.AccessToken
	a.user = resp.User
	a.company = resp.Company
	a.showMode("dashboard")
	setText(a.status, fmt.Sprintf("Conectado como %s — %s", a.user.Name, companyName(a.company)))
	a.refreshDevices()
}

func (a *App) logout() {
	a.token = ""
	a.user = AuthUser{}
	a.company = nil
	a.devices = nil
	a.showMode("login")
	setText(a.controls[idLoginPassword], "")
}

func (a *App) refreshDevices() {
	if a.token == "" {
		return
	}
	setText(a.status, "Atualizando computadores...")
	var devices []Device
	err := a.request("GET", a.serverURL+"/api/devices", nil, a.token, &devices)
	if err != nil {
		setText(a.status, "Falha ao atualizar: "+err.Error())
		return
	}
	a.devices = devices
	list := a.controls[idDevicesList]
	procSendMessage.Call(uintptr(list), LB_RESETCONTENT, 0, 0)
	online := 0
	for _, d := range devices {
		if d.Online {
			online++
		}
		status := "OFFLINE"
		if d.Online {
			status = "ONLINE"
		}
		ram := "—"
		disk := "—"
		temp := ""
		if d.Telemetry != nil {
			if d.Telemetry.MemoryPercent != nil {
				ram = fmt.Sprintf("%.0f%%", *d.Telemetry.MemoryPercent)
			}
			if d.Telemetry.DiskPercent != nil {
				disk = fmt.Sprintf("%.0f%%", *d.Telemetry.DiskPercent)
			}
			if d.Telemetry.TemperatureC != nil {
				temp = fmt.Sprintf(" | Temp %.0f°C", *d.Telemetry.TemperatureC)
			}
		}
		line := fmt.Sprintf("[%s] %-28s | Saúde %3d | RAM %s | Disco %s%s | Alertas %d", status, d.Name, d.HealthScore, ram, disk, temp, d.AlertsOpen)
		procSendMessage.Call(uintptr(list), LB_ADDSTRING, 0, uintptr(unsafe.Pointer(utf16(line))))
	}
	setText(a.status, fmt.Sprintf("%s — %d computadores, %d online, %d offline", companyName(a.company), len(devices), online, len(devices)-online))
}

func (a *App) installCurrent() {
	if a.token == "" {
		return
	}
	name := strings.TrimSpace(getText(a.controls[idDeviceName]))
	if name == "" {
		message("CoreTuner", "Informe o nome deste computador.", MB_OK|MB_ICONERROR)
		return
	}
	setText(a.status, "Coletando identificação do computador...")
	enable(a.controls[idInstall], false)
	machine, err := collectMachine()
	if err != nil {
		enable(a.controls[idInstall], true)
		message("Diagnóstico", err.Error(), MB_OK|MB_ICONERROR)
		return
	}
	companyID := 0
	if a.user.CompanyID != nil {
		companyID = *a.user.CompanyID
	} else if a.company != nil {
		companyID = a.company.ID
	}
	payload := map[string]any{
		"company_id": companyID, "device_uid": machine.DeviceUID, "name": name, "hostname": machine.Hostname,
		"sector": strings.TrimSpace(getText(a.controls[idSector])), "location": strings.TrimSpace(getText(a.controls[idLocation])),
		"manufacturer": machine.Manufacturer, "model": machine.Model, "serial_number": machine.SerialNumber,
		"os_name": machine.OSName, "os_version": machine.OSVersion, "agent_version": appVersion,
	}
	installRemote := message(
		"Acesso remoto CoreTuner",
		"Deseja instalar também o acesso remoto para suporte técnico?\n\nO CoreTuner criará e vinculará automaticamente este computador à empresa correta no servidor remoto. Se existir um Mesh Agent antigo ou ligado a outro servidor, ele será substituído após a autorização do Windows.\n\nTécnicos autorizados poderão controlar a tela somente durante um atendimento.",
		MB_YESNO|MB_ICONQUESTION,
	) == IDYES
	payload["install_remote"] = installRemote

	setText(a.status, "Cadastrando o computador na empresa...")
	var resp InstallResponse
	err = a.request("POST", a.serverURL+"/api/devices/install", payload, a.token, &resp)
	if err == nil {
		err = a.installFiles(machine, name, getText(a.controls[idSector]), getText(a.controls[idLocation]), resp, installRemote)
	}
	enable(a.controls[idInstall], true)
	if err != nil {
		setText(a.status, "Instalação não concluída.")
		message("Instalação não concluída", err.Error(), MB_OK|MB_ICONERROR)
		return
	}
	setText(a.status, "CoreTuner instalado e conectado com sucesso.")
	notice := strings.TrimSpace(a.installNotice)
	if notice == "" {
		notice = "Acesso remoto não foi solicitado nesta instalação."
	}
	message("CoreTuner instalado", fmt.Sprintf("Empresa: %s\nComputador: %s\n\nO agente de diagnóstico já está enviando informações técnicas.\n%s", resp.CompanyName, name, notice), MB_OK|MB_ICONINFORMATION)
	procDestroyWindow.Call(uintptr(a.hwnd))
}

func (a *App) installFiles(machine Machine, name, sector, location string, resp InstallResponse, installRemote bool) error {
	a.installNotice = ""
	localAppData := os.Getenv("LOCALAPPDATA")
	if localAppData == "" {
		home, _ := os.UserHomeDir()
		localAppData = filepath.Join(home, "AppData", "Local")
	}
	installDir := filepath.Join(localAppData, "Programs", "CoreTuner")
	agentDataDir := filepath.Join(localAppData, "CoreTuner", "Agent")
	userDataDir := configDir()
	if err := os.MkdirAll(installDir, 0755); err != nil {
		return fmt.Errorf("não foi possível criar a pasta de instalação: %w", err)
	}
	if err := os.MkdirAll(agentDataDir, 0755); err != nil {
		return fmt.Errorf("não foi possível criar a pasta do agente: %w", err)
	}
	if err := os.MkdirAll(userDataDir, 0755); err != nil {
		return fmt.Errorf("não foi possível criar a pasta do aplicativo: %w", err)
	}

	setText(a.status, "Baixando componentes oficiais do CoreTuner...")
	var manifest ComponentManifest
	if err := a.request("GET", a.serverURL+"/api/desktop/manifest", nil, a.token, &manifest); err != nil {
		return fmt.Errorf("não foi possível obter os componentes oficiais: %w", err)
	}
	coreInfo, ok := manifest.Files["CoreTuner.exe"]
	if !ok {
		return errors.New("o servidor não forneceu o CoreTuner.exe")
	}
	agentInfo, ok := manifest.Files["CoreTunerAgent.exe"]
	if !ok {
		return errors.New("o servidor não forneceu o CoreTunerAgent.exe")
	}

	coreBytes, err := a.downloadComponent(coreInfo)
	if err != nil {
		return fmt.Errorf("falha ao baixar CoreTuner.exe: %w", err)
	}
	agentBytes, err := a.downloadComponent(agentInfo)
	if err != nil {
		return fmt.Errorf("falha ao baixar CoreTunerAgent.exe: %w", err)
	}

	agentPath := filepath.Join(installDir, "CoreTunerAgent.exe")
	corePath := filepath.Join(installDir, "CoreTuner.exe")
	// Encerra somente agentes antigos instalados pelo CoreTuner, sem PowerShell e sem janela.
	stopExistingAgent(agentPath)
	// Remove a inicialização anterior antes de substituir os componentes.
	_ = hiddenCommand("reg.exe", "delete", `HKCU\Software\Microsoft\Windows\CurrentVersion\Run`, "/v", "CoreTunerAgent", "/f").Run()
	time.Sleep(300 * time.Millisecond)
	if err := writeAtomic(agentPath, agentBytes, 0755); err != nil {
		return fmt.Errorf("não foi possível instalar o agente: %w", err)
	}
	if err := writeAtomic(corePath, coreBytes, 0755); err != nil {
		return fmt.Errorf("não foi possível instalar o aplicativo completo: %w", err)
	}

	cfg := map[string]any{
		"server_url":          a.serverURL,
		"agent_secret":        resp.AgentSecret,
		"device_id":           resp.DeviceID,
		"interval_seconds":    30,
		"allow_insecure_http": strings.HasPrefix(strings.ToLower(a.serverURL), "http://127.0.0.1") || strings.HasPrefix(strings.ToLower(a.serverURL), "http://localhost"),
		"name":                name,
		"sector":              strings.TrimSpace(sector),
		"location":            strings.TrimSpace(location),
	}
	raw, _ := json.MarshalIndent(cfg, "", "  ")
	configPath := filepath.Join(agentDataDir, "agent-config.json")
	if err := writeAtomic(configPath, raw, 0600); err != nil {
		return fmt.Errorf("não foi possível salvar a configuração: %w", err)
	}
	serverPath := filepath.Join(userDataDir, "server-url.txt")
	if err := writeAtomic(serverPath, []byte(a.serverURL), 0644); err != nil {
		return fmt.Errorf("não foi possível salvar o endereço do servidor: %w", err)
	}
	session := map[string]any{"server_url": a.serverURL, "access_token": a.token, "user": a.user, "company": a.company, "saved_at": time.Now()}
	sessionRaw, _ := json.MarshalIndent(session, "", "  ")
	if err := writeAtomic(filepath.Join(userDataDir, "session.json"), sessionRaw, 0600); err != nil {
		return fmt.Errorf("não foi possível salvar a sessão inicial: %w", err)
	}

	runCommand := fmt.Sprintf(`"%s" -config "%s"`, agentPath, configPath)
	if out, err := hiddenCommand("reg.exe", "add", `HKCU\Software\Microsoft\Windows\CurrentVersion\Run`, "/v", "CoreTunerAgent", "/t", "REG_SZ", "/d", runCommand, "/f").CombinedOutput(); err != nil {
		return fmt.Errorf("não foi possível configurar a inicialização do agente: %s", strings.TrimSpace(string(out)))
	}
	cmd := hiddenCommand(agentPath, "-config", configPath)
	_ = cmd.Start()

	if installRemote {
		setText(a.status, "Preparando acesso remoto automático...")
		if resp.RemoteAgent == nil {
			reason := strings.TrimSpace(resp.RemoteWarning)
			if reason == "" {
				reason = "o servidor não forneceu o agente remoto da empresa"
			}
			a.installNotice = "O CoreTuner foi instalado, mas o acesso remoto não foi concluído: " + reason
		} else if err := a.installRemoteAgent(*resp.RemoteAgent, resp.DeviceID); err != nil {
			a.installNotice = "O CoreTuner foi instalado, mas o acesso remoto não foi concluído: " + err.Error()
		} else {
			a.installNotice = "Acesso remoto instalado, vinculado à empresa correta e confirmado online."
		}
	}

	if err := exec.Command(corePath).Start(); err != nil {
		return fmt.Errorf("o CoreTuner foi instalado, mas não foi possível abrir o painel: %w", err)
	}
	return nil
}

func (a *App) installRemoteAgent(info RemoteAgentInfo, deviceID int) error {
	if strings.TrimSpace(info.URL) == "" || strings.TrimSpace(info.SHA256) == "" || strings.TrimSpace(info.MeshGroupHex) == "" {
		return errors.New("o servidor retornou dados incompletos para o acesso remoto")
	}

	installed, running := remoteAgentStatus()
	matches := installed && remoteAgentMatches(info.MeshGroupHex, info.ServerURL)
	if matches && running {
		setText(a.status, "Confirmando o acesso remoto no servidor...")
		if connected, warning := a.waitRemoteRegistration(deviceID, 90*time.Second); connected {
			return nil
		} else if warning != "" {
			return fmt.Errorf("o Mesh Agent correto está instalado, mas o servidor não confirmou a conexão: %s", warning)
		}
		return errors.New("o Mesh Agent correto está instalado, mas o computador não apareceu online no MeshCentral")
	}

	if installed && !matches {
		setText(a.status, "Substituindo vínculo remoto antigo...")
		if err := uninstallExistingRemoteAgent(); err != nil {
			return fmt.Errorf("não foi possível remover o Mesh Agent antigo: %w", err)
		}
	} else if installed && !running {
		setText(a.status, "Reinstalando o serviço remoto...")
		if err := uninstallExistingRemoteAgent(); err != nil {
			return fmt.Errorf("não foi possível limpar a instalação remota incompleta: %w", err)
		}
	}

	setText(a.status, "Baixando o agente remoto exclusivo desta empresa...")
	raw, err := a.downloadComponent(ComponentInfo{
		Filename: info.Filename,
		URL:      info.URL,
		SHA256:   info.SHA256,
		Size:     info.Size,
	})
	if err != nil {
		return fmt.Errorf("falha ao baixar o agente remoto da empresa: %w", err)
	}
	tempDir := filepath.Join(os.TempDir(), "CoreTuner")
	if err := os.MkdirAll(tempDir, 0700); err != nil {
		return fmt.Errorf("não foi possível preparar o instalador remoto: %w", err)
	}
	filename := strings.TrimSpace(info.Filename)
	if filename == "" {
		filename = "CoreTunerRemoteAgent.exe"
	}
	installerPath := filepath.Join(tempDir, filename)
	defer os.Remove(installerPath)
	if err := writeAtomic(installerPath, raw, 0700); err != nil {
		return fmt.Errorf("não foi possível preparar o instalador remoto: %w", err)
	}

	setText(a.status, "Aguardando autorização do Windows para instalar o acesso remoto...")
	if err := runElevatedAndWait(installerPath, "-fullinstall", 3*time.Minute); err != nil {
		return err
	}
	for i := 0; i < 45; i++ {
		if installedNow, runningNow := remoteAgentStatus(); installedNow && runningNow {
			if !remoteAgentMatches(info.MeshGroupHex, info.ServerURL) {
				return errors.New("o Mesh Agent foi instalado, mas não ficou vinculado ao grupo remoto desta empresa")
			}
			setText(a.status, "Aguardando o computador aparecer online no servidor remoto...")
			if connected, warning := a.waitRemoteRegistration(deviceID, 90*time.Second); connected {
				return nil
			} else if warning != "" {
				return fmt.Errorf("o agente iniciou, mas o servidor remoto não confirmou a conexão: %s", warning)
			}
			return errors.New("o agente iniciou, mas o computador não apareceu online no MeshCentral dentro do prazo")
		}
		time.Sleep(time.Second)
	}
	return errors.New("o serviço remoto não iniciou dentro do tempo esperado")
}

func (a *App) waitRemoteRegistration(deviceID int, timeout time.Duration) (bool, string) {
	deadline := time.Now().Add(timeout)
	lastWarning := ""
	for time.Now().Before(deadline) {
		var status RemoteStatusResponse
		err := a.request("GET", fmt.Sprintf("%s/api/devices/%d/remote-status", a.serverURL, deviceID), nil, a.token, &status)
		if err == nil {
			if status.MeshConnected && status.ServiceRunning && status.Available && strings.TrimSpace(status.MeshNodeID) != "" {
				return true, ""
			}
			if strings.TrimSpace(status.Warning) != "" {
				lastWarning = strings.TrimSpace(status.Warning)
			}
		} else {
			lastWarning = err.Error()
		}
		time.Sleep(3 * time.Second)
	}
	return false, lastWarning
}

func (a *App) downloadComponent(info ComponentInfo) ([]byte, error) {
	if strings.TrimSpace(info.URL) == "" || strings.TrimSpace(info.SHA256) == "" {
		return nil, errors.New("manifesto de componente inválido")
	}
	endpoint := info.URL
	if strings.HasPrefix(endpoint, "/") {
		endpoint = a.serverURL + endpoint
	}
	req, err := http.NewRequest(http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+a.token)
	req.Header.Set("User-Agent", "CoreTunerSetup/"+appVersion)
	resp, err := a.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		return nil, fmt.Errorf("servidor retornou %s: %s", resp.Status, strings.TrimSpace(string(raw)))
	}
	limit := int64(100 << 20)
	raw, err := io.ReadAll(io.LimitReader(resp.Body, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(raw)) > limit {
		return nil, errors.New("componente excede o tamanho permitido")
	}
	sum := sha256.Sum256(raw)
	actual := hex.EncodeToString(sum[:])
	if !strings.EqualFold(actual, info.SHA256) {
		return nil, errors.New("a verificação de integridade do componente falhou")
	}
	if len(raw) < 2 || raw[0] != 'M' || raw[1] != 'Z' {
		return nil, errors.New("o componente recebido não é um executável Windows válido")
	}
	return raw, nil
}

func writeAtomic(path string, data []byte, perm os.FileMode) error {
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, perm); err != nil {
		return err
	}
	_ = os.Remove(path)
	return os.Rename(tmp, path)
}

func procMessageBoxSimple(title, text string, flags uintptr) int {
	r, _, _ := procMessageBox.Call(0, uintptr(unsafe.Pointer(utf16(text))), uintptr(unsafe.Pointer(utf16(title))), flags)
	return int(r)
}

func (a *App) openPasswordRecovery() {
	server, err := a.server()
	if err != nil {
		procMessageBoxSimple("CoreTuner", err.Error(), MB_OK|MB_ICONERROR)
		return
	}
	target := server + "/?forgot=1"
	procShellExecute.Call(uintptr(a.hwnd), uintptr(unsafe.Pointer(utf16("open"))), uintptr(unsafe.Pointer(utf16(target))), 0, 0, SW_SHOWNORMAL)
}

func (a *App) openCentral() {
	if a.serverURL == "" {
		return
	}
	procShellExecute.Call(uintptr(a.hwnd), uintptr(unsafe.Pointer(utf16("open"))), uintptr(unsafe.Pointer(utf16(a.serverURL+"/central"))), 0, 0, SW_SHOWNORMAL)
}

func (a *App) request(method, endpoint string, payload any, bearer string, out any) error {
	var body io.Reader
	if payload != nil {
		raw, err := json.Marshal(payload)
		if err != nil {
			return err
		}
		body = bytes.NewReader(raw)
	}
	req, err := http.NewRequest(method, endpoint, body)
	if err != nil {
		return err
	}
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	req.Header.Set("User-Agent", "CoreTunerSetup/"+appVersion)
	resp, err := a.client.Do(req)
	if err != nil {
		return fmt.Errorf("não foi possível conectar ao servidor: %w", err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var apiErr APIError
		_ = json.Unmarshal(raw, &apiErr)
		msg := detailText(apiErr.Detail)
		if msg == "" {
			msg = strings.TrimSpace(string(raw))
		}
		if msg == "" {
			msg = resp.Status
		}
		return errors.New(msg)
	}
	if out != nil && len(raw) > 0 {
		return json.Unmarshal(raw, out)
	}
	return nil
}
func detailText(v any) string {
	switch x := v.(type) {
	case string:
		return x
	case []any:
		parts := []string{}
		for _, it := range x {
			if m, ok := it.(map[string]any); ok {
				if msg, ok := m["msg"].(string); ok {
					parts = append(parts, msg)
				}
			}
		}
		return strings.Join(parts, " ")
	}
	return ""
}

func collectMachine() (Machine, error) {
	return collectMachineNative()
}

func configDir() string {
	// Preferência do Console por usuário. A credencial do Agent continua em
	// ProgramData, protegida para SYSTEM/Administradores.
	d := os.Getenv("LOCALAPPDATA")
	if d == "" {
		home, _ := os.UserHomeDir()
		d = filepath.Join(home, "AppData", "Local")
	}
	return filepath.Join(d, "CoreTuner")
}
func loadServerURL() string {
	raw, err := os.ReadFile(filepath.Join(configDir(), "console.json"))
	if err == nil {
		var c struct {
			ServerURL string `json:"server_url"`
		}
		if json.Unmarshal(raw, &c) == nil && c.ServerURL != "" {
			return c.ServerURL
		}
	}
	return defaultServerURL
}
func saveServerURL(value string) {
	_ = os.MkdirAll(configDir(), 0755)
	raw, _ := json.Marshal(map[string]string{"server_url": value})
	_ = os.WriteFile(filepath.Join(configDir(), "console.json"), raw, 0600)
}
