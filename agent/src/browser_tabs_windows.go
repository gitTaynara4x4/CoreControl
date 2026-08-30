//go:build windows

package main

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
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
	Source    string `json:"source,omitempty"`
}

type agentBrowserStored struct {
	CapturedAt string            `json:"captured_at"`
	Tabs       []agentBrowserTab `json:"tabs"`
}

type agentBrowserState struct {
	UpdatedAt string                        `json:"updated_at"`
	Browsers  map[string]agentBrowserStored `json:"browsers"`
}

// loadAgentBrowserTabs first uses the native browser bridge when available,
// because it can provide the real URL/domain. For browsers without a recent
// bridge snapshot, it falls back to Windows UI Automation and reads the tab
// strip directly from Chrome/Edge/Opera. This means the Agent can enumerate
// open tabs even when the CoreControl browser extension was never installed.
func loadAgentBrowserTabs() []agentBrowserTab {
	bridgeTabs, bridgeBrowsers := loadBrowserBridgeTabs()
	uiaTabs := collectBrowserTabsUIA()

	out := make([]agentBrowserTab, 0, len(bridgeTabs)+len(uiaTabs))
	out = append(out, bridgeTabs...)
	for _, tab := range uiaTabs {
		browser := normalizeAgentBrowser(tab.Browser)
		if browser == "" || bridgeBrowsers[browser] {
			// A fresh native bridge snapshot is more complete and includes URL.
			continue
		}
		tab.Browser = browser
		tab.Source = "windows-uia"
		out = append(out, tab)
	}

	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Active != out[j].Active {
			return out[i].Active
		}
		if out[i].Browser != out[j].Browser {
			return out[i].Browser < out[j].Browser
		}
		return strings.ToLower(out[i].Title) < strings.ToLower(out[j].Title)
	})
	if len(out) > 500 {
		out = out[:500]
	}
	return out
}

func loadBrowserBridgeTabs() ([]agentBrowserTab, map[string]bool) {
	freshBrowsers := map[string]bool{}
	local := strings.TrimSpace(os.Getenv("LOCALAPPDATA"))
	if local == "" {
		return nil, freshBrowsers
	}
	raw, err := os.ReadFile(filepath.Join(local, "CoreTuner", "Browser", "browser-tabs.json"))
	if err != nil {
		return nil, freshBrowsers
	}
	var state agentBrowserState
	if json.Unmarshal(raw, &state) != nil || state.Browsers == nil {
		return nil, freshBrowsers
	}
	out := make([]agentBrowserTab, 0, 32)
	for browser, snapshot := range state.Browsers {
		key := normalizeAgentBrowser(browser)
		if key == "" {
			continue
		}
		captured, err := time.Parse(time.RFC3339, snapshot.CapturedAt)
		if err == nil && time.Since(captured) > 5*time.Minute {
			continue
		}
		if len(snapshot.Tabs) == 0 {
			continue
		}
		freshBrowsers[key] = true
		for _, tab := range snapshot.Tabs {
			tab.Browser = key
			tab.Title = strings.TrimSpace(tab.Title)
			if tab.Title == "" {
				tab.Title = tab.Domain
			}
			if tab.Title == "" {
				continue
			}
			tab.Source = "browser-bridge"
			out = append(out, tab)
		}
	}
	return out, freshBrowsers
}

func normalizeAgentBrowser(value string) string {
	switch strings.ToLower(strings.TrimSpace(strings.TrimSuffix(value, ".exe"))) {
	case "chrome", "google chrome":
		return "chrome"
	case "msedge", "edge", "microsoft edge":
		return "edge"
	case "opera", "opera_gx", "opera gx":
		return "opera"
	case "brave", "brave-browser":
		return "brave"
	default:
		return ""
	}
}

type browserUITab struct {
	Browser string `json:"browser"`
	Title   string `json:"title"`
	Active  bool   `json:"active"`
}

