//go:build windows

package main

import (
	"runtime"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"
	"unicode/utf16"
	"unsafe"
)

type foregroundActivity struct {
	ProcessName string `json:"process_name"`
	WindowTitle string `json:"window_title"`
	PID         int    `json:"pid"`
	Browser     string `json:"browser,omitempty"`
	Domain      string `json:"domain,omitempty"`
	URL         string `json:"url,omitempty"`
	CapturedAt  string `json:"captured_at"`
}

type activityApplication struct {
	ProcessName string  `json:"process_name"`
	WindowTitle string  `json:"window_title"`
	PID         int     `json:"pid"`
	CPUPercent  float64 `json:"cpu_percent"`
	MemoryMB    float64 `json:"memory_mb"`
	Focused     bool    `json:"focused"`
}

type activitySnapshotResult struct {
	CapturedAt  string                `json:"captured_at"`
	Foreground  foregroundActivity    `json:"foreground"`
	Apps        []activityApplication `json:"apps"`
	BrowserTabs []agentBrowserTab     `json:"browser_tabs,omitempty"`
}

type agentWindowInfo struct {
	PID     int
	Title   string
	Focused bool
}

type agentProcessInfo struct {
	Name string
	PID  int
}

type agentProcessMemoryCounters struct {
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

type agentCPUSample struct {
	Total uint64
	At    time.Time
}

var (
	activityUser32                   = syscall.NewLazyDLL("user32.dll")
	activityPsapi                    = syscall.NewLazyDLL("psapi.dll")
	activityGetForegroundWindow      = activityUser32.NewProc("GetForegroundWindow")
	activityEnumWindows              = activityUser32.NewProc("EnumWindows")
	activityIsWindowVisible          = activityUser32.NewProc("IsWindowVisible")
	activityGetWindowTextLengthW     = activityUser32.NewProc("GetWindowTextLengthW")
	activityGetWindowTextW           = activityUser32.NewProc("GetWindowTextW")
	activityGetWindowThreadProcessID = activityUser32.NewProc("GetWindowThreadProcessId")
	activityOpenProcess              = kernel32.NewProc("OpenProcess")
	activityGetProcessTimes          = kernel32.NewProc("GetProcessTimes")
	activityGetProcessMemoryInfo     = activityPsapi.NewProc("GetProcessMemoryInfo")
	activityCPUMu                    sync.Mutex
	activityCPUSamples               = map[int]agentCPUSample{}
)

const (
	activityProcessQueryLimited = 0x1000
	activityProcessVMRead       = 0x0010
)

func collectForegroundActivity() foregroundActivity {
	windows := collectAgentWindows()
	if len(windows) == 0 {
		return foregroundActivity{CapturedAt: time.Now().UTC().Format(time.RFC3339)}
	}
	processes := agentProcessMap()
	browserTabs := loadAgentBrowserTabs()
	for _, window := range windows {
		if !window.Focused {
			continue
		}
		name := processes[window.PID]
		result := foregroundActivity{
			ProcessName: name,
			WindowTitle: window.Title,
			PID:         window.PID,
			CapturedAt:  time.Now().UTC().Format(time.RFC3339),
		}
		if tab := activeBrowserTab(name, browserTabs); tab != nil {
			result.WindowTitle = tab.Title
			result.Browser = tab.Browser
			result.Domain = tab.Domain
			result.URL = tab.URL
		}
		return result
	}
	return foregroundActivity{CapturedAt: time.Now().UTC().Format(time.RFC3339)}
}

func collectActivitySnapshot() activitySnapshotResult {
	windows := collectAgentWindows()
	processes := agentProcessMap()
	browserTabs := loadAgentBrowserTabs()
	cpuCache := map[int]float64{}
	apps := make([]activityApplication, 0, len(windows))
	seen := map[string]bool{}
	var foreground foregroundActivity
	for _, window := range windows {
		name := strings.TrimSpace(processes[window.PID])
		if name == "" {
			continue
		}
		key := strings.ToLower(name) + "\x00" + window.Title
		if seen[key] {
			continue
		}
		seen[key] = true
		cpu, ok := cpuCache[window.PID]
		if !ok {
			cpu = agentProcessCPU(window.PID)
			cpuCache[window.PID] = cpu
		}
		item := activityApplication{
			ProcessName: name,
			WindowTitle: window.Title,
			PID:         window.PID,
			CPUPercent:  round2(cpu),
			MemoryMB:    round2(agentProcessMemoryMB(window.PID)),
			Focused:     window.Focused,
		}
		apps = append(apps, item)
		if window.Focused {
			foreground = foregroundActivity{ProcessName: name, WindowTitle: window.Title, PID: window.PID}
			if tab := activeBrowserTab(name, browserTabs); tab != nil {
				foreground.WindowTitle = tab.Title
				foreground.Browser = tab.Browser
				foreground.Domain = tab.Domain
				foreground.URL = tab.URL
			}
		}
	}
	sort.Slice(apps, func(i, j int) bool {
		if apps[i].Focused != apps[j].Focused {
			return apps[i].Focused
		}
		if apps[i].CPUPercent != apps[j].CPUPercent {
			return apps[i].CPUPercent > apps[j].CPUPercent
		}
		return apps[i].MemoryMB > apps[j].MemoryMB
	})
	if len(apps) > 24 {
		apps = apps[:24]
	}
	now := time.Now().UTC().Format(time.RFC3339)
	foreground.CapturedAt = now
	return activitySnapshotResult{CapturedAt: now, Foreground: foreground, Apps: apps, BrowserTabs: browserTabs}
}

func collectAgentWindows() []agentWindowInfo {
	foreground, _, _ := activityGetForegroundWindow.Call()
	items := make([]agentWindowInfo, 0, 16)
	callback := syscall.NewCallback(func(hwnd, lParam uintptr) uintptr {
		visible, _, _ := activityIsWindowVisible.Call(hwnd)
		if visible == 0 {
			return 1
		}
		length, _, _ := activityGetWindowTextLengthW.Call(hwnd)
		if length == 0 || length > 2048 {
			return 1
		}
		buffer := make([]uint16, length+1)
		read, _, _ := activityGetWindowTextW.Call(hwnd, uintptr(unsafe.Pointer(&buffer[0])), length+1)
		if read == 0 {
			return 1
		}
		title := strings.TrimSpace(syscall.UTF16ToString(buffer))
		if title == "" || strings.EqualFold(title, "Program Manager") {
			return 1
		}
		var pid uint32
		activityGetWindowThreadProcessID.Call(hwnd, uintptr(unsafe.Pointer(&pid)))
		if pid == 0 {
			return 1
		}
		items = append(items, agentWindowInfo{PID: int(pid), Title: title, Focused: hwnd == foreground})
		return 1
	})
	activityEnumWindows.Call(callback, 0)
	return items
}

func agentProcessMap() map[int]string {
	result := map[int]string{}
	snapshot, _, _ := procCreateToolhelp32Snap.Call(th32csSnapProcess, 0)
	if snapshot == 0 || snapshot == ^uintptr(0) {
		return result
	}
	defer procCloseHandle.Call(snapshot)
	entry := processEntry32{Size: uint32(unsafe.Sizeof(processEntry32{}))}
	ok, _, _ := procProcess32FirstW.Call(snapshot, uintptr(unsafe.Pointer(&entry)))
	for ok != 0 {
		name := strings.TrimSpace(string(utf16.Decode(trimUTF16(entry.ExeFile[:]))))
		name = strings.TrimSuffix(name, ".exe")
		if name != "" {
			result[int(entry.ProcessID)] = name
		}
		entry.Size = uint32(unsafe.Sizeof(processEntry32{}))
		ok, _, _ = procProcess32NextW.Call(snapshot, uintptr(unsafe.Pointer(&entry)))
	}
	return result
}

func agentProcessMemoryMB(pid int) float64 {
	handle, _, _ := activityOpenProcess.Call(activityProcessQueryLimited|activityProcessVMRead, 0, uintptr(pid))
	if handle == 0 {
		return 0
	}
	defer procCloseHandle.Call(handle)
	counters := agentProcessMemoryCounters{Cb: uint32(unsafe.Sizeof(agentProcessMemoryCounters{}))}
	ok, _, _ := activityGetProcessMemoryInfo.Call(handle, uintptr(unsafe.Pointer(&counters)), uintptr(counters.Cb))
	if ok == 0 {
		return 0
	}
	return float64(counters.WorkingSetSize) / (1 << 20)
}

func agentProcessCPU(pid int) float64 {
	handle, _, _ := activityOpenProcess.Call(activityProcessQueryLimited, 0, uintptr(pid))
	if handle == 0 {
		return 0
	}
	defer procCloseHandle.Call(handle)
	var creation, exit, kernel, user fileTime
	ok, _, _ := activityGetProcessTimes.Call(
		handle,
		uintptr(unsafe.Pointer(&creation)),
		uintptr(unsafe.Pointer(&exit)),
		uintptr(unsafe.Pointer(&kernel)),
		uintptr(unsafe.Pointer(&user)),
	)
	if ok == 0 {
		return 0
	}
	total := fileTimeValue(kernel) + fileTimeValue(user)
	now := time.Now()
	activityCPUMu.Lock()
	previous, exists := activityCPUSamples[pid]
	activityCPUSamples[pid] = agentCPUSample{Total: total, At: now}
	activityCPUMu.Unlock()
	if !exists || total < previous.Total {
		return 0
	}
	elapsed := now.Sub(previous.At).Seconds()
	if elapsed <= 0 {
		return 0
	}
	value := (float64(total-previous.Total) / 1e7) / elapsed / float64(maxAgentInt(1, runtime.NumCPU())) * 100
	if value < 0 {
		value = 0
	}
	if value > 100 {
		value = 100
	}
	return value
}

func maxAgentInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
