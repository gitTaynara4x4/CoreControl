//go:build windows

package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"
	unicodeutf16 "unicode/utf16"
	"unsafe"
)

const (
	createNoWindowSetup      = 0x08000000
	th32csSnapProcessSetup   = 0x00000002
	processQueryLimitedSetup = 0x1000
	processTerminateSetup    = 0x0001
	seeMaskNoCloseProcess    = 0x00000040
	waitObject0              = 0x00000000
	waitTimeout              = 0x00000102
	scManagerConnectSetup    = 0x0001
	serviceQueryStatusSetup  = 0x0004
	scStatusProcessInfoSetup = 0
	serviceRunningSetup      = 4
)

type shellExecuteInfo struct {
	CbSize       uint32
	FMask        uint32
	Hwnd         uintptr
	LpVerb       *uint16
	LpFile       *uint16
	LpParameters *uint16
	LpDirectory  *uint16
	NShow        int32
	HInstApp     uintptr
	LpIDList     uintptr
	LpClass      *uint16
	HkeyClass    uintptr
	DwHotKey     uint32
	HIcon        uintptr
	HProcess     uintptr
}

type serviceStatusProcessSetup struct {
	ServiceType, CurrentState, ControlsAccepted   uint32
	Win32ExitCode, ServiceSpecificExitCode        uint32
	CheckPoint, WaitHint, ProcessID, ServiceFlags uint32
}

type processEntrySetup struct {
	Size            uint32
	Usage           uint32
	ProcessID       uint32
	DefaultHeapID   uintptr
	ModuleID        uint32
	Threads         uint32
	ParentProcessID uint32
	PriClassBase    int32
	Flags           uint32
	ExeFile         [260]uint16
}

var (
	setupAdvapi32             = syscall.NewLazyDLL("advapi32.dll")
	setupKernel32             = syscall.NewLazyDLL("kernel32.dll")
	setupShell32              = syscall.NewLazyDLL("shell32.dll")
	setupCreateSnapshot       = setupKernel32.NewProc("CreateToolhelp32Snapshot")
	setupProcess32FirstW      = setupKernel32.NewProc("Process32FirstW")
	setupProcess32NextW       = setupKernel32.NewProc("Process32NextW")
	setupOpenProcess          = setupKernel32.NewProc("OpenProcess")
	setupQueryFullProcessName = setupKernel32.NewProc("QueryFullProcessImageNameW")
	setupTerminateProcess     = setupKernel32.NewProc("TerminateProcess")
	setupCloseHandle          = setupKernel32.NewProc("CloseHandle")
	setupWaitForSingleObject  = setupKernel32.NewProc("WaitForSingleObject")
	setupGetExitCodeProcess   = setupKernel32.NewProc("GetExitCodeProcess")
	setupShellExecuteExW      = setupShell32.NewProc("ShellExecuteExW")
	setupRegOpenKeyExW        = setupAdvapi32.NewProc("RegOpenKeyExW")
	setupRegQueryValueExW     = setupAdvapi32.NewProc("RegQueryValueExW")
	setupRegCloseKey          = setupAdvapi32.NewProc("RegCloseKey")
	setupOpenSCManagerW       = setupAdvapi32.NewProc("OpenSCManagerW")
	setupOpenServiceW         = setupAdvapi32.NewProc("OpenServiceW")
	setupQueryServiceStatusEx = setupAdvapi32.NewProc("QueryServiceStatusEx")
	setupCloseServiceHandle   = setupAdvapi32.NewProc("CloseServiceHandle")
)

func hiddenCommand(name string, args ...string) *exec.Cmd {
	cmd := exec.Command(name, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: createNoWindowSetup,
	}
	return cmd
}

