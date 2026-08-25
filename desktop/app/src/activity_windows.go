//go:build windows

package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"
	"unsafe"
)

type ActivityApp struct {
	Name          string
	WindowTitle   string
	PID           int
	MemoryMB      float64
	CPU           float64
	Focused       bool
	ActiveSeconds int64
}

type ActivitySession struct {
	ProcessName   string    `json:"process_name"`
	WindowTitle   string    `json:"window_title"`
	PID           int       `json:"pid"`
	StartedAt     time.Time `json:"started_at"`
	EndedAt       time.Time `json:"ended_at,omitempty"`
	ActiveSeconds int64     `json:"active_seconds"`
}

type activityStateFile struct {
	Sessions []ActivitySession `json:"sessions"`
	Current  *ActivitySession  `json:"current,omitempty"`
	SavedAt  time.Time         `json:"saved_at"`
}

type activityWindow struct {
	PID     int
	Title   string
	Focused bool
}

type activityCPUSample struct {
	Total uint64
	At    time.Time
}

var (
	activityUser32                   = syscall.NewLazyDLL("user32.dll")
	activityKernel32                 = syscall.NewLazyDLL("kernel32.dll")
	activityGetForegroundWindow      = activityUser32.NewProc("GetForegroundWindow")
	activityEnumWindows              = activityUser32.NewProc("EnumWindows")
	activityIsWindowVisible          = activityUser32.NewProc("IsWindowVisible")
	activityGetWindowThreadProcessId = activityUser32.NewProc("GetWindowThreadProcessId")
	activityGetProcessTimes          = activityKernel32.NewProc("GetProcessTimes")
	activityCPUMu                    sync.Mutex
	activityCPUSamples               = map[int]activityCPUSample{}
)

func (a *App) loadActivity() {
	path := filepath.Join(dataDir(), "activity.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		return
	}
	var state activityStateFile
	if json.Unmarshal(raw, &state) != nil {
		return
	}
	cutoff := time.Now().Add(-7 * 24 * time.Hour)
	sessions := make([]ActivitySession, 0, len(state.Sessions)+1)
	for _, item := range state.Sessions {
		if item.StartedAt.After(cutoff) {
			sessions = append(sessions, item)
		}
	}
	if state.Current != nil && !state.Current.StartedAt.IsZero() {
		closed := *state.Current
		end := state.SavedAt
		if end.IsZero() || end.Before(closed.StartedAt) {
			end = time.Now()
		}
		closed.EndedAt = end
		closed.ActiveSeconds = maxInt64(0, int64(end.Sub(closed.StartedAt).Seconds()))
		if closed.StartedAt.After(cutoff) {
			sessions = append(sessions, closed)
		}
	}
	if len(sessions) > 500 {
		sessions = sessions[len(sessions)-500:]
	}
	a.mu.Lock()
	a.activitySessions = sessions
	a.activityCurrent = nil
	a.activityLastSave = time.Now()
	a.mu.Unlock()
}

func (a *App) refreshActivity() {
	a.mu.RLock()
	baseSys := a.sys
	page := a.page
	a.mu.RUnlock()
	apps, processes := collectActivitySnapshot()
	browserTabs := loadBrowserTabs()
	var dynamicSys SystemInfo
	if page == 4 && baseSys.Hostname != "" {
		dynamicSys = collectSystemDynamic(baseSys)
	}
	now := time.Now()
	var focused *ActivityApp
	for i := range apps {
		if apps[i].Focused {
			focused = &apps[i]
			break
		}
	}

	a.mu.Lock()
	changed := false
	if a.activityCurrent != nil {
		same := focused != nil && a.activityCurrent.PID == focused.PID && strings.EqualFold(a.activityCurrent.ProcessName, focused.Name) && a.activityCurrent.WindowTitle == focused.WindowTitle
		if same {
			a.activityCurrent.ActiveSeconds = maxInt64(0, int64(now.Sub(a.activityCurrent.StartedAt).Seconds()))
		} else {
			closed := *a.activityCurrent
			closed.EndedAt = now
			closed.ActiveSeconds = maxInt64(0, int64(now.Sub(closed.StartedAt).Seconds()))
			if closed.ActiveSeconds > 0 {
				a.activitySessions = append(a.activitySessions, closed)
			}
			a.activityCurrent = nil
			changed = true
		}
	}
	if a.activityCurrent == nil && focused != nil {
		a.activityCurrent = &ActivitySession{
			ProcessName: focused.Name,
			WindowTitle: focused.WindowTitle,
			PID:         focused.PID,
			StartedAt:   now,
		}
		changed = true
	}
	cutoff := now.Add(-7 * 24 * time.Hour)
	first := 0
	for first < len(a.activitySessions) && a.activitySessions[first].StartedAt.Before(cutoff) {
		first++
	}
	if first > 0 {
		a.activitySessions = append([]ActivitySession(nil), a.activitySessions[first:]...)
	}
	if len(a.activitySessions) > 500 {
		a.activitySessions = append([]ActivitySession(nil), a.activitySessions[len(a.activitySessions)-500:]...)
	}

	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	totals := map[string]int64{}
	for _, item := range a.activitySessions {
		if !item.StartedAt.Before(today) {
			totals[strings.ToLower(item.ProcessName)] += item.ActiveSeconds
		}
	}
	if a.activityCurrent != nil && !a.activityCurrent.StartedAt.Before(today) {
		a.activityCurrent.ActiveSeconds = maxInt64(0, int64(now.Sub(a.activityCurrent.StartedAt).Seconds()))
		totals[strings.ToLower(a.activityCurrent.ProcessName)] += a.activityCurrent.ActiveSeconds
	}
	for i := range apps {
		apps[i].ActiveSeconds = totals[strings.ToLower(apps[i].Name)]
	}
	a.activityApps = apps
	a.browserTabs = browserTabs
	a.processes = processes
	if page == 4 && dynamicSys.Hostname != "" {
		a.sys = dynamicSys
	}
	shouldSave := changed || a.activityLastSave.IsZero() || now.Sub(a.activityLastSave) >= 30*time.Second
	if shouldSave {
		a.activityLastSave = now
	}
	a.mu.Unlock()

	if shouldSave {
		a.saveActivityState()
	}
	a.invalidate()
}

