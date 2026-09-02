//go:build windows

package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"syscall"
	"time"
	"unicode/utf16"
	"unsafe"
)

const (
	optimizationStateVersion = 1

	optSPIGetAnimation           = 0x0048
	optSPISetAnimation           = 0x0049
	optSPIGetClientAreaAnimation = 0x1042
	optSPISetClientAreaAnimation = 0x1043
	optSPIFUpdateIniFile         = 0x0001
	optSPIFSendChange            = 0x0002

	optProcessQueryLimited   = 0x1000
	optProcessSetInformation = 0x0200
	optAboveNormalPriority   = 0x00008000
)

type optimizationAnimationSnapshot struct {
	MinimizeWindows bool `json:"minimize_windows"`
	ClientArea      bool `json:"client_area"`
}

type optimizationProcessPrioritySnapshot struct {
	PID      uint32 `json:"pid"`
	Name     string `json:"name"`
	Priority uint32 `json:"priority"`
}

type optimizationState struct {
	Version             int                                            `json:"version"`
	CreatedAt           time.Time                                      `json:"created_at"`
	UpdatedAt           time.Time                                      `json:"updated_at"`
	ActiveProfile       int                                            `json:"active_profile"`
	ActiveProfileName   string                                         `json:"active_profile_name"`
	OriginalAnimations  optimizationAnimationSnapshot                  `json:"original_animations"`
	OriginalPowerScheme string                                         `json:"original_power_scheme"`
	ProcessPriorities   map[string]optimizationProcessPrioritySnapshot `json:"process_priorities,omitempty"`
	AppliedChanges      []string                                       `json:"applied_changes,omitempty"`
	Warnings            []string                                       `json:"warnings,omitempty"`
}

type optimizationPlan struct {
	Profile              int
	Name                 string
	MinimizeWindows      *bool
	ClientAreaAnimations *bool
	PowerSchemeGUID      string
	PowerSchemeLabel     string
	PrioritizeWorkApps   bool
	Notes                []string
}

type optimizationResult struct {
	Profile           int      `json:"profile"`
	ProfileName       string   `json:"profile_name"`
	ActiveProfile     int      `json:"active_profile"`
	ActiveProfileName string   `json:"active_profile_name"`
	Changed           []string `json:"changed"`
	Warnings          []string `json:"warnings"`
	Restored          bool     `json:"restored"`
	AppliedAt         string   `json:"applied_at"`
	BackupAvailable   bool     `json:"backup_available"`
}

type optimizationAnimationInfo struct {
	CbSize     uint32
	MinAnimate int32
}

type optimizationPowerStatus struct {
	ACLineStatus        byte
	BatteryFlag         byte
	BatteryLifePercent  byte
	SystemStatusFlag    byte
	BatteryLifeTime     uint32
	BatteryFullLifeTime uint32
}

type optimizationProcessInfo struct {
	PID       uint32
	ParentPID uint32
	Name      string
}

var (
	optimizationUser32               = syscall.NewLazyDLL("user32.dll")
	optimizationKernel32             = syscall.NewLazyDLL("kernel32.dll")
	optimizationSystemParametersInfo = optimizationUser32.NewProc("SystemParametersInfoW")
	optimizationGetPowerStatus       = optimizationKernel32.NewProc("GetSystemPowerStatus")
	optimizationOpenProcess          = optimizationKernel32.NewProc("OpenProcess")
	optimizationGetPriorityClass     = optimizationKernel32.NewProc("GetPriorityClass")
	optimizationSetPriorityClass     = optimizationKernel32.NewProc("SetPriorityClass")
	optimizationPowerGUIDPattern     = regexp.MustCompile(`(?i)[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}`)
)

func optimizationBool(value bool) *bool { return &value }

func optimizationProfileName(profile int) string {
	switch profile {
	case 1:
		return "Conservador"
	case 2:
		return "Equilibrado"
	case 3:
		return "Modo Atendimento"
	case 4:
		return "Alto Desempenho"
	case 5:
		return "Desativar otimização"
	default:
		return ""
	}
}

