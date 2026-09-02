//go:build windows

package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"regexp"
	"sort"
	"strings"
	"syscall"
	"time"
	"unicode/utf16"
)

type updateItem struct {
	ID               string `json:"id"`
	Title            string `json:"title"`
	KB               string `json:"kb,omitempty"`
	Severity         string `json:"severity,omitempty"`
	Downloaded       bool   `json:"downloaded,omitempty"`
	CurrentVersion   string `json:"current_version,omitempty"`
	AvailableVersion string `json:"available_version,omitempty"`
	SourceName       string `json:"source_name,omitempty"`
}

type windowsScanPayload struct {
	Windows []updateItem `json:"windows"`
	Drivers []updateItem `json:"drivers"`
}

type updateScanResult struct {
	Windows        []updateItem `json:"windows"`
	Drivers        []updateItem `json:"drivers"`
	Apps           []updateItem `json:"apps"`
	RebootRequired bool         `json:"reboot_required"`
	Warnings       []string     `json:"warnings"`
}

type updateInstallPayload struct {
	WindowsIDs []string `json:"windows_ids"`
	DriverIDs  []string `json:"driver_ids"`
	AppIDs     []string `json:"app_ids"`
}

type optimizationApplyPayload struct {
	Profile int `json:"profile"`
}

type installedItem struct {
	ID         string `json:"id"`
	Title      string `json:"title,omitempty"`
	Source     string `json:"source"`
	ResultCode int    `json:"result_code,omitempty"`
	Output     string `json:"output,omitempty"`
}

type updateInstallResult struct {
	Installed      []installedItem `json:"installed"`
	Failed         []installedItem `json:"failed"`
	RebootRequired bool            `json:"reboot_required"`
	Warnings       []string        `json:"warnings"`
}

type wuaInstallPayload struct {
	WindowsIDs []string `json:"windows_ids"`
	DriverIDs  []string `json:"driver_ids"`
}

var ansiPattern = regexp.MustCompile(`\x1b\[[0-9;?]*[ -/]*[@-~]`)

func executeAgentCommand(command pendingCommand) (map[string]interface{}, error) {
	switch command.Type {
	case "updates.scan":
		result, err := scanUpdates()
		if err != nil {
			return nil, err
		}
		return mapFromStruct(result)
	case "activity.snapshot":
		return mapFromStruct(collectActivitySnapshot())
	case "optimization.diagnose":
		return mapFromStruct(diagnoseOptimization())
	case "optimization.cleanup_temp":
		result, err := cleanupOptimizationTemp()
		mapped, mapErr := mapFromStruct(result)
		if mapErr != nil {
			return nil, mapErr
		}
		if err != nil {
			return mapped, err
		}
		return mapped, nil
	case "optimization.apply":
		var payload optimizationApplyPayload
		if len(command.Payload) > 0 {
			if err := json.Unmarshal(command.Payload, &payload); err != nil {
				return nil, fmt.Errorf("payload de otimização inválido: %w", err)
			}
		}
		if payload.Profile < 1 || payload.Profile > 5 {
			return nil, errors.New("perfil de otimização inválido")
		}
		result, err := applyOptimizationProfile(payload.Profile)
		mapped, mapErr := mapFromStruct(result)
		if mapErr != nil {
			return nil, mapErr
		}
		if err != nil {
			return mapped, err
		}
		return mapped, nil
	case "updates.install":
		var payload updateInstallPayload
		if len(command.Payload) > 0 {
			if err := json.Unmarshal(command.Payload, &payload); err != nil {
				return nil, fmt.Errorf("payload de atualização inválido: %w", err)
			}
		}
		if len(payload.WindowsIDs)+len(payload.DriverIDs)+len(payload.AppIDs) == 0 {
			return nil, errors.New("nenhuma atualização foi selecionada")
		}
		result, err := installUpdates(payload)
		mapped, mapErr := mapFromStruct(result)
		if mapErr != nil {
			return nil, mapErr
		}
		if err != nil {
			return mapped, err
		}
		return mapped, nil
	default:
		return nil, fmt.Errorf("tipo de comando não permitido: %s", command.Type)
	}
}

func mapFromStruct(value interface{}) (map[string]interface{}, error) {
	raw, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	var result map[string]interface{}
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, err
	}
	return result, nil
}

