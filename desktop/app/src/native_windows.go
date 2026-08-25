//go:build windows

package main

import (
	"strings"
	"syscall"
	unicodeutf16 "unicode/utf16"
	"unsafe"
)

const (
	nativeHkeyLocalMachine = uintptr(0x80000002)
	nativeKeyRead          = 0x20019
	nativeKeyWow6464       = 0x0100
	th32csSnapProcess      = 0x00000002
	processQueryLimited    = 0x1000
	processVMRead          = 0x0010
)

type processEntry32 struct {
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

type processMemoryCounters struct {
	Cb                         uint32
	PageFaultCount             uint32
	PeakWorkingSetSize         uintptr
	WorkingSetSize             uintptr
	QuotaPeakPagedPoolUsage    uintptr
	QuotaPagedPoolUsage        uintptr
	QuotaPeakNonPagedPoolUsage uintptr
	QuotaNonPagedPoolUsage     uintptr
	PagefileUsage              uintptr
	PeakPagefileUsage          uintptr
}

var (
	nativeAdvapi32 = syscall.NewLazyDLL("advapi32.dll")
	nativeKernel32 = syscall.NewLazyDLL("kernel32.dll")
	nativePsapi    = syscall.NewLazyDLL("psapi.dll")
	nativeWinmm    = syscall.NewLazyDLL("winmm.dll")

	nativeRegOpenKeyExW              = nativeAdvapi32.NewProc("RegOpenKeyExW")
	nativeRegQueryValueExW           = nativeAdvapi32.NewProc("RegQueryValueExW")
	nativeRegCloseKey                = nativeAdvapi32.NewProc("RegCloseKey")
	nativeCreateSnapshot             = nativeKernel32.NewProc("CreateToolhelp32Snapshot")
	nativeProcess32FirstW            = nativeKernel32.NewProc("Process32FirstW")
	nativeProcess32NextW             = nativeKernel32.NewProc("Process32NextW")
	nativeOpenProcess                = nativeKernel32.NewProc("OpenProcess")
	nativeCloseHandle                = nativeKernel32.NewProc("CloseHandle")
	nativeQueryFullProcessImageNameW = nativeKernel32.NewProc("QueryFullProcessImageNameW")
	nativeGetProcessMemory           = nativePsapi.NewProc("GetProcessMemoryInfo")
	nativeWaveOutGetNumDevs          = nativeWinmm.NewProc("waveOutGetNumDevs")
	nativeWaveInGetNumDevs           = nativeWinmm.NewProc("waveInGetNumDevs")
)

func nativeSystemDetails() map[string]string {
	values := map[string]string{
		"Manufacturer": nativeRegString(`HARDWARE\DESCRIPTION\System\BIOS`, "SystemManufacturer"),
		"Model":        nativeRegString(`HARDWARE\DESCRIPTION\System\BIOS`, "SystemProductName"),
		"Serial":       nativeRegString(`HARDWARE\DESCRIPTION\System\BIOS`, "SystemSerialNumber"),
		"CPU":          nativeRegString(`HARDWARE\DESCRIPTION\System\CentralProcessor\0`, "ProcessorNameString"),
		"OS":           nativeOSName(),
		"Disk":         "Unidade principal (C:)",
		"DiskType":     "Armazenamento local",
	}
	return values
}

func nativeOSName() string {
	product := nativeRegString(`SOFTWARE\Microsoft\Windows NT\CurrentVersion`, "ProductName")
	display := nativeRegString(`SOFTWARE\Microsoft\Windows NT\CurrentVersion`, "DisplayVersion")
	arch := "64 bits"
	if product == "" {
		product = "Windows"
	}
	if display != "" {
		return strings.TrimSpace(product + " " + display + " " + arch)
	}
	return strings.TrimSpace(product + " " + arch)
}

func nativeAudioStatus() (bool, bool) {
	outputs, _, _ := nativeWaveOutGetNumDevs.Call()
	inputs, _, _ := nativeWaveInGetNumDevs.Call()
	return outputs > 0, inputs > 0
}

func nativeProcesses() []ProcessInfo {
	snapshot, _, _ := nativeCreateSnapshot.Call(th32csSnapProcess, 0)
	if snapshot == 0 || snapshot == ^uintptr(0) {
		return nil
	}
	defer nativeCloseHandle.Call(snapshot)

	entry := processEntry32{Size: uint32(unsafe.Sizeof(processEntry32{}))}
	ok, _, _ := nativeProcess32FirstW.Call(snapshot, uintptr(unsafe.Pointer(&entry)))
	if ok == 0 {
		return nil
	}

	processes := make([]ProcessInfo, 0, 64)
	for {
		name := strings.TrimSpace(string(unicodeutf16.Decode(trimUTF16(entry.ExeFile[:]))))
		if name != "" {
			memoryMB := processMemoryMB(entry.ProcessID)
			processes = append(processes, ProcessInfo{
				Name:      strings.TrimSuffix(name, ".exe"),
				PID:       int(entry.ProcessID),
				ParentPID: int(entry.ParentProcessID),
				ExePath:   processExecutablePath(entry.ProcessID),
				MemoryMB:  memoryMB,
				CPU:       0,
			})
		}
		entry.Size = uint32(unsafe.Sizeof(processEntry32{}))
		next, _, _ := nativeProcess32NextW.Call(snapshot, uintptr(unsafe.Pointer(&entry)))
		if next == 0 {
			break
		}
	}
	return processes
}

func processExecutablePath(pid uint32) string {
	handle, _, _ := nativeOpenProcess.Call(processQueryLimited, 0, uintptr(pid))
	if handle == 0 {
		return ""
	}
	defer nativeCloseHandle.Call(handle)
	buffer := make([]uint16, 32768)
	size := uint32(len(buffer))
	ok, _, _ := nativeQueryFullProcessImageNameW.Call(
		handle,
		0,
		uintptr(unsafe.Pointer(&buffer[0])),
		uintptr(unsafe.Pointer(&size)),
	)
	if ok == 0 || size == 0 || int(size) > len(buffer) {
		return ""
	}
	return strings.TrimSpace(string(unicodeutf16.Decode(buffer[:size])))
}

func processMemoryMB(pid uint32) float64 {
	handle, _, _ := nativeOpenProcess.Call(processQueryLimited|processVMRead, 0, uintptr(pid))
	if handle == 0 {
		return 0
	}
	defer nativeCloseHandle.Call(handle)
	counters := processMemoryCounters{Cb: uint32(unsafe.Sizeof(processMemoryCounters{}))}
	ok, _, _ := nativeGetProcessMemory.Call(
		handle,
		uintptr(unsafe.Pointer(&counters)),
		uintptr(counters.Cb),
	)
	if ok == 0 {
		return 0
	}
	return float64(counters.WorkingSetSize) / (1 << 20)
}

func trimUTF16(values []uint16) []uint16 {
	for index, value := range values {
		if value == 0 {
			return values[:index]
		}
	}
	return values
}

func nativeRegString(subkey, name string) string {
	subkeyPtr, _ := syscall.UTF16PtrFromString(subkey)
	var key uintptr
	result, _, _ := nativeRegOpenKeyExW.Call(
		nativeHkeyLocalMachine,
		uintptr(unsafe.Pointer(subkeyPtr)),
		0,
		nativeKeyRead|nativeKeyWow6464,
		uintptr(unsafe.Pointer(&key)),
	)
	if result != 0 || key == 0 {
		return ""
	}
	defer nativeRegCloseKey.Call(key)

	namePtr, _ := syscall.UTF16PtrFromString(name)
	var valueType uint32
	var size uint32
	result, _, _ = nativeRegQueryValueExW.Call(
		key,
		uintptr(unsafe.Pointer(namePtr)),
		0,
		uintptr(unsafe.Pointer(&valueType)),
		0,
		uintptr(unsafe.Pointer(&size)),
	)
	if result != 0 || size < 2 || size > 1<<20 {
		return ""
	}
	buffer := make([]byte, size)
	result, _, _ = nativeRegQueryValueExW.Call(
		key,
		uintptr(unsafe.Pointer(namePtr)),
		0,
		uintptr(unsafe.Pointer(&valueType)),
		uintptr(unsafe.Pointer(&buffer[0])),
		uintptr(unsafe.Pointer(&size)),
	)
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
