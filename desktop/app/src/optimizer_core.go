package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const optimizationStateVersion = 1

// AnimationSnapshot stores only Windows settings that CoreTuner is allowed to
// change. Each value is captured before the first optimization and restored
// exactly when the user chooses "Restaurar Original".
type AnimationSnapshot struct {
	MinimizeWindows bool `json:"minimize_windows"`
	ClientArea      bool `json:"client_area"`
}

type ProcessPrioritySnapshot struct {
	PID      uint32 `json:"pid"`
	Name     string `json:"name"`
	Priority uint32 `json:"priority"`
}

type OptimizationState struct {
	Version             int                                `json:"version"`
	CreatedAt           time.Time                          `json:"created_at"`
	UpdatedAt           time.Time                          `json:"updated_at"`
	ActiveProfile       int                                `json:"active_profile"`
	ActiveProfileName   string                             `json:"active_profile_name"`
	OriginalAnimations  AnimationSnapshot                  `json:"original_animations"`
	OriginalPowerScheme string                             `json:"original_power_scheme"`
	ProcessPriorities   map[string]ProcessPrioritySnapshot `json:"process_priorities,omitempty"`
	AppliedChanges      []string                           `json:"applied_changes,omitempty"`
	Warnings            []string                           `json:"warnings,omitempty"`
}

type OptimizationPlan struct {
	Profile              int
	Name                 string
	MinimizeWindows      *bool
	ClientAreaAnimations *bool
	PowerSchemeGUID      string
	PowerSchemeLabel     string
	PrioritizeWorkApps   bool
	Notes                []string
}

type OptimizationResult struct {
	ProfileName string
	Changed     []string
	Warnings    []string
	Restored    bool
	AppliedAt   time.Time
}

func boolPtr(value bool) *bool { return &value }

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
		return "Restaurar Original"
	default:
		return ""
	}
}

func optimizationPlan(profile int, onBattery bool) (OptimizationPlan, error) {
	balanced := "381b4222-f694-41f0-9685-ff5bb260df2e"
	highPerformance := "8c5e7fda-e8bf-4a96-9a85-a6e23a8c635c"
	plan := OptimizationPlan{Profile: profile, Name: optimizationProfileName(profile)}
	if plan.Name == "" {
		return plan, errors.New("perfil de otimização inválido")
	}

	switch profile {
	case 1:
		plan.ClientAreaAnimations = boolPtr(false)
	case 2:
		plan.MinimizeWindows = boolPtr(false)
		plan.ClientAreaAnimations = boolPtr(false)
		plan.PowerSchemeGUID = balanced
		plan.PowerSchemeLabel = "Equilibrado"
	case 3:
		plan.MinimizeWindows = boolPtr(false)
		plan.ClientAreaAnimations = boolPtr(false)
		plan.PowerSchemeGUID = balanced
		plan.PowerSchemeLabel = "Equilibrado"
		plan.PrioritizeWorkApps = true
	case 4:
		plan.MinimizeWindows = boolPtr(false)
		plan.ClientAreaAnimations = boolPtr(false)
		plan.PrioritizeWorkApps = true
		if onBattery {
			plan.PowerSchemeGUID = balanced
			plan.PowerSchemeLabel = "Equilibrado"
			plan.Notes = []string{"O plano Alto desempenho não é ativado enquanto o computador estiver usando bateria."}
		} else {
			plan.PowerSchemeGUID = highPerformance
			plan.PowerSchemeLabel = "Alto desempenho"
		}
	case 5:
		// Restore does not create a new plan; it consumes the saved snapshot.
	}
	return plan, nil
}

type OptimizationProfileExplanation struct {
	Name    string
	Short   string
	Summary string
	Actions []string
	Result  string
}

