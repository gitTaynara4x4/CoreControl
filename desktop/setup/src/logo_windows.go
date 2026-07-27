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

//go:embed coretuner-logo.bmp
var coreTunerLogoBytes []byte

func coreTunerLogoBitmap() syscall.Handle {
	path := coreTunerLogoPath()
	if path == "" {
		return 0
	}
	bitmap, _, _ := procLoadImageW.Call(
		0,
		uintptr(unsafe.Pointer(utf16(path))),
		IMAGE_BITMAP,
		0,
		0,
		coreTunerLRLoadFromFile,
	)
	return syscall.Handle(bitmap)
}

func coreTunerLogoPath() string {
	if len(coreTunerLogoBytes) < 54 || coreTunerLogoBytes[0] != 'B' || coreTunerLogoBytes[1] != 'M' {
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
	digest := sha256.Sum256(coreTunerLogoBytes)
	path := filepath.Join(directory, "CoreTuner-Logo-"+hex.EncodeToString(digest[:8])+".bmp")
	if current, err := os.ReadFile(path); err == nil && bytes.Equal(current, coreTunerLogoBytes) {
		return path
	}
	temporary := path + ".tmp"
	if err := os.WriteFile(temporary, coreTunerLogoBytes, 0644); err != nil {
		return ""
	}
	if err := os.Rename(temporary, path); err != nil {
		_ = os.Remove(temporary)
		if current, readErr := os.ReadFile(path); readErr == nil && bytes.Equal(current, coreTunerLogoBytes) {
			return path
		}
		return ""
	}
	return path
}

func createCoreTunerLogo(parent syscall.Handle, x, y, width, height int32) (syscall.Handle, syscall.Handle) {
	control := createControl("STATIC", "", WS_CHILD|WS_VISIBLE|SS_BITMAP, x, y, width, height, parent, 0)
	bitmap := coreTunerLogoBitmap()
	if control != 0 && bitmap != 0 {
		procSendMessage.Call(uintptr(control), STM_SETIMAGE, IMAGE_BITMAP, uintptr(bitmap))
	}
	return control, bitmap
}
