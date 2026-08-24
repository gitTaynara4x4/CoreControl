//go:build windows

package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"syscall"
	"time"
	"unsafe"
)

const (
	spiGetAnimation           = 0x0048
	spiSetAnimation           = 0x0049
	spiGetClientAreaAnimation = 0x1042
	spiSetClientAreaAnimation = 0x1043
	spifUpdateIniFile         = 0x0001
	spifSendChange            = 0x0002

	processSetInformation = 0x0200
	aboveNormalPriority   = 0x00008000
)

type animationInfo struct {
	CbSize     uint32
	MinAnimate int32
}

type systemPowerStatus struct {
	ACLineStatus        byte
	BatteryFlag         byte
	BatteryLifePercent  byte
	SystemStatusFlag    byte
	BatteryLifeTime     uint32
	BatteryFullLifeTime uint32
}

var (
	optimizerUser32               = syscall.NewLazyDLL("user32.dll")
	optimizerKernel32             = syscall.NewLazyDLL("kernel32.dll")
	procSystemParametersInfoW     = optimizerUser32.NewProc("SystemParametersInfoW")
	procGetSystemPowerStatus      = optimizerKernel32.NewProc("GetSystemPowerStatus")
	procGetPriorityClassOptimizer = optimizerKernel32.NewProc("GetPriorityClass")
	procSetPriorityClassOptimizer = optimizerKernel32.NewProc("SetPriorityClass")
	powerSchemeGUIDPattern        = regexp.MustCompile(`(?i)[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}`)
)

func optimizationStatePath() string { return optimizationStateFile(dataDir()) }

func currentAnimationSnapshot() (AnimationSnapshot, error) {
	var clientArea int32
	ok, _, callErr := procSystemParametersInfoW.Call(
		spiGetClientAreaAnimation,
		0,
		uintptr(unsafe.Pointer(&clientArea)),
		0,
	)
	if ok == 0 {
		return AnimationSnapshot{}, fmt.Errorf("não foi possível ler as animações da interface: %v", callErr)
	}
	info := animationInfo{CbSize: uint32(unsafe.Sizeof(animationInfo{}))}
	ok, _, callErr = procSystemParametersInfoW.Call(
		spiGetAnimation,
		uintptr(info.CbSize),
		uintptr(unsafe.Pointer(&info)),
		0,
	)
	if ok == 0 {
		return AnimationSnapshot{}, fmt.Errorf("não foi possível ler a animação das janelas: %v", callErr)
	}
	return AnimationSnapshot{MinimizeWindows: info.MinAnimate != 0, ClientArea: clientArea != 0}, nil
}

func setMinimizeAnimation(enabled bool) error {
	value := int32(0)
	if enabled {
		value = 1
	}
	info := animationInfo{CbSize: uint32(unsafe.Sizeof(animationInfo{})), MinAnimate: value}
	ok, _, callErr := procSystemParametersInfoW.Call(
		spiSetAnimation,
		uintptr(info.CbSize),
		uintptr(unsafe.Pointer(&info)),
		spifUpdateIniFile|spifSendChange,
	)
	if ok == 0 {
		return fmt.Errorf("não foi possível ajustar a animação das janelas: %v", callErr)
	}
	return nil
}

func setClientAreaAnimation(enabled bool) error {
	value := int32(0)
	if enabled {
		value = 1
	}
	ok, _, callErr := procSystemParametersInfoW.Call(
		spiSetClientAreaAnimation,
		0,
		uintptr(unsafe.Pointer(&value)),
		spifUpdateIniFile|spifSendChange,
	)
	if ok == 0 {
		return fmt.Errorf("não foi possível ajustar as animações da interface: %v", callErr)
	}
	return nil
}

func currentPowerScheme() (string, error) {
	out, err := hiddenCommand("powercfg.exe", "/getactivescheme").CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("não foi possível consultar o plano de energia: %s", strings.TrimSpace(string(out)))
	}
	guid := powerSchemeGUIDPattern.FindString(string(out))
	if guid == "" {
		return "", errors.New("o Windows não informou o plano de energia ativo")
	}
	return strings.ToLower(guid), nil
}

func availablePowerSchemes() (map[string]bool, error) {
	out, err := hiddenCommand("powercfg.exe", "/list").CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("não foi possível listar os planos de energia: %s", strings.TrimSpace(string(out)))
	}
	result := map[string]bool{}
	for _, guid := range powerSchemeGUIDPattern.FindAllString(string(out), -1) {
		result[strings.ToLower(guid)] = true
	}
	return result, nil
}