// optimizationProfileExplanation contains the exact user-facing explanation of
// what each profile changes. Keep these texts aligned with optimizationPlan and
// applyOptimizationProfile so the interface never promises an action that the
// optimizer does not perform.
func optimizationProfileExplanation(profile int) OptimizationProfileExplanation {
	switch profile {
	case 1:
		return OptimizationProfileExplanation{
			Name:    "Conservador",
			Short:   "Deixa o Windows mais leve sem mudar energia ou programas.",
			Summary: "Faz a menor alteração possível para reduzir pequenos efeitos visuais.",
			Actions: []string{
				"Cria ou preserva o backup das configurações originais.",
				"Desativa pequenas animações internas de menus e telas do Windows.",
				"Mantém o plano de energia que já está em uso.",
				"Não altera a prioridade dos programas.",
			},
			Result: "Na prática: menus e telas podem responder de forma um pouco mais direta, com impacto mínimo no visual.",
		}
	case 2:
		return OptimizationProfileExplanation{
			Name:    "Equilibrado",
			Short:   "Reduz efeitos visuais e mantém desempenho e consumo equilibrados.",
			Summary: "Combina uma interface mais leve com o plano Equilibrado do Windows.",
			Actions: []string{
				"Cria ou preserva o backup das configurações originais.",
				"Desativa animações ao minimizar ou maximizar janelas e pequenas animações de menus.",
				"Ativa o plano de energia Equilibrado do Windows.",
				"Não altera a prioridade dos programas.",
			},
			Result: "Na prática: reduz efeitos visuais sem colocar o computador em consumo máximo de energia.",
		}
	case 3:
		return OptimizationProfileExplanation{
			Name:    "Modo Atendimento",
			Short:   "Prepara o PC para navegador, WhatsApp, CRM e discador.",
			Summary: "Aplica os ajustes equilibrados e dá preferência moderada aos aplicativos de trabalho que já estiverem abertos.",
			Actions: []string{
				"Cria ou preserva o backup das configurações originais.",
				"Reduz animações de janelas, menus e telas do Windows.",
				"Ativa o plano de energia Equilibrado do Windows.",
				"Dá prioridade moderada a navegadores, WhatsApp, ZapsChat, Teams e discadores compatíveis que já estiverem abertos.",
			},
			Result: "Na prática: o Windows tenta atender primeiro os programas de trabalho. Aplicativos abertos depois precisam de uma nova aplicação do perfil.",
		}
	case 4:
		return OptimizationProfileExplanation{
			Name:    "Alto Desempenho",
			Short:   "Entrega mais resposta quando o computador está ligado à tomada.",
			Summary: "Prioriza velocidade, mas mantém uma proteção automática para notebooks usando bateria.",
			Actions: []string{
				"Cria ou preserva o backup das configurações originais.",
				"Reduz animações de janelas, menus e telas do Windows.",
				"Dá prioridade moderada aos aplicativos de atendimento compatíveis que já estiverem abertos.",
				"Na tomada, ativa Alto desempenho; na bateria ou sem confirmação de energia, usa Equilibrado.",
			},
			Result: "Na prática: busca maior resposta e pode consumir mais energia. Se Alto desempenho não existir neste PC, o plano atual é preservado e o CoreTuner avisa.",
		}
	case 5:
		return OptimizationProfileExplanation{
			Name:    "Restaurar Original",
			Short:   "Desfaz as mudanças do CoreTuner e volta ao estado salvo.",
			Summary: "Usa o backup criado antes da primeira otimização para devolver as configurações anteriores.",
			Actions: []string{
				"Restaura as animações originais do Windows.",
				"Restaura o plano de energia que estava ativo antes da otimização.",
				"Restaura a prioridade dos aplicativos compatíveis que ainda estiverem abertos.",
				"Só arquiva o backup depois que a restauração termina sem falhas.",
			},
			Result: "Na prática: o computador volta ao estado salvo antes da primeira otimização. Se algo falhar, o backup é mantido para tentar novamente.",
		}
	default:
		return OptimizationProfileExplanation{}
	}
}

