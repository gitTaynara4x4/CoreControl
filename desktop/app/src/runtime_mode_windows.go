//go:build windows

package main

import (
	"os"
	"path/filepath"
	"strings"
)

func isDevMode() bool {
	v := strings.TrimSpace(strings.ToLower(os.Getenv("CORETUNER_DEV")))
	return v == "1" || v == "true" || v == "yes" || v == "sim"
}

func configuredDataDir() string {
	if v := strings.TrimSpace(os.Getenv("CORETUNER_DATA_DIR")); v != "" {
		return filepath.Clean(v)
	}
	if p := os.Getenv("LOCALAPPDATA"); p != "" {
		if isDevMode() {
			return filepath.Join(p, "CoreTuner-Dev")
		}
		return filepath.Join(p, "CoreTuner")
	}
	home, _ := os.UserHomeDir()
	if home != "" {
		name := "CoreTuner"
		if isDevMode() {
			name = "CoreTuner-Dev"
		}
		return filepath.Join(home, "AppData", "Local", name)
	}
	name := "CoreTuner"
	if isDevMode() {
		name = "CoreTuner-Dev"
	}
	return filepath.Join(os.TempDir(), name)
}

func appWindowTitle() string {
	if isDevMode() {
		return "CoreTuner DEV — Diagnóstico e gestão segura"
	}
	return "CoreTuner — Diagnóstico e gestão segura"
}
