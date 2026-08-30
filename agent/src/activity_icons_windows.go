//go:build windows

package main

import (
	"bytes"
	"encoding/base64"
	"image"
	"image/color"
	"image/png"
	"strings"
	"sync"
	"syscall"
	"unsafe"
)

const (
	activitySHGFIIcon       = 0x000000100
	activitySHGFISmallIcon  = 0x000000001
	activityDIBRGBColors    = 0
	activityBINone          = 0
	activityDINormal        = 0x0003
	activityIconSize        = 32
	activityMaxIconBytes    = 64 * 1024
	activityWMGetIcon       = 0x007F
	activityIconSmall       = 0
	activityIconBig         = 1
	activityIconSmall2      = 2
	activityGCLPHIcon       = -14
	activityGCLPHIconSmall  = -34
	activitySMTOAbortIfHung = 0x0002
)

type activitySHFileInfoW struct {
	HIcon       syscall.Handle
	IIcon       int32
	Attributes  uint32
	DisplayName [260]uint16
	TypeName    [80]uint16
}

type activityBitmapInfoHeader struct {
	Size          uint32
	Width         int32
	Height        int32
	Planes        uint16
	BitCount      uint16
	Compression   uint32
	SizeImage     uint32
	XPelsPerMeter int32
	YPelsPerMeter int32
	ClrUsed       uint32
	ClrImportant  uint32
}

type activityBitmapInfo struct {
	Header activityBitmapInfoHeader
	Colors [1]uint32
}

type activityIconInfo struct {
	Icon     int32
	XHotspot uint32
	YHotspot uint32
	HbmMask  syscall.Handle
	HbmColor syscall.Handle
}

type activityBitmap struct {
	Type       int32
	Width      int32
	Height     int32
	WidthBytes int32
	Planes     uint16
	BitsPixel  uint16
	Bits       uintptr
}

type activityAppAsset struct {
	ProcessName string `json:"process_name"`
	DisplayName string `json:"display_name"`
	IconData    string `json:"icon_data,omitempty"`
	IconSource  string `json:"icon_source,omitempty"`
}

var (
	activityIconShell32 = syscall.NewLazyDLL("shell32.dll")
	activityIconGDI32   = syscall.NewLazyDLL("gdi32.dll")
	activityIconUser32  = syscall.NewLazyDLL("user32.dll")

	activityQueryFullProcessImageNameW = kernel32.NewProc("QueryFullProcessImageNameW")
	activitySHGetFileInfoW             = activityIconShell32.NewProc("SHGetFileInfoW")
	activityExtractIconExW             = activityIconShell32.NewProc("ExtractIconExW")
	activityCreateCompatibleDC         = activityIconGDI32.NewProc("CreateCompatibleDC")
	activityDeleteDC                   = activityIconGDI32.NewProc("DeleteDC")
	activityCreateDIBSection           = activityIconGDI32.NewProc("CreateDIBSection")
	activitySelectObject               = activityIconGDI32.NewProc("SelectObject")
	activityDeleteObject               = activityIconGDI32.NewProc("DeleteObject")
	activityDrawIconEx                 = activityIconUser32.NewProc("DrawIconEx")
	activityDestroyIcon                = activityIconUser32.NewProc("DestroyIcon")
	activitySendMessageTimeoutW        = activityIconUser32.NewProc("SendMessageTimeoutW")
	activityGetClassLongPtrW           = activityIconUser32.NewProc("GetClassLongPtrW")
	activityGetIconInfo                = activityIconUser32.NewProc("GetIconInfo")
	activityGetObjectW                 = activityIconGDI32.NewProc("GetObjectW")
	activityGetDIBits                  = activityIconGDI32.NewProc("GetDIBits")
	activityExtractAssociatedIconW     = activityIconShell32.NewProc("ExtractAssociatedIconW")

	activityIconCacheMu sync.Mutex
	activityIconCache   = map[string]activityAppAsset{}
)