func setPowerScheme(guid string) error {
	guid = strings.ToLower(strings.TrimSpace(guid))
	if !powerSchemeGUIDPattern.MatchString(guid) {
		return errors.New("identificador de plano de energia inválido")
	}
	out, err := hiddenCommand("powercfg.exe", "/setactive", guid).CombinedOutput()
	if err != nil {
		detail := strings.TrimSpace(string(out))
		if detail == "" {
			detail = err.Error()
		}
		return fmt.Errorf("não foi possível ativar o plano de energia: %s", detail)
	}
	return nil
}

func runningOnBattery() bool {
	var status systemPowerStatus
	ok, _, _ := procGetSystemPowerStatus.Call(uintptr(unsafe.Pointer(&status)))
	// Alto desempenho só é permitido quando o Windows confirma alimentação pela
	// tomada. Falha de leitura ou estado desconhecido usa o caminho conservador.
	if ok == 0 {
		return true
	}
	return status.ACLineStatus != 1
}

func getProcessPriority(pid uint32) (uint32, error) {
	handle, _, callErr := nativeOpenProcess.Call(processQueryLimited, 0, uintptr(pid))
	if handle == 0 {
		return 0, fmt.Errorf("não foi possível abrir o processo %d: %v", pid, callErr)
	}
	defer nativeCloseHandle.Call(handle)
	priority, _, callErr := procGetPriorityClassOptimizer.Call(handle)
	if priority == 0 {
		return 0, fmt.Errorf("não foi possível consultar a prioridade do processo %d: %v", pid, callErr)
	}
	return uint32(priority), nil
}

func setProcessPriority(pid uint32, priority uint32) error {
	handle, _, callErr := nativeOpenProcess.Call(processQueryLimited|processSetInformation, 0, uintptr(pid))
	if handle == 0 {
		return fmt.Errorf("não foi possível abrir o processo %d para ajuste: %v", pid, callErr)
	}
	defer nativeCloseHandle.Call(handle)
	ok, _, callErr := procSetPriorityClassOptimizer.Call(handle, uintptr(priority))
	if ok == 0 {
		return fmt.Errorf("não foi possível alterar a prioridade do processo %d: %v", pid, callErr)
	}
	return nil
}

func workProcessesToPrioritize() []ProcessInfo {
	all := nativeProcesses()
	byPID := make(map[int]string, len(all))
	for _, proc := range all {
		byPID[proc.PID] = normalizedProcessName(proc.Name)
	}
	var targets []ProcessInfo
	for _, proc := range all {
		name := normalizedProcessName(proc.Name)
		if !isWorkProcess(name) {
			continue
		}
		// Browser and communication apps spawn many child processes. Changing only
		// the root process avoids raising dozens of renderer processes.
		if parentName, ok := byPID[proc.ParentPID]; ok && parentName == name {
			continue
		}
		targets = append(targets, proc)
	}
	sort.Slice(targets, func(i, j int) bool {
		if normalizedProcessName(targets[i].Name) == normalizedProcessName(targets[j].Name) {
			return targets[i].PID < targets[j].PID
		}
		return normalizedProcessName(targets[i].Name) < normalizedProcessName(targets[j].Name)
	})
	if len(targets) > 12 {
		targets = targets[:12]
	}
	return targets
}

func createOptimizationBaseline(path string) (*OptimizationState, error) {
	animations, err := currentAnimationSnapshot()
	if err != nil {
		return nil, err
	}
	power, err := currentPowerScheme()
	if err != nil {
		return nil, err
	}
	now := time.Now()
	state := &OptimizationState{
		Version:             optimizationStateVersion,
		CreatedAt:           now,
		UpdatedAt:           now,
		OriginalAnimations:  animations,
		OriginalPowerScheme: power,
		ProcessPriorities:   map[string]ProcessPrioritySnapshot{},
	}
	// The backup must exist on disk before the first Windows setting is changed.
	if err := saveOptimizationState(path, state); err != nil {
		return nil, fmt.Errorf("não foi possível salvar o backup automático; nenhuma alteração foi aplicada: %w", err)
	}
	return state, nil
}