func scanUpdates() (updateScanResult, error) {
	result := updateScanResult{
		Windows:  []updateItem{},
		Drivers:  []updateItem{},
		Apps:     []updateItem{},
		Warnings: []string{},
	}
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Minute)
	defer cancel()

	wua, err := scanWindowsUpdates(ctx)
	if err != nil {
		result.Warnings = append(result.Warnings, "Windows Update: "+err.Error())
	} else {
		result.Windows = wua.Windows
		result.Drivers = wua.Drivers
	}

	apps, warning := scanWingetApps(ctx)
	result.Apps = apps
	if warning != "" {
		result.Warnings = append(result.Warnings, warning)
	}

	if err != nil {
		return result, fmt.Errorf("não foi possível consultar o Windows Update: %w", err)
	}
	return result, nil
}

func scanWindowsUpdates(ctx context.Context) (windowsScanPayload, error) {
	script := `$ErrorActionPreference='Stop'
$ProgressPreference='SilentlyContinue'
$session = New-Object -ComObject Microsoft.Update.Session
$searcher = $session.CreateUpdateSearcher()
function Read-Updates([string]$criteria) {
  $result = $searcher.Search($criteria)
  $items = @()
  for ($i = 0; $i -lt $result.Updates.Count; $i++) {
    $u = $result.Updates.Item($i)
    $severity = ''
    try { $severity = [string]$u.MsrcSeverity } catch {}
    $kb = ''
    try { $kb = (@($u.KBArticleIDs) -join ',') } catch {}
    $items += [PSCustomObject]@{
      id = [string]$u.Identity.UpdateID
      title = [string]$u.Title
      kb = $kb
      severity = $severity
      downloaded = [bool]$u.IsDownloaded
    }
  }
  return @($items)
}
$obj = [PSCustomObject]@{
  windows = @(Read-Updates "IsInstalled=0 and IsHidden=0 and Type='Software'")
  drivers = @(Read-Updates "IsInstalled=0 and IsHidden=0 and Type='Driver'")
}
$obj | ConvertTo-Json -Depth 6 -Compress`
	var result windowsScanPayload
	if err := runPowerShellJSON(ctx, script, &result); err != nil {
		return windowsScanPayload{}, err
	}
	if result.Windows == nil {
		result.Windows = []updateItem{}
	}
	if result.Drivers == nil {
		result.Drivers = []updateItem{}
	}
	return result, nil
}

func installUpdates(payload updateInstallPayload) (updateInstallResult, error) {
	result := updateInstallResult{Installed: []installedItem{}, Failed: []installedItem{}, Warnings: []string{}}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Minute)
	defer cancel()

	if len(payload.WindowsIDs)+len(payload.DriverIDs) > 0 {
		windowsResult, err := installWindowsUpdates(ctx, payload.WindowsIDs, payload.DriverIDs)
		result.Installed = append(result.Installed, windowsResult.Installed...)
		result.Failed = append(result.Failed, windowsResult.Failed...)
		result.Warnings = append(result.Warnings, windowsResult.Warnings...)
		result.RebootRequired = result.RebootRequired || windowsResult.RebootRequired
		if err != nil {
			result.Warnings = append(result.Warnings, "Windows Update: "+err.Error())
		}
	}

	for _, id := range uniqueStrings(payload.AppIDs) {
		item := installedItem{ID: id, Source: "app"}
		output, err := runWingetUpgrade(ctx, id)
		item.Output = truncateText(output, 1200)
		if err != nil {
			result.Failed = append(result.Failed, item)
			continue
		}
		result.Installed = append(result.Installed, item)
	}

	if len(result.Failed) > 0 {
		return result, fmt.Errorf("%d atualização(ões) falharam", len(result.Failed))
	}
	return result, nil
}

