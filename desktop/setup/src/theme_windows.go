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
	OPAQUE            = 2
	PS_SOLID          = 0
	DT_CENTER         = 0x00000001
	DT_VCENTER        = 0x00000004
	DT_SINGLELINE     = 0x00000020
	HOLLOW_BRUSH      = 5
	RDW_INVALIDATE    = 0x0001
	RDW_ERASE         = 0x0004
	RDW_ALLCHILDREN   = 0x0080
	RDW_UPDATENOW     = 0x0100
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
	procRedrawWindow     = user32.NewProc("RedrawWindow")
	procCreateSolidBrush = gdi32.NewProc("CreateSolidBrush")
	procCreatePen        = gdi32.NewProc("CreatePen")
	procSelectObject     = gdi32.NewProc("SelectObject")
	procDeleteObject     = gdi32.NewProc("DeleteObject")
	procRoundRect        = gdi32.NewProc("RoundRect")
	procEllipse          = gdi32.NewProc("Ellipse")
	procMoveToEx         = gdi32.NewProc("MoveToEx")
	procLineTo           = gdi32.NewProc("LineTo")
	procSetBkMode        = gdi32.NewProc("SetBkMode")
	procSetBkColor       = gdi32.NewProc("SetBkColor")
	procSetTextColor     = gdi32.NewProc("SetTextColor")
	procDrawTextW        = user32.NewProc("DrawTextW")
)

var (
	themeWindowBrush syscall.Handle
	themeWhiteBrush  syscall.Handle
	themeEditBrush   syscall.Handle
	themeInfoBrush   syscall.Handle
)

func colorRef(r, g, b byte) uintptr {
	return uintptr(r) | uintptr(g)<<8 | uintptr(b)<<16
}

