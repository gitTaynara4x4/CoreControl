//go:build windows

package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"
	"unsafe"
)

const (
	coreControlServiceName        = "CoreControlAgent"
	coreControlServiceDisplayName = "CoreControl Agent"

	agentServiceWin32OwnProcess = 0x00000010
	agentServiceStopped         = 0x00000001
	agentServiceStartPending    = 0x00000002
	agentServiceStopPending     = 0x00000003
	agentServiceRunning         = 0x00000004

	agentServiceAcceptStop     = 0x00000001
	agentServiceAcceptShutdown = 0x00000004

	agentServiceControlStop        = 0x00000001
	agentServiceControlInterrogate = 0x00000004
	agentServiceControlShutdown    = 0x00000005

	serviceErrorFailedControllerConnect = 1063
	serviceCreateNoWindow               = 0x08000000
)

type agentServiceStatus struct {
	ServiceType             uint32
	CurrentState            uint32
	ControlsAccepted        uint32
	Win32ExitCode           uint32
	ServiceSpecificExitCode uint32
	CheckPoint              uint32
	WaitHint                uint32
}

type agentServiceTableEntry struct {
	ServiceName *uint16
	ServiceProc uintptr
}

type agentServiceRuntime struct {
	handle uintptr
	stop   chan struct{}
	once   sync.Once
	run    func(<-chan struct{})
	mu     sync.Mutex
	state  uint32
}

var (
	agentServiceAdvapi32               = syscall.NewLazyDLL("advapi32.dll")
	agentStartServiceCtrlDispatcherW   = agentServiceAdvapi32.NewProc("StartServiceCtrlDispatcherW")
	agentRegisterServiceCtrlHandlerExW = agentServiceAdvapi32.NewProc("RegisterServiceCtrlHandlerExW")
	agentSetServiceStatus              = agentServiceAdvapi32.NewProc("SetServiceStatus")
	agentServiceMainCallback           = syscall.NewCallback(agentServiceMain)
	agentServiceControlHandlerCallback = syscall.NewCallback(agentServiceControlHandler)
	agentServiceRuntimeMu              sync.Mutex
	activeAgentServiceRuntime          *agentServiceRuntime
)

func runWindowsAgentService(run func(stop <-chan struct{})) error {
	if run == nil {
		return errors.New("rotina do serviço não informada")
	}
	namePtr, err := syscall.UTF16PtrFromString(coreControlServiceName)
	if err != nil {
		return err
	}
	runtime := &agentServiceRuntime{stop: make(chan struct{}), run: run, state: agentServiceStartPending}
	agentServiceRuntimeMu.Lock()
	activeAgentServiceRuntime = runtime
	agentServiceRuntimeMu.Unlock()
	defer func() {
		agentServiceRuntimeMu.Lock()
		if activeAgentServiceRuntime == runtime {
			activeAgentServiceRuntime = nil
		}
		agentServiceRuntimeMu.Unlock()
	}()

	table := [2]agentServiceTableEntry{
		{ServiceName: namePtr, ServiceProc: agentServiceMainCallback},
		{},
	}
	ok, _, callErr := agentStartServiceCtrlDispatcherW.Call(uintptr(unsafe.Pointer(&table[0])))
	if ok == 0 {
		if errno, ok := callErr.(syscall.Errno); ok && errno == serviceErrorFailedControllerConnect {
			return errors.New("o Agent foi iniciado com -service fora do Gerenciador de Serviços do Windows")
		}
		return fmt.Errorf("StartServiceCtrlDispatcherW falhou: %v", callErr)
	}
	return nil
}

func agentServiceMain(argc uintptr, argv uintptr) uintptr {
	_ = argc
	_ = argv
	runtime := currentAgentServiceRuntime()
	if runtime == nil {
		return 0
	}
	namePtr, _ := syscall.UTF16PtrFromString(coreControlServiceName)
	handle, _, _ := agentRegisterServiceCtrlHandlerExW.Call(
		uintptr(unsafe.Pointer(namePtr)),
		agentServiceControlHandlerCallback,
		0,
	)
	if handle == 0 {
		return 0
	}
	runtime.handle = handle
	runtime.report(agentServiceStartPending, 0, 1, 8000)
	runtime.report(agentServiceRunning, agentServiceAcceptStop|agentServiceAcceptShutdown, 0, 0)

	runtime.run(runtime.stop)
	runtime.report(agentServiceStopped, 0, 0, 0)
	return 0
}

func agentServiceControlHandler(control uintptr, eventType uintptr, eventData uintptr, context uintptr) uintptr {
	_ = eventType
	_ = eventData
	_ = context
	runtime := currentAgentServiceRuntime()
	if runtime == nil {
		return 0
	}
	switch uint32(control) {
	case agentServiceControlStop, agentServiceControlShutdown:
		runtime.report(agentServiceStopPending, 0, 1, 15000)
		runtime.once.Do(func() { close(runtime.stop) })
	case agentServiceControlInterrogate:
		runtime.mu.Lock()
		state := runtime.state
		runtime.mu.Unlock()
		accepted := uint32(0)
		if state == agentServiceRunning {
			accepted = agentServiceAcceptStop | agentServiceAcceptShutdown
		}
		runtime.report(state, accepted, 0, 0)
	}
	return 0
}

