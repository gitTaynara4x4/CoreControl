//go:build windows

package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"unsafe"
)

type appearanceConfig struct {
	Mode string `json:"mode"`
}

var (
	themeDwmapi                = syscall.NewLazyDLL("dwmapi.dll")
	themeDwmSetWindowAttribute = themeDwmapi.NewProc("DwmSetWindowAttribute")
)

func (a *App) loadAppearance() {
	mode := "system"
	raw, err := os.ReadFile(filepath.Join(dataDir(), "appearance.json"))
	if err == nil {
		var cfg appearanceConfig
		if json.Unmarshal(raw, &cfg) == nil {
			switch strings.ToLower(strings.TrimSpace(cfg.Mode)) {
			case "light", "dark", "system":
				mode = strings.ToLower(strings.TrimSpace(cfg.Mode))
			}
		}
	}
	a.applyAppearance(mode, false)
}

func (a *App) applyAppearance(mode string, persist bool) {
	mode = strings.ToLower(strings.TrimSpace(mode))
	if mode != "light" && mode != "dark" && mode != "system" {
		mode = "system"
	}
	dark := mode == "dark" || (mode == "system" && windowsPrefersDark())
	a.mu.Lock()
	a.themeMode = mode
	a.themeDark = dark
	a.themeMenuOpen = false
	a.mu.Unlock()
	if persist {
		_ = os.MkdirAll(dataDir(), 0755)
		if raw, err := json.MarshalIndent(appearanceConfig{Mode: mode}, "", "  "); err == nil {
			_ = os.WriteFile(filepath.Join(dataDir(), "appearance.json"), raw, 0600)
		}
	}
	applyWindowDarkTitlebar(a.hwnd, dark)
	a.invalidate()
}

func (a *App) isDarkTheme() bool {
	if a == nil {
		return false
	}
	a.mu.RLock()
	dark := a.themeDark
	a.mu.RUnlock()
	return dark
}

func (a *App) appearanceMode() string {
	if a == nil {
		return "system"
	}
	a.mu.RLock()
	mode := a.themeMode
	a.mu.RUnlock()
	if mode == "" {
		return "system"
	}
	return mode
}

func windowsPrefersDark() bool {
	cmd := hiddenCommand("reg", "query", `HKCU\Software\Microsoft\Windows\CurrentVersion\Themes\Personalize`, "/v", "AppsUseLightTheme")
	out, err := cmd.Output()
	if err != nil {
		return false
	}
	s := strings.ToLower(string(out))
	idx := strings.Index(s, "appsuselighttheme")
	if idx < 0 {
		return false
	}
	tail := s[idx:]
	fields := strings.Fields(tail)
	for _, f := range fields {
		if strings.HasPrefix(f, "0x") {
			n, err := strconv.ParseUint(strings.TrimPrefix(f, "0x"), 16, 32)
			if err == nil {
				return n == 0
			}
		}
	}
	return false
}

func applyWindowDarkTitlebar(hwnd syscall.Handle, dark bool) {
	if hwnd == 0 {
		return
	}
	value := int32(0)
	if dark {
		value = 1
	}
	// DWMWA_USE_IMMERSIVE_DARK_MODE = 20 on modern Windows; 19 on some older builds.
	for _, attr := range []uintptr{20, 19} {
		r, _, _ := themeDwmSetWindowAttribute.Call(uintptr(hwnd), attr, uintptr(unsafe.Pointer(&value)), unsafe.Sizeof(value))
		if int32(r) == 0 {
			break
		}
	}
}

func colorParts(c uintptr) (byte, byte, byte) {
	return byte(c & 0xff), byte((c >> 8) & 0xff), byte((c >> 16) & 0xff)
}

func rawRGB(r, g, b byte) uintptr { return uintptr(r) | uintptr(g)<<8 | uintptr(b)<<16 }

func colorSpread(r, g, b byte) int {
	maxV, minV := int(r), int(r)
	if int(g) > maxV {
		maxV = int(g)
	}
	if int(b) > maxV {
		maxV = int(b)
	}
	if int(g) < minV {
		minV = int(g)
	}
	if int(b) < minV {
		minV = int(b)
	}
	return maxV - minV
}

func themeSurfaceColor(c uintptr) uintptr {
	if app == nil || !app.isDarkTheme() {
		return c
	}
	r, g, b := colorParts(c)
	avg := (int(r) + int(g) + int(b)) / 3
	spread := colorSpread(r, g, b)

	// Cores semânticas fortes permanecem reconhecíveis.
	if spread >= 48 && avg < 220 {
		return c
	}
	// Fundos muito claros e neutros viram superfícies ChatGPT-like.
	if spread < 18 {
		switch {
		case avg >= 252:
			return rawRGB(42, 42, 42) // cards
		case avg >= 246:
			return rawRGB(48, 48, 48) // hover / superfície elevada
		case avg >= 220:
			return rawRGB(58, 58, 58) // bordas/divisórias
		}
	}
	// Tintas claras (azul/verde/laranja muito suaves) viram tintas escuras discretas.
	if avg >= 220 {
		rr := byte(26 + int(r)*18/255)
		gg := byte(26 + int(g)*18/255)
		bb := byte(26 + int(b)*18/255)
		return rawRGB(rr, gg, bb)
	}
	return c
}

func themeBorderColor(c uintptr) uintptr {
	if app == nil || !app.isDarkTheme() {
		return c
	}
	r, g, b := colorParts(c)
	avg := (int(r) + int(g) + int(b)) / 3
	spread := colorSpread(r, g, b)
	if spread < 24 && avg >= 180 {
		return rawRGB(58, 58, 58)
	}
	if avg >= 220 {
		return themeSurfaceColor(c)
	}
	return c
}

func themeTextColor(c uintptr) uintptr {
	if app == nil || !app.isDarkTheme() {
		return c
	}
	r, g, b := colorParts(c)
	// Branco explícito (texto sobre azul/avatar) permanece branco.
	if r >= 245 && g >= 245 && b >= 245 {
		return c
	}
	avg := (int(r) + int(g) + int(b)) / 3
	spread := colorSpread(r, g, b)
	if spread >= 45 { // azul, verde, laranja, vermelho semânticos
		if avg < 75 {
			return rawRGB(byte(minInt(255, int(r)+50)), byte(minInt(255, int(g)+50)), byte(minInt(255, int(b)+50)))
		}
		return c
	}
	switch {
	case avg < 70:
		return rawRGB(245, 245, 245)
	case avg < 120:
		return rawRGB(214, 214, 218)
	case avg < 175:
		return rawRGB(161, 161, 170)
	default:
		return rawRGB(139, 139, 147)
	}
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func themeMainBackground() uintptr {
	if app != nil && app.isDarkTheme() {
		return rawRGB(33, 33, 33)
	}
	return rawRGB(248, 250, 253)
}

func themeShellBackground() uintptr {
	if app != nil && app.isDarkTheme() {
		return rawRGB(23, 23, 23)
	}
	return rawRGB(255, 255, 255)
}