func optimizationConfirmation(plan OptimizationPlan) string {
	if plan.Profile == 5 {
		return "O CoreTuner restaurará exatamente as configurações salvas antes da primeira otimização.\n\nNenhum arquivo será apagado e nenhum programa será fechado à força.\n\nDeseja continuar?"
	}
	lines := []string{"O CoreTuner criará ou preservará um backup automático antes de alterar o Windows."}
	if plan.ClientAreaAnimations != nil || plan.MinimizeWindows != nil {
		lines = append(lines, "• Reduzir animações do Windows")
	}
	if plan.PowerSchemeGUID != "" {
		lines = append(lines, "• Usar plano de energia: "+plan.PowerSchemeLabel)
	}
	if plan.PrioritizeWorkApps {
		lines = append(lines, "• Priorizar moderadamente aplicativos de atendimento em execução")
	}
	lines = append(lines, "", "Não serão apagados arquivos, alterados Defender/Firewall ou encerrados programas.", "", "Aplicar o perfil "+plan.Name+"?")
	return strings.Join(lines, "\n")
}

func optimizationStateFile(baseDir string) string {
	return filepath.Join(baseDir, "optimization-state.json")
}

func loadOptimizationState(path string) (*OptimizationState, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var state OptimizationState
	if err := json.Unmarshal(raw, &state); err != nil {
		return nil, fmt.Errorf("backup de otimização inválido: %w", err)
	}
	if state.Version != optimizationStateVersion {
		return nil, fmt.Errorf("versão de backup de otimização não suportada: %d", state.Version)
	}
	if state.ProcessPriorities == nil {
		state.ProcessPriorities = map[string]ProcessPrioritySnapshot{}
	}
	return &state, nil
}

func saveOptimizationState(path string, state *OptimizationState) error {
	if state == nil {
		return errors.New("estado de otimização ausente")
	}
	state.Version = optimizationStateVersion
	state.UpdatedAt = time.Now()
	if state.ProcessPriorities == nil {
		state.ProcessPriorities = map[string]ProcessPrioritySnapshot{}
	}
	raw, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
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

func archiveOptimizationState(path string, restoredAt time.Time) (string, error) {
	if _, err := os.Stat(path); err != nil {
		return "", err
	}
	archiveDir := filepath.Join(filepath.Dir(path), "Backups")
	if err := os.MkdirAll(archiveDir, 0755); err != nil {
		return "", err
	}
	archivePath := filepath.Join(archiveDir, "optimization-restored-"+restoredAt.Format("20060102-150405")+".json")
	if err := os.Rename(path, archivePath); err != nil {
		return "", err
	}
	return archivePath, nil
}

func processPriorityKey(pid uint32, name string) string {
	return fmt.Sprintf("%d:%s", pid, strings.ToLower(strings.TrimSpace(name)))
}

func normalizedProcessName(name string) string {
	name = strings.ToLower(strings.TrimSpace(name))
	name = strings.TrimSuffix(name, ".exe")
	return name
}

func isWorkProcess(name string) bool {
	switch normalizedProcessName(name) {
	case "chrome", "msedge", "firefox", "brave", "opera", "opera_gx", "whatsapp", "zapschat", "zapschat-desktop", "teams", "ms-teams", "zoiper", "microsip", "3cx", "3cxdesktopapp", "softphone", "discador":
		return true
	default:
		return false
	}
}

func summarizeOptimizationResult(result OptimizationResult) string {
	parts := []string{}
	if result.Restored {
		parts = append(parts, "Configurações anteriores restauradas.")
	} else {
		parts = append(parts, "Perfil "+result.ProfileName+" aplicado com backup automático.")
	}
	if len(result.Changed) > 0 {
		changed := append([]string(nil), result.Changed...)
		sort.Strings(changed)
		parts = append(parts, "\nAlterações concluídas:\n• "+strings.Join(changed, "\n• "))
	}
	if len(result.Warnings) > 0 {
		warnings := append([]string(nil), result.Warnings...)
		sort.Strings(warnings)
		parts = append(parts, "\nAvisos:\n• "+strings.Join(warnings, "\n• "))
	}
	return strings.Join(parts, "\n")
}