func optimizationBuildPlan(profile int, onBattery bool) (optimizationPlan, error) {
	const balanced = "381b4222-f694-41f0-9685-ff5bb260df2e"
	const highPerformance = "8c5e7fda-e8bf-4a96-9a85-a6e23a8c635c"
	plan := optimizationPlan{Profile: profile, Name: optimizationProfileName(profile)}
	if plan.Name == "" {
		return plan, errors.New("perfil de otimização inválido")
	}
	switch profile {
	case 1:
		plan.ClientAreaAnimations = optimizationBool(false)
	case 2:
		plan.MinimizeWindows = optimizationBool(false)
		plan.ClientAreaAnimations = optimizationBool(false)
		plan.PowerSchemeGUID = balanced
		plan.PowerSchemeLabel = "Equilibrado"
	case 3:
		plan.MinimizeWindows = optimizationBool(false)
		plan.ClientAreaAnimations = optimizationBool(false)
		plan.PowerSchemeGUID = balanced
		plan.PowerSchemeLabel = "Equilibrado"
		plan.PrioritizeWorkApps = true
	case 4:
		plan.MinimizeWindows = optimizationBool(false)
		plan.ClientAreaAnimations = optimizationBool(false)
		plan.PrioritizeWorkApps = true
		if onBattery {
			plan.PowerSchemeGUID = balanced
			plan.PowerSchemeLabel = "Equilibrado"
			plan.Notes = []string{"O plano Alto desempenho não é ativado enquanto o computador estiver usando bateria."}
		} else {
			plan.PowerSchemeGUID = highPerformance
			plan.PowerSchemeLabel = "Alto desempenho"
		}
	}
	return plan, nil
}

func optimizationDataDir() string {
	if local := strings.TrimSpace(os.Getenv("LOCALAPPDATA")); local != "" {
		return filepath.Join(local, "CoreTuner")
	}
	home, _ := os.UserHomeDir()
	if home != "" {
		return filepath.Join(home, "AppData", "Local", "CoreTuner")
	}
	return filepath.Join(os.TempDir(), "CoreTuner")
}

func optimizationStatePath() string {
	return filepath.Join(optimizationDataDir(), "optimization-state.json")
}

func loadOptimizationState() (*optimizationState, error) {
	raw, err := os.ReadFile(optimizationStatePath())
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var state optimizationState
	if err := json.Unmarshal(raw, &state); err != nil {
		return nil, fmt.Errorf("backup de otimização inválido: %w", err)
	}
	if state.Version != optimizationStateVersion {
		return nil, fmt.Errorf("versão de backup de otimização não suportada: %d", state.Version)
	}
	if state.ProcessPriorities == nil {
		state.ProcessPriorities = map[string]optimizationProcessPrioritySnapshot{}
	}
	return &state, nil
}

func saveOptimizationState(state *optimizationState) error {
	if state == nil {
		return errors.New("estado de otimização ausente")
	}
	state.Version = optimizationStateVersion
	state.UpdatedAt = time.Now()
	if state.ProcessPriorities == nil {
		state.ProcessPriorities = map[string]optimizationProcessPrioritySnapshot{}
	}
	raw, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	path := optimizationStatePath()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, raw, 0600); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return nil
}

func archiveOptimizationState(restoredAt time.Time) (string, error) {
	path := optimizationStatePath()
	if _, err := os.Stat(path); err != nil {
		return "", err
	}
	archiveDir := filepath.Join(filepath.Dir(path), "Backups")
	if err := os.MkdirAll(archiveDir, 0755); err != nil {
		return "", err
	}
	archive := filepath.Join(archiveDir, "optimization-restored-"+restoredAt.Format("20060102-150405")+".json")
	if err := os.Rename(path, archive); err != nil {
		return "", err
	}
	return archive, nil
}

func currentOptimizationProfileName() string {
	state, err := loadOptimizationState()
	if err != nil || state == nil || state.ActiveProfile <= 0 {
		return "Nenhum"
	}
	if name := strings.TrimSpace(state.ActiveProfileName); name != "" {
		return name
	}
	if name := optimizationProfileName(state.ActiveProfile); name != "" {
		return name
	}
	return "Nenhum"
}