// activityAssetForProcess tenta primeiro o ícone da própria janela. Isso é mais
// confiável para Chrome, Spotify, AnyDesk e também para apps empacotados/UWP.
// Se a janela não expuser um HICON, usamos o executável como fallback.
func activityAssetForProcess(pid int, processName string, hwnd uintptr) activityAppAsset {
	processName = strings.TrimSpace(strings.TrimSuffix(processName, ".exe"))
	asset := activityAppAsset{
		ProcessName: processName,
		DisplayName: friendlyActivityProcessName(processName),
	}

	path := activityProcessExecutablePath(pid)
	cacheKey := strings.ToLower(strings.TrimSpace(path))
	if cacheKey == "" {
		cacheKey = "process:" + strings.ToLower(processName)
	}

	activityIconCacheMu.Lock()
	cached, ok := activityIconCache[cacheKey]
	activityIconCacheMu.Unlock()
	if ok && cached.IconData != "" {
		cached.ProcessName = processName
		cached.DisplayName = asset.DisplayName
		return cached
	}

	if iconData := activityWindowIconData(hwnd); iconData != "" {
		asset.IconData = iconData
		asset.IconSource = "window"
	} else if path != "" {
		if iconData := activityExecutableIconData(path); iconData != "" {
			asset.IconData = iconData
			asset.IconSource = "executable-shell"
		} else if iconData := activityExtractExecutableIconData(path); iconData != "" {
			asset.IconData = iconData
			asset.IconSource = "executable-resource"
		}
	}

	// Não memorize falhas. Alguns aplicativos só passam a expor o ícone depois
	// que a janela termina de inicializar; uma coleta posterior deve tentar de novo.
	if asset.IconData != "" {
		activityIconCacheMu.Lock()
		activityIconCache[cacheKey] = asset
		activityIconCacheMu.Unlock()
	}
	return asset
}

func activityProcessExecutablePath(pid int) string {
	if pid <= 0 {
		return ""
	}
	handle, _, _ := activityOpenProcess.Call(activityProcessQueryLimited, 0, uintptr(pid))
	if handle == 0 {
		return ""
	}
	defer procCloseHandle.Call(handle)

	buffer := make([]uint16, 32768)
	size := uint32(len(buffer))
	ok, _, _ := activityQueryFullProcessImageNameW.Call(
		handle,
		0,
		uintptr(unsafe.Pointer(&buffer[0])),
		uintptr(unsafe.Pointer(&size)),
	)
	if ok == 0 || size == 0 || int(size) > len(buffer) {
		return ""
	}
	return strings.TrimSpace(syscall.UTF16ToString(buffer[:size]))
}

func activityWindowIconData(hwnd uintptr) string {
	if hwnd == 0 {
		return ""
	}

	// WM_GETICON cobre a maioria dos aplicativos Win32/Chromium/Electron.
	for _, kind := range []uintptr{activityIconBig, activityIconSmall2, activityIconSmall} {
		var icon uintptr
		ok, _, _ := activitySendMessageTimeoutW.Call(
			hwnd,
			activityWMGetIcon,
			kind,
			0,
			activitySMTOAbortIfHung,
			250,
			uintptr(unsafe.Pointer(&icon)),
		)
		if ok != 0 && icon != 0 {
			if data := activityHIconData(syscall.Handle(icon)); data != "" {
				return data
			}
		}
	}

	// Algumas janelas não respondem WM_GETICON, mas mantêm o ícone na classe.
	for _, index := range []int32{activityGCLPHIcon, activityGCLPHIconSmall} {
		idx := index
		icon, _, _ := activityGetClassLongPtrW.Call(hwnd, uintptr(int64(idx)))
		if icon != 0 {
			if data := activityHIconData(syscall.Handle(icon)); data != "" {
				return data
			}
		}
	}
	return ""
}

func activityExecutableIconData(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return ""
	}
	pathPtr, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return ""
	}
	// Alguns shells devolvem um HICON diferente para LARGE e SMALL. Tentamos
	// ambos porque há aplicações em que apenas um deles é rasterizável.
	for _, flags := range []uintptr{activitySHGFIIcon, activitySHGFIIcon | activitySHGFISmallIcon} {
		var info activitySHFileInfoW
		ret, _, _ := activitySHGetFileInfoW.Call(
			uintptr(unsafe.Pointer(pathPtr)),
			0,
			uintptr(unsafe.Pointer(&info)),
			unsafe.Sizeof(info),
			flags,
		)
		if ret == 0 || info.HIcon == 0 {
			continue
		}
		data := activityHIconData(info.HIcon)
		activityDestroyIcon.Call(uintptr(info.HIcon))
		if data != "" {
			return data
		}
	}
	return activityAssociatedExecutableIconData(path)
}

