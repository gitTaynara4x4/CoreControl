//go:build windows

package main

import (
	"net"
	"net/http"
	"os"
	"sort"
	"sync"
	"time"
	"unsafe"
)

func collectSystemDynamic(s SystemInfo) SystemInfo {
	s.Updated = time.Now()
	var mem MEMORYSTATUSEX
	mem.DwLength = uint32(unsafe.Sizeof(mem))
	if r, _, _ := procGlobalMemoryStatusEx.Call(uintptr(unsafe.Pointer(&mem))); r != 0 {
		s.Memory = float64(mem.DwMemoryLoad)
		s.TotalRAMGB = float64(mem.UllTotalPhys) / (1 << 30)
		s.UsedRAMGB = float64(mem.UllTotalPhys-mem.UllAvailPhys) / (1 << 30)
	}
	root := utf16("C:\\")
	var free, total, totalFree uint64
	if r, _, _ := procGetDiskFreeSpaceEx.Call(uintptr(unsafe.Pointer(root)), uintptr(unsafe.Pointer(&free)), uintptr(unsafe.Pointer(&total)), uintptr(unsafe.Pointer(&totalFree))); r != 0 {
		s.DiskTotalGB = float64(total) / (1 << 30)
		s.DiskUsedGB = float64(total-free) / (1 << 30)
		if total > 0 {
			s.Disk = s.DiskUsedGB / s.DiskTotalGB * 100
		}
	}
	tick, _, _ := procGetTickCount64.Call()
	s.Uptime = time.Duration(tick) * time.Millisecond
	s.CPU = cpuUsage()
	return s
}

func collectSystemFull(client *http.Client, server string) SystemInfo {
	h, _ := os.Hostname()
	u := os.Getenv("USERNAME")
	s := collectSystemDynamic(SystemInfo{Hostname: h, Username: u, Updated: time.Now()})
	details := nativeSystemDetails()
	for k, v := range details {
		switch k {
		case "Manufacturer":
			s.Manufacturer = v
		case "Model":
			s.Model = v
		case "Serial":
			s.Serial = v
		case "CPU":
			s.CPUName = v
		case "OS":
			s.OS = v
		case "Disk":
			s.DiskName = v
		case "DiskType":
			s.DiskType = v
		}
	}
	s.AudioOK, s.MicOK = nativeAudioStatus()
	start := time.Now()
	conn, err := net.DialTimeout("tcp", "1.1.1.1:443", 3*time.Second)
	if err == nil {
		s.InternetOK = true
		s.LatencyMS = time.Since(start).Milliseconds()
		conn.Close()
	} else {
		s.InternetOK = false
		s.LatencyMS = 0
	}
	return s
}

var cpuMu sync.Mutex
var prevIdle, prevKernel, prevUser uint64

func cpuUsage() float64 {
	cpuMu.Lock()
	defer cpuMu.Unlock()
	var idle, kernel, user FILETIME
	if r, _, _ := procGetSystemTimes.Call(uintptr(unsafe.Pointer(&idle)), uintptr(unsafe.Pointer(&kernel)), uintptr(unsafe.Pointer(&user))); r == 0 {
		return 0
	}
	cv := func(f FILETIME) uint64 { return uint64(f.High)<<32 | uint64(f.Low) }
	i, k, u := cv(idle), cv(kernel), cv(user)
	if prevKernel == 0 {
		prevIdle = i
		prevKernel = k
		prevUser = u
		return 0
	}
	sys := (k - prevKernel) + (u - prevUser)
	id := i - prevIdle
	prevIdle = i
	prevKernel = k
	prevUser = u
	if sys == 0 {
		return 0
	}
	v := 100 * (float64(sys-id) / float64(sys))
	if v < 0 {
		v = 0
	}
	if v > 100 {
		v = 100
	}
	return v
}
func collectProcesses() []ProcessInfo {
	out := nativeProcesses()
	sort.Slice(out, func(i, j int) bool { return out[i].MemoryMB > out[j].MemoryMB })
	if len(out) > 15 {
		out = out[:15]
	}
	return out
}