func ensureThemeResources() {
	if themeWindowBrush == 0 {
		brush, _, _ := procCreateSolidBrush.Call(colorRef(248, 250, 252))
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
	if themeInfoBrush == 0 {
		brush, _, _ := procCreateSolidBrush.Call(colorRef(248, 251, 255))
		themeInfoBrush = syscall.Handle(brush)
	}
}

func invalidateTheme(hwnd syscall.Handle) {
	procInvalidateRect.Call(uintptr(hwnd), 0, 1)
}

func forceRedraw(hwnd syscall.Handle) {
	if hwnd == 0 {
		return
	}
	procRedrawWindow.Call(
		uintptr(hwnd), 0, 0,
		RDW_INVALIDATE|RDW_ERASE|RDW_ALLCHILDREN|RDW_UPDATENOW,
	)
}

func redrawControl(hwnd syscall.Handle) {
	if hwnd == 0 {
		return
	}
	procRedrawWindow.Call(uintptr(hwnd), 0, 0, RDW_INVALIDATE|RDW_ERASE|RDW_UPDATENOW)
}

func eraseThemeBackground(hwnd syscall.Handle, hdc uintptr) {
	if hwnd == 0 || hdc == 0 {
		return
	}
	ensureThemeResources()
	var client RECT
	procGetClientRect.Call(uintptr(hwnd), uintptr(unsafe.Pointer(&client)))
	procFillRect.Call(hdc, uintptr(unsafe.Pointer(&client)), uintptr(themeWindowBrush))
}

func roundedPanel(hdc uintptr, left, top, right, bottom int32, fill, border uintptr, radius int32) {
	brush, _, _ := procCreateSolidBrush.Call(fill)
	pen, _, _ := procCreatePen.Call(PS_SOLID, 1, border)
	oldBrush, _, _ := procSelectObject.Call(hdc, brush)
	oldPen, _, _ := procSelectObject.Call(hdc, pen)
	procRoundRect.Call(hdc, uintptr(left), uintptr(top), uintptr(right), uintptr(bottom), uintptr(radius), uintptr(radius))
	procSelectObject.Call(hdc, oldBrush)
	procSelectObject.Call(hdc, oldPen)
	procDeleteObject.Call(brush)
	procDeleteObject.Call(pen)
}

func strokedLine(hdc uintptr, x1, y1, x2, y2 int32, color uintptr, width int32) {
	pen, _, _ := procCreatePen.Call(PS_SOLID, uintptr(width), color)
	oldPen, _, _ := procSelectObject.Call(hdc, pen)
	procMoveToEx.Call(hdc, uintptr(x1), uintptr(y1), 0)
	procLineTo.Call(hdc, uintptr(x2), uintptr(y2))
	procSelectObject.Call(hdc, oldPen)
	procDeleteObject.Call(pen)
}

func filledCircle(hdc uintptr, left, top, right, bottom int32, fill, border uintptr) {
	brush, _, _ := procCreateSolidBrush.Call(fill)
	pen, _, _ := procCreatePen.Call(PS_SOLID, 1, border)
	oldBrush, _, _ := procSelectObject.Call(hdc, brush)
	oldPen, _, _ := procSelectObject.Call(hdc, pen)
	procEllipse.Call(hdc, uintptr(left), uintptr(top), uintptr(right), uintptr(bottom))
	procSelectObject.Call(hdc, oldBrush)
	procSelectObject.Call(hdc, oldPen)
	procDeleteObject.Call(brush)
	procDeleteObject.Call(pen)
}

func drawCheckIcon(hdc uintptr, cx, cy int32) {
	filledCircle(hdc, cx-22, cy-22, cx+22, cy+22, colorRef(229, 248, 238), colorRef(229, 248, 238))
	green := colorRef(18, 151, 91)
	strokedLine(hdc, cx-9, cy, cx-2, cy+7, green, 3)
	strokedLine(hdc, cx-2, cy+7, cx+11, cy-8, green, 3)
}

func drawBuildingIcon(hdc uintptr, cx, cy int32) {
	filledCircle(hdc, cx-22, cy-22, cx+22, cy+22, colorRef(233, 242, 255), colorRef(233, 242, 255))
	blue := colorRef(20, 100, 222)
	// Corpo principal e anexo.
	strokedLine(hdc, cx-9, cy-10, cx-9, cy+11, blue, 2)
	strokedLine(hdc, cx+4, cy-10, cx+4, cy+11, blue, 2)
	strokedLine(hdc, cx-9, cy-10, cx+4, cy-10, blue, 2)
	strokedLine(hdc, cx-12, cy+11, cx+12, cy+11, blue, 2)
	strokedLine(hdc, cx+4, cy-3, cx+11, cy-3, blue, 2)
	strokedLine(hdc, cx+11, cy-3, cx+11, cy+11, blue, 2)
	for _, y := range []int32{cy - 5, cy + 1, cy + 7} {
		strokedLine(hdc, cx-5, y, cx-2, y, blue, 2)
	}
}

func drawComputerIcon(hdc uintptr, cx, cy int32, color uintptr) {
	// Monitor
	strokedLine(hdc, cx-11, cy-8, cx+11, cy-8, color, 2)
	strokedLine(hdc, cx-11, cy-8, cx-11, cy+6, color, 2)
	strokedLine(hdc, cx+11, cy-8, cx+11, cy+6, color, 2)
	strokedLine(hdc, cx-11, cy+6, cx+11, cy+6, color, 2)
	strokedLine(hdc, cx, cy+6, cx, cy+11, color, 2)
	strokedLine(hdc, cx-6, cy+11, cx+6, cy+11, color, 2)
}

func drawBriefcaseIcon(hdc uintptr, cx, cy int32, color uintptr) {
	strokedLine(hdc, cx-10, cy-4, cx+10, cy-4, color, 2)
	strokedLine(hdc, cx-10, cy-4, cx-10, cy+9, color, 2)
	strokedLine(hdc, cx+10, cy-4, cx+10, cy+9, color, 2)
	strokedLine(hdc, cx-10, cy+9, cx+10, cy+9, color, 2)
	strokedLine(hdc, cx-5, cy-4, cx-5, cy-8, color, 2)
	strokedLine(hdc, cx+5, cy-4, cx+5, cy-8, color, 2)
	strokedLine(hdc, cx-5, cy-8, cx+5, cy-8, color, 2)
}

func drawPinIcon(hdc uintptr, cx, cy int32, color uintptr) {
	filledCircle(hdc, cx-7, cy-9, cx+7, cy+5, colorRef(255, 255, 255), color)
	filledCircle(hdc, cx-2, cy-4, cx+2, cy, color, color)
	strokedLine(hdc, cx-5, cy+2, cx, cy+10, color, 2)
	strokedLine(hdc, cx, cy+10, cx+5, cy+2, color, 2)
}

func drawLockIcon(hdc uintptr, cx, cy int32, color uintptr) {
	// Corpo.
	strokedLine(hdc, cx-9, cy, cx+9, cy, color, 2)
	strokedLine(hdc, cx-9, cy, cx-9, cy+13, color, 2)
	strokedLine(hdc, cx+9, cy, cx+9, cy+13, color, 2)
	strokedLine(hdc, cx-9, cy+13, cx+9, cy+13, color, 2)
	// Arco aproximado por segmentos.
	strokedLine(hdc, cx-6, cy, cx-6, cy-5, color, 2)
	strokedLine(hdc, cx-6, cy-5, cx-3, cy-9, color, 2)
	strokedLine(hdc, cx-3, cy-9, cx+3, cy-9, color, 2)
	strokedLine(hdc, cx+3, cy-9, cx+6, cy-5, color, 2)
	strokedLine(hdc, cx+6, cy-5, cx+6, cy, color, 2)
}

func drawShieldIcon(hdc uintptr, cx, cy int32, color uintptr) {
	strokedLine(hdc, cx, cy-9, cx+8, cy-5, color, 2)
	strokedLine(hdc, cx+8, cy-5, cx+6, cy+5, color, 2)
	strokedLine(hdc, cx+6, cy+5, cx, cy+10, color, 2)
	strokedLine(hdc, cx, cy+10, cx-6, cy+5, color, 2)
	strokedLine(hdc, cx-6, cy+5, cx-8, cy-5, color, 2)
	strokedLine(hdc, cx-8, cy-5, cx, cy-9, color, 2)
}

func paintDashboardTheme(hdc uintptr, client RECT) {
	// Cabeçalho totalmente limpo para a marca centralizada.
	header := RECT{Left: 0, Top: 0, Right: client.Right, Bottom: 140}
	procFillRect.Call(hdc, uintptr(unsafe.Pointer(&header)), uintptr(themeWhiteBrush))

	// Card principal.
	roundedPanel(hdc, 40, 146, client.Right-40, client.Bottom-22, colorRef(255, 255, 255), colorRef(229, 233, 240), 20)

	// Confirmação.
	drawCheckIcon(hdc, 88, 194)

	// Empresa vinculada.
	roundedPanel(hdc, 66, 248, client.Right-66, 404, colorRef(248, 251, 255), colorRef(205, 222, 250), 16)
	drawBuildingIcon(hdc, 112, 312)
	drawShieldIcon(hdc, 96, 368, colorRef(37, 73, 132))

	// Identificação do computador.
	roundedPanel(hdc, 66, 418, client.Right-66, 680, colorRef(255, 255, 255), colorRef(226, 231, 238), 16)
	drawComputerIcon(hdc, 92, 451, colorRef(20, 100, 222))

	// Campos arredondados — os EDITs sem borda ficam encaixados por cima.
	roundedPanel(hdc, 88, 538, client.Right-88, 580, colorRef(255, 255, 255), colorRef(205, 213, 225), 10)
	roundedPanel(hdc, 88, 620, 368, 662, colorRef(255, 255, 255), colorRef(205, 213, 225), 10)
	roundedPanel(hdc, 390, 620, client.Right-88, 662, colorRef(255, 255, 255), colorRef(205, 213, 225), 10)
	drawComputerIcon(hdc, 108, 559, colorRef(102, 115, 137))
	drawBriefcaseIcon(hdc, 108, 641, colorRef(102, 115, 137))
	drawPinIcon(hdc, 410, 641, colorRef(102, 115, 137))

	// Aviso seguro.
	roundedPanel(hdc, 66, 690, client.Right-66, 764, colorRef(248, 251, 255), colorRef(199, 219, 251), 14)
	drawLockIcon(hdc, 92, 725, colorRef(20, 100, 222))

	// Rodapé de confiança.
	drawShieldIcon(hdc, 224, 873, colorRef(107, 120, 142))
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

	if app != nil && app.mode == "dashboard" {
		paintDashboardTheme(hdc, client)
		return
	}

	// Demais etapas mantêm o layout compacto original.
	header := RECT{Left: 0, Top: 0, Right: client.Right, Bottom: 112}
	procFillRect.Call(hdc, uintptr(unsafe.Pointer(&header)), uintptr(themeWhiteBrush))
	roundedPanel(hdc, 32, 124, client.Right-32, client.Bottom-14, colorRef(255, 255, 255), colorRef(228, 231, 236), 18)
}

func staticControlColor(hdc uintptr, hwnd syscall.Handle) uintptr {
	ensureThemeResources()
	procSetBkMode.Call(hdc, OPAQUE)

	background := colorRef(255, 255, 255)
	brush := themeWhiteBrush
	textColor := colorRef(37, 48, 67)

	if app != nil {
		if hwnd == app.companyCaption || hwnd == app.companyLabel || hwnd == app.status || hwnd == app.secureTitle || hwnd == app.secureText {
			background = colorRef(248, 251, 255)
			brush = themeInfoBrush
		}
		switch hwnd {
		case app.brand:
			textColor = colorRef(15, 48, 96)
		case app.subtitle, app.verifiedDescription, app.companyCaption, app.identityHelp, app.secureText, app.footerText:
			textColor = colorRef(91, 105, 129)
		case app.verifiedLabel, app.companyLabel, app.identityTitle, app.secureTitle:
			textColor = colorRef(13, 43, 87)
		case app.status:
			textColor = colorRef(54, 72, 104)
		}
	}

	procSetBkColor.Call(hdc, background)
	procSetTextColor.Call(hdc, textColor)
	return uintptr(brush)
}

func editControlColor(hdc uintptr) uintptr {
	ensureThemeResources()
	procSetBkMode.Call(hdc, OPAQUE)
	procSetBkColor.Call(hdc, colorRef(255, 255, 255))
	procSetTextColor.Call(hdc, colorRef(36, 52, 78))
	return uintptr(themeEditBrush)
}

func drawInstallArrow(hdc uintptr, rect RECT) {
	cx := (rect.Left+rect.Right)/2 - 174
	cy := (rect.Top + rect.Bottom) / 2
	white := colorRef(255, 255, 255)
	strokedLine(hdc, cx, cy-9, cx, cy+6, white, 2)
	strokedLine(hdc, cx-6, cy+1, cx, cy+7, white, 2)
	strokedLine(hdc, cx, cy+7, cx+6, cy+1, white, 2)
	strokedLine(hdc, cx-8, cy+11, cx+8, cy+11, white, 2)
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
		fill = colorRef(9, 101, 238)
		border = colorRef(9, 101, 238)
		textColor = colorRef(255, 255, 255)
		if pressed {
			fill = colorRef(7, 82, 195)
			border = fill
		}
	case idForgotPassword:
		fill = colorRef(239, 246, 255)
		border = colorRef(218, 232, 252)
		textColor = colorRef(22, 93, 210)
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
		16, 16,
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
	if id == idInstall && !disabled {
		drawInstallArrow(uintptr(dis.Hdc), rect)
		textRect.Left += 24
	}
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