func optimizationAnimationState() (optimizationAnimationSnapshot, error) {
	var clientArea int32
	ok, _, callErr := optimizationSystemParametersInfo.Call(optSPIGetClientAreaAnimation, 0, uintptr(unsafe.Pointer(&clientArea)), 0)
	if ok == 0 {
		return optimizationAnimationSnapshot{}, fmt.Errorf("não foi possível ler as animações da interface: %v", callErr)
	}
	info := optimizationAnimationInfo{CbSize: uint32(unsafe.Sizeof(optimizationAnimationInfo{}))}
	ok, _, callErr = optimizationSystemParametersInfo.Call(optSPIGetAnimation, uintptr(info.CbSize), uintptr(unsafe.Pointer(&info)), 0)
	if ok == 0 {
		return optimizationAnimationSnapshot{}, fmt.Errorf("não foi possível ler a animação das janelas: %v", callErr)
	}
	return optimizationAnimationSnapshot{MinimizeWindows: info.MinAnimate != 0, ClientArea: clientArea != 0}, nil
}

func optimizationSetMinimizeAnimation(enabled bool) error {
	value := int32(0)
	if enabled {
		value = 1
	}
	info := optimizationAnimationInfo{CbSize: uint32(unsafe.Sizeof(optimizationAnimationInfo{})), MinAnimate: value}
	ok, _, callErr := optimizationSystemParametersInfo.Call(optSPISetAnimation, uintptr(info.CbSize), uintptr(unsafe.Pointer(&info)), optSPIFUpdateIniFile|optSPIFSendChange)
	if ok == 0 {
		return fmt.Errorf("não foi possível ajustar a animação das janelas: %v", callErr)
	}
	return nil
}

func optimizationSetClientAreaAnimation(enabled bool) error {
	value := int32(0)
	if enabled {
		value = 1
	}
	ok, _, callErr := optimizationSystemParametersInfo.Call(optSPISetClientAreaAnimation, 0, uintptr(unsafe.Pointer(&value)), optSPIFUpdateIniFile|optSPIFSendChange)
	if ok == 0 {
		return fmt.Errorf("não foi possível ajustar as animações da interface: %v", callErr)
	}
	return nil
}

func optimizationHiddenCommand(name string, args ...string) *exec.Cmd {
	cmd := exec.Command(name, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000}
	return cmd
}

func optimizationCurrentPowerScheme() (string, error) {
	out, err := optimizationHiddenCommand("powercfg.exe", "/getactivescheme").CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("não foi possível consultar o plano de energia: %s", strings.TrimSpace(string(out)))
	}
	guid := optimizationPowerGUIDPattern.FindString(string(out))
	if guid == "" {
		return "", errors.New("o Windows não informou o plano de energia ativo")
	}
	return strings.ToLower(guid), nil
}

func optimizationAvailablePowerSchemes() (map[string]bool, error) {
	out, err := optimizationHiddenCommand("powercfg.exe", "/list").CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("não foi possível listar os planos de energia: %s", strings.TrimSpace(string(out)))
	}
	result := map[string]bool{}
	for _, guid := range optimizationPowerGUIDPattern.FindAllString(string(out), -1) {
		result[strings.ToLower(guid)] = true
	}
	return result, nil
}

func optimizationSetPowerScheme(guid string) error {
	guid = strings.ToLower(strings.TrimSpace(guid))
	if !optimizationPowerGUIDPattern.MatchString(guid) {
		return errors.New("identificador de plano de energia inválido")
	}
	out, err := optimizationHiddenCommand("powercfg.exe", "/setactive", guid).CombinedOutput()
	if err != nil {
		detail := strings.TrimSpace(string(out))
		if detail == "" {
			detail = err.Error()
		}
		return fmt.Errorf("não foi possível ativar o plano de energia: %s", detail)
	}
	return nil
}

func optimizationRunningOnBattery() bool {
	var status optimizationPowerStatus
	ok, _, _ := optimizationGetPowerStatus.Call(uintptr(unsafe.Pointer(&status)))
	if ok == 0 {
		return true
	}
	return status.ACLineStatus != 1
}

func optimizationNormalizeProcessName(name string) string {
	name = strings.ToLower(strings.TrimSpace(name))
	return strings.TrimSuffix(name, ".exe")
}

