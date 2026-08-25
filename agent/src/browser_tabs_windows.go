//go:build windows

package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type agentBrowserTab struct {
	Browser   string `json:"browser"`
	Title     string `json:"title"`
	URL       string `json:"url"`
	Domain    string `json:"domain"`
	Active    bool   `json:"active"`
	Audible   bool   `json:"audible"`
	Pinned    bool   `json:"pinned"`
	Discarded bool   `json:"discarded"`
}

type agentBrowserStored struct {
	CapturedAt string            `json:"captured_at"`
	Tabs       []agentBrowserTab `json:"tabs"`
}

type agentBrowserState struct {
	UpdatedAt string                        `json:"updated_at"`
	Browsers  map[string]agentBrowserStored `json:"browsers"`
}

func loadAgentBrowserTabs() []agentBrowserTab {
	local := strings.TrimSpace(os.Getenv("LOCALAPPDATA"))
	if local == "" {
		return nil
	}
	raw, err := os.ReadFile(filepath.Join(local, "CoreTuner", "Browser", "browser-tabs.json"))
	if err != nil {
		return nil
	}
	var state agentBrowserState
	if json.Unmarshal(raw, &state) != nil || state.Browsers == nil {
		return nil
	}
	out := make([]agentBrowserTab, 0, 32)
	for browser, snapshot := range state.Browsers {
		captured, err := time.Parse(time.RFC3339, snapshot.CapturedAt)
		if err == nil && time.Since(captured) > 5*time.Minute {
			continue
		}
		for _, tab := range snapshot.Tabs {
			tab.Browser = strings.ToLower(strings.TrimSpace(browser))
			out = append(out, tab)
		}
	}
	return out
}

func agentBrowserKeyForProcess(name string) string {
	switch strings.ToLower(strings.TrimSpace(strings.TrimSuffix(name, ".exe"))) {
	case "opera", "opera_gx", "opera gx":
		return "opera"
	case "chrome", "google chrome":
		return "chrome"
	case "msedge", "microsoft edge":
		return "edge"
	default:
		return ""
	}
}

func activeBrowserTab(processName string, tabs []agentBrowserTab) *agentBrowserTab {
	key := agentBrowserKeyForProcess(processName)
	if key == "" {
		return nil
	}
	for i := range tabs {
		if tabs[i].Browser == key && tabs[i].Active {
			copyTab := tabs[i]
			return &copyTab
		}
	}
	return nil
}
