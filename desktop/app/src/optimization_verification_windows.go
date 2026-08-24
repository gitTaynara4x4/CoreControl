//go:build windows

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type OptimizationVerificationItem struct {
	Label  string `json:"label"`
	Before string `json:"before,omitempty"`
	After  string `json:"after,omitempty"`
	Status string `json:"status"` // verified, warning, info
	Note   string `json:"note,omitempty"`
}

type OptimizationVerification struct {
	Profile   int                            `json:"profile"`
	Name      string                         `json:"name"`
	At        time.Time                      `json:"at"`
	Confirmed int                            `json:"confirmed"`
	Total     int                            `json:"total"`
	Items     []OptimizationVerificationItem `json:"items"`
	Before    MetricsSnapshot                `json:"before"`
	After     MetricsSnapshot                `json:"after"`
	Error     string                         `json:"error,omitempty"`
}

type optimizationRuntimeSnapshot struct {
	Animations AnimationSnapshot
	PowerGUID  string
	Priorities map[string]uint32
	Backup     bool
}

func optimizationVerificationPath() string {
	return filepath.Join(dataDir(), "optimization-verification.json")
}

func processSnapshotKey(proc ProcessInfo) string {
	return fmt.Sprintf("%d:%s", proc.PID, normalizedProcessName(proc.Name))
}

func captureOptimizationRuntime() optimizationRuntimeSnapshot {
	snap := optimizationRuntimeSnapshot{Priorities: map[string]uint32{}}
	if anim, err := currentAnimationSnapshot(); err == nil {
		snap.Animations = anim
	}
	if power, err := currentPowerScheme(); err == nil {
		snap.PowerGUID = strings.ToLower(power)
	}
	for _, proc := range workProcessesToPrioritize() {
		if priority, err := getProcessPriority(uint32(proc.PID)); err == nil {
			snap.Priorities[processSnapshotKey(proc)] = priority
		}
	}
	if _, err := os.Stat(optimizationStatePath()); err == nil {
		snap.Backup = true
	}
	return snap
}

func yesNo(v bool) string {
	if v {
		return "Ativadas"
	}
	return "Reduzidas"
}

func powerLabel(guid string) string {
	switch strings.ToLower(strings.TrimSpace(guid)) {
	case "381b4222-f694-41f0-9685-ff5bb260df2e":
		return "Equilibrado"
	case "8c5e7fda-e8bf-4a96-9a85-a6e23a8c635c":
		return "Alto desempenho"
	case "a1841308-3541-4fab-bc81-f71556f20b4a":
		return "Economia de energia"
	case "":
		return "Não identificado"
	default:
		return "Personalizado"
	}
}

func addVerificationItem(v *OptimizationVerification, item OptimizationVerificationItem) {
	v.Items = append(v.Items, item)
	if item.Status == "verified" {
		v.Confirmed++
		v.Total++
	} else if item.Status == "warning" {
		v.Total++
	}
}

