package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestOptimizationPlansAreSafe(t *testing.T) {
	for _, profile := range []int{1, 2, 3, 4, 5} {
		plan, err := optimizationPlan(profile, false)
		if err != nil {
			t.Fatalf("profile %d: %v", profile, err)
		}
		if plan.Name == "" {
			t.Fatalf("profile %d has no name", profile)
		}
	}
	plan, _ := optimizationPlan(4, true)
	if plan.PowerSchemeLabel != "Equilibrado" {
		t.Fatalf("high performance on battery must fall back to balanced, got %q", plan.PowerSchemeLabel)
	}
	if len(plan.Notes) == 0 {
		t.Fatal("battery fallback must be explained")
	}
}

func TestOptimizationConfirmationStatesSafety(t *testing.T) {
	plan, _ := optimizationPlan(3, false)
	text := optimizationConfirmation(plan)
	for _, required := range []string{"backup automático", "Não serão apagados arquivos", "Defender/Firewall", "encerrados programas"} {
		if !strings.Contains(text, required) {
			t.Fatalf("confirmation missing %q: %s", required, text)
		}
	}
}

func TestOptimizationStateRoundTripAndArchive(t *testing.T) {
	dir := t.TempDir()
	path := optimizationStateFile(dir)
	state := &OptimizationState{
		CreatedAt:           time.Now(),
		ActiveProfile:       3,
		ActiveProfileName:   "Modo Atendimento",
		OriginalAnimations:  AnimationSnapshot{MinimizeWindows: true, ClientArea: true},
		OriginalPowerScheme: "abc",
		ProcessPriorities: map[string]ProcessPrioritySnapshot{
			processPriorityKey(10, "chrome"): {PID: 10, Name: "chrome", Priority: 32},
		},
	}
	if err := saveOptimizationState(path, state); err != nil {
		t.Fatal(err)
	}
	loaded, err := loadOptimizationState(path)
	if err != nil {
		t.Fatal(err)
	}
	if loaded == nil || loaded.ActiveProfile != 3 || loaded.OriginalPowerScheme != "abc" {
		t.Fatalf("unexpected loaded state: %#v", loaded)
	}
	archive, err := archiveOptimizationState(path, time.Date(2026, 7, 27, 12, 0, 0, 0, time.Local))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("active state should be moved, stat err=%v", err)
	}
	if filepath.Base(archive) != "optimization-restored-20260727-120000.json" {
		t.Fatalf("unexpected archive name %q", archive)
	}
}

func TestWorkProcessAllowlist(t *testing.T) {
	for _, name := range []string{"chrome.exe", "msedge", "WhatsApp.exe", "Zoiper.exe", "ZapsChat-Desktop.exe"} {
		if !isWorkProcess(name) {
			t.Fatalf("expected work process: %s", name)
		}
	}
	for _, name := range []string{"explorer.exe", "powershell.exe", "cmd.exe", "svchost.exe"} {
		if isWorkProcess(name) {
			t.Fatalf("unsafe process unexpectedly allowlisted: %s", name)
		}
	}
}
