//go:build windows

package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"
	"unsafe"
)

const (
	shgfiIcon       = 0x000000100
	shgfiSmallIcon  = 0x000000001
	diNormal        = 0x0003
	dibRGBColors    = 0
	biRGB           = 0
	srccopyActivity = 0x00CC0020
)

type shFileInfoW struct {
	HIcon       syscall.Handle
	IIcon       int32
	Attributes  uint32
	DisplayName [260]uint16
	TypeName    [80]uint16
}

type bitmapInfoHeaderActivity struct {
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

type bitmapInfoActivity struct {
	Header bitmapInfoHeaderActivity
	Colors [1]uint32
}

type cachedRasterIcon struct {
	Pixels  []byte
	Width   int32
	Height  int32
	ICOPath string
	Loading bool
	Failed  bool
}

var (
	activityShell32       = syscall.NewLazyDLL("shell32.dll")
	activitySHGetFileInfo = activityShell32.NewProc("SHGetFileInfoW")
	activityDrawIconEx    = user32.NewProc("DrawIconEx")
	activityDestroyIcon   = user32.NewProc("DestroyIcon")
	activityStretchDIBits = gdi32.NewProc("StretchDIBits")

	activityExeIconMu sync.Mutex
	activityExeIcons  = map[string]syscall.Handle{}

	activityWebIconMu sync.Mutex
	activityWebIcons  = map[string]*cachedRasterIcon{}
	activityHTTP      = &http.Client{Timeout: 4 * time.Second}
)

func drawProcessIcon(dc syscall.Handle, exePath string, r Rect) bool {
	exePath = strings.TrimSpace(exePath)
	if exePath == "" {
		return false
	}
	key := strings.ToLower(exePath)
	activityExeIconMu.Lock()
	icon := activityExeIcons[key]
	activityExeIconMu.Unlock()
	if icon == 0 {
		pathPtr, err := syscall.UTF16PtrFromString(exePath)
		if err != nil {
			return false
		}
		var info shFileInfoW
		ret, _, _ := activitySHGetFileInfo.Call(
			uintptr(unsafe.Pointer(pathPtr)),
			0,
			uintptr(unsafe.Pointer(&info)),
			unsafe.Sizeof(info),
			shgfiIcon|shgfiSmallIcon,
		)
		if ret == 0 || info.HIcon == 0 {
			return false
		}
		icon = info.HIcon
		activityExeIconMu.Lock()
		activityExeIcons[key] = icon
		activityExeIconMu.Unlock()
	}
	activityDrawIconEx.Call(
		uintptr(dc), uintptr(r.X), uintptr(r.Y), uintptr(icon),
		uintptr(r.W), uintptr(r.H), 0, 0, diNormal,
	)
	return true
}

func drawBrowserFavicon(dc syscall.Handle, favIconURL, domain string, r Rect) bool {
	favIconURL = strings.TrimSpace(favIconURL)
	if favIconURL == "" {
		drawSiteFallbackIcon(dc, domain, r)
		return false
	}
	activityWebIconMu.Lock()
	entry := activityWebIcons[favIconURL]
	if entry == nil {
		entry = &cachedRasterIcon{Loading: true}
		activityWebIcons[favIconURL] = entry
		go loadBrowserFavicon(favIconURL)
	}
	copyEntry := *entry
	activityWebIconMu.Unlock()

	if copyEntry.ICOPath != "" {
		pathPtr, err := syscall.UTF16PtrFromString(copyEntry.ICOPath)
		if err == nil {
			h, _, _ := procLoadImageW.Call(0, uintptr(unsafe.Pointer(pathPtr)), coreTunerImageIcon, uintptr(r.W), uintptr(r.H), coreTunerLRLoadFromFile)
			if h != 0 {
				activityDrawIconEx.Call(uintptr(dc), uintptr(r.X), uintptr(r.Y), h, uintptr(r.W), uintptr(r.H), 0, 0, diNormal)
				activityDestroyIcon.Call(h)
				return true
			}
		}
	}
	if len(copyEntry.Pixels) > 0 && copyEntry.Width > 0 && copyEntry.Height > 0 {
		drawRasterBGRA(dc, copyEntry.Pixels, copyEntry.Width, copyEntry.Height, r)
		return true
	}
	drawSiteFallbackIcon(dc, domain, r)
	return false
}

func loadBrowserFavicon(rawURL string) {
	data, contentType, err := readFaviconBytes(rawURL)
	loaded := &cachedRasterIcon{}
	if err == nil && len(data) > 0 {
		if isICO(data, contentType, rawURL) {
			if path := cacheICO(rawURL, data); path != "" {
				loaded.ICOPath = path
			}
		} else if img, _, decErr := image.Decode(bytes.NewReader(data)); decErr == nil {
			loaded.Pixels, loaded.Width, loaded.Height = imageToBGRA(img)
		}
	}
	if loaded.ICOPath == "" && len(loaded.Pixels) == 0 {
		loaded.Failed = true
	}
	activityWebIconMu.Lock()
	activityWebIcons[rawURL] = loaded
	activityWebIconMu.Unlock()
	if app != nil {
		app.invalidate()
	}
}

func readFaviconBytes(rawURL string) ([]byte, string, error) {
	if strings.HasPrefix(strings.ToLower(rawURL), "data:") {
		comma := strings.Index(rawURL, ",")
		if comma <= 0 {
			return nil, "", io.ErrUnexpectedEOF
		}
		header, body := rawURL[:comma], rawURL[comma+1:]
		contentType := strings.TrimPrefix(strings.Split(header, ";")[0], "data:")
		if strings.Contains(header, ";base64") {
			decoded, err := base64.StdEncoding.DecodeString(body)
			return decoded, contentType, err
		}
		return []byte(body), contentType, nil
	}
	if !strings.HasPrefix(rawURL, "https://") && !strings.HasPrefix(rawURL, "http://") {
		return nil, "", io.EOF
	}
	req, err := http.NewRequest(http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, "", err
	}
	req.Header.Set("User-Agent", "CoreControl/BrowserIcon")
	resp, err := activityHTTP.Do(req)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, resp.Header.Get("Content-Type"), io.EOF
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, 1024*1024))
	return data, resp.Header.Get("Content-Type"), err
}