func installWindowsUpdates(ctx context.Context, windowsIDs, driverIDs []string) (updateInstallResult, error) {
	payload := wuaInstallPayload{WindowsIDs: uniqueStrings(windowsIDs), DriverIDs: uniqueStrings(driverIDs)}
	raw, _ := json.Marshal(payload)
	encodedPayload := base64.StdEncoding.EncodeToString(raw)
	script := fmt.Sprintf(`$ErrorActionPreference='Stop'
$ProgressPreference='SilentlyContinue'
$payloadJson = [Text.Encoding]::UTF8.GetString([Convert]::FromBase64String('%s'))
$payload = $payloadJson | ConvertFrom-Json
$softwareIds = @($payload.windows_ids)
$driverIds = @($payload.driver_ids)
$session = New-Object -ComObject Microsoft.Update.Session
$searcher = $session.CreateUpdateSearcher()
$selected = New-Object -ComObject Microsoft.Update.UpdateColl
$meta = @()
function Add-Selected([string]$criteria, [object[]]$ids, [string]$source) {
  if ($ids.Count -eq 0) { return }
  $lookup = @{}
  foreach ($id in $ids) { $lookup[[string]$id] = $true }
  $found = $searcher.Search($criteria)
  for ($i = 0; $i -lt $found.Updates.Count; $i++) {
    $u = $found.Updates.Item($i)
    $id = [string]$u.Identity.UpdateID
    if ($lookup.ContainsKey($id)) {
      if (-not $u.EulaAccepted) { try { $u.AcceptEula() } catch {} }
      [void]$selected.Add($u)
      $script:meta += [PSCustomObject]@{ id=$id; title=[string]$u.Title; source=$source }
    }
  }
}
Add-Selected "IsInstalled=0 and IsHidden=0 and Type='Software'" $softwareIds 'windows'
Add-Selected "IsInstalled=0 and IsHidden=0 and Type='Driver'" $driverIds 'driver'
if ($selected.Count -eq 0) {
  [PSCustomObject]@{installed=@();failed=@();reboot_required=$false;warnings=@('As atualizações selecionadas não estão mais pendentes.')} | ConvertTo-Json -Depth 6 -Compress
  exit 0
}
$downloader = $session.CreateUpdateDownloader()
$downloader.Updates = $selected
$downloadResult = $downloader.Download()
$installer = $session.CreateUpdateInstaller()
$installer.Updates = $selected
$installResult = $installer.Install()
$installed = @()
$failed = @()
for ($i = 0; $i -lt $selected.Count; $i++) {
  $r = $installResult.GetUpdateResult($i)
  $m = $meta[$i]
  $entry = [PSCustomObject]@{id=$m.id;title=$m.title;source=$m.source;result_code=[int]$r.ResultCode;output=('HRESULT 0x{0:X8}' -f ([uint32]$r.HResult))}
  if ($r.ResultCode -eq 2 -or $r.ResultCode -eq 3) { $installed += $entry } else { $failed += $entry }
}
[PSCustomObject]@{installed=$installed;failed=$failed;reboot_required=[bool]$installResult.RebootRequired;warnings=@()} | ConvertTo-Json -Depth 6 -Compress`, encodedPayload)
	var result updateInstallResult
	if err := runPowerShellJSON(ctx, script, &result); err != nil {
		return updateInstallResult{Installed: []installedItem{}, Failed: []installedItem{}, Warnings: []string{}}, err
	}
	if result.Installed == nil {
		result.Installed = []installedItem{}
	}
	if result.Failed == nil {
		result.Failed = []installedItem{}
	}
	if result.Warnings == nil {
		result.Warnings = []string{}
	}
	return result, nil
}

func runPowerShellJSON(ctx context.Context, script string, output interface{}) error {
	encoded := encodePowerShell(script)
	cmd := hiddenCommandContext(ctx, "powershell.exe", "-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass", "-EncodedCommand", encoded)
	raw, err := cmd.CombinedOutput()
	if ctx.Err() != nil {
		return fmt.Errorf("tempo limite excedido")
	}
	text := strings.TrimSpace(string(raw))
	if err != nil {
		return fmt.Errorf("%v: %s", err, truncateText(text, 1000))
	}
	if text == "" {
		return errors.New("PowerShell não retornou resultado")
	}
	if json.Unmarshal([]byte(text), output) == nil {
		return nil
	}
	lines := strings.Split(text, "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		candidate := strings.TrimSpace(lines[i])
		if strings.HasPrefix(candidate, "{") && json.Unmarshal([]byte(candidate), output) == nil {
			return nil
		}
	}
	return fmt.Errorf("resposta inválida do Windows Update: %s", truncateText(text, 1000))
}

func encodePowerShell(script string) string {
	encoded := utf16.Encode([]rune(script))
	bytes := make([]byte, len(encoded)*2)
	for i, value := range encoded {
		bytes[i*2] = byte(value)
		bytes[i*2+1] = byte(value >> 8)
	}
	return base64.StdEncoding.EncodeToString(bytes)
}

func hiddenCommandContext(ctx context.Context, name string, args ...string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000}
	return cmd
}

