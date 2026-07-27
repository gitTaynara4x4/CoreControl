//go:build windows

package main

import (
	"syscall"
	"unsafe"
)

const (
	WM_PAINT          = 0x000F
	WM_ERASEBKGND     = 0x0014
	WM_DRAWITEM       = 0x002B
	WM_CTLCOLORSTATIC = 0x0138
	WM_CTLCOLOREDIT   = 0x0133
	BS_OWNERDRAW      = 0x0000000B
	SS_BITMAP         = 0x0000000E
	STM_SETIMAGE      = 0x0172
	IMAGE_BITMAP      = 0
	ODS_SELECTED      = 0x0001
	ODS_DISABLED      = 0x0004
	TRANSPARENT       = 1
	PS_SOLID          = 0
	DT_CENTER         = 0x00000001
	DT_VCENTER        = 0x00000004
	DT_SINGLELINE     = 0x00000020
	HOLLOW_BRUSH      = 5
)

type RECT struct {
	Left, Top, Right, Bottom int32
}

type PAINTSTRUCT struct {
	Hdc         syscall.Handle
	Erase       int32
	RcPaint     RECT
	Restore     int32
	IncUpdate   int32
	RGBReserved [32]byte
}

type DRAWITEMSTRUCT struct {
	CtlType    uint32
	CtlID      uint32
	ItemID     uint32
	ItemAction uint32
	ItemState  uint32
	HwndItem   syscall.Handle
	Hdc        syscall.Handle
	RcItem     RECT
	ItemData   uintptr
}

var (
	procBeginPaint       = user32.NewProc("BeginPaint")
	procEndPaint         = user32.NewProc("EndPaint")
	procFillRect         = user32.NewProc("FillRect")
	procGetClientRect    = user32.NewProc("GetClientRect")
	procInvalidateRect   = user32.NewProc("InvalidateRect")
	procCreateSolidBrush = gdi32.NewProc("CreateSolidBrush")
	procCreatePen        = gdi32.NewProc("CreatePen")
	procSelectObject     = gdi32.NewProc("SelectObject")
	procDeleteObject     = gdi32.NewProc("DeleteObject")
	procRoundRect        = gdi32.NewProc("RoundRect")
	procSetBkMode        = gdi32.NewProc("SetBkMode")
	procSetTextColor     = gdi32.NewProc("SetTextColor")
	procDrawTextW        = user32.NewProc("DrawTextW")
)

var (
	themeWindowBrush syscall.Handle
	themeWhiteBrush  syscall.Handle
	themeEditBrush   syscall.Handle
)

func colorRef(r, g, b byte) uintptr {
	return uintptr(r) | uintptr(g)<<8 | uintptr(b)<<16
}

func ensureThemeResources() {
	if themeWindowBrush == 0 {
		brush, _, _ := procCreateSolidBrush.Call(colorRef(245, 248, 252))
		themeWindowBrush = syscall.Handle(brush)
	}
	if themeWhiteBrush == 0 {
		brush, _, _ := procCreateSolidBrush.Call(colorRef(255, 255, 255))
		themeWhiteBrush = syscall.Handle(brush)
	}
	if themeEditBrush == 0 {
		brush, _, _ := procCreateSolidBrush.Call(colorRef(255, 255, 255))
		themeEditBrush = syscall.Handle(brush)
	}
}

func invalidateTheme(hwnd syscall.Handle) {
	procInvalidateRect.Call(uintptr(hwnd), 0, 1)
}