func ensureProcessBaseline(path string, state *OptimizationState, targets []ProcessInfo) []string {
	var warnings []string
	changedState := false
	for _, proc := range targets {
		key := processPriorityKey(uint32(proc.PID), proc.Name)
		if _, exists := state.ProcessPriorities[key]; exists {
			continue
		}
		priority, err := getProcessPriority(uint32(proc.PID))
		if err != nil {
			warnings = append(warnings, normalizedProcessName(proc.Name)+": prioridade original não pôde ser lida")
			continue
		}
		state.ProcessPriorities[key] = ProcessPrioritySnapshot{PID: uint32(proc.PID), Name: normalizedProcessName(proc.Name), Priority: priority}
		changedState = true
	}
	if changedState {
		if err := saveOptimizationState(path, state); err != nil {
			warnings = append(warnings, "novas prioridades não foram alteradas porque o backup não pôde ser atualizado")
			// Remove entries that were not safely persisted by reloading the disk state.
			if persisted, loadErr := loadOptimizationState(path); loadErr == nil && persisted != nil {
				state.ProcessPriorities = persisted.ProcessPriorities
			}
		}
	}
	return warnings
}

func applyOptimizationProfile(profile int) (OptimizationResult, error) {
	plan, err := optimizationPlan(profile, runningOnBattery())
	if err != nil {
		return OptimizationResult{}, err
	}
	if profile == 5 {
		return restoreOptimizationOriginal()
	}

	path := optimizationStatePath()
	state, err := loadOptimizationState(path)
	if err != nil {
		return OptimizationResult{}, err
	}
	if state == nil {
		state, err = createOptimizationBaseline(path)
		if err != nil {
			return OptimizationResult{}, err
		}
	}

	result := OptimizationResult{ProfileName: plan.Name, AppliedAt: time.Now()}
	if plan.MinimizeWindows != nil {
		if err := setMinimizeAnimation(*plan.MinimizeWindows); err != nil {
			result.Warnings = append(result.Warnings, err.Error())
		} else {
			result.Changed = append(result.Changed, "Animação de minimizar e maximizar janelas reduzida")
		}
	}
	if plan.ClientAreaAnimations != nil {
		if err := setClientAreaAnimation(*plan.ClientAreaAnimations); err != nil {
			result.Warnings = append(result.Warnings, err.Error())
		} else {
			result.Changed = append(result.Changed, "Animações transitórias da interface reduzidas")
		}
	}
	if plan.PowerSchemeGUID != "" {
		schemes, listErr := availablePowerSchemes()
		if listErr != nil {
			result.Warnings = append(result.Warnings, listErr.Error())
		} else if !schemes[strings.ToLower(plan.PowerSchemeGUID)] {
			result.Warnings = append(result.Warnings, "O plano "+plan.PowerSchemeLabel+" não está disponível neste computador; o plano atual foi preservado")
		} else if err := setPowerScheme(plan.PowerSchemeGUID); err != nil {
			result.Warnings = append(result.Warnings, err.Error())
		} else {
			result.Changed = append(result.Changed, "Plano de energia alterado para "+plan.PowerSchemeLabel)
		}
	}
	result.Warnings = append(result.Warnings, plan.Notes...)

	if plan.PrioritizeWorkApps {
		targets := workProcessesToPrioritize()
		result.Warnings = append(result.Warnings, ensureProcessBaseline(path, state, targets)...)
		prioritized := 0
		for _, proc := range targets {
			key := processPriorityKey(uint32(proc.PID), proc.Name)
			if _, safelyBackedUp := state.ProcessPriorities[key]; !safelyBackedUp {
				continue
			}
			current, priorityErr := getProcessPriority(uint32(proc.PID))
			if priorityErr != nil {
				continue
			}
			if current == aboveNormalPriority {
				continue
			}
			if priorityErr = setProcessPriority(uint32(proc.PID), aboveNormalPriority); priorityErr != nil {
				result.Warnings = append(result.Warnings, normalizedProcessName(proc.Name)+": não foi possível ajustar a prioridade")
				continue
			}
			prioritized++
		}
		if prioritized > 0 {
			result.Changed = append(result.Changed, fmt.Sprintf("%d aplicativo(s) de atendimento priorizado(s) moderadamente", prioritized))
		} else if len(targets) == 0 {
			result.Warnings = append(result.Warnings, "Nenhum aplicativo de atendimento compatível estava aberto; execute o perfil novamente depois de abri-los")
		}
	}

	state.ActiveProfile = profile
	state.ActiveProfileName = plan.Name
	state.AppliedChanges = append([]string(nil), result.Changed...)
	state.Warnings = append([]string(nil), result.Warnings...)
	if err := saveOptimizationState(path, state); err != nil {
		// The original baseline was already saved. Keep the active file and report
		// that the status metadata could not be refreshed, without pretending the
		// Windows changes failed.
		result.Warnings = append(result.Warnings, "As alterações foram aplicadas, mas o resumo do perfil não pôde ser atualizado: "+err.Error())
	}
	if len(result.Changed) == 0 {
		return result, errors.New("nenhum ajuste pôde ser aplicado; as configurações originais permanecem salvas")
	}
	return result, nil
}