func optimizationIsWorkProcess(name string) bool {
	switch optimizationNormalizeProcessName(name) {
	case "chrome", "msedge", "firefox", "brave", "opera", "opera_gx", "whatsapp", "zapschat", "zapschat-desktop", "teams", "ms-teams", "zoiper", "microsip", "3cx", "3cxdesktopapp", "softphone", "discador":
		return true
	default:
		return false
	}
}

func optimizationProcesses() []optimizationProcessInfo {
	snapshot, _, _ := procCreateToolhelp32Snap.Call(th32csSnapProcess, 0)
	if snapshot == 0 || snapshot == ^uintptr(0) {
		return nil
	}
	defer procCloseHandle.Call(snapshot)
	out := []optimizationProcessInfo{}
	entry := processEntry32{Size: uint32(unsafe.Sizeof(processEntry32{}))}
	ok, _, _ := procProcess32FirstW.Call(snapshot, uintptr(unsafe.Pointer(&entry)))
	for ok != 0 {
		name := strings.TrimSpace(string(utf16.Decode(trimUTF16(entry.ExeFile[:]))))
		if name != "" {
			out = append(out, optimizationProcessInfo{PID: entry.ProcessID, ParentPID: entry.ParentProcessID, Name: name})
		}
		entry.Size = uint32(unsafe.Sizeof(processEntry32{}))
		ok, _, _ = procProcess32NextW.Call(snapshot, uintptr(unsafe.Pointer(&entry)))
	}
	return out
}

func optimizationWorkProcesses() []optimizationProcessInfo {
	all := optimizationProcesses()
	byPID := make(map[uint32]string, len(all))
	for _, proc := range all {
		byPID[proc.PID] = optimizationNormalizeProcessName(proc.Name)
	}
	targets := []optimizationProcessInfo{}
	for _, proc := range all {
		name := optimizationNormalizeProcessName(proc.Name)
		if !optimizationIsWorkProcess(name) {
			continue
		}
		if parentName, ok := byPID[proc.ParentPID]; ok && parentName == name {
			continue
		}
		targets = append(targets, proc)
	}
	sort.Slice(targets, func(i, j int) bool {
		ni, nj := optimizationNormalizeProcessName(targets[i].Name), optimizationNormalizeProcessName(targets[j].Name)
		if ni == nj {
			return targets[i].PID < targets[j].PID
		}
		return ni < nj
	})
	if len(targets) > 12 {
		targets = targets[:12]
	}
	return targets
}

func optimizationGetPriority(pid uint32) (uint32, error) {
	handle, _, callErr := optimizationOpenProcess.Call(optProcessQueryLimited, 0, uintptr(pid))
	if handle == 0 {
		return 0, fmt.Errorf("não foi possível abrir o processo %d: %v", pid, callErr)
	}
	defer procCloseHandle.Call(handle)
	priority, _, callErr := optimizationGetPriorityClass.Call(handle)
	if priority == 0 {
		return 0, fmt.Errorf("não foi possível consultar a prioridade do processo %d: %v", pid, callErr)
	}
	return uint32(priority), nil
}

func optimizationSetPriority(pid uint32, priority uint32) error {
	handle, _, callErr := optimizationOpenProcess.Call(optProcessQueryLimited|optProcessSetInformation, 0, uintptr(pid))
	if handle == 0 {
		return fmt.Errorf("não foi possível abrir o processo %d para ajuste: %v", pid, callErr)
	}
	defer procCloseHandle.Call(handle)
	ok, _, callErr := optimizationSetPriorityClass.Call(handle, uintptr(priority))
	if ok == 0 {
		return fmt.Errorf("não foi possível alterar a prioridade do processo %d: %v", pid, callErr)
	}
	return nil
}

func optimizationPriorityKey(pid uint32, name string) string {
	return fmt.Sprintf("%d:%s", pid, optimizationNormalizeProcessName(name))
}

func createOptimizationBaseline() (*optimizationState, error) {
	animations, err := optimizationAnimationState()
	if err != nil {
		return nil, err
	}
	power, err := optimizationCurrentPowerScheme()
	if err != nil {
		return nil, err
	}
	now := time.Now()
	state := &optimizationState{
		Version: optimizationStateVersion, CreatedAt: now, UpdatedAt: now,
		OriginalAnimations: animations, OriginalPowerScheme: power,
		ProcessPriorities: map[string]optimizationProcessPrioritySnapshot{},
	}
	if err := saveOptimizationState(state); err != nil {
		return nil, fmt.Errorf("não foi possível salvar o backup automático; nenhuma alteração foi aplicada: %w", err)
	}
	return state, nil
}