func isICO(data []byte, contentType, rawURL string) bool {
	if len(data) >= 4 && data[0] == 0 && data[1] == 0 && data[2] == 1 && data[3] == 0 {
		return true
	}
	contentType = strings.ToLower(contentType)
	return strings.Contains(contentType, "icon") || strings.HasSuffix(strings.ToLower(strings.Split(rawURL, "?")[0]), ".ico")
}

func cacheICO(rawURL string, data []byte) string {
	base, err := os.UserCacheDir()
	if err != nil || base == "" {
		base = os.TempDir()
	}
	dir := filepath.Join(base, "CoreTuner", "BrowserIcons")
	if os.MkdirAll(dir, 0755) != nil {
		return ""
	}
	sum := sha256.Sum256([]byte(rawURL))
	path := filepath.Join(dir, hex.EncodeToString(sum[:10])+".ico")
	if existing, err := os.ReadFile(path); err == nil && bytes.Equal(existing, data) {
		return path
	}
	if os.WriteFile(path, data, 0600) != nil {
		return ""
	}
	return path
}

func imageToBGRA(img image.Image) ([]byte, int32, int32) {
	bounds := img.Bounds()
	w, h := bounds.Dx(), bounds.Dy()
	if w <= 0 || h <= 0 || w > 1024 || h > 1024 {
		return nil, 0, 0
	}
	pixels := make([]byte, w*h*4)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			r16, g16, b16, a16 := img.At(bounds.Min.X+x, bounds.Min.Y+y).RGBA()
			a := uint8(a16 >> 8)
			r := uint8(r16 >> 8)
			g := uint8(g16 >> 8)
			b := uint8(b16 >> 8)
			// Achata transparência em branco para o GDI clássico não criar halo preto.
			if a < 255 {
				r = uint8((uint16(r)*uint16(a) + 255*uint16(255-a)) / 255)
				g = uint8((uint16(g)*uint16(a) + 255*uint16(255-a)) / 255)
				b = uint8((uint16(b)*uint16(a) + 255*uint16(255-a)) / 255)
			}
			o := (y*w + x) * 4
			pixels[o+0], pixels[o+1], pixels[o+2], pixels[o+3] = b, g, r, 0
		}
	}
	return pixels, int32(w), int32(h)
}

func drawRasterBGRA(dc syscall.Handle, pixels []byte, width, height int32, r Rect) {
	if len(pixels) == 0 || width <= 0 || height <= 0 || r.W <= 0 || r.H <= 0 {
		return
	}
	info := bitmapInfoActivity{Header: bitmapInfoHeaderActivity{
		Size: uint32(unsafe.Sizeof(bitmapInfoHeaderActivity{})), Width: width, Height: -height,
		Planes: 1, BitCount: 32, Compression: biRGB, SizeImage: uint32(len(pixels)),
	}}
	activityStretchDIBits.Call(
		uintptr(dc), uintptr(r.X), uintptr(r.Y), uintptr(r.W), uintptr(r.H),
		0, 0, uintptr(width), uintptr(height),
		uintptr(unsafe.Pointer(&pixels[0])), uintptr(unsafe.Pointer(&info)), dibRGBColors, srccopyActivity,
	)
}

func drawSiteFallbackIcon(dc syscall.Handle, domain string, r Rect) {
	label := "•"
	domain = strings.TrimSpace(strings.TrimPrefix(strings.ToLower(domain), "www."))
	if domain != "" {
		for _, ch := range domain {
			if ch >= 'a' && ch <= 'z' {
				label = strings.ToUpper(string(ch))
				break
			}
		}
	}
	roundedBox(dc, r, rgb(239, 245, 255), rgb(218, 229, 246), 6)
	text(dc, label, r, app.fonts["small"], rgb(47, 124, 246), DT_CENTER|DT_VCENTER|DT_SINGLELINE)
}
