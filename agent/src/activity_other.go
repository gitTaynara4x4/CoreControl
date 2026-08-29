//go:build !windows

package main

import "time"

type foregroundActivity struct {
	ProcessName string `json:"process_name"`
	WindowTitle string `json:"window_title"`
	PID         int    `json:"pid"`
	CapturedAt  string `json:"captured_at"`
}

type activityApplication struct {
	ProcessName string  `json:"process_name"`
	DisplayName string  `json:"display_name,omitempty"`
	WindowTitle string  `json:"window_title"`
	PID         int     `json:"pid"`
	CPUPercent  float64 `json:"cpu_percent"`
	MemoryMB    float64 `json:"memory_mb"`
	Focused     bool    `json:"focused"`
}

type activityAppAsset struct {
	ProcessName string `json:"process_name"`
	DisplayName string `json:"display_name"`
	IconData    string `json:"icon_data,omitempty"`
}

type activitySnapshotResult struct {
	CapturedAt string                      `json:"captured_at"`
	Foreground foregroundActivity          `json:"foreground"`
	Apps       []activityApplication       `json:"apps"`
	AppAssets  map[string]activityAppAsset `json:"app_assets,omitempty"`
}

func collectForegroundActivity() foregroundActivity {
	return foregroundActivity{CapturedAt: time.Now().UTC().Format(time.RFC3339)}
}

func collectActivitySnapshot() activitySnapshotResult {
	now := time.Now().UTC().Format(time.RFC3339)
	return activitySnapshotResult{CapturedAt: now, Foreground: foregroundActivity{CapturedAt: now}, Apps: []activityApplication{}}
}