func ensureOptimizationProcessBaseline(state *optimizationState, targets []optimizationProcessInfo) []string {
	warnings := []string{}
	changed := false
	for _, proc := range targets {
		key := optimizationPriorityKey(proc.PID, proc.Name)
		if _, exists := state.ProcessPriorities[key]; exists {
			continue
		}
		priority, err := optimizationGetPriority(proc.PID)
		if err != nil {
			warnings = append(warnings, optimizationNormalizeProcessName(proc.Name)+": prioridade original não pôde ser lida")
			continue
		}
		state.ProcessPriorities[key] = optimizationProcessPrioritySnapshot{PID: proc.PID, Name: optimizationNormalizeProcessName(proc.Name), Priority: priority}
		changed = true
	}
	if changed {
		if err := saveOptimizationState(state); err != nil {
			warnings = append(warnings, "novas prioridades não foram alteradas porque o backup não pôde ser atualizado")
			if persisted, loadErr := loadOptimizationState(); loadErr == nil && persisted != nil {
				state.ProcessPriorities = persisted.ProcessPriorities
			}
		}
	}
	return warnings
}

func applyOptimizationProfile(profile int) (optimizationResult, error) {
	plan, err := optimizationBuildPlan(profile, optimizationRunningOnBattery())
	if err != nil {
		return optimizationResult{}, err
	}
	if profile == 5 {
		return restoreOptimizationOriginal()
	}

	state, err := loadOptimizationState()
	if err != nil {
		return optimizationResult{}, err
	}
	if state == nil {
		state, err = createOptimizationBaseline()
		if err != nil {
			return optimizationResult{}, err
		}
	}

	now := time.Now()
	result := optimizationResult{Profile: profile, ProfileName: plan.Name, Changed: []string{}, Warnings: []string{}, AppliedAt: now.UTC().Format(time.RFC3339), BackupAvailable: true}

	if plan.MinimizeWindows != nil {
		if err := optimizationSetMinimizeAnimation(*plan.MinimizeWindows); err != nil {
			result.Warnings = append(result.Warnings, err.Error())
		} else {
			result.Changed = append(result.Changed, "Animação de minimizar e maximizar janelas reduzida")
		}
	}
	if plan.ClientAreaAnimations != nil {
		if err := optimizationSetClientAreaAnimation(*plan.ClientAreaAnimations); err != nil {
			result.Warnings = append(result.Warnings, err.Error())
		} else {
			result.Changed = append(result.Changed, "Animações transitórias da interface reduzidas")
		}
	}
	if plan.PowerSchemeGUID != "" {
		schemes, listErr := optimizationAvailablePowerSchemes()
		if listErr != nil {
			result.Warnings = append(result.Warnings, listErr.Error())
		} else if !schemes[strings.ToLower(plan.PowerSchemeGUID)] {
			result.Warnings = append(result.Warnings, "O plano "+plan.PowerSchemeLabel+" não está disponível neste computador; o plano atual foi preservado")
		} else if err := optimizationSetPowerScheme(plan.PowerSchemeGUID); err != nil {
			result.Warnings = append(result.Warnings, err.Error())
		} else {
			result.Changed = append(result.Changed, "Plano de energia alterado para "+plan.PowerSchemeLabel)
		}
	}
	result.Warnings = append(result.Warnings, plan.Notes...)

	if plan.PrioritizeWorkApps {
		targets := optimizationWorkProcesses()
		result.Warnings = append(result.Warnings, ensureOptimizationProcessBaseline(state, targets)...)
		prioritized := 0
		for _, proc := range targets {
			key := optimizationPriorityKey(proc.PID, proc.Name)
			if _, backedUp := state.ProcessPriorities[key]; !backedUp {
				continue
			}
			current, priorityErr := optimizationGetPriority(proc.PID)
			if priorityErr != nil || current == optAboveNormalPriority {
				continue
			}
			if err := optimizationSetPriority(proc.PID, optAboveNormalPriority); err != nil {
				result.Warnings = append(result.Warnings, optimizationNormalizeProcessName(proc.Name)+": não foi possível ajustar a prioridade")
				continue
			}
			prioritized++
		}
		if prioritized > 0 {
			result.Changed = append(result.Changed, fmt.Sprintf("%d aplicativo(s) de atendimento priorizado(s) moderadamente", prioritized))
		} else if len(targets) == 0 {
			result.Warnings = append(result.Warnings, "Nenhum aplicativo de atendimento compatível estava aberto; aplique o perfil novamente depois de abri-los")
		}
	}

	state.ActiveProfile = profile
	state.ActiveProfileName = plan.Name
	state.AppliedChanges = append([]string(nil), result.Changed...)
	state.Warnings = append([]string(nil), result.Warnings...)
	if err := saveOptimizationState(state); err != nil {
		result.Warnings = append(result.Warnings, "As alterações foram aplicadas, mas o resumo do perfil não pôde ser atualizado: "+err.Error())
	}
	result.ActiveProfile = profile
	result.ActiveProfileName = plan.Name
	if len(result.Changed) == 0 {
		return result, errors.New("nenhum ajuste pôde ser aplicado; as configurações originais permanecem salvas")
	}
	return result, nil
}

