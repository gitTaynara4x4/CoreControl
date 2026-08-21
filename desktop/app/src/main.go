//go:build windows

package main

import (
	"net/http"
	"runtime"
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
	h, _, _ := procCreateWindowEx.Call(0, uintptr(unsafe.Pointer(cls)), uintptr(unsafe.Pointer(utf16(appWindowTitle()))), WS_OVERLAPPEDWINDOW, 40, 30, 1420, 900, 0, 0, hinst, 0)
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