func activityAssociatedExecutableIconData(path string) string {
	buffer, err := syscall.UTF16FromString(strings.TrimSpace(path))
	if err != nil || len(buffer) == 0 {
		return ""
	}
	var index uint16
	icon, _, _ := activityExtractAssociatedIconW.Call(
		0,
		uintptr(unsafe.Pointer(&buffer[0])),
		uintptr(unsafe.Pointer(&index)),
	)
	if icon == 0 {
		return ""
	}
	defer activityDestroyIcon.Call(icon)
	return activityHIconData(syscall.Handle(icon))
}

func activityExtractExecutableIconData(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return ""
	}
	pathPtr, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return ""
	}
	var largeIcon syscall.Handle
	var smallIcon syscall.Handle
	count, _, _ := activityExtractIconExW.Call(
		uintptr(unsafe.Pointer(pathPtr)),
		0,
		uintptr(unsafe.Pointer(&largeIcon)),
		uintptr(unsafe.Pointer(&smallIcon)),
		1,
	)
	if count == 0 {
		return ""
	}
	if largeIcon != 0 {
		defer activityDestroyIcon.Call(uintptr(largeIcon))
	}
	if smallIcon != 0 {
		defer activityDestroyIcon.Call(uintptr(smallIcon))
	}
	if largeIcon != 0 {
		if data := activityHIconData(largeIcon); data != "" {
			return data
		}
	}
	if smallIcon != 0 {
		return activityHIconData(smallIcon)
	}
	return ""
}

func activityHIconData(icon syscall.Handle) string {
	if icon == 0 {
		return ""
	}
	// Primeiro lê o bitmap de cor do próprio HICON. Isso evita uma falha que
	// ocorre em algumas máquinas quando DrawIconEx é usado em um DIBSection.
	if data := activityHIconBitmapData(icon); data != "" {
		return data
	}
	return activityHIconDrawData(icon)
}

func activityHIconBitmapData(icon syscall.Handle) string {
	var info activityIconInfo
	ok, _, _ := activityGetIconInfo.Call(uintptr(icon), uintptr(unsafe.Pointer(&info)))
	if ok == 0 {
		return ""
	}
	if info.HbmMask != 0 {
		defer activityDeleteObject.Call(uintptr(info.HbmMask))
	}
	if info.HbmColor != 0 {
		defer activityDeleteObject.Call(uintptr(info.HbmColor))
	}
	if info.HbmColor == 0 {
		return ""
	}

	var bitmap activityBitmap
	got, _, _ := activityGetObjectW.Call(
		uintptr(info.HbmColor),
		unsafe.Sizeof(bitmap),
		uintptr(unsafe.Pointer(&bitmap)),
	)
	width, height := int(bitmap.Width), int(bitmap.Height)
	if got == 0 || width <= 0 || height <= 0 || width > 256 || height > 256 {
		return ""
	}

	dc, _, _ := activityCreateCompatibleDC.Call(0)
	if dc == 0 {
		return ""
	}
	defer activityDeleteDC.Call(dc)

	raw := make([]byte, width*height*4)
	bmi := activityBitmapInfo{Header: activityBitmapInfoHeader{
		Size:        uint32(unsafe.Sizeof(activityBitmapInfoHeader{})),
		Width:       int32(width),
		Height:      -int32(height),
		Planes:      1,
		BitCount:    32,
		Compression: activityBINone,
		SizeImage:   uint32(len(raw)),
	}}
	lines, _, _ := activityGetDIBits.Call(
		dc,
		uintptr(info.HbmColor),
		0,
		uintptr(height),
		uintptr(unsafe.Pointer(&raw[0])),
		uintptr(unsafe.Pointer(&bmi)),
		activityDIBRGBColors,
	)
	if lines == 0 {
		return ""
	}

	img := image.NewNRGBA(image.Rect(0, 0, width, height))
	hasAlpha := false
	for i := 3; i < len(raw); i += 4 {
		if raw[i] != 0 {
			hasAlpha = true
			break
		}
	}
	// Sem alpha, o bitmap de cor sozinho não preserva corretamente a máscara
	// do HICON; nesse caso deixamos o caminho DrawIconEx fazer a composição.
	if !hasAlpha {
		return ""
	}
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			o := (y*width + x) * 4
			img.SetNRGBA(x, y, color.NRGBA{R: raw[o+2], G: raw[o+1], B: raw[o], A: raw[o+3]})
		}
	}
	return activityEncodeIconPNG(img)
}

