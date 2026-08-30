//go:build windows

package main

import (
	"context"
	"encoding/json"
	"os/exec"
	"strings"
	"time"
)

type explorerFolderTab struct {
	PID   int    `json:"pid"`
	Title string `json:"title"`
}

// collectExplorerFolderTabs complements EnumWindows on Windows 11.
// File Explorer can host many folders as tabs inside a single top-level
// CabinetWClass window; EnumWindows only exposes the container/current tab.
// UI Automation exposes those TabItem controls, so we return them as virtual
// windows with process_name=explorer. Failure is intentionally non-fatal.
func collectExplorerFolderTabs() []agentWindowInfo {
	powershell, err := exec.LookPath("powershell.exe")
	if err != nil {
		return nil
	}

	const script = `$ErrorActionPreference='SilentlyContinue';
Add-Type -AssemblyName UIAutomationClient;
Add-Type -AssemblyName UIAutomationTypes;
$root=[System.Windows.Automation.AutomationElement]::RootElement;
$wins=$root.FindAll([System.Windows.Automation.TreeScope]::Children,[System.Windows.Automation.Condition]::TrueCondition);
$out=New-Object System.Collections.Generic.List[object];
foreach($w in $wins){
  try {
    $class=[string]$w.Current.ClassName;
    if($class -ne 'CabinetWClass' -and $class -ne 'ExploreWClass'){ continue }
    $windowPid=[int]$w.Current.ProcessId;
    if($windowPid -le 0){ continue }
    $tabCondition=[System.Windows.Automation.PropertyCondition]::new([System.Windows.Automation.AutomationElement]::ControlTypeProperty,[System.Windows.Automation.ControlType]::TabItem);
    $tabs=$w.FindAll([System.Windows.Automation.TreeScope]::Descendants,$tabCondition);
    $added=0;
    foreach($tab in $tabs){
      try {
        $name=([string]$tab.Current.Name).Trim();
        if([string]::IsNullOrWhiteSpace($name)){ continue }
        $out.Add([pscustomobject]@{pid=$windowPid;title=$name});
        $added++;
      } catch {}
    }
    if($added -eq 0){
      $name=([string]$w.Current.Name).Trim();
      if(-not [string]::IsNullOrWhiteSpace($name)){
        $out.Add([pscustomobject]@{pid=$windowPid;title=$name});
      }
    }
  } catch {}
}
[Console]::Out.Write((ConvertTo-Json -InputObject @($out) -Compress -Depth 3));`

	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
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
	var tabs []explorerFolderTab
	if json.Unmarshal([]byte(text), &tabs) != nil {
		return nil
	}
	result := make([]agentWindowInfo, 0, len(tabs))
	for _, tab := range tabs {
		title := strings.TrimSpace(tab.Title)
		if tab.PID <= 0 || title == "" || strings.EqualFold(title, "File Explorer") || strings.EqualFold(title, "Explorador de Arquivos") {
			continue
		}
		result = append(result, agentWindowInfo{PID: tab.PID, ProcessName: "explorer", Title: title})
	}
	return result
}
