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
	"regexp"
	"runtime"
	"strings"
	"syscall"
	"time"
	"unsafe"
)

const appVersion = "0.4.15"

var defaultServerURL = "http://127.0.0.1:8002"

const (
	WS_OVERLAPPEDWINDOW = 0x00CF0000
	WS_CAPTION          = 0x00C00000
	WS_SYSMENU          = 0x00080000
	WS_MINIMIZEBOX      = 0x00020000
	WS_CLIPCHILDREN     = 0x02000000
	WS_VISIBLE          = 0x10000000
	WS_CHILD            = 0x40000000
	WS_BORDER           = 0x00800000
	WS_VSCROLL          = 0x00200000
	WS_TABSTOP          = 0x00010000
	ES_AUTOHSCROLL      = 0x0080
	ES_PASSWORD         = 0x0020
	BS_PUSHBUTTON       = 0x00000000
	BS_DEFPUSHBUTTON    = 0x00000001
	SS_CENTER           = 0x00000001
	SW_SHOW             = 5
	SW_HIDE             = 0
	SW_SHOWNORMAL       = 1
	CW_USEDEFAULT       = ^uintptr(0x7fffffff)
	WM_DESTROY          = 0x0002
	WM_SETCURSOR        = 0x0020
	WM_COMMAND          = 0x0111
	WM_SETFONT          = 0x0030
	WM_CLOSE            = 0x0010
	WM_APP              = 0x8000
	MB_OK               = 0x00000000
	MB_ICONINFORMATION  = 0x00000040
	MB_ICONERROR        = 0x00000010
	MB_YESNO            = 0x00000004
	MB_ICONQUESTION     = 0x00000020
	IDYES               = 6
	COLOR_WINDOW        = 5
	DEFAULT_GUI_FONT    = 17
	IDC_ARROW           = 32512
	IDC_HAND            = 32649
)