func activityHIconDrawData(icon syscall.Handle) string {
	if icon == 0 {
		return ""
	}

	dc, _, _ := activityCreateCompatibleDC.Call(0)
	if dc == 0 {
		return ""
	}
	defer activityDeleteDC.Call(dc)

	var pixelPtr unsafe.Pointer
	bitmapInfo := activityBitmapInfo{Header: activityBitmapInfoHeader{
		Size:        uint32(unsafe.Sizeof(activityBitmapInfoHeader{})),
		Width:       activityIconSize,
		Height:      -activityIconSize,
		Planes:      1,
		BitCount:    32,
		Compression: activityBINone,
		SizeImage:   activityIconSize * activityIconSize * 4,
	}}
	bitmap, _, _ := activityCreateDIBSection.Call(
		dc,
		uintptr(unsafe.Pointer(&bitmapInfo)),
		activityDIBRGBColors,
		uintptr(unsafe.Pointer(&pixelPtr)),
		0,
		0,
	)
	if bitmap == 0 || pixelPtr == nil {
		return ""
	}
	defer activityDeleteObject.Call(bitmap)

	old, _, _ := activitySelectObject.Call(dc, bitmap)
	if old != 0 {
		defer activitySelectObject.Call(dc, old)
	}
	drawn, _, _ := activityDrawIconEx.Call(
		dc,
		0,
		0,
		uintptr(icon),
		activityIconSize,
		activityIconSize,
		0,
		0,
		activityDINormal,
	)
	if drawn == 0 {
		return ""
	}

	rawSize := activityIconSize * activityIconSize * 4
	raw := make([]byte, rawSize)
	copy(raw, unsafe.Slice((*byte)(pixelPtr), rawSize))

	img := image.NewNRGBA(image.Rect(0, 0, activityIconSize, activityIconSize))
	hasAlpha := false
	for index := 3; index < len(raw); index += 4 {
		if raw[index] != 0 {
			hasAlpha = true
			break
		}
	}
	for y := 0; y < activityIconSize; y++ {
		for x := 0; x < activityIconSize; x++ {
			offset := (y*activityIconSize + x) * 4
			b, g, r, a := raw[offset], raw[offset+1], raw[offset+2], raw[offset+3]
			if !hasAlpha && (r != 0 || g != 0 || b != 0) {
				a = 255
			}
			img.SetNRGBA(x, y, color.NRGBA{R: r, G: g, B: b, A: a})
		}
	}

	return activityEncodeIconPNG(img)
}

func activityEncodeIconPNG(img image.Image) string {
	if img == nil {
		return ""
	}
	var encoded bytes.Buffer
	encoder := png.Encoder{CompressionLevel: png.BestSpeed}
	if err := encoder.Encode(&encoded, img); err != nil || encoded.Len() == 0 || encoded.Len() > activityMaxIconBytes {
		return ""
	}
	return "data:image/png;base64," + base64.StdEncoding.EncodeToString(encoded.Bytes())
}

func friendlyActivityProcessName(name string) string {
	clean := strings.TrimSpace(strings.TrimSuffix(name, ".exe"))
	switch strings.ToLower(clean) {
	case "chrome":
		return "Google Chrome"
	case "msedge":
		return "Microsoft Edge"
	case "firefox":
		return "Mozilla Firefox"
	case "opera":
		return "Opera"
	case "opera_gx", "opera gx":
		return "Opera GX"
	case "spotify":
		return "Spotify"
	case "anydesk":
		return "AnyDesk"
	case "teamviewer":
		return "TeamViewer"
	case "code":
		return "Visual Studio Code"
	case "whatsapp":
		return "WhatsApp"
	case "discord":
		return "Discord"
	case "slack":
		return "Slack"
	case "explorer":
		return "Explorador de Arquivos"
	case "systemsettings":
		return "Configurações"
	case "applicationframehost":
		return "Aplicativos do Windows"
	case "textinputhost":
		return "Microsoft Text Input"
	case "searchhost", "searchapp":
		return "Pesquisa do Windows"
	case "startmenuexperiencehost":
		return "Menu Iniciar"
	case "taskmgr":
		return "Gerenciador de Tarefas"
	case "notepad":
		return "Bloco de Notas"
	case "calculatorapp", "calc":
		return "Calculadora"
	case "powershell", "pwsh":
		return "PowerShell"
	case "cmd":
		return "Prompt de Comando"
	case "nvidia overlay", "nvidiaoverlay", "nvidia share":
		return "NVIDIA Overlay"
	}
	if clean == "" {
		return "Aplicativo"
	}
	return clean
}
