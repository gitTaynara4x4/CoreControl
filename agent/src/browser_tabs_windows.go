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

// loadAgentBrowserTabs prioritizes the native browser bridge when available,
// because it can provide the real URL/domain. For browsers without a recent
// bridge snapshot it tries two read-only Windows accessibility paths:
// UI Automation first and MSAA/IAccessible as a compatibility fallback.
//
// Chromium versions differ in how the tab strip is exposed. Some builds map
// tabs directly to UIA ControlType.TabItem while others only expose the legacy
// ROLE_SYSTEM_PAGETAB role. Using both paths makes the collection much closer
// to what Task Manager/assistive technologies can see without reading page
// contents, keystrokes, cookies or passwords.
func loadAgentBrowserTabs() []agentBrowserTab {
	bridgeTabs, bridgeBrowsers := loadBrowserBridgeTabs()
	visibleBrowsers := visibleAgentBrowserKeys()

	out := make([]agentBrowserTab, 0, len(bridgeTabs)+32)
	out = append(out, bridgeTabs...)

	missing := missingAgentBrowsers(visibleBrowsers, bridgeBrowsers)
	if len(missing) > 0 {
		uiaByBrowser := browserTabsByBrowser(collectBrowserTabsUIA(), missing)
		needMSAA := map[string]bool{}
		for browser := range missing {
			// If UIA saw no tab (the failure observed on Luiza's Chrome) or only
			// one tab, ask MSAA too. A real single-tab browser is harmless here;
			// we simply keep whichever accessibility path returns the fuller list.
			if len(uiaByBrowser[browser]) <= 1 {
				needMSAA[browser] = true
			}
		}

		msaaByBrowser := map[string][]agentBrowserTab{}
		if len(needMSAA) > 0 {
			msaaByBrowser = browserTabsByBrowser(collectBrowserTabsMSAA(), needMSAA)
		}

		for browser := range missing {
			chosen := uiaByBrowser[browser]
			if len(msaaByBrowser[browser]) > len(chosen) {
				chosen = msaaByBrowser[browser]
			}
			out = append(out, chosen...)
		}
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

func visibleAgentBrowserKeys() map[string]bool {
	result := map[string]bool{}
	processes := agentProcessMap()
	for _, window := range collectAgentWindows() {
		name := strings.TrimSpace(window.ProcessName)
		if name == "" {
			name = strings.TrimSpace(processes[window.PID])
		}
		if key := normalizeAgentBrowser(name); key != "" {
			result[key] = true
		}
	}
	return result
}

func browserTabsByBrowser(tabs []agentBrowserTab, allowed map[string]bool) map[string][]agentBrowserTab {
	result := map[string][]agentBrowserTab{}
	for _, tab := range tabs {
		browser := normalizeAgentBrowser(tab.Browser)
		if browser == "" || (allowed != nil && !allowed[browser]) {
			continue
		}
		tab.Browser = browser
		if strings.TrimSpace(tab.Source) == "" {
			tab.Source = "windows-accessibility"
		}
		result[browser] = append(result[browser], tab)
	}
	return result
}

func missingAgentBrowsers(wanted, found map[string]bool) map[string]bool {
	result := map[string]bool{}
	for browser := range wanted {
		if !found[browser] {
			result[browser] = true
		}
	}
	return result
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

    # Newer Chromium builds do not always map the tab role directly to
    # ControlType.TabItem. Scan the accessibility descendants and accept both
    # the modern UIA type and the legacy ROLE_SYSTEM_PAGETAB (37).
    $nodes=$w.FindAll([System.Windows.Automation.TreeScope]::Descendants,[System.Windows.Automation.Condition]::TrueCondition);
    if(-not $nodes -or $nodes.Count -eq 0){ continue }
    $windowActive=([int64]$w.Current.NativeWindowHandle -eq $fg);
    $windowRect=$w.Current.BoundingRectangle;
    $walker=[System.Windows.Automation.TreeWalker]::ControlViewWalker;
    foreach($node in $nodes){
      try {
        $isTab=($node.Current.ControlType -eq [System.Windows.Automation.ControlType]::TabItem);
        $legacy=$null;
        if(-not $isTab -and $node.TryGetCurrentPattern([System.Windows.Automation.LegacyIAccessiblePattern]::Pattern,[ref]$legacy)){
          try { $isTab=([int]$legacy.Current.Role -eq 37) } catch {}
        }
        if(-not $isTab){ continue }

        # Chromium exposes ARIA role="tab" widgets from the web page through
        # accessibility too. Those are page sections (for example Disney+
        # "Sugestões", "Detalhes", "Extras"), not browser tabs. Reject any
        # candidate living under the page Document/render host.
        $insidePage=$false;
        $ancestor=$walker.GetParent($node);
        $depth=0;
        while($ancestor -ne $null -and $depth -lt 24){
          try {
            if($ancestor.Current.ControlType -eq [System.Windows.Automation.ControlType]::Document){
              $insidePage=$true;
              break;
            }
            $ancestorClass=([string]$ancestor.Current.ClassName);
            if($ancestorClass -match 'Chrome_RenderWidgetHostHWND|Internet Explorer_Server'){
              $insidePage=$true;
              break;
            }
          } catch {}
          $ancestor=$walker.GetParent($ancestor);
          $depth++;
        }
        if($insidePage){ continue }

        # Real horizontal browser tabs live in the browser chrome at the top
        # of the window. This second guard catches accessibility trees that do
        # not expose a Document ancestor for page widgets.
        $tabRect=$node.Current.BoundingRectangle;
        if($tabRect.Width -le 1 -or $tabRect.Height -le 1){ continue }
        if($windowRect.Height -gt 0){
          $chromeBand=[Math]::Min(260.0,[Math]::Max(140.0,$windowRect.Height * 0.24));
          if($tabRect.Top -gt ($windowRect.Top + $chromeBand)){ continue }
        }

        $name=([string]$node.Current.Name).Trim();
        if([string]::IsNullOrWhiteSpace($name) -and $legacy){
          try { $name=([string]$legacy.Current.Name).Trim() } catch {}
        }
        if([string]::IsNullOrWhiteSpace($name)){ continue }
        $selected=$false;
        $pattern=$null;
        if($node.TryGetCurrentPattern([System.Windows.Automation.SelectionItemPattern]::Pattern,[ref]$pattern)){
          try { $selected=[bool]$pattern.Current.IsSelected } catch {}
        }
        if(-not $selected -and $legacy){
          try { $selected=(([int]$legacy.Current.State -band 2) -ne 0) } catch {}
        }
        $out.Add([pscustomobject]@{browser=$browser;title=$name;active=([bool]($windowActive -and $selected))});
      } catch {}
    }
  } catch {}
}
[Console]::Out.Write((ConvertTo-Json -InputObject @($out) -Compress -Depth 3));`

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cmd := hiddenCommandContext(ctx, powershell, "-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass", "-EncodedCommand", encodePowerShell(script))
	raw, err := cmd.Output()
	if err != nil || ctx.Err() != nil {
		return nil
	}
	return decodeBrowserUITabs(raw, "windows-uia")
}

// collectBrowserTabsMSAA is the compatibility fallback for Chromium builds
// whose tab strip is not surfaced as regular UI Automation TabItem elements.
// It uses the Windows accessibility API (oleacc/IAccessible) and reads only
// ROLE_SYSTEM_PAGETAB names and selection state from visible browser windows.
func collectBrowserTabsMSAA() []agentBrowserTab {
	powershell, err := exec.LookPath("powershell.exe")
	if err != nil {
		return nil
	}

	const script = `$ErrorActionPreference='SilentlyContinue';
Add-Type -AssemblyName Accessibility;
if(-not ('CoreControl.AccessibleBrowserTabs' -as [type])){
Add-Type -ReferencedAssemblies 'Accessibility.dll' -TypeDefinition @'
using System;
using System.Collections.Generic;
using System.Diagnostics;
using System.Runtime.InteropServices;
using System.Text;
using Accessibility;

namespace CoreControl {
  public sealed class AccessibleTabInfo {
    public string browser { get; set; }
    public string title { get; set; }
    public bool active { get; set; }
  }

  public static class AccessibleBrowserTabs {
    private const uint OBJID_CLIENT = 0xFFFFFFFC;
    private const int ROLE_SYSTEM_DOCUMENT = 0x0F;
    private const int ROLE_SYSTEM_PAGETAB = 0x25;
    private const int STATE_SYSTEM_SELECTED = 0x2;
    private const int STATE_SYSTEM_FOCUSED = 0x4;
    private const int MaxNodesPerWindow = 12000;
    private const int MaxDepth = 22;

    [StructLayout(LayoutKind.Sequential)]
    private struct RECT { public int Left; public int Top; public int Right; public int Bottom; }

    private delegate bool EnumWindowsProc(IntPtr hwnd, IntPtr lParam);

    [DllImport("user32.dll")] private static extern bool EnumWindows(EnumWindowsProc callback, IntPtr lParam);
    [DllImport("user32.dll")] private static extern bool IsWindowVisible(IntPtr hwnd);
    [DllImport("user32.dll")] private static extern uint GetWindowThreadProcessId(IntPtr hwnd, out uint pid);
    [DllImport("user32.dll")] private static extern IntPtr GetForegroundWindow();
    [DllImport("user32.dll")] private static extern bool GetWindowRect(IntPtr hwnd, out RECT rect);
    [DllImport("oleacc.dll")] private static extern int AccessibleObjectFromWindow(IntPtr hwnd, uint objectId, ref Guid riid, [MarshalAs(UnmanagedType.Interface)] out IAccessible accessible);
    [DllImport("oleacc.dll")] private static extern int AccessibleChildren(IAccessible container, int childStart, int childCount, [Out, MarshalAs(UnmanagedType.LPArray, SizeParamIndex=2)] object[] children, out int obtained);

    public static AccessibleTabInfo[] Collect() {
      var result = new List<AccessibleTabInfo>();
      IntPtr foreground = GetForegroundWindow();
      EnumWindows(delegate(IntPtr hwnd, IntPtr _) {
        try {
          if (!IsWindowVisible(hwnd)) return true;
          uint pid;
          GetWindowThreadProcessId(hwnd, out pid);
          if (pid == 0) return true;
          string process = "";
          try { process = Process.GetProcessById((int)pid).ProcessName.ToLowerInvariant(); } catch { return true; }
          string browser = NormalizeBrowser(process);
          if (browser == null) return true;

          Guid iid = new Guid("618736E0-3C3D-11CF-810C-00AA00389B71");
          IAccessible root;
          if (AccessibleObjectFromWindow(hwnd, OBJID_CLIENT, ref iid, out root) < 0 || root == null) return true;
          RECT rect;
          if (!GetWindowRect(hwnd, out rect)) return true;
          int windowHeight = Math.Max(1, rect.Bottom - rect.Top);
          int seen = 0;
          Walk(root, 0, browser, hwnd == foreground, result, ref seen, 0, false, rect.Top, windowHeight);
        } catch { }
        return true;
      }, IntPtr.Zero);
      return result.ToArray();
    }

    private static string NormalizeBrowser(string process) {
      if (process == "chrome") return "chrome";
      if (process == "msedge") return "edge";
      if (process == "opera" || process == "opera_gx") return "opera";
      if (process == "brave") return "brave";
      return null;
    }

    private static int SafeRole(IAccessible acc, int childId) {
      try { return Convert.ToInt32(acc.get_accRole(childId)); } catch { return 0; }
    }

    private static void Walk(IAccessible acc, int selfChildId, string browser, bool windowActive, List<AccessibleTabInfo> result, ref int seen, int depth, bool insideDocument, int windowTop, int windowHeight) {
      if (acc == null || depth > MaxDepth || seen >= MaxNodesPerWindow) return;
      seen++;

      int selfRole = SafeRole(acc, selfChildId);
      bool currentInsideDocument = insideDocument || selfRole == ROLE_SYSTEM_DOCUMENT;
      Inspect(acc, selfChildId, browser, windowActive, result, currentInsideDocument, windowTop, windowHeight);

      int count = 0;
      try { count = acc.accChildCount; } catch { return; }
      if (count <= 0) return;
      count = Math.Min(count, MaxNodesPerWindow - seen);
      if (count <= 0) return;
      object[] children = new object[count];
      int obtained = 0;
      try {
        if (AccessibleChildren(acc, 0, count, children, out obtained) < 0 || obtained <= 0) return;
      } catch { return; }

      for (int i = 0; i < obtained && seen < MaxNodesPerWindow; i++) {
        object child = children[i];
        if (child == null) continue;
        IAccessible childAcc = child as IAccessible;
        if (childAcc != null) {
          Walk(childAcc, 0, browser, windowActive, result, ref seen, depth + 1, currentInsideDocument, windowTop, windowHeight);
          continue;
        }

        int childId;
        try { childId = Convert.ToInt32(child); } catch { continue; }
        int childRole = SafeRole(acc, childId);
        bool childInsideDocument = currentInsideDocument || childRole == ROLE_SYSTEM_DOCUMENT;
        seen++;
        Inspect(acc, childId, browser, windowActive, result, childInsideDocument, windowTop, windowHeight);
        try {
          IAccessible nested = acc.get_accChild(childId) as IAccessible;
          if (nested != null) Walk(nested, 0, browser, windowActive, result, ref seen, depth + 1, childInsideDocument, windowTop, windowHeight);
        } catch { }
      }
    }

    private static void Inspect(IAccessible acc, int childId, string browser, bool windowActive, List<AccessibleTabInfo> result, bool insideDocument, int windowTop, int windowHeight) {
      if (insideDocument) return;

      int role = SafeRole(acc, childId);
      if (role != ROLE_SYSTEM_PAGETAB) return;

      // The browser tab strip is part of the top chrome. Page widgets with
      // role=tab are normally lower in the renderer; reject them by geometry
      // as a second guard when the accessibility tree does not expose Document.
      int x = 0, y = 0, width = 0, height = 0;
      try { acc.accLocation(out x, out y, out width, out height, childId); } catch { return; }
      if (width <= 1 || height <= 1) return;
      int chromeBand = Math.Min(260, Math.Max(140, (int)Math.Round(windowHeight * 0.24)));
      if (y > windowTop + chromeBand) return;

      string name = "";
      try { name = (acc.get_accName(childId) ?? "").Trim(); } catch { }
      if (String.IsNullOrWhiteSpace(name)) return;

      int state = 0;
      try { state = Convert.ToInt32(acc.get_accState(childId)); } catch { }
      bool selected = (state & (STATE_SYSTEM_SELECTED | STATE_SYSTEM_FOCUSED)) != 0;
      result.Add(new AccessibleTabInfo { browser = browser, title = name, active = windowActive && selected });
    }
  }
}
'@;
}
[Console]::OutputEncoding=[System.Text.UTF8Encoding]::new($false);
$items=[CoreControl.AccessibleBrowserTabs]::Collect();
[Console]::Out.Write((ConvertTo-Json -InputObject @($items) -Compress -Depth 3));`

	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
	defer cancel()
	cmd := hiddenCommandContext(ctx, powershell, "-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass", "-EncodedCommand", encodePowerShell(script))
	raw, err := cmd.Output()
	if err != nil || ctx.Err() != nil {
		return nil
	}
	return decodeBrowserUITabs(raw, "windows-msaa")
}

func decodeBrowserUITabs(raw []byte, source string) []agentBrowserTab {
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
	// the same page title. Each accessibility page-tab object represents an
	// open tab and should remain visible in CoreControl.
	for _, item := range items {
		browser := normalizeAgentBrowser(item.Browser)
		title := strings.TrimSpace(item.Title)
		if browser == "" || title == "" {
			continue
		}
		out = append(out, agentBrowserTab{Browser: browser, Title: title, Active: item.Active, Source: source})
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
