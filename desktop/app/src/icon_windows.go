//go:build windows

package main

import (
	"bytes"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"os"
	"path/filepath"
	"syscall"
	"unsafe"
)

const (
	coreTunerImageIcon      = 1
	coreTunerLRLoadFromFile = 0x00000010
	coreTunerWMSetIcon      = 0x0080
	coreTunerIconSmall      = 0
	coreTunerIconBig        = 1
)

//go:embed coretuner.ico
var coreTunerIconBytes []byte

var procLoadImageW = user32.NewProc("LoadImageW")

// coreTunerWindowIcons materializa o .ico incorporado em uma pasta estável do
// usuário e carrega versões adequadas para a janela e para a barra de tarefas.
// O arquivo recebe um nome baseado no conteúdo, evitando cache de ícone antigo
// do Windows quando a identidade visual for atualizada.
func coreTunerWindowIcons() (syscall.Handle, syscall.Handle) {
	path := coreTunerIconPath()
	if path == "" {
		return 0, 0
	}
	load := func(size uintptr) syscall.Handle {
		handle, _, _ := procLoadImageW.Call(
			0,
			uintptr(unsafe.Pointer(utf16(path))),
			coreTunerImageIcon,
			size,
			size,
			coreTunerLRLoadFromFile,
		)
		return syscall.Handle(handle)
	}
	large := load(32)
	small := load(16)
	if small == 0 {
		small = large
	}
	return large, small
}

func coreTunerIconPath() string {
	if len(coreTunerIconBytes) < 6 ||
		coreTunerIconBytes[0] != 0 || coreTunerIconBytes[1] != 0 ||
		coreTunerIconBytes[2] != 1 || coreTunerIconBytes[3] != 0 {
		return ""
	}
	base, err := os.UserCacheDir()
	if err != nil || base == "" {
		base = os.TempDir()
	}
	directory := filepath.Join(base, "CoreTuner", "Assets")
	if err := os.MkdirAll(directory, 0755); err != nil {
		return ""
	}
	digest := sha256.Sum256(coreTunerIconBytes)
	filename := "CoreTuner-" + hex.EncodeToString(digest[:8]) + ".ico"
	path := filepath.Join(directory, filename)
	if current, err := os.ReadFile(path); err == nil && bytes.Equal(current, coreTunerIconBytes) {
		return path
	}
	temporary := path + ".tmp"
	if err := os.WriteFile(temporary, coreTunerIconBytes, 0644); err != nil {
		return ""
	}
	if err := os.Rename(temporary, path); err != nil {
		_ = os.Remove(temporary)
		if current, readErr := os.ReadFile(path); readErr == nil && bytes.Equal(current, coreTunerIconBytes) {
			return path
		}
		return ""
	}
	return path
}

func applyCoreTunerWindowIcons(hwnd syscall.Handle, large, small syscall.Handle) {
	if hwnd == 0 {
		return
	}
	if large != 0 {
		procSendMessage.Call(uintptr(hwnd), coreTunerWMSetIcon, coreTunerIconBig, uintptr(large))
	}
	if small != 0 {
		procSendMessage.Call(uintptr(hwnd), coreTunerWMSetIcon, coreTunerIconSmall, uintptr(small))
	}
}
