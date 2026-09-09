//go:build windows

package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"unsafe"
)

const gatewayTaskName = "CoreControl Box"

func installGateway(token, serverURL string) error {
	if !isElevatedWindows() {
		return relaunchElevated(token, serverURL)
	}

	source, err := os.Executable()
	if err != nil {
		return err
	}
	configPath := gatewayConfigPath()
	installDir := filepath.Dir(configPath)
	if err := os.MkdirAll(installDir, 0700); err != nil {
		return err
	}
	destination := filepath.Join(installDir, "CoreControlGateway.exe")
	if !samePath(source, destination) {
		raw, err := os.ReadFile(source)
		if err != nil {
			return err
		}
		if err := os.WriteFile(destination, raw, 0755); err != nil {
			return err
		}
	}

	cfg := Config{ServerURL: serverURL, EnrollmentToken: token}
	if err := saveConfig(configPath, cfg); err != nil {
		return err
	}

	taskCommand := fmt.Sprintf(`"%s" --run`, destination)
	create := exec.Command("schtasks.exe", "/Create", "/TN", gatewayTaskName, "/TR", taskCommand, "/SC", "ONSTART", "/RU", "SYSTEM", "/RL", "HIGHEST", "/F")
	create.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000}
	if output, err := create.CombinedOutput(); err != nil {
		return fmt.Errorf("não foi possível registrar inicialização automática: %s", strings.TrimSpace(string(output)))
	}

	run := exec.Command("schtasks.exe", "/Run", "/TN", gatewayTaskName)
	run.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000}
	if output, err := run.CombinedOutput(); err != nil {
		return fmt.Errorf("Gateway instalado, mas não iniciou: %s", strings.TrimSpace(string(output)))
	}
	showMessage("CoreControl Box", "Gateway instalado. Ele ficará online automaticamente sempre que este equipamento estiver ligado.", false)
	return nil
}

func uninstallGateway() error {
	if !isElevatedWindows() {
		return errors.New("execute como administrador para remover o Gateway")
	}
	cmd := exec.Command("schtasks.exe", "/Delete", "/TN", gatewayTaskName, "/F")
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000}
	_ = cmd.Run()
	return os.RemoveAll(filepath.Dir(gatewayConfigPath()))
}

func isElevatedWindows() bool {
	cmd := exec.Command("net.exe", "session")
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000}
	return cmd.Run() == nil
}

func relaunchElevated(token, serverURL string) error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	shell32 := syscall.NewLazyDLL("shell32.dll")
	proc := shell32.NewProc("ShellExecuteW")
	verb, _ := syscall.UTF16PtrFromString("runas")
	file, _ := syscall.UTF16PtrFromString(exe)
	args, _ := syscall.UTF16PtrFromString(fmt.Sprintf(`--install-token %q --server %q`, token, serverURL))
	result, _, callErr := proc.Call(0, uintptr(unsafe.Pointer(verb)), uintptr(unsafe.Pointer(file)), uintptr(unsafe.Pointer(args)), 0, 0)
	if result <= 32 {
		return fmt.Errorf("elevação administrativa recusada: %v", callErr)
	}
	return nil
}

func showMessage(title, message string, isError bool) {
	user32 := syscall.NewLazyDLL("user32.dll")
	proc := user32.NewProc("MessageBoxW")
	titlePtr, _ := syscall.UTF16PtrFromString(title)
	messagePtr, _ := syscall.UTF16PtrFromString(message)
	flags := uintptr(0x00000040) // information
	if isError {
		flags = 0x00000010
	}
	proc.Call(0, uintptr(unsafe.Pointer(messagePtr)), uintptr(unsafe.Pointer(titlePtr)), flags)
}

func samePath(a, b string) bool {
	aa, _ := filepath.Abs(a)
	bb, _ := filepath.Abs(b)
	return strings.EqualFold(filepath.Clean(aa), filepath.Clean(bb))
}