// stopExistingAgent encerra apenas processos chamados CoreTunerAgent.exe instalados
// em uma pasta do CoreTuner. Isso permite atualizar o binário sem usar taskkill,
// PowerShell ou abrir qualquer janela de console.
func stopExistingAgent(expectedPath string) {
	snapshot, _, _ := setupCreateSnapshot.Call(th32csSnapProcessSetup, 0)
	if snapshot == 0 || snapshot == ^uintptr(0) {
		return
	}
	defer setupCloseHandle.Call(snapshot)

	expected := strings.ToLower(filepath.Clean(expectedPath))
	entry := processEntrySetup{Size: uint32(unsafe.Sizeof(processEntrySetup{}))}
	ok, _, _ := setupProcess32FirstW.Call(snapshot, uintptr(unsafe.Pointer(&entry)))
	if ok == 0 {
		return
	}
	stopped := false
	for {
		name := strings.ToLower(string(unicodeutf16.Decode(trimSetupUTF16(entry.ExeFile[:]))))
		if name == "coretuneragent.exe" {
			handle, _, _ := setupOpenProcess.Call(
				processQueryLimitedSetup|processTerminateSetup,
				0,
				uintptr(entry.ProcessID),
			)
			if handle != 0 {
				path := strings.ToLower(queryProcessPath(handle))
				if path == expected || strings.Contains(path, `\coretuner\`) {
					setupTerminateProcess.Call(handle, 0)
					stopped = true
				}
				setupCloseHandle.Call(handle)
			}
		}
		entry.Size = uint32(unsafe.Sizeof(processEntrySetup{}))
		next, _, _ := setupProcess32NextW.Call(snapshot, uintptr(unsafe.Pointer(&entry)))
		if next == 0 {
			break
		}
	}
	if stopped {
		time.Sleep(800 * time.Millisecond)
	}
}

func queryProcessPath(handle uintptr) string {
	buffer := make([]uint16, 32768)
	size := uint32(len(buffer))
	ok, _, _ := setupQueryFullProcessName.Call(
		handle,
		0,
		uintptr(unsafe.Pointer(&buffer[0])),
		uintptr(unsafe.Pointer(&size)),
	)
	if ok == 0 || size == 0 {
		return ""
	}
	return string(unicodeutf16.Decode(buffer[:size]))
}

func trimSetupUTF16(values []uint16) []uint16 {
	for index, value := range values {
		if value == 0 {
			return values[:index]
		}
	}
	return values
}

func runElevatedAndWait(executable, parameters string, timeout time.Duration) error {
	verb, _ := syscall.UTF16PtrFromString("runas")
	file, _ := syscall.UTF16PtrFromString(executable)
	params, _ := syscall.UTF16PtrFromString(parameters)
	sei := shellExecuteInfo{
		CbSize:       uint32(unsafe.Sizeof(shellExecuteInfo{})),
		FMask:        seeMaskNoCloseProcess,
		LpVerb:       verb,
		LpFile:       file,
		LpParameters: params,
		NShow:        0,
	}
	ok, _, callErr := setupShellExecuteExW.Call(uintptr(unsafe.Pointer(&sei)))
	if ok == 0 || sei.HProcess == 0 {
		return fmt.Errorf("a autorização de administrador foi cancelada ou falhou: %v", callErr)
	}
	defer setupCloseHandle.Call(sei.HProcess)
	milliseconds := uint32(timeout / time.Millisecond)
	result, _, _ := setupWaitForSingleObject.Call(sei.HProcess, uintptr(milliseconds))
	if result == waitTimeout {
		return fmt.Errorf("a instalação remota excedeu o tempo limite")
	}
	if result != waitObject0 {
		return fmt.Errorf("não foi possível aguardar a instalação remota")
	}
	var exitCode uint32
	if ok, _, _ := setupGetExitCodeProcess.Call(sei.HProcess, uintptr(unsafe.Pointer(&exitCode))); ok != 0 && exitCode != 0 {
		return fmt.Errorf("o instalador remoto terminou com código %d", exitCode)
	}
	return nil
}

func remoteAgentStatus() (bool, bool) {
	serviceNames := []string{"Mesh Agent", "MeshAgent", "MeshCentral Agent"}
	for _, serviceName := range serviceNames {
		if running, known := setupServiceRunning(serviceName); known {
			return true, running
		}
	}
	if setupProcessExists("meshagent.exe") {
		return true, true
	}
	paths := []string{
		filepath.Join(os.Getenv("ProgramFiles"), "Mesh Agent", "MeshAgent.exe"),
		filepath.Join(os.Getenv("ProgramFiles(x86)"), "Mesh Agent", "MeshAgent.exe"),
	}
	for _, path := range paths {
		if _, err := os.Stat(path); err == nil {
			return true, false
		}
	}
	return false, false
}

func setupServiceRunning(name string) (bool, bool) {
	manager, _, _ := setupOpenSCManagerW.Call(0, 0, scManagerConnectSetup)
	if manager == 0 {
		return false, false
	}
	defer setupCloseServiceHandle.Call(manager)
	namePtr, _ := syscall.UTF16PtrFromString(name)
	service, _, _ := setupOpenServiceW.Call(manager, uintptr(unsafe.Pointer(namePtr)), serviceQueryStatusSetup)
	if service == 0 {
		return false, false
	}
	defer setupCloseServiceHandle.Call(service)
	var status serviceStatusProcessSetup
	var needed uint32
	ok, _, _ := setupQueryServiceStatusEx.Call(service, scStatusProcessInfoSetup, uintptr(unsafe.Pointer(&status)), unsafe.Sizeof(status), uintptr(unsafe.Pointer(&needed)))
	if ok == 0 {
		return false, false
	}
	return status.CurrentState == serviceRunningSetup, true
}

func setupProcessExists(target string) bool {
	snapshot, _, _ := setupCreateSnapshot.Call(th32csSnapProcessSetup, 0)
	if snapshot == 0 || snapshot == ^uintptr(0) {
		return false
	}
	defer setupCloseHandle.Call(snapshot)
	entry := processEntrySetup{Size: uint32(unsafe.Sizeof(processEntrySetup{}))}
	ok, _, _ := setupProcess32FirstW.Call(snapshot, uintptr(unsafe.Pointer(&entry)))
	for ok != 0 {
		name := strings.ToLower(string(unicodeutf16.Decode(trimSetupUTF16(entry.ExeFile[:]))))
		if name == strings.ToLower(target) {
			return true
		}
		entry.Size = uint32(unsafe.Sizeof(processEntrySetup{}))
		ok, _, _ = setupProcess32NextW.Call(snapshot, uintptr(unsafe.Pointer(&entry)))
	}
	return false
}

func collectMachineNative() (Machine, error) {
	hostname, _ := os.Hostname()
	machine := Machine{
		DeviceUID:    setupRegString(`SOFTWARE\Microsoft\Cryptography`, "MachineGuid"),
		Hostname:     hostname,
		Manufacturer: setupRegString(`HARDWARE\DESCRIPTION\System\BIOS`, "SystemManufacturer"),
		Model:        setupRegString(`HARDWARE\DESCRIPTION\System\BIOS`, "SystemProductName"),
		SerialNumber: setupRegString(`HARDWARE\DESCRIPTION\System\BIOS`, "SystemSerialNumber"),
		OSName:       setupRegString(`SOFTWARE\Microsoft\Windows NT\CurrentVersion`, "ProductName"),
		OSVersion:    setupRegString(`SOFTWARE\Microsoft\Windows NT\CurrentVersion`, "DisplayVersion"),
	}
	if machine.DeviceUID == "" {
		return machine, fmt.Errorf("o Windows não forneceu a identificação única do computador")
	}
	if machine.OSVersion == "" {
		machine.OSVersion = setupRegString(`SOFTWARE\Microsoft\Windows NT\CurrentVersion`, "CurrentBuildNumber")
	}
	return machine, nil
}

func setupRegString(subkey, name string) string {
	const hkeyLocalMachine = uintptr(0x80000002)
	const keyRead = 0x20019
	const keyWow6464 = 0x0100
	subkeyPtr, _ := syscall.UTF16PtrFromString(subkey)
	var key uintptr
	result, _, _ := setupRegOpenKeyExW.Call(
		hkeyLocalMachine,
		uintptr(unsafe.Pointer(subkeyPtr)),
		0,
		keyRead|keyWow6464,
		uintptr(unsafe.Pointer(&key)),
	)
	if result != 0 || key == 0 {
		return ""
	}
	defer setupRegCloseKey.Call(key)
	namePtr, _ := syscall.UTF16PtrFromString(name)
	var valueType uint32
	var size uint32
	result, _, _ = setupRegQueryValueExW.Call(key, uintptr(unsafe.Pointer(namePtr)), 0, uintptr(unsafe.Pointer(&valueType)), 0, uintptr(unsafe.Pointer(&size)))
	if result != 0 || size < 2 || size > 1<<20 {
		return ""
	}
	buffer := make([]byte, size)
	result, _, _ = setupRegQueryValueExW.Call(key, uintptr(unsafe.Pointer(namePtr)), 0, uintptr(unsafe.Pointer(&valueType)), uintptr(unsafe.Pointer(&buffer[0])), uintptr(unsafe.Pointer(&size)))
	if result != 0 {
		return ""
	}
	units := make([]uint16, 0, size/2)
	for index := 0; index+1 < int(size); index += 2 {
		unit := uint16(buffer[index]) | uint16(buffer[index+1])<<8
		if unit == 0 {
			break
		}
		units = append(units, unit)
	}
	return strings.TrimSpace(string(unicodeutf16.Decode(units)))
}