func paintTheme(hwnd syscall.Handle) {
	ensureThemeResources()
	var ps PAINTSTRUCT
	hdc, _, _ := procBeginPaint.Call(uintptr(hwnd), uintptr(unsafe.Pointer(&ps)))
	if hdc == 0 {
		return
	}
	defer procEndPaint.Call(uintptr(hwnd), uintptr(unsafe.Pointer(&ps)))

	var client RECT
	procGetClientRect.Call(uintptr(hwnd), uintptr(unsafe.Pointer(&client)))
	procFillRect.Call(hdc, uintptr(unsafe.Pointer(&client)), uintptr(themeWindowBrush))

	header := RECT{Left: 0, Top: 0, Right: client.Right, Bottom: 226}
	procFillRect.Call(hdc, uintptr(unsafe.Pointer(&header)), uintptr(themeWhiteBrush))

	// Linha de identidade visual entre o cabeçalho e o formulário.
	accentBrush, _, _ := procCreateSolidBrush.Call(colorRef(18, 183, 240))
	accent := RECT{Left: 44, Top: 222, Right: client.Right - 44, Bottom: 226}
	procFillRect.Call(hdc, uintptr(unsafe.Pointer(&accent)), accentBrush)
	procDeleteObject.Call(accentBrush)

	// Cartão principal da etapa atual.
	brush, _, _ := procCreateSolidBrush.Call(colorRef(255, 255, 255))
	pen, _, _ := procCreatePen.Call(PS_SOLID, 1, colorRef(221, 229, 240))
	oldBrush, _, _ := procSelectObject.Call(hdc, brush)
	oldPen, _, _ := procSelectObject.Call(hdc, pen)
	procRoundRect.Call(hdc, 44, 242, uintptr(client.Right-44), uintptr(client.Bottom-32), 18, 18)
	procSelectObject.Call(hdc, oldBrush)
	procSelectObject.Call(hdc, oldPen)
	procDeleteObject.Call(brush)
	procDeleteObject.Call(pen)
}

func staticControlColor(hdc uintptr) uintptr {
	procSetBkMode.Call(hdc, TRANSPARENT)
	procSetTextColor.Call(hdc, colorRef(38, 50, 74))
	hollow, _, _ := procGetStockObject.Call(HOLLOW_BRUSH)
	return hollow
}

func editControlColor(hdc uintptr) uintptr {
	ensureThemeResources()
	procSetBkMode.Call(hdc, TRANSPARENT)
	procSetTextColor.Call(hdc, colorRef(24, 39, 67))
	return uintptr(themeEditBrush)
}

func drawOwnerButton(dis *DRAWITEMSTRUCT) {
	if dis == nil || dis.Hdc == 0 {
		return
	}
	id := int(dis.CtlID)
	disabled := dis.ItemState&ODS_DISABLED != 0
	pressed := dis.ItemState&ODS_SELECTED != 0

	fill := colorRef(255, 255, 255)
	border := colorRef(208, 220, 236)
	textColor := colorRef(24, 55, 103)

	switch id {
	case idLoginButton, idRegisterButton, idInstall:
		fill = colorRef(13, 91, 245)
		border = colorRef(13, 91, 245)
		textColor = colorRef(255, 255, 255)
		if pressed {
			fill = colorRef(8, 70, 196)
			border = fill
		}
	case idForgotPassword:
		fill = colorRef(239, 246, 255)
		border = colorRef(218, 232, 252)
		textColor = colorRef(13, 91, 245)
	}

	if disabled {
		fill = colorRef(232, 237, 244)
		border = colorRef(219, 225, 233)
		textColor = colorRef(139, 151, 170)
	}

	rect := dis.RcItem
	if pressed {
		rect.Top++
		rect.Bottom++
	}

	brush, _, _ := procCreateSolidBrush.Call(fill)
	pen, _, _ := procCreatePen.Call(PS_SOLID, 1, border)
	oldBrush, _, _ := procSelectObject.Call(uintptr(dis.Hdc), brush)
	oldPen, _, _ := procSelectObject.Call(uintptr(dis.Hdc), pen)
	procRoundRect.Call(
		uintptr(dis.Hdc),
		uintptr(rect.Left), uintptr(rect.Top), uintptr(rect.Right), uintptr(rect.Bottom),
		14, 14,
	)
	procSelectObject.Call(uintptr(dis.Hdc), oldBrush)
	procSelectObject.Call(uintptr(dis.Hdc), oldPen)
	procDeleteObject.Call(brush)
	procDeleteObject.Call(pen)

	oldFont := uintptr(0)
	if app != nil && app.buttonFont != 0 {
		oldFont, _, _ = procSelectObject.Call(uintptr(dis.Hdc), app.buttonFont)
	}
	procSetBkMode.Call(uintptr(dis.Hdc), TRANSPARENT)
	procSetTextColor.Call(uintptr(dis.Hdc), textColor)
	label := getText(dis.HwndItem)
	textRect := rect
	procDrawTextW.Call(
		uintptr(dis.Hdc),
		uintptr(unsafe.Pointer(utf16(label))),
		^uintptr(0),
		uintptr(unsafe.Pointer(&textRect)),
		DT_CENTER|DT_VCENTER|DT_SINGLELINE,
	)
	if oldFont != 0 {
		procSelectObject.Call(uintptr(dis.Hdc), oldFont)
	}
}