func buildOptimizationVerification(profile int, plan OptimizationPlan, original *OptimizationState, before, after optimizationRuntimeSnapshot, result OptimizationResult, beforeSys, afterSys SystemInfo, applyErr error) OptimizationVerification {
	v := OptimizationVerification{
		Profile: profile,
		Name:    result.ProfileName,
		At:      time.Now(),
		Before:  metricsSnapshot(beforeSys),
		After:   metricsSnapshot(afterSys),
	}
	if v.Name == "" {
		v.Name = optimizationProfileName(profile)
	}
	if applyErr != nil {
		v.Error = applyErr.Error()
	}

	if profile == 5 {
		if original == nil {
			addVerificationItem(&v, OptimizationVerificationItem{Label: "Backup original", Status: "warning", Note: "O backup original não pôde ser relido para confirmar a restauração."})
			return v
		}
		animOK := after.Animations == original.OriginalAnimations
		addVerificationItem(&v, OptimizationVerificationItem{
			Label:  "Efeitos visuais",
			Before: "Perfil otimizado",
			After:  yesNo(after.Animations.ClientArea),
			Status: chooseText(animOK, "verified", "warning"),
			Note:   chooseText(animOK, "Configurações originais confirmadas pelo Windows.", "O Windows não confirmou todas as animações originais."),
		})
		if original.OriginalPowerScheme != "" {
			powerOK := strings.EqualFold(after.PowerGUID, original.OriginalPowerScheme)
			addVerificationItem(&v, OptimizationVerificationItem{
				Label:  "Plano de energia",
				Before: powerLabel(before.PowerGUID),
				After:  powerLabel(after.PowerGUID),
				Status: chooseText(powerOK, "verified", "warning"),
				Note:   chooseText(powerOK, "Plano original confirmado pelo Windows.", "O plano original não foi confirmado."),
			})
		}
		noActiveBackup := !after.Backup
		addVerificationItem(&v, OptimizationVerificationItem{
			Label:  "Restauração",
			Before: "Backup ativo",
			After:  chooseText(noActiveBackup, "Concluída", "Pendente"),
			Status: chooseText(noActiveBackup, "verified", "warning"),
			Note:   chooseText(noActiveBackup, "Backup arquivado após a restauração.", "O backup continua ativo porque a restauração precisa de revisão."),
		})
		return v
	}

	addVerificationItem(&v, OptimizationVerificationItem{
		Label:  "Backup de segurança",
		Before: chooseText(before.Backup, "Já existente", "Não existia"),
		After:  chooseText(after.Backup, "Disponível", "Não confirmado"),
		Status: chooseText(after.Backup, "verified", "warning"),
		Note:   chooseText(after.Backup, "Backup original disponível para restauração.", "O backup não foi confirmado no disco."),
	})

	if plan.MinimizeWindows != nil {
		ok := after.Animations.MinimizeWindows == *plan.MinimizeWindows
		addVerificationItem(&v, OptimizationVerificationItem{
			Label:  "Animação de janelas",
			Before: yesNo(before.Animations.MinimizeWindows),
			After:  yesNo(after.Animations.MinimizeWindows),
			Status: chooseText(ok, "verified", "warning"),
			Note:   chooseText(ok, "Estado confirmado pelo Windows.", "O Windows não confirmou o estado solicitado."),
		})
	}
	if plan.ClientAreaAnimations != nil {
		ok := after.Animations.ClientArea == *plan.ClientAreaAnimations
		addVerificationItem(&v, OptimizationVerificationItem{
			Label:  "Animações da interface",
			Before: yesNo(before.Animations.ClientArea),
			After:  yesNo(after.Animations.ClientArea),
			Status: chooseText(ok, "verified", "warning"),
			Note:   chooseText(ok, "Estado confirmado pelo Windows.", "O Windows não confirmou o estado solicitado."),
		})
	}
	if plan.PowerSchemeGUID != "" {
		ok := strings.EqualFold(after.PowerGUID, plan.PowerSchemeGUID)
		addVerificationItem(&v, OptimizationVerificationItem{
			Label:  "Plano de energia",
			Before: powerLabel(before.PowerGUID),
			After:  powerLabel(after.PowerGUID),
			Status: chooseText(ok, "verified", "warning"),
			Note:   chooseText(ok, "Plano ativo confirmado pelo Windows.", "O plano solicitado não ficou ativo; o Windows preservou outro plano."),
		})
	}
	if plan.PrioritizeWorkApps {
		eligible := 0
		verified := 0
		for key := range before.Priorities {
			eligible++
			if after.Priorities[key] == aboveNormalPriority {
				verified++
			}
		}
		if eligible == 0 {
			v.Items = append(v.Items, OptimizationVerificationItem{Label: "Aplicativos de trabalho", After: "Nenhum compatível aberto", Status: "info", Note: "Abra navegador, WhatsApp, Teams ou app compatível e aplique o perfil novamente."})
		} else {
			ok := verified == eligible
			addVerificationItem(&v, OptimizationVerificationItem{
				Label:  "Aplicativos priorizados",
				Before: fmt.Sprintf("%d detectado(s)", eligible),
				After:  fmt.Sprintf("%d confirmado(s)", verified),
				Status: chooseText(ok, "verified", "warning"),
				Note:   chooseText(ok, "Prioridade moderada confirmada nos aplicativos compatíveis.", "Nem todos os aplicativos permaneceram com a prioridade solicitada."),
			})
		}
	}
	return v
}

func saveOptimizationVerification(v OptimizationVerification) error {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dataDir(), 0755); err != nil {
		return err
	}
	return os.WriteFile(optimizationVerificationPath(), b, 0600)
}

func loadOptimizationVerification() (*OptimizationVerification, error) {
	b, err := os.ReadFile(optimizationVerificationPath())
	if err != nil {
		return nil, err
	}
	var v OptimizationVerification
	if err := json.Unmarshal(b, &v); err != nil {
		return nil, err
	}
	return &v, nil
}

func optimizationVerificationMessage(v OptimizationVerification) string {
	title := fmt.Sprintf("%d de %d alterações confirmadas pelo Windows.", v.Confirmed, v.Total)
	if v.Total == 0 {
		title = "A operação foi concluída e registrada."
	}
	var lines []string
	for _, item := range v.Items {
		prefix := "•"
		switch item.Status {
		case "verified":
			prefix = "✓"
		case "warning":
			prefix = "!"
		}
		after := item.After
		if after == "" {
			after = item.Note
		}
		lines = append(lines, prefix+" "+item.Label+": "+after)
	}
	if v.Error != "" {
		lines = append(lines, "\nAtenção: "+v.Error)
	}
	if len(lines) == 0 {
		return title
	}
	return title + "\n\n" + strings.Join(lines, "\n")
}
