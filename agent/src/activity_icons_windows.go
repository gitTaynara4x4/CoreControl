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
	activitySHGFIIcon    = 0x000000100
	activityDIBRGBColors = 0
	activityBINone       = 0
	activityDINormal     = 0x0003
	activityIconSize     = 32
	activityMaxIconBytes = 20 * 1024
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

type activityAppAsset struct {
	ProcessName string `json:"process_name"`
	DisplayName string `json:"display_name"`
	IconData    string `json:"icon_data,omitempty"`
}

var (
	activityIconShell32 = syscall.NewLazyDLL("shell32.dll")
	activityIconGDI32   = syscall.NewLazyDLL("gdi32.dll")
	activityIconUser32  = syscall.NewLazyDLL("user32.dll")

	activityQueryFullProcessImageNameW = kernel32.NewProc("QueryFullProcessImageNameW")
	activitySHGetFileInfoW             = activityIconShell32.NewProc("SHGetFileInfoW")
	activityCreateCompatibleDC         = activityIconGDI32.NewProc("CreateCompatibleDC")
	activityDeleteDC                   = activityIconGDI32.NewProc("DeleteDC")
	activityCreateDIBSection           = activityIconGDI32.NewProc("CreateDIBSection")
	activitySelectObject               = activityIconGDI32.NewProc("SelectObject")
	activityDeleteObject               = activityIconGDI32.NewProc("DeleteObject")
	activityDrawIconEx                 = activityIconUser32.NewProc("DrawIconEx")
	activityDestroyIcon                = activityIconUser32.NewProc("DestroyIcon")

	activityIconCacheMu sync.Mutex
	activityIconCache   = map[string]string{}
)

func activityAssetForProcess(pid int, processName string) activityAppAsset {
	processName = strings.TrimSpace(strings.TrimSuffix(processName, ".exe"))
	asset := activityAppAsset{
		ProcessName: processName,
		DisplayName: friendlyActivityProcessName(processName),
	}
	path := activityProcessExecutablePath(pid)
	if path == "" {
		return asset
	}
	key := strings.ToLower(path)
	activityIconCacheMu.Lock()
	iconData, cached := activityIconCache[key]
	activityIconCacheMu.Unlock()
	if cached {
		asset.IconData = iconData
		return asset
	}
	iconData = activityExecutableIconData(path)
	activityIconCacheMu.Lock()
	activityIconCache[key] = iconData
	activityIconCacheMu.Unlock()
	asset.IconData = iconData
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

func activityExecutableIconData(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return ""
	}
	pathPtr, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return ""
	}
	var info activitySHFileInfoW
	ret, _, _ := activitySHGetFileInfoW.Call(
		uintptr(unsafe.Pointer(pathPtr)),
		0,
		uintptr(unsafe.Pointer(&info)),
		unsafe.Sizeof(info),
		activitySHGFIIcon,
	)
	if ret == 0 || info.HIcon == 0 {
		return ""
	}
	defer activityDestroyIcon.Call(uintptr(info.HIcon))

	dc, _, _ := activityCreateCompatibleDC.Call(0)
	if dc == 0 {
		return ""
	}
	defer activityDeleteDC.Call(dc)

	var pixelPtr uintptr
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
	if bitmap == 0 || pixelPtr == 0 {
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
		uintptr(info.HIcon),
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
	copy(raw, unsafe.Slice((*byte)(unsafe.Pointer(pixelPtr)), rawSize))

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
			if !hasAlpha {
				if r != 0 || g != 0 || b != 0 {
					a = 255
				}
			}
			img.SetNRGBA(x, y, color.NRGBA{R: r, G: g, B: b, A: a})
		}
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