func scanWingetApps(ctx context.Context) ([]updateItem, string) {
	path, err := exec.LookPath("winget.exe")
	if err != nil {
		return []updateItem{}, "Aplicativos: Windows Package Manager (winget) não está disponível neste computador."
	}
	cmd := hiddenCommandContext(ctx, path, "upgrade", "--accept-source-agreements", "--disable-interactivity")
	raw, commandErr := cmd.CombinedOutput()
	if ctx.Err() != nil {
		return []updateItem{}, "Aplicativos: a consulta do winget excedeu o tempo limite."
	}
	text := ansiPattern.ReplaceAllString(string(raw), "")
	items := parseWingetUpgradeTable(text)
	if commandErr != nil && len(items) == 0 {
		return []updateItem{}, "Aplicativos: winget não conseguiu consultar atualizações."
	}
	return items, ""
}

func parseWingetUpgradeTable(text string) []updateItem {
	lines := strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")
	separatorIndex := -1
	var starts []int
	for index, line := range lines {
		trimmed := strings.TrimSpace(line)
		if len(trimmed) < 20 || strings.Trim(trimmed, "-") != "" {
			continue
		}
		headerIndex := index - 1
		for headerIndex >= 0 && strings.TrimSpace(lines[headerIndex]) == "" {
			headerIndex--
		}
		if headerIndex < 0 {
			continue
		}
		starts = tableColumnStarts(lines[headerIndex])
		if len(starts) >= 4 {
			separatorIndex = index
			break
		}
	}
	if separatorIndex < 0 || len(starts) < 4 {
		return []updateItem{}
	}
	if len(starts) > 5 {
		starts = starts[:5]
	}
	items := []updateItem{}
	seen := map[string]bool{}
	for _, line := range lines[separatorIndex+1:] {
		if strings.TrimSpace(line) == "" {
			continue
		}
		values := make([]string, len(starts))
		for i, start := range starts {
			end := len(line)
			if i+1 < len(starts) {
				end = starts[i+1]
			}
			if start >= len(line) {
				continue
			}
			if end > len(line) {
				end = len(line)
			}
			values[i] = strings.TrimSpace(line[start:end])
		}
		if len(values) < 4 {
			continue
		}
		name := values[0]
		id := values[1]
		current := values[2]
		available := values[3]
		source := "winget"
		if len(values) >= 5 && values[4] != "" {
			source = values[4]
		}
		if id == "" || current == "" || available == "" || seen[id] {
			continue
		}
		// Linhas de resumo do winget não respeitam o identificador de pacote e
		// normalmente não chegam a preencher todas as colunas.
		if strings.Contains(strings.ToLower(id), "upgrade") && !strings.Contains(id, ".") {
			continue
		}
		seen[id] = true
		items = append(items, updateItem{
			ID:               id,
			Title:            name,
			CurrentVersion:   current,
			AvailableVersion: available,
			SourceName:       source,
		})
	}
	sort.Slice(items, func(i, j int) bool { return strings.ToLower(items[i].Title) < strings.ToLower(items[j].Title) })
	return items
}

func tableColumnStarts(header string) []int {
	starts := []int{}
	inText := false
	spaceRun := 0
	for index := 0; index < len(header); index++ {
		if header[index] == ' ' || header[index] == '\t' {
			if inText {
				spaceRun++
			}
			continue
		}
		if !inText {
			starts = append(starts, index)
			inText = true
			spaceRun = 0
			continue
		}
		if spaceRun >= 2 {
			starts = append(starts, index)
		}
		spaceRun = 0
	}
	return starts
}

func runWingetUpgrade(ctx context.Context, id string) (string, error) {
	path, err := exec.LookPath("winget.exe")
	if err != nil {
		return "", errors.New("winget não está disponível")
	}
	cmd := hiddenCommandContext(
		ctx,
		path,
		"upgrade", "--id", id, "--exact", "--silent",
		"--accept-package-agreements", "--accept-source-agreements", "--disable-interactivity",
	)
	raw, err := cmd.CombinedOutput()
	if ctx.Err() != nil {
		return string(raw), errors.New("tempo limite excedido ao atualizar aplicativo")
	}
	return strings.TrimSpace(ansiPattern.ReplaceAllString(string(raw), "")), err
}

func uniqueStrings(values []string) []string {
	seen := map[string]bool{}
	result := []string{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}

func truncateText(value string, limit int) string {
	value = strings.TrimSpace(value)
	if len(value) <= limit {
		return value
	}
	return value[:limit] + "…"
}