func (a *App) saveActivityState() {
	a.mu.RLock()
	state := activityStateFile{
		Sessions: append([]ActivitySession(nil), a.activitySessions...),
		SavedAt:  time.Now(),
	}
	if a.activityCurrent != nil {
		copyCurrent := *a.activityCurrent
		state.Current = &copyCurrent
	}
	a.mu.RUnlock()
	if len(state.Sessions) > 500 {
		state.Sessions = state.Sessions[len(state.Sessions)-500:]
	}
	raw, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return
	}
	_ = os.MkdirAll(dataDir(), 0755)
	_ = os.WriteFile(filepath.Join(dataDir(), "activity.json"), raw, 0600)
}

func collectActivitySnapshot() ([]ActivityApp, []ProcessInfo) {
	processes := nativeProcesses()
	procByPID := make(map[int]ProcessInfo, len(processes))
	for _, item := range processes {
		procByPID[item.PID] = item
	}
	windows := nativeVisibleWindows()
	cpuCache := map[int]float64{}
	apps := make([]ActivityApp, 0, len(windows))
	seen := map[string]bool{}
	for _, win := range windows {
		proc, ok := procByPID[win.PID]
		if !ok {
			continue
		}
		key := strings.ToLower(proc.Name) + "\x00" + win.Title
		if seen[key] {
			continue
		}
		seen[key] = true
		cpu, ok := cpuCache[win.PID]
		if !ok {
			cpu = activityProcessCPU(win.PID)
			cpuCache[win.PID] = cpu
		}
		apps = append(apps, ActivityApp{
			Name:        proc.Name,
			WindowTitle: win.Title,
			PID:         win.PID,
			MemoryMB:    proc.MemoryMB,
			CPU:         cpu,
			Focused:     win.Focused,
		})
	}
	for i := range processes {
		if cpu, ok := cpuCache[processes[i].PID]; ok {
			processes[i].CPU = cpu
		}
	}
	sort.Slice(apps, func(i, j int) bool {
		if apps[i].Focused != apps[j].Focused {
			return apps[i].Focused
		}
		if apps[i].CPU != apps[j].CPU {
			return apps[i].CPU > apps[j].CPU
		}
		return apps[i].MemoryMB > apps[j].MemoryMB
	})
	sort.Slice(processes, func(i, j int) bool { return processes[i].MemoryMB > processes[j].MemoryMB })
	return apps, processes
}

func nativeVisibleWindows() []activityWindow {
	foreground, _, _ := activityGetForegroundWindow.Call()
	items := make([]activityWindow, 0, 16)
	callback := syscall.NewCallback(func(hwnd, lParam uintptr) uintptr {
		visible, _, _ := activityIsWindowVisible.Call(hwnd)
		if visible == 0 {
			return 1
		}
		length, _, _ := procGetWindowTextLength.Call(hwnd)
		if length == 0 || length > 2048 {
			return 1
		}
		buffer := make([]uint16, length+1)
		read, _, _ := procGetWindowText.Call(hwnd, uintptr(unsafe.Pointer(&buffer[0])), length+1)
		if read == 0 {
			return 1
		}
		title := strings.TrimSpace(syscall.UTF16ToString(buffer))
		if title == "" || strings.EqualFold(title, "Program Manager") {
			return 1
		}
		var pid uint32
		activityGetWindowThreadProcessId.Call(hwnd, uintptr(unsafe.Pointer(&pid)))
		if pid == 0 {
			return 1
		}
		items = append(items, activityWindow{PID: int(pid), Title: title, Focused: hwnd == foreground})
		return 1
	})
	activityEnumWindows.Call(callback, 0)
	return items
}

func activityProcessCPU(pid int) float64 {
	handle, _, _ := nativeOpenProcess.Call(processQueryLimited, 0, uintptr(pid))
	if handle == 0 {
		return 0
	}
	defer nativeCloseHandle.Call(handle)
	var creation, exit, kernel, user FILETIME
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
	total := activityFileTimeValue(kernel) + activityFileTimeValue(user)
	now := time.Now()
	activityCPUMu.Lock()
	previous, exists := activityCPUSamples[pid]
	activityCPUSamples[pid] = activityCPUSample{Total: total, At: now}
	activityCPUMu.Unlock()
	if !exists || total < previous.Total {
		return 0
	}
	elapsed := now.Sub(previous.At).Seconds()
	if elapsed <= 0 {
		return 0
	}
	value := (float64(total-previous.Total) / 1e7) / elapsed / float64(maxInt(1, runtime.NumCPU())) * 100
	if value < 0 {
		value = 0
	}
	if value > 100 {
		value = 100
	}
	return value
}

func activityFileTimeValue(value FILETIME) uint64 {
	return uint64(value.High)<<32 | uint64(value.Low)
}

func maxInt64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