func restoreOptimizationOriginal() (OptimizationResult, error) {
	path := optimizationStatePath()
	state, err := loadOptimizationState(path)
	if err != nil {
		return OptimizationResult{}, err
	}
	if state == nil {
		return OptimizationResult{}, errors.New("não existe backup ativo para restaurar; o CoreControl ainda não alterou este computador")
	}

	result := OptimizationResult{ProfileName: "Desativar otimização", Restored: true, AppliedAt: time.Now()}
	var failures []string
	if err := setMinimizeAnimation(state.OriginalAnimations.MinimizeWindows); err != nil {
		failures = append(failures, err.Error())
	} else {
		result.Changed = append(result.Changed, "Animação original das janelas restaurada")
	}
	if err := setClientAreaAnimation(state.OriginalAnimations.ClientArea); err != nil {
		failures = append(failures, err.Error())
	} else {
		result.Changed = append(result.Changed, "Animações originais da interface restauradas")
	}
	if state.OriginalPowerScheme != "" {
		if err := setPowerScheme(state.OriginalPowerScheme); err != nil {
			failures = append(failures, err.Error())
		} else {
			result.Changed = append(result.Changed, "Plano de energia original restaurado")
		}
	}

	all := nativeProcesses()
	byPID := make(map[uint32]ProcessInfo, len(all))
	for _, proc := range all {
		byPID[uint32(proc.PID)] = proc
	}
	restoredProcesses := 0
	for _, saved := range state.ProcessPriorities {
		proc, running := byPID[saved.PID]
		if !running || normalizedProcessName(proc.Name) != normalizedProcessName(saved.Name) {
			// The process ended or Windows reused the PID. Its changed priority no
			// longer exists, so there is nothing unsafe to restore.
			continue
		}
		if err := setProcessPriority(saved.PID, saved.Priority); err != nil {
			failures = append(failures, normalizedProcessName(saved.Name)+": prioridade original não pôde ser restaurada")
			continue
		}
		restoredProcesses++
	}
	if restoredProcesses > 0 {
		result.Changed = append(result.Changed, fmt.Sprintf("Prioridade original de %d aplicativo(s) restaurada", restoredProcesses))
	}

	if len(failures) > 0 {
		state.Warnings = failures
		_ = saveOptimizationState(path, state)
		result.Warnings = failures
		return result, errors.New("a restauração ficou incompleta; o backup foi mantido para uma nova tentativa")
	}
	archive, err := archiveOptimizationState(path, result.AppliedAt)
	if err != nil {
		result.Warnings = append(result.Warnings, "As configurações foram restauradas, mas o backup não pôde ser arquivado: "+err.Error())
		return result, nil
	}
	result.Changed = append(result.Changed, "Backup arquivado em "+filepath.Dir(archive))
	return result, nil
}

func loadOptimizationSummary() (activeProfile int, appliedAt time.Time, note string) {
	state, err := loadOptimizationState(optimizationStatePath())
	if err != nil {
		return 0, time.Time{}, "Backup de otimização precisa de revisão: " + err.Error()
	}
	if state == nil {
		return 0, time.Time{}, "Nenhum perfil ativo. O CoreControl ainda não alterou o Windows."
	}
	return state.ActiveProfile, state.UpdatedAt, "Backup automático disponível para desativar a otimização e restaurar o estado anterior."
}

func optimizationBackupDirectory() string {
	return filepath.Join(dataDir(), "Backups")
}

func ensureOptimizationDirectories() error {
	return os.MkdirAll(optimizationBackupDirectory(), 0755)
}