// collectBrowserTabsUIA reads Chromium tab strips through Windows UI
// Automation. It does not inspect page contents, keyboard input, cookies or
// passwords. In this fallback mode only the visible tab title is collected;
// URL/domain stay empty unless the native browser bridge is installed.
func collectBrowserTabsUIA() []agentBrowserTab {
	powershell, err := exec.LookPath("powershell.exe")
	if err != nil {
		return nil
	}

	const script = `$ErrorActionPreference='SilentlyContinue';
Add-Type -AssemblyName UIAutomationClient;
Add-Type -AssemblyName UIAutomationTypes;
if(-not ('CoreControl.NativeUser32' -as [type])){
  Add-Type -TypeDefinition @'
using System;
using System.Runtime.InteropServices;
namespace CoreControl {
  public static class NativeUser32 {
    [DllImport("user32.dll")] public static extern IntPtr GetForegroundWindow();
  }
}
'@;
}
[Console]::OutputEncoding=[System.Text.UTF8Encoding]::new($false);
$fg=[CoreControl.NativeUser32]::GetForegroundWindow().ToInt64();
$root=[System.Windows.Automation.AutomationElement]::RootElement;
$wins=$root.FindAll([System.Windows.Automation.TreeScope]::Children,[System.Windows.Automation.Condition]::TrueCondition);
$out=New-Object System.Collections.Generic.List[object];
$tabCondition=[System.Windows.Automation.PropertyCondition]::new([System.Windows.Automation.AutomationElement]::ControlTypeProperty,[System.Windows.Automation.ControlType]::TabItem);
foreach($w in $wins){
  try {
    $processId=[int]$w.Current.ProcessId;
    if($processId -le 0){ continue }
    $p=Get-Process -Id $processId -ErrorAction SilentlyContinue;
    if(-not $p){ continue }
    $process=([string]$p.ProcessName).ToLowerInvariant();
    $browser='';
    if($process -eq 'chrome'){ $browser='chrome' }
    elseif($process -eq 'msedge'){ $browser='edge' }
    elseif($process -eq 'opera' -or $process -eq 'opera_gx'){ $browser='opera' }
    elseif($process -eq 'brave'){ $browser='brave' }
    if([string]::IsNullOrWhiteSpace($browser)){ continue }

    $tabs=$w.FindAll([System.Windows.Automation.TreeScope]::Descendants,$tabCondition);
    if(-not $tabs -or $tabs.Count -eq 0){ continue }
    $windowActive=([int64]$w.Current.NativeWindowHandle -eq $fg);
    foreach($tab in $tabs){
      try {
        $name=([string]$tab.Current.Name).Trim();
        if([string]::IsNullOrWhiteSpace($name)){ continue }
        $selected=$false;
        $pattern=$null;
        if($tab.TryGetCurrentPattern([System.Windows.Automation.SelectionItemPattern]::Pattern,[ref]$pattern)){
          try { $selected=[bool]$pattern.Current.IsSelected } catch {}
        }
        $out.Add([pscustomobject]@{browser=$browser;title=$name;active=([bool]($windowActive -and $selected))});
      } catch {}
    }
  } catch {}
}
[Console]::Out.Write((ConvertTo-Json -InputObject @($out) -Compress -Depth 3));`

	ctx, cancel := context.WithTimeout(context.Background(), 6*time.Second)
	defer cancel()
	cmd := hiddenCommandContext(ctx, powershell, "-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass", "-EncodedCommand", encodePowerShell(script))
	raw, err := cmd.Output()
	if err != nil || ctx.Err() != nil {
		return nil
	}
	text := strings.TrimSpace(string(raw))
	if text == "" || text == "null" {
		return nil
	}

	var items []browserUITab
	if json.Unmarshal([]byte(text), &items) != nil {
		return nil
	}
	out := make([]agentBrowserTab, 0, len(items))
	// Do not deduplicate by title: users can legitimately keep two tabs with
	// the same page title (for example two YouTube tabs). Each UI Automation
	// TabItem represents an open tab and should remain visible in CoreControl.
	for _, item := range items {
		browser := normalizeAgentBrowser(item.Browser)
		title := strings.TrimSpace(item.Title)
		if browser == "" || title == "" {
			continue
		}
		out = append(out, agentBrowserTab{Browser: browser, Title: title, Active: item.Active, Source: "windows-uia"})
	}
	return out
}

func agentBrowserKeyForProcess(name string) string {
	return normalizeAgentBrowser(name)
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