const (
	idServer           = 101
	idLoginEmail       = 102
	idLoginPassword    = 103
	idLoginButton      = 104
	idShowRegister     = 105
	idForgotPassword   = 106
	idUseInstallCode   = 107
	idInstallCode      = 108
	idValidateCode     = 109
	idRegisterCompany  = 110
	idRegisterName     = 111
	idRegisterEmail    = 112
	idRegisterPassword = 113
	idRegisterConfirm  = 114
	idRegisterButton   = 115
	idShowLogin        = 116
	idCodeBack         = 117
	idDeviceName       = 201
	idSector           = 202
	idLocation         = 203
	idInstall          = 204
	idOpenCentral      = 206
	idLogout           = 207
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
	procSetCursor           = user32.NewProc("SetCursor")
	procLoadCursor          = user32.NewProc("LoadCursorW")
	procGetDlgCtrlID        = user32.NewProc("GetDlgCtrlID")
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

type EnrollmentInfo struct {
	CompanyID   int    `json:"company_id"`
	CompanyName string `json:"company_name"`
	ExpiresAt   string `json:"expires_at"`
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
	hwnd            syscall.Handle
	font            uintptr
	titleFont       uintptr
	sectionFont     uintptr
	smallFont       uintptr
	buttonFont      uintptr
	controls        map[int]syscall.Handle
	loginGroup      []syscall.Handle
	codeGroup       []syscall.Handle
	registerGroup   []syscall.Handle
	dashboardGroup  []syscall.Handle
	client          *http.Client
	serverURL       string
	enrollmentToken string
	token           string
	user            AuthUser
	company         *Company
	status          syscall.Handle
	companyLabel    syscall.Handle
	title           syscall.Handle
	subtitle        syscall.Handle
	installNotice   string
	mode            string
	logoBitmap      syscall.Handle
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
	if h == 0 {
		return
	}
	procSetWindowText.Call(uintptr(h), uintptr(unsafe.Pointer(utf16(text))))
	redrawControl(h)
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
	largeIcon, smallIcon := coreTunerWindowIcons()
	ensureThemeResources()
	arrowCursor, _, _ := procLoadCursor.Call(0, IDC_ARROW)
	wc := WNDCLASSEX{CbSize: uint32(unsafe.Sizeof(WNDCLASSEX{})), LpfnWndProc: syscall.NewCallback(wndProc), HInstance: syscall.Handle(hinst), HIcon: largeIcon, HCursor: syscall.Handle(arrowCursor), HbrBackground: themeWindowBrush, LpszClassName: className, HIconSm: smallIcon}
	procRegisterClassEx.Call(uintptr(unsafe.Pointer(&wc)))
	windowStyle := uintptr(WS_CAPTION | WS_SYSMENU | WS_MINIMIZEBOX)
	h, _, _ := procCreateWindowEx.Call(0, uintptr(unsafe.Pointer(className)), uintptr(unsafe.Pointer(utf16("CoreControl — Instalação segura"))), windowStyle, 180, 40, 720, 850, 0, 0, hinst, 0)
	if h == 0 {
		return
	}
	applyCoreTunerWindowIcons(syscall.Handle(h), largeIcon, smallIcon)
	font, _, _ := procGetStockObject.Call(DEFAULT_GUI_FONT)
	launchCredential := enrollmentCredentialFromLaunch()
	serverURL := loadServerURL()
	if launchCredential != "" {
		// Um instalador vindo de um link de empresa sempre usa o servidor
		// incorporado no build, ignorando configurações antigas desta máquina.
		serverURL = defaultServerURL
	}
	app = &App{hwnd: syscall.Handle(h), font: font, controls: map[int]syscall.Handle{}, client: &http.Client{Timeout: 25 * time.Second}, serverURL: serverURL, enrollmentToken: launchCredential}
	app.createFonts()
	buildUI()
	if launchCredential != "" {
		source := "link"
		if normalizeEnrollmentCode(launchCredential) != "" {
			source = "código"
		}
		if err := app.activateEnrollmentCredential(source); err != nil {
			app.enrollmentToken = ""
			message("Autorização de instalação", err.Error(), MB_OK|MB_ICONERROR)
		}
	}
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

func wndProc(hwnd uintptr, msg uint32, wParam uintptr, lParam unsafe.Pointer) uintptr {
	switch msg {
	case WM_COMMAND:
		if app != nil {
			app.handleCommand(loword(wParam))
		}
		return 0
	case WM_PAINT:
		paintTheme(syscall.Handle(hwnd))
		return 0
	case WM_ERASEBKGND:
		eraseThemeBackground(syscall.Handle(hwnd), wParam)
		return 1
	case WM_DRAWITEM:
		drawOwnerButton((*DRAWITEMSTRUCT)(lParam))
		return 1
	case WM_SETCURSOR:
		if app != nil && app.isClickableControl(syscall.Handle(wParam)) {
			hand, _, _ := procLoadCursor.Call(0, IDC_HAND)
			procSetCursor.Call(hand)
			return 1
		}
	case WM_CTLCOLORSTATIC:
		return staticControlColor(wParam)
	case WM_CTLCOLOREDIT:
		return editControlColor(wParam)
	case WM_CLOSE:
		procDestroyWindow.Call(hwnd)
		return 0
	case WM_DESTROY:
		procPostQuitMessage.Call(0)
		return 0
	}
	r, _, _ := procDefWindowProc.Call(hwnd, uintptr(msg), wParam, uintptr(lParam))
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
	a.font = create(17, 400)
	a.titleFont = create(28, 650)
	a.sectionFont = create(22, 650)
	a.smallFont = create(15, 400)
	a.buttonFont = create(16, 650)
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

	brand := createControl("STATIC", "CoreControl", WS_CHILD|WS_VISIBLE|SS_CENTER, 155, 42, 390, 48, a.hwnd, 0)
	applyFont(brand, a.titleFont)
	a.title = createControl("STATIC", "", WS_CHILD, 0, 0, 1, 1, a.hwnd, 0)
	a.subtitle = createControl("STATIC", "Instalação segura e monitoramento inteligente", WS_CHILD|WS_VISIBLE, 70, 116, 560, 26, a.hwnd, 0)
	applyFont(a.subtitle, a.smallFont)

	serverLabel := createControl("STATIC", "Servidor do CoreControl", WS_CHILD|WS_VISIBLE, 70, 154, 220, 22, a.hwnd, 0)
	applyFont(serverLabel, a.smallFont)
	a.add(idServer, createControl("EDIT", a.serverURL, WS_CHILD|WS_VISIBLE|WS_BORDER|WS_TABSTOP|ES_AUTOHSCROLL, 70, 180, 560, 38, a.hwnd, idServer), nil)

	// Login
	l1 := createControl("STATIC", "Acesse sua empresa", WS_CHILD|WS_VISIBLE, 80, 270, 540, 34, a.hwnd, 0)
	applyFont(l1, a.sectionFont)
	a.loginGroup = append(a.loginGroup, l1)
	loginIntro := createControl("STATIC", "Entre com sua conta para instalar e vincular este computador.", WS_CHILD|WS_VISIBLE, 80, 308, 540, 26, a.hwnd, 0)
	applyFont(loginIntro, a.smallFont)
	a.loginGroup = append(a.loginGroup, loginIntro)

	l2 := createControl("STATIC", "E-mail", WS_CHILD|WS_VISIBLE, 80, 352, 180, 22, a.hwnd, 0)
	applyFont(l2, a.smallFont)
	a.loginGroup = append(a.loginGroup, l2)
	a.add(idLoginEmail, createControl("EDIT", "", WS_CHILD|WS_VISIBLE|WS_BORDER|WS_TABSTOP|ES_AUTOHSCROLL, 80, 378, 540, 42, a.hwnd, idLoginEmail), &a.loginGroup)

	l3 := createControl("STATIC", "Senha", WS_CHILD|WS_VISIBLE, 80, 438, 180, 22, a.hwnd, 0)
	applyFont(l3, a.smallFont)
	a.loginGroup = append(a.loginGroup, l3)
	a.add(idLoginPassword, createControl("EDIT", "", WS_CHILD|WS_VISIBLE|WS_BORDER|WS_TABSTOP|ES_PASSWORD|ES_AUTOHSCROLL, 80, 464, 540, 42, a.hwnd, idLoginPassword), &a.loginGroup)

	a.add(idLoginButton, createControl("BUTTON", "Entrar no CoreControl", WS_CHILD|WS_VISIBLE|WS_TABSTOP|BS_DEFPUSHBUTTON|BS_OWNERDRAW, 80, 536, 540, 50, a.hwnd, idLoginButton), &a.loginGroup)
	a.add(idUseInstallCode, createControl("BUTTON", "Tenho um código de instalação", WS_CHILD|WS_VISIBLE|WS_TABSTOP|BS_OWNERDRAW, 80, 606, 540, 46, a.hwnd, idUseInstallCode), &a.loginGroup)
	a.add(idShowRegister, createControl("BUTTON", "Criar uma empresa", WS_CHILD|WS_VISIBLE|WS_TABSTOP|BS_OWNERDRAW, 80, 670, 260, 46, a.hwnd, idShowRegister), &a.loginGroup)
	a.add(idForgotPassword, createControl("BUTTON", "Esqueci minha senha", WS_CHILD|WS_VISIBLE|WS_TABSTOP|BS_OWNERDRAW, 360, 670, 260, 46, a.hwnd, idForgotPassword), &a.loginGroup)
	applyFont(a.controls[idLoginButton], a.buttonFont)
	applyFont(a.controls[idUseInstallCode], a.buttonFont)
	applyFont(a.controls[idShowRegister], a.buttonFont)
	applyFont(a.controls[idForgotPassword], a.buttonFont)

	loginNote := createControl("STATIC", "✓  Funcionários podem instalar com um código temporário sem conhecer a senha da empresa.", WS_CHILD|WS_VISIBLE, 80, 744, 540, 46, a.hwnd, 0)
	applyFont(loginNote, a.smallFont)
	a.loginGroup = append(a.loginGroup, loginNote)

	// Instalação por código temporário
	c1 := createControl("STATIC", "Vincular com código", WS_CHILD, 80, 270, 540, 34, a.hwnd, 0)
	applyFont(c1, a.sectionFont)
	a.codeGroup = append(a.codeGroup, c1)
	cIntro := createControl("STATIC", "Digite o código fornecido pelo administrador da empresa. Nenhum login ou senha é necessário.", WS_CHILD, 80, 310, 540, 48, a.hwnd, 0)
	applyFont(cIntro, a.smallFont)
	a.codeGroup = append(a.codeGroup, cIntro)
	cLabel := createControl("STATIC", "Código de instalação", WS_CHILD, 80, 382, 220, 22, a.hwnd, 0)
	applyFont(cLabel, a.smallFont)
	a.codeGroup = append(a.codeGroup, cLabel)
	a.add(idInstallCode, createControl("EDIT", "", WS_CHILD|WS_BORDER|WS_TABSTOP|ES_AUTOHSCROLL, 80, 410, 540, 46, a.hwnd, idInstallCode), &a.codeGroup)
	a.add(idValidateCode, createControl("BUTTON", "Validar código e continuar", WS_CHILD|WS_TABSTOP|BS_DEFPUSHBUTTON|BS_OWNERDRAW, 80, 486, 540, 50, a.hwnd, idValidateCode), &a.codeGroup)
	a.add(idCodeBack, createControl("BUTTON", "Voltar", WS_CHILD|WS_TABSTOP|BS_OWNERDRAW, 80, 556, 540, 44, a.hwnd, idCodeBack), &a.codeGroup)
	applyFont(a.controls[idValidateCode], a.buttonFont)
	applyFont(a.controls[idCodeBack], a.buttonFont)
	cNote := createControl("STATIC", "Exemplo: CC-7K4P-9M2X  •  O código expira e só pode vincular um computador.", WS_CHILD, 80, 630, 540, 46, a.hwnd, 0)
	applyFont(cNote, a.smallFont)
	a.codeGroup = append(a.codeGroup, cNote)

	// Cadastro
	r1 := createControl("STATIC", "Crie sua empresa", WS_CHILD, 80, 264, 540, 34, a.hwnd, 0)
	applyFont(r1, a.sectionFont)
	a.registerGroup = append(a.registerGroup, r1)
	rIntro := createControl("STATIC", "Configure a conta principal e comece a cadastrar os computadores.", WS_CHILD, 80, 302, 540, 24, a.hwnd, 0)
	applyFont(rIntro, a.smallFont)
	a.registerGroup = append(a.registerGroup, rIntro)
	fields := []struct {
		id    int
		label string
		y     int32
		pass  bool
	}{
		{idRegisterCompany, "Nome da empresa", 342, false},
		{idRegisterName, "Nome do responsável", 412, false},
		{idRegisterEmail, "E-mail", 482, false},
		{idRegisterPassword, "Senha (mínimo 10 caracteres)", 552, true},
		{idRegisterConfirm, "Confirmar senha", 622, true},
	}
	for _, f := range fields {
		lab := createControl("STATIC", f.label, WS_CHILD, 80, f.y, 360, 20, a.hwnd, 0)
		applyFont(lab, a.smallFont)
		a.registerGroup = append(a.registerGroup, lab)
		style := uint32(WS_CHILD | WS_BORDER | WS_TABSTOP | ES_AUTOHSCROLL)
		if f.pass {
			style |= ES_PASSWORD
		}
		a.add(f.id, createControl("EDIT", "", style, 80, f.y+24, 540, 38, a.hwnd, f.id), &a.registerGroup)
	}
	a.add(idRegisterButton, createControl("BUTTON", "Criar empresa e continuar", WS_CHILD|WS_TABSTOP|BS_DEFPUSHBUTTON|BS_OWNERDRAW, 80, 710, 340, 48, a.hwnd, idRegisterButton), &a.registerGroup)
	a.add(idShowLogin, createControl("BUTTON", "Voltar para o login", WS_CHILD|WS_TABSTOP|BS_OWNERDRAW, 440, 710, 180, 48, a.hwnd, idShowLogin), &a.registerGroup)
	applyFont(a.controls[idRegisterButton], a.buttonFont)
	applyFont(a.controls[idShowLogin], a.buttonFont)

	// Assistente de instalação. Depois da instalação, o Setup abre o painel e fecha.
	statusTitle := createControl("STATIC", "Empresa vinculada", WS_CHILD, 80, 272, 300, 24, a.hwnd, 0)
	applyFont(statusTitle, a.smallFont)
	a.dashboardGroup = append(a.dashboardGroup, statusTitle)
	a.companyLabel = createControl("STATIC", "", WS_CHILD, 80, 304, 540, 30, a.hwnd, 0)
	applyFont(a.companyLabel, a.sectionFont)
	a.dashboardGroup = append(a.dashboardGroup, a.companyLabel)
	a.status = createControl("STATIC", "", WS_CHILD, 80, 340, 540, 32, a.hwnd, 0)
	applyFont(a.status, a.smallFont)
	a.dashboardGroup = append(a.dashboardGroup, a.status)

	steps := createControl("STATIC", "1  Vínculo     2  Computador     3  Instalação     4  Concluído", WS_CHILD, 80, 382, 540, 26, a.hwnd, 0)
	applyFont(steps, a.smallFont)
	a.dashboardGroup = append(a.dashboardGroup, steps)
	section := createControl("STATIC", "Identifique este computador", WS_CHILD, 80, 424, 540, 32, a.hwnd, 0)
	applyFont(section, a.sectionFont)
	a.dashboardGroup = append(a.dashboardGroup, section)
	help := createControl("STATIC", "Essas informações ajudam a localizar o equipamento no painel.", WS_CHILD, 80, 462, 540, 24, a.hwnd, 0)
	applyFont(help, a.smallFont)
	a.dashboardGroup = append(a.dashboardGroup, help)

	labs := []struct {
		text string
		x, y int32
	}{
		{"Nome deste computador", 80, 506},
		{"Setor", 80, 584},
		{"Local / unidade", 360, 584},
	}
	for _, v := range labs {
		h := createControl("STATIC", v.text, WS_CHILD, v.x, v.y, 260, 20, a.hwnd, 0)
		applyFont(h, a.smallFont)
		a.dashboardGroup = append(a.dashboardGroup, h)
	}
	host, _ := os.Hostname()
	a.add(idDeviceName, createControl("EDIT", host, WS_CHILD|WS_BORDER|WS_TABSTOP|ES_AUTOHSCROLL, 80, 532, 540, 40, a.hwnd, idDeviceName), &a.dashboardGroup)
	a.add(idSector, createControl("EDIT", "", WS_CHILD|WS_BORDER|WS_TABSTOP|ES_AUTOHSCROLL, 80, 610, 260, 40, a.hwnd, idSector), &a.dashboardGroup)
	a.add(idLocation, createControl("EDIT", "", WS_CHILD|WS_BORDER|WS_TABSTOP|ES_AUTOHSCROLL, 360, 610, 260, 40, a.hwnd, idLocation), &a.dashboardGroup)
	a.add(idInstall, createControl("BUTTON", "Instalar e continuar", WS_CHILD|WS_TABSTOP|BS_DEFPUSHBUTTON|BS_OWNERDRAW, 80, 678, 540, 50, a.hwnd, idInstall), &a.dashboardGroup)
	applyFont(a.controls[idInstall], a.buttonFont)
	a.add(idOpenCentral, createControl("BUTTON", "Abrir painel web", WS_CHILD|WS_TABSTOP|BS_OWNERDRAW, 80, 748, 260, 44, a.hwnd, idOpenCentral), &a.dashboardGroup)
	a.add(idLogout, createControl("BUTTON", "Trocar conta", WS_CHILD|WS_TABSTOP|BS_OWNERDRAW, 360, 748, 260, 44, a.hwnd, idLogout), &a.dashboardGroup)
	applyFont(a.controls[idOpenCentral], a.buttonFont)
	applyFont(a.controls[idLogout], a.buttonFont)

	a.showMode("login")
}

func (a *App) showMode(mode string) {
	a.mode = mode

	// Esconde todas as etapas antes de mostrar a próxima. O redesenho entre
	// as duas operações limpa completamente os pixels dos controles anteriores,
	// inclusive botões owner-draw e textos STATIC do Win32.
	for _, h := range a.loginGroup {
		show(h, false)
	}
	for _, h := range a.codeGroup {
		show(h, false)
	}
	for _, h := range a.registerGroup {
		show(h, false)
	}
	for _, h := range a.dashboardGroup {
		show(h, false)
	}
	forceRedraw(a.hwnd)

	if mode == "dashboard" {
		setText(a.subtitle, "Instalação segura — "+companyName(a.company))
		for _, h := range a.dashboardGroup {
			show(h, true)
		}
	} else if mode == "code" {
		setText(a.subtitle, "Instalação sem login da empresa")
		for _, h := range a.codeGroup {
			show(h, true)
		}
		procSetFocus.Call(uintptr(a.controls[idInstallCode]))
	} else if mode == "register" {
		setText(a.subtitle, "Crie sua conta e conecte o primeiro computador")
		for _, h := range a.registerGroup {
			show(h, true)
		}
	} else {
		setText(a.subtitle, "Instalação segura e monitoramento inteligente")
		for _, h := range a.loginGroup {
			show(h, true)
		}
	}
	forceRedraw(a.hwnd)
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
	case idUseInstallCode:
		a.showMode("code")
	case idValidateCode:
		a.activateEnrollmentCode()
	case idCodeBack, idShowLogin:
		a.showMode("login")
	case idLoginButton:
		a.login()
	case idForgotPassword:
		a.openPasswordRecovery()
	case idRegisterButton:
		a.register()
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
		return "", errors.New("Informe um endereço válido do CoreControl")
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

var enrollmentTokenPattern = regexp.MustCompile(`ctenr_[A-Za-z0-9_-]{20,}`)
var enrollmentCodePattern = regexp.MustCompile(`(?i)CC[-_ ]?[A-Z0-9]{4}[-_ ]?[A-Z0-9]{4}`)

func enrollmentCredentialFromText(value string) string {
	if token := enrollmentTokenPattern.FindString(value); token != "" {
		return token
	}
	if code := enrollmentCodePattern.FindString(value); code != "" {
		return normalizeEnrollmentCode(code)
	}
	return ""
}

func enrollmentCredentialFromLaunch() string {
	for _, arg := range os.Args[1:] {
		if credential := enrollmentCredentialFromText(arg); credential != "" {
			return credential
		}
	}
	exe, err := os.Executable()
	if err != nil {
		return ""
	}
	return enrollmentCredentialFromText(filepath.Base(exe))
}

func normalizeEnrollmentCode(value string) string {
	compact := strings.ToUpper(regexp.MustCompile(`[^A-Z0-9]`).ReplaceAllString(strings.TrimSpace(value), ""))
	if len(compact) == 8 {
		compact = "CC" + compact
	}
	if len(compact) != 10 || !strings.HasPrefix(compact, "CC") {
		return ""
	}
	body := compact[2:]
	return "CC-" + body[:4] + "-" + body[4:]
}

func (a *App) activateEnrollmentCredential(source string) error {
	if a.enrollmentToken == "" {
		return nil
	}
	var info EnrollmentInfo
	endpoint := a.serverURL + "/api/enrollment/" + url.PathEscape(a.enrollmentToken) + "/info"
	if err := a.request("GET", endpoint, nil, "", &info); err != nil {
		return err
	}
	a.company = &Company{ID: info.CompanyID, Name: info.CompanyName}
	setText(a.companyLabel, info.CompanyName)
	if source == "código" {
		setText(a.status, "Código validado. Nenhum login ou senha da empresa foi usado neste computador.")
	} else {
		setText(a.status, "Instalação autorizada por link. Nenhum login ou senha da empresa é necessário.")
	}
	a.showMode("dashboard")
	setText(a.controls[idInstall], "Instalar CoreControl neste computador")
	show(a.controls[idOpenCentral], false)
	show(a.controls[idLogout], false)
	return nil
}

func (a *App) activateEnrollmentCode() {
	code := normalizeEnrollmentCode(getText(a.controls[idInstallCode]))
	if code == "" {
		message("Código de instalação", "Digite um código válido, por exemplo CC-7K4P-9M2X.", MB_OK|MB_ICONERROR)
		return
	}

	// O Setup oficial usa o servidor incorporado no build para códigos temporários,
	// evitando que uma configuração antiga desta máquina envie o código para outro ambiente.
	a.serverURL = strings.TrimRight(defaultServerURL, "/")
	setText(a.controls[idServer], a.serverURL)
	a.enrollmentToken = code
	enable(a.controls[idValidateCode], false)
	setText(a.subtitle, "Validando código de instalação...")
	err := a.activateEnrollmentCredential("código")
	enable(a.controls[idValidateCode], true)
	if err != nil {
		a.enrollmentToken = ""
		setText(a.subtitle, "Código não validado.")
		message("Código de instalação", err.Error(), MB_OK|MB_ICONERROR)
		return
	}
}

func (a *App) login() {
	server, err := a.server()
	if err != nil {
		message("CoreControl", err.Error(), MB_OK|MB_ICONERROR)
		return
	}
	email := strings.TrimSpace(getText(a.controls[idLoginEmail]))
	password := getText(a.controls[idLoginPassword])
	if email == "" || password == "" {
		message("CoreControl", "Preencha e-mail e senha.", MB_OK|MB_ICONERROR)
		return
	}
	enable(a.controls[idLoginButton], false)
	setText(a.subtitle, "Conectando ao CoreControl...")
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
		message("CoreControl", err.Error(), MB_OK|MB_ICONERROR)
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
		message("CoreControl", "Preencha todos os campos.", MB_OK|MB_ICONERROR)
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
	message("CoreControl", "Empresa criada com sucesso. Agora você pode instalar este computador.", MB_OK|MB_ICONINFORMATION)
}

func (a *App) applyAuth(resp AuthResponse) {
	a.token = resp.AccessToken
	a.user = resp.User
	a.company = resp.Company
	// Preenche os textos enquanto a etapa ainda está oculta. Assim, o usuário
	// nunca vê os placeholders sendo substituídos sobre a tela anterior.
	setText(a.companyLabel, companyName(a.company))
	setText(a.status, fmt.Sprintf("Usuário conectado: %s  •  Este computador será adicionado a essa empresa.", a.user.Name))
	a.showMode("dashboard")
}

func (a *App) logout() {
	a.token = ""
	a.user = AuthUser{}
	a.company = nil
	setText(a.companyLabel, "")
	a.showMode("login")
	setText(a.controls[idLoginPassword], "")
}

func (a *App) isClickableControl(hwnd syscall.Handle) bool {
	if hwnd == 0 || hwnd == a.hwnd {
		return false
	}
	id, _, _ := procGetDlgCtrlID.Call(uintptr(hwnd))
	switch int(id) {
	case idLoginButton, idUseInstallCode, idValidateCode, idCodeBack, idShowRegister, idForgotPassword, idRegisterButton, idShowLogin, idInstall, idOpenCentral, idLogout:
		return true
	default:
		return false
	}
}

func (a *App) installCurrent() {
	if a.enrollmentToken != "" {
		a.installCurrentEnrollment()
		return
	}
	if a.token == "" {
		return
	}
	name := strings.TrimSpace(getText(a.controls[idDeviceName]))
	if name == "" {
		message("CoreControl", "Informe o nome deste computador.", MB_OK|MB_ICONERROR)
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
		"Acesso remoto CoreControl",
		"Deseja instalar também o acesso remoto para suporte técnico?\n\nO CoreControl criará e vinculará automaticamente este computador à empresa correta no servidor remoto. Se existir um Mesh Agent antigo ou ligado a outro servidor, ele será substituído após a autorização do Windows.\n\nTécnicos autorizados poderão controlar a tela somente durante um atendimento.",
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
	setText(a.status, "CoreControl instalado e conectado com sucesso.")
	notice := strings.TrimSpace(a.installNotice)
	if notice == "" {
		notice = "Acesso remoto não foi solicitado nesta instalação."
	}
	message("CoreControl instalado", fmt.Sprintf("Empresa: %s\nComputador: %s\n\nO agente de diagnóstico já está enviando informações técnicas.\n%s", resp.CompanyName, name, notice), MB_OK|MB_ICONINFORMATION)
	procDestroyWindow.Call(uintptr(a.hwnd))
}

func (a *App) installCurrentEnrollment() {
	name := strings.TrimSpace(getText(a.controls[idDeviceName]))
	if name == "" {
		message("CoreControl", "Informe o nome deste computador.", MB_OK|MB_ICONERROR)
		return
	}
	enable(a.controls[idInstall], false)
	setText(a.status, "Validando a autorização e preparando os componentes...")

	machine, err := collectMachine()
	if err != nil {
		enable(a.controls[idInstall], true)
		message("Diagnóstico", err.Error(), MB_OK|MB_ICONERROR)
		return
	}

	var manifest ComponentManifest
	manifestURL := a.serverURL + "/api/enrollment/" + url.PathEscape(a.enrollmentToken) + "/manifest"
	if err = a.request("GET", manifestURL, nil, "", &manifest); err != nil {
		enable(a.controls[idInstall], true)
		setText(a.status, "O código ou link de instalação não está mais disponível.")
		message("Link de instalação", err.Error(), MB_OK|MB_ICONERROR)
		return
	}
	coreInfo, ok := manifest.Files["CoreControl.exe"]
	if !ok {
		enable(a.controls[idInstall], true)
		message("CoreControl", "O servidor não forneceu o CoreControl.exe.", MB_OK|MB_ICONERROR)
		return
	}
	agentInfo, ok := manifest.Files["CoreControlAgent.exe"]
	if !ok {
		enable(a.controls[idInstall], true)
		message("CoreControl", "O servidor não forneceu o CoreControlAgent.exe.", MB_OK|MB_ICONERROR)
		return
	}

	setText(a.status, "Baixando componentes oficiais do CoreControl...")
	coreBytes, err := a.downloadComponent(coreInfo)
	if err != nil {
		enable(a.controls[idInstall], true)
		message("Download", "Falha ao baixar CoreControl.exe: "+err.Error(), MB_OK|MB_ICONERROR)
		return
	}
	agentBytes, err := a.downloadComponent(agentInfo)
	if err != nil {
		enable(a.controls[idInstall], true)
		message("Download", "Falha ao baixar CoreControlAgent.exe: "+err.Error(), MB_OK|MB_ICONERROR)
		return
	}

	payload := map[string]any{
		"enrollment_token": a.enrollmentToken,
		"device_uid":       machine.DeviceUID,
		"name":             name,
		"hostname":         machine.Hostname,
		"sector":           strings.TrimSpace(getText(a.controls[idSector])),
		"location":         strings.TrimSpace(getText(a.controls[idLocation])),
		"manufacturer":     machine.Manufacturer,
		"model":            machine.Model,
		"serial_number":    machine.SerialNumber,
		"os_name":          machine.OSName,
		"os_version":       machine.OSVersion,
		"agent_version":    appVersion,
	}

	setText(a.status, "Vinculando este computador à empresa...")
	var resp InstallResponse
	err = a.request("POST", a.serverURL+"/api/agent/enroll", payload, "", &resp)
	if err == nil {
		err = a.writeEnrollmentFiles(machine, name, getText(a.controls[idSector]), getText(a.controls[idLocation]), resp, coreBytes, agentBytes)
	}
	enable(a.controls[idInstall], true)
	if err != nil {
		setText(a.status, "Instalação não concluída.")
		message("Instalação não concluída", err.Error(), MB_OK|MB_ICONERROR)
		return
	}

	a.enrollmentToken = ""
	setText(a.status, "CoreControl instalado e conectado com sucesso.")
	message(
		"CoreControl instalado",
		fmt.Sprintf("Empresa: %s\nComputador: %s\n\nPronto. Este computador já está vinculado e o agente começou a enviar as informações técnicas.\n\nNenhum login ou senha da empresa foi armazenado neste computador.", resp.CompanyName, name),
		MB_OK|MB_ICONINFORMATION,
	)
	procDestroyWindow.Call(uintptr(a.hwnd))
}

func (a *App) writeEnrollmentFiles(machine Machine, name, sector, location string, resp InstallResponse, coreBytes, agentBytes []byte) error {
	localAppData := os.Getenv("LOCALAPPDATA")
	if localAppData == "" {
		home, _ := os.UserHomeDir()
		localAppData = filepath.Join(home, "AppData", "Local")
	}
	installDir := filepath.Join(localAppData, "Programs", "CoreControl")
	legacyInstallDir := filepath.Join(localAppData, "Programs", "CoreTuner")
	agentDataDir := filepath.Join(localAppData, "CoreTuner", "Agent")
	userDataDir := configDir()
	for _, dir := range []string{installDir, agentDataDir, userDataDir} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("não foi possível preparar a pasta de instalação: %w", err)
		}
	}

	agentPath := filepath.Join(installDir, "CoreControlAgent.exe")
	corePath := filepath.Join(installDir, "CoreControl.exe")
	legacyAgentPath := filepath.Join(legacyInstallDir, "CoreTunerAgent.exe")
	legacyCorePath := filepath.Join(legacyInstallDir, "CoreTuner.exe")
	stopExistingAgent(agentPath)
	stopExistingAgent(legacyAgentPath)
	_ = os.Remove(agentPath)
	_ = os.Remove(legacyAgentPath)
	_ = os.Remove(legacyCorePath)
	_ = hiddenCommand("reg.exe", "delete", `HKCU\Software\Microsoft\Windows\CurrentVersion\Run`, "/v", "CoreTunerAgent", "/f").Run()
	_ = hiddenCommand("reg.exe", "delete", `HKCU\Software\Microsoft\Windows\CurrentVersion\Run`, "/v", "CoreControlAgent", "/f").Run()
	time.Sleep(300 * time.Millisecond)

	if err := writeAtomic(agentPath, agentBytes, 0755); err != nil {
		return fmt.Errorf("não foi possível instalar o agente: %w", err)
	}
	if err := writeAtomic(corePath, coreBytes, 0755); err != nil {
		return fmt.Errorf("não foi possível instalar o aplicativo: %w", err)
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
		return fmt.Errorf("não foi possível salvar a configuração do agente: %w", err)
	}
	if err := writeAtomic(filepath.Join(userDataDir, "server-url.txt"), []byte(a.serverURL), 0644); err != nil {
		return fmt.Errorf("não foi possível salvar o endereço do servidor: %w", err)
	}

	// Instalação por link não deve herdar nem criar sessão administrativa.
	_ = os.Remove(filepath.Join(userDataDir, "session.json"))

	runCommand := fmt.Sprintf(`"%s" -config "%s"`, agentPath, configPath)
	if out, err := hiddenCommand("reg.exe", "add", `HKCU\Software\Microsoft\Windows\CurrentVersion\Run`, "/v", "CoreControlAgent", "/t", "REG_SZ", "/d", runCommand, "/f").CombinedOutput(); err != nil {
		return fmt.Errorf("não foi possível configurar a inicialização do agente: %s", strings.TrimSpace(string(out)))
	}
	cmd := hiddenCommand(agentPath, "-config", configPath)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("não foi possível iniciar o agente: %w", err)
	}
	return nil
}

func (a *App) installFiles(machine Machine, name, sector, location string, resp InstallResponse, installRemote bool) error {
	a.installNotice = ""
	localAppData := os.Getenv("LOCALAPPDATA")
	if localAppData == "" {
		home, _ := os.UserHomeDir()
		localAppData = filepath.Join(home, "AppData", "Local")
	}
	installDir := filepath.Join(localAppData, "Programs", "CoreControl")
	legacyInstallDir := filepath.Join(localAppData, "Programs", "CoreTuner")
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

	setText(a.status, "Baixando componentes oficiais do CoreControl...")
	var manifest ComponentManifest
	if err := a.request("GET", a.serverURL+"/api/desktop/manifest", nil, a.token, &manifest); err != nil {
		return fmt.Errorf("não foi possível obter os componentes oficiais: %w", err)
	}
	coreInfo, ok := manifest.Files["CoreControl.exe"]
	if !ok {
		coreInfo, ok = manifest.Files["CoreTuner.exe"]
	}
	if !ok {
		return errors.New("o servidor não forneceu o CoreControl.exe")
	}
	agentInfo, ok := manifest.Files["CoreControlAgent.exe"]
	if !ok {
		agentInfo, ok = manifest.Files["CoreTunerAgent.exe"]
	}
	if !ok {
		return errors.New("o servidor não forneceu o CoreControlAgent.exe")
	}

	coreBytes, err := a.downloadComponent(coreInfo)
	if err != nil {
		return fmt.Errorf("falha ao baixar CoreControl.exe: %w", err)
	}
	agentBytes, err := a.downloadComponent(agentInfo)
	if err != nil {
		return fmt.Errorf("falha ao baixar CoreControlAgent.exe: %w", err)
	}

	agentPath := filepath.Join(installDir, "CoreControlAgent.exe")
	corePath := filepath.Join(installDir, "CoreControl.exe")
	legacyAgentPath := filepath.Join(legacyInstallDir, "CoreTunerAgent.exe")
	legacyCorePath := filepath.Join(legacyInstallDir, "CoreTuner.exe")
	// Encerra a versão atual e qualquer agente legado antes de substituir os componentes.
	stopExistingAgent(agentPath)
	stopExistingAgent(legacyAgentPath)
	_ = os.Remove(agentPath)
	_ = os.Remove(legacyAgentPath)
	_ = os.Remove(legacyCorePath)
	// Remove a inicialização anterior antes de substituir os componentes.
	_ = hiddenCommand("reg.exe", "delete", `HKCU\Software\Microsoft\Windows\CurrentVersion\Run`, "/v", "CoreTunerAgent", "/f").Run()
	_ = hiddenCommand("reg.exe", "delete", `HKCU\Software\Microsoft\Windows\CurrentVersion\Run`, "/v", "CoreControlAgent", "/f").Run()
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
	if out, err := hiddenCommand("reg.exe", "add", `HKCU\Software\Microsoft\Windows\CurrentVersion\Run`, "/v", "CoreControlAgent", "/t", "REG_SZ", "/d", runCommand, "/f").CombinedOutput(); err != nil {
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
			a.installNotice = "O CoreControl foi instalado, mas o acesso remoto não foi concluído: " + reason
		} else if err := a.installRemoteAgent(*resp.RemoteAgent, resp.DeviceID); err != nil {
			a.installNotice = "O CoreControl foi instalado, mas o acesso remoto não foi concluído: " + err.Error()
		} else {
			a.installNotice = "Acesso remoto instalado, vinculado à empresa correta e confirmado online."
		}
	}

	if err := exec.Command(corePath).Start(); err != nil {
		return fmt.Errorf("o CoreControl foi instalado, mas não foi possível abrir o painel: %w", err)
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
			// O estado confirmado pelo MeshCentral é a fonte de verdade. A leitura
			// local do MeshAgent.msh é apenas uma verificação auxiliar: versões
			// diferentes do agente podem gravar o mesmo vínculo em formatos
			// distintos, gerando um falso erro mesmo quando o computador já está
			// online no grupo correto.
			setText(a.status, "Aguardando o computador aparecer online no servidor remoto...")
			if connected, warning := a.waitRemoteRegistration(deviceID, 90*time.Second); connected {
				return nil
			} else if warning != "" {
				return fmt.Errorf("o agente iniciou, mas o servidor remoto não confirmou a conexão: %s", warning)
			}
			if !remoteAgentMatches(info.MeshGroupHex, info.ServerURL) {
				return errors.New("o Mesh Agent iniciou, mas o servidor não confirmou o vínculo com o grupo remoto desta empresa")
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
	if a.token != "" {
		req.Header.Set("Authorization", "Bearer "+a.token)
	}
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
		procMessageBoxSimple("CoreControl", err.Error(), MB_OK|MB_ICONERROR)
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
