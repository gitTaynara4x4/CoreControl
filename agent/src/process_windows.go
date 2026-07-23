//go:build windows

package main

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"
	"unicode/utf16"
	"unsafe"
)

const (
	hkeyLocalMachine = uintptr(0x80000002)
	keyRead          = 0x20019
	keyWow6464       = 0x0100

	scManagerConnect    = 0x0001
	serviceQueryStatus  = 0x0004
	scStatusProcessInfo = 0
	serviceRunning      = 4
)

type memoryStatusEx struct {
	Length                                             uint32
	MemoryLoad                                         uint32
	TotalPhys, AvailPhys, TotalPageFile, AvailPageFile uint64
	TotalVirtual, AvailVirtual, AvailExtendedVirtual   uint64
}

type fileTime struct{ LowDateTime, HighDateTime uint32 }

type serviceStatusProcess struct {
	ServiceType, CurrentState, ControlsAccepted   uint32
	Win32ExitCode, ServiceSpecificExitCode        uint32
	CheckPoint, WaitHint, ProcessID, ServiceFlags uint32
}

var (
	kernel32 = syscall.NewLazyDLL("kernel32.dll")
	advapi32 = syscall.NewLazyDLL("advapi32.dll")

	procCreateMutexW         = kernel32.NewProc("CreateMutexW")
	procCloseHandle          = kernel32.NewProc("CloseHandle")
	procGlobalMemoryStatusEx = kernel32.NewProc("GlobalMemoryStatusEx")
	procGetDiskFreeSpaceExW  = kernel32.NewProc("GetDiskFreeSpaceExW")
	procGetSystemTimes       = kernel32.NewProc("GetSystemTimes")
	procGetTickCount64       = kernel32.NewProc("GetTickCount64")

	procRegOpenKeyExW        = advapi32.NewProc("RegOpenKeyExW")
	procRegQueryValueExW     = advapi32.NewProc("RegQueryValueExW")
	procRegCloseKey          = advapi32.NewProc("RegCloseKey")
	procOpenSCManagerW       = advapi32.NewProc("OpenSCManagerW")
	procOpenServiceW         = advapi32.NewProc("OpenServiceW")
	procQueryServiceStatusEx = advapi32.NewProc("QueryServiceStatusEx")
	procCloseServiceHandle   = advapi32.NewProc("CloseServiceHandle")
)

func acquireSingleInstance(name string) (func(), error) {
	namePtr, err := syscall.UTF16PtrFromString(name)
	if err != nil {
		return func() {}, err
	}
	handle, _, callErr := procCreateMutexW.Call(0, 0, uintptr(unsafe.Pointer(namePtr)))
	if handle == 0 {
		return func() {}, fmt.Errorf("CreateMutexW falhou: %v", callErr)
	}
	if errno, ok := callErr.(syscall.Errno); ok && errno == syscall.ERROR_ALREADY_EXISTS {
		procCloseHandle.Call(handle)
		return func() {}, fmt.Errorf("instância já existente")
	}
	return func() { procCloseHandle.Call(handle) }, nil
}

