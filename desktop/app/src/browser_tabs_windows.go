//go:build windows

package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type BrowserTab struct {
	Browser    string `json:"browser"`
	TabID      int    `json:"tab_id"`
	WindowID   int    `json:"window_id"`
	Title      string `json:"title"`
	URL        string `json:"url"`
	Domain     string `json:"domain"`
	FavIconURL string `json:"fav_icon_url,omitempty"`
	Active     bool   `json:"active"`
	Audible    bool   `json:"audible"`
	Pinned     bool   `json:"pinned"`
	Discarded  bool   `json:"discarded"`
	CapturedAt string `json:"-"`
}

type browserStoredSnapshot struct {
	CapturedAt string       `json:"captured_at"`
	Tabs       []BrowserTab `json:"tabs"`
}

type browserBridgeState struct {
	Version   int                              `json:"version"`
	UpdatedAt string                           `json:"updated_at"`
	Browsers  map[string]browserStoredSnapshot `json:"browsers"`
}

func loadBrowserTabs() []BrowserTab {
	paths := []string{}
	if local := strings.TrimSpace(os.Getenv("LOCALAPPDATA")); local != "" {
		paths = append(paths, filepath.Join(local, "CoreTuner", "Browser", "browser-tabs.json"))
	}
	paths = append(paths, filepath.Join(dataDir(), "Browser", "browser-tabs.json"))
	seenPath := map[string]bool{}
	var newest browserBridgeState
	var newestAt time.Time
	for _, path := range paths {
		if path == "" || seenPath[strings.ToLower(path)] {
			continue
		}
		seenPath[strings.ToLower(path)] = true
		raw, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		var state browserBridgeState
		if json.Unmarshal(raw, &state) != nil {
			continue
		}
		updated, _ := time.Parse(time.RFC3339, state.UpdatedAt)
		if newestAt.IsZero() || updated.After(newestAt) {
			newest, newestAt = state, updated
		}
	}
	if newest.Browsers == nil {
		return nil
	}
	out := make([]BrowserTab, 0, 32)
	for browser, snapshot := range newest.Browsers {
		captured, err := time.Parse(time.RFC3339, snapshot.CapturedAt)
		if err == nil && time.Since(captured) > 5*time.Minute {
			continue
		}
		for _, tab := range snapshot.Tabs {
			tab.Browser = strings.ToLower(strings.TrimSpace(browser))
			tab.CapturedAt = snapshot.CapturedAt
			out = append(out, tab)
		}
	}
	return out
}

func browserKeyForProcess(name string) string {
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

func browserTabsForProcess(name string, tabs []BrowserTab) []BrowserTab {
	key := browserKeyForProcess(name)
	if key == "" {
		return nil
	}
	result := make([]BrowserTab, 0, len(tabs))
	for _, tab := range tabs {
		if tab.Browser == key {
			result = append(result, tab)
		}
	}
	return result
}