func currentAgentServiceRuntime() *agentServiceRuntime {
	agentServiceRuntimeMu.Lock()
	defer agentServiceRuntimeMu.Unlock()
	return activeAgentServiceRuntime
}

func (runtime *agentServiceRuntime) report(state, accepted, checkpoint, waitHint uint32) {
	if runtime == nil || runtime.handle == 0 {
		return
	}
	runtime.mu.Lock()
	runtime.state = state
	runtime.mu.Unlock()
	status := agentServiceStatus{
		ServiceType:      agentServiceWin32OwnProcess,
		CurrentState:     state,
		ControlsAccepted: accepted,
		CheckPoint:       checkpoint,
		WaitHint:         waitHint,
	}
	agentSetServiceStatus.Call(runtime.handle, uintptr(unsafe.Pointer(&status)))
}

func installWindowsAgentService(configPath, activityCachePath string) error {
	configPath = strings.TrimSpace(configPath)
	activityCachePath = strings.TrimSpace(activityCachePath)
	if configPath == "" {
		return errors.New("configuração do Agent não informada")
	}
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	exe, err = filepathAbs(exe)
	if err != nil {
		return err
	}

	// Atualização idempotente: se o serviço antigo existir, interrompe e recria
	// apontando para o binário/configuração atuais.
	_ = runSC("stop", coreControlServiceName)
	for i := 0; i < 30; i++ {
		if !windowsServiceExists(coreControlServiceName) {
			break
		}
		out, _ := runSCOutput("query", coreControlServiceName)
		if !strings.Contains(strings.ToUpper(out), "STOP_PENDING") && !strings.Contains(strings.ToUpper(out), "RUNNING") {
			break
		}
		time.Sleep(200 * time.Millisecond)
	}
	_ = runSC("delete", coreControlServiceName)
	for i := 0; i < 30 && windowsServiceExists(coreControlServiceName); i++ {
		time.Sleep(200 * time.Millisecond)
	}

	binaryPath := quoteWindowsArg(exe) + " -service -config " + quoteWindowsArg(configPath)
	if activityCachePath != "" {
		binaryPath += " -activity-cache " + quoteWindowsArg(activityCachePath)
	}
	if out, err := runSCOutput(
		"create", coreControlServiceName,
		"binPath=", binaryPath,
		"start=", "auto",
		"DisplayName=", coreControlServiceDisplayName,
	); err != nil {
		return fmt.Errorf("não foi possível criar o serviço: %s", strings.TrimSpace(out))
	}
	_, _ = runSCOutput("description", coreControlServiceName, "Mantém telemetria e comandos do CoreControl ativos mesmo sem usuário logado.")
	_, _ = runSCOutput("failure", coreControlServiceName, "reset=", "86400", "actions=", "restart/5000/restart/15000/restart/60000")
	_, _ = runSCOutput("failureflag", coreControlServiceName, "1")
	if out, err := runSCOutput("start", coreControlServiceName); err != nil {
		return fmt.Errorf("o serviço foi criado, mas não iniciou: %s", strings.TrimSpace(out))
	}
	return nil
}

func uninstallWindowsAgentService() error {
	if !windowsServiceExists(coreControlServiceName) {
		return nil
	}
	_ = runSC("stop", coreControlServiceName)
	for i := 0; i < 40; i++ {
		out, _ := runSCOutput("query", coreControlServiceName)
		upper := strings.ToUpper(out)
		if !strings.Contains(upper, "RUNNING") && !strings.Contains(upper, "STOP_PENDING") {
			break
		}
		time.Sleep(250 * time.Millisecond)
	}
	out, err := runSCOutput("delete", coreControlServiceName)
	if err != nil && windowsServiceExists(coreControlServiceName) {
		return fmt.Errorf("não foi possível remover o serviço: %s", strings.TrimSpace(out))
	}
	return nil
}

func windowsServiceExists(name string) bool {
	_, err := runSCOutput("query", name)
	return err == nil
}

func runSC(args ...string) error {
	_, err := runSCOutput(args...)
	return err
}

func runSCOutput(args ...string) (string, error) {
	cmd := exec.Command("sc.exe", args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: serviceCreateNoWindow}
	raw, err := cmd.CombinedOutput()
	return string(raw), err
}

func quoteWindowsArg(value string) string {
	value = strings.ReplaceAll(value, `"`, `\"`)
	return `"` + value + `"`
}

func filepathAbs(path string) (string, error) {
	return filepath.Abs(path)
}