func restoreOptimizationOriginal() (optimizationResult, error) {
	state, err := loadOptimizationState()
	if err != nil {
		return optimizationResult{}, err
	}
	if state == nil {
		return optimizationResult{}, errors.New("não existe backup ativo para restaurar; o CoreControl ainda não alterou este computador")
	}

	now := time.Now()
	result := optimizationResult{Profile: 5, ProfileName: "Desativar otimização", Changed: []string{}, Warnings: []string{}, Restored: true, AppliedAt: now.UTC().Format(time.RFC3339)}
	failures := []string{}
	if err := optimizationSetMinimizeAnimation(state.OriginalAnimations.MinimizeWindows); err != nil {
		failures = append(failures, err.Error())
	} else {
		result.Changed = append(result.Changed, "Animação original das janelas restaurada")
	}
	if err := optimizationSetClientAreaAnimation(state.OriginalAnimations.ClientArea); err != nil {
		failures = append(failures, err.Error())
	} else {
		result.Changed = append(result.Changed, "Animações originais da interface restauradas")
	}
	if state.OriginalPowerScheme != "" {
		if err := optimizationSetPowerScheme(state.OriginalPowerScheme); err != nil {
			failures = append(failures, err.Error())
		} else {
			result.Changed = append(result.Changed, "Plano de energia original restaurado")
		}
	}

	running := optimizationProcesses()
	byPID := make(map[uint32]optimizationProcessInfo, len(running))
	for _, proc := range running {
		byPID[proc.PID] = proc
	}
	restoredProcesses := 0
	for _, saved := range state.ProcessPriorities {
		proc, exists := byPID[saved.PID]
		if !exists || optimizationNormalizeProcessName(proc.Name) != optimizationNormalizeProcessName(saved.Name) {
			continue
		}
		if err := optimizationSetPriority(saved.PID, saved.Priority); err != nil {
			failures = append(failures, optimizationNormalizeProcessName(saved.Name)+": prioridade original não pôde ser restaurada")
			continue
		}
		restoredProcesses++
	}
	if restoredProcesses > 0 {
		result.Changed = append(result.Changed, fmt.Sprintf("Prioridade original de %d aplicativo(s) restaurada", restoredProcesses))
	}

	if len(failures) > 0 {
		state.Warnings = failures
		_ = saveOptimizationState(state)
		result.Warnings = failures
		result.ActiveProfile = state.ActiveProfile
		result.ActiveProfileName = state.ActiveProfileName
		result.BackupAvailable = true
		return result, errors.New("a restauração ficou incompleta; o backup foi mantido para uma nova tentativa")
	}
	if _, err := archiveOptimizationState(now); err != nil {
		result.Warnings = append(result.Warnings, "As configurações foram restauradas, mas o backup não pôde ser arquivado: "+err.Error())
		return result, nil
	}
	result.ActiveProfile = 0
	result.ActiveProfileName = "Nenhum"
	result.BackupAvailable = false
	return result, nil
}