func collectWindowsSnapshotNative() (MachineSnapshot, error) {
	hostname, _ := os.Hostname()
	snapshot := MachineSnapshot{
		Hostname:     hostname,
		DeviceUID:    regString(`SOFTWARE\Microsoft\Cryptography`, "MachineGuid"),
		Manufacturer: regString(`HARDWARE\DESCRIPTION\System\BIOS`, "SystemManufacturer"),
		Model:        regString(`HARDWARE\DESCRIPTION\System\BIOS`, "SystemProductName"),
		SerialNumber: regString(`HARDWARE\DESCRIPTION\System\BIOS`, "SystemSerialNumber"),
		OSName:       regString(`SOFTWARE\Microsoft\Windows NT\CurrentVersion`, "ProductName"),
		OSVersion:    windowsVersion(),
		Profile:      readLocalProfile(),
	}

	if snapshot.DeviceUID == "" {
		snapshot.DeviceUID = stableFallbackUID(snapshot.Hostname, snapshot.SerialNumber)
	}

	if cpu := sampleCPU(250 * time.Millisecond); cpu >= 0 {
		snapshot.CPUPercent = float64Ptr(cpu)
	}

	var mem memoryStatusEx
	mem.Length = uint32(unsafe.Sizeof(mem))
	if ok, _, _ := procGlobalMemoryStatusEx.Call(uintptr(unsafe.Pointer(&mem))); ok != 0 && mem.TotalPhys > 0 {
		total := float64(mem.TotalPhys) / (1 << 30)
		used := float64(mem.TotalPhys-mem.AvailPhys) / (1 << 30)
		pct := (used / total) * 100
		snapshot.MemoryTotalGB = float64Ptr(round2(total))
		snapshot.MemoryUsedGB = float64Ptr(round2(used))
		snapshot.MemoryPercent = float64Ptr(round2(pct))
	}

	root, _ := syscall.UTF16PtrFromString(`C:\`)
	var freeAvailable, totalBytes, totalFree uint64
	if ok, _, _ := procGetDiskFreeSpaceExW.Call(
		uintptr(unsafe.Pointer(root)),
		uintptr(unsafe.Pointer(&freeAvailable)),
		uintptr(unsafe.Pointer(&totalBytes)),
		uintptr(unsafe.Pointer(&totalFree)),
	); ok != 0 && totalBytes > 0 {
		total := float64(totalBytes) / (1 << 30)
		free := float64(totalFree) / (1 << 30)
		usedPct := (float64(totalBytes-totalFree) / float64(totalBytes)) * 100
		snapshot.DiskTotalGB = float64Ptr(round2(total))
		snapshot.DiskFreeGB = float64Ptr(round2(free))
		snapshot.DiskPercent = float64Ptr(round2(usedPct))
	}

	if ticks, _, _ := procGetTickCount64.Call(); ticks > 0 {
		uptime := int64(ticks / 1000)
		snapshot.UptimeSeconds = &uptime
	}

	snapshot.IPLocal, snapshot.NetworkName = primaryNetwork()
	if value, known := serviceIsRunning("WinDefend"); known {
		snapshot.DefenderActive = boolPtr(value)
	}
	if value, known := serviceIsRunning("MpsSvc"); known {
		snapshot.FirewallActive = boolPtr(value)
	}

	// Temperatura permanece nula quando o hardware não oferece uma API nativa segura.
	// Esta versão não abre PowerShell, WMI externo ou console para consultar sensores.
	snapshot.TemperatureC = nil
	return snapshot, nil
}

func windowsVersion() string {
	display := regString(`SOFTWARE\Microsoft\Windows NT\CurrentVersion`, "DisplayVersion")
	build := regString(`SOFTWARE\Microsoft\Windows NT\CurrentVersion`, "CurrentBuildNumber")
	if display != "" && build != "" {
		return display + " (build " + build + ")"
	}
	if build != "" {
		return "build " + build
	}
	return regString(`SOFTWARE\Microsoft\Windows NT\CurrentVersion`, "CurrentVersion")
}

func readLocalProfile() string {
	base := os.Getenv("LOCALAPPDATA")
	if base == "" {
		return "Nenhum"
	}
	raw, err := os.ReadFile(filepath.Join(base, "CoreTuner", "profile.json"))
	if err != nil {
		return "Nenhum"
	}
	var value struct {
		Profile string `json:"profile"`
	}
	if json.Unmarshal(raw, &value) != nil || strings.TrimSpace(value.Profile) == "" {
		return "Nenhum"
	}
	return strings.TrimSpace(value.Profile)
}

func primaryNetwork() (string, string) {
	interfaces, err := net.Interfaces()
	if err != nil {
		return "", ""
	}
	for _, iface := range interfaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, _ := iface.Addrs()
		for _, addr := range addrs {
			var ip net.IP
			switch value := addr.(type) {
			case *net.IPNet:
				ip = value.IP
			case *net.IPAddr:
				ip = value.IP
			}
			v4 := ip.To4()
			if v4 == nil || v4[0] == 169 && v4[1] == 254 {
				continue
			}
			return v4.String(), iface.Name
		}
	}
	return "", ""
}

func sampleCPU(wait time.Duration) float64 {
	idle1, kernel1, user1, ok := systemTimes()
	if !ok {
		return -1
	}
	time.Sleep(wait)
	idle2, kernel2, user2, ok := systemTimes()
	if !ok {
		return -1
	}
	idle := idle2 - idle1
	total := (kernel2 - kernel1) + (user2 - user1)
	if total == 0 || idle > total {
		return 0
	}
	value := 100 * float64(total-idle) / float64(total)
	if value < 0 {
		value = 0
	}
	if value > 100 {
		value = 100
	}
	return round2(value)
}

func systemTimes() (uint64, uint64, uint64, bool) {
	var idle, kernel, user fileTime
	ok, _, _ := procGetSystemTimes.Call(
		uintptr(unsafe.Pointer(&idle)),
		uintptr(unsafe.Pointer(&kernel)),
		uintptr(unsafe.Pointer(&user)),
	)
	if ok == 0 {
		return 0, 0, 0, false
	}
	return fileTimeValue(idle), fileTimeValue(kernel), fileTimeValue(user), true
}

func fileTimeValue(value fileTime) uint64 {
	return uint64(value.HighDateTime)<<32 | uint64(value.LowDateTime)
}

func serviceIsRunning(name string) (bool, bool) {
	manager, _, _ := procOpenSCManagerW.Call(0, 0, scManagerConnect)
	if manager == 0 {
		return false, false
	}
	defer procCloseServiceHandle.Call(manager)
	namePtr, _ := syscall.UTF16PtrFromString(name)
	service, _, _ := procOpenServiceW.Call(manager, uintptr(unsafe.Pointer(namePtr)), serviceQueryStatus)
	if service == 0 {
		return false, false
	}
	defer procCloseServiceHandle.Call(service)
	var status serviceStatusProcess
	var needed uint32
	ok, _, _ := procQueryServiceStatusEx.Call(
		service,
		scStatusProcessInfo,
		uintptr(unsafe.Pointer(&status)),
		unsafe.Sizeof(status),
		uintptr(unsafe.Pointer(&needed)),
	)
	if ok == 0 {
		return false, false
	}
	return status.CurrentState == serviceRunning, true
}

func regString(subkey, name string) string {
	key, ok := openRegistryKey(subkey)
	if !ok {
		return ""
	}
	defer procRegCloseKey.Call(key)
	namePtr, _ := syscall.UTF16PtrFromString(name)
	var valueType uint32
	var size uint32
	result, _, _ := procRegQueryValueExW.Call(
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
	result, _, _ = procRegQueryValueExW.Call(
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
	for i := 0; i+1 < int(size); i += 2 {
		unit := uint16(buffer[i]) | uint16(buffer[i+1])<<8
		if unit == 0 {
			break
		}
		units = append(units, unit)
	}
	return strings.TrimSpace(string(utf16.Decode(units)))
}

func openRegistryKey(subkey string) (uintptr, bool) {
	subkeyPtr, _ := syscall.UTF16PtrFromString(subkey)
	var key uintptr
	result, _, _ := procRegOpenKeyExW.Call(
		hkeyLocalMachine,
		uintptr(unsafe.Pointer(subkeyPtr)),
		0,
		keyRead|keyWow6464,
		uintptr(unsafe.Pointer(&key)),
	)
	return key, result == 0
}

func round2(value float64) float64 {
	if value >= 0 {
		return float64(int64(value*100+0.5)) / 100
	}
	return float64(int64(value*100-0.5)) / 100
}

func float64Ptr(value float64) *float64 { return &value }
func boolPtr(value bool) *bool          { return &value }
