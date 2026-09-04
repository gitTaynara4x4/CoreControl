//go:build windows

package main

import (
	"context"
	"strings"
	"sync"
	"time"
)

var wolCapabilityCache struct {
	sync.Mutex
	MAC       string
	Value     wolCapability
	ExpiresAt time.Time
}

func collectWOLCapability(primaryMAC string) wolCapability {
	mac := strings.ToLower(strings.TrimSpace(primaryMAC))
	now := time.Now()

	wolCapabilityCache.Lock()
	if mac != "" && wolCapabilityCache.MAC == mac && now.Before(wolCapabilityCache.ExpiresAt) {
		value := wolCapabilityCache.Value
		wolCapabilityCache.Unlock()
		return value
	}
	wolCapabilityCache.Unlock()

	value := inspectAndPrepareWindowsWOL(mac)
	wolCapabilityCache.Lock()
	wolCapabilityCache.MAC = mac
	wolCapabilityCache.Value = value
	wolCapabilityCache.ExpiresAt = now.Add(10 * time.Minute)
	wolCapabilityCache.Unlock()
	return value
}

func inspectAndPrepareWindowsWOL(primaryMAC string) wolCapability {
	result := wolCapability{
		Checked:    true,
		CheckedAt:  time.Now().UTC().Format(time.RFC3339),
		MACAddress: strings.ToLower(strings.TrimSpace(primaryMAC)),
	}
	if result.MACAddress == "" {
		result.Reason = "O endereço MAC principal ainda não foi identificado."
		return result
	}

	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
	defer cancel()

	// A checagem usa somente cmdlets nativos do Windows. O Agent roda como serviço
	// com privilégios suficientes para preparar a placa quando o driver permite.
	// Não altera BIOS/UEFI: isso varia por fabricante e não pode ser afirmado como
	// configurado sem suporte específico do firmware.
	script := `
$ErrorActionPreference = 'SilentlyContinue'
$mac = '` + powerShellSingleQuoted(result.MACAddress) + `'
$macNorm = ($mac -replace ':','-').ToUpperInvariant()

function Is-EnabledValue($value) {
  if ($null -eq $value) { return $false }
  $text = [string]$value
  return ($text -match '^(1|True|Enabled)$')
}
function Has-Device($lines, $candidates) {
  foreach ($line in @($lines)) {
    $trimmed = ([string]$line).Trim()
    if (-not $trimmed) { continue }
    foreach ($candidate in @($candidates)) {
      if ($candidate -and $trimmed -eq ([string]$candidate).Trim()) { return $true }
    }
  }
  return $false
}

$out = [ordered]@{
  checked = $true
  adapter_name = ''
  interface_description = ''
  mac_address = $mac.ToLowerInvariant()
  link_type = 'unknown'
  magic_packet_supported = $false
  magic_packet_enabled = $false
  wake_programmable = $false
  wake_armed = $false
  s5_driver_hint = $false
  intel_amt_detected = $false
  auto_configured = $false
  windows_prepared = $false
  firmware_needs_check = $true
  reason = ''
  error = ''
}

$virtualPattern = '(?i)(radmin|famatech|vpn|tailscale|zerotier|hamachi|hyper-v|vethernet|vmware|virtualbox|host-only|tap-windows|wintun|wireguard|openvpn|nordlynx|proton|warp|loopback|bluetooth|docker|container)'
function Is-PhysicalAdapter($candidate) {
  if (-not $candidate) { return $false }
  $text = (([string]$candidate.Name) + ' ' + ([string]$candidate.InterfaceDescription))
  if ($text -match $virtualPattern) { return $false }
  if ($null -ne $candidate.PSObject.Properties['HardwareInterface'] -and -not [bool]$candidate.HardwareInterface) { return $false }
  if ($null -ne $candidate.PSObject.Properties['Virtual'] -and [bool]$candidate.Virtual) { return $false }
  return $true
}

$adapter = Get-NetAdapter -IncludeHidden | Where-Object { $_.MacAddress -eq $macNorm } | Sort-Object ifIndex | Select-Object -First 1

# Defesa em profundidade: mesmo que a coleta principal tenha recebido o MAC de
# Radmin/TAP/VPN, o preflight nunca prepara uma placa virtual para Wake-on-LAN.
if (-not (Is-PhysicalAdapter $adapter)) {
  $physical = @(Get-NetAdapter -IncludeHidden | Where-Object {
    $_.Status -eq 'Up' -and $_.MacAddress -and (Is-PhysicalAdapter $_)
  })
  $defaultRoutes = @(Get-NetRoute -AddressFamily IPv4 -DestinationPrefix '0.0.0.0/0' -ErrorAction SilentlyContinue |
    Sort-Object @{Expression={ [int]$_.RouteMetric }; Ascending=$true}, @{Expression={ [int]$_.ifMetric }; Ascending=$true})
  $adapter = $null
  foreach ($route in $defaultRoutes) {
    $match = $physical | Where-Object { [int]$_.ifIndex -eq [int]$route.ifIndex } | Select-Object -First 1
    if ($match) { $adapter = $match; break }
  }
  if (-not $adapter) { $adapter = $physical | Sort-Object ifIndex | Select-Object -First 1 }
}

if (-not $adapter) {
  $out.error = 'Nenhuma placa de rede física ativa foi encontrada para preparar Wake-on-LAN.'
  $out.reason = $out.error
  $out | ConvertTo-Json -Compress -Depth 5
  exit 0
}

$out.adapter_name = [string]$adapter.Name
$out.interface_description = [string]$adapter.InterfaceDescription
if ($adapter.MacAddress) { $out.mac_address = (([string]$adapter.MacAddress) -replace '-',':').ToLowerInvariant() }
$medium = (([string]$adapter.MediaType) + ' ' + ([string]$adapter.PhysicalMediaType) + ' ' + ([string]$adapter.InterfaceDescription)).ToLowerInvariant()
if ($medium -match '802\.11|wireless|wi-fi|wifi|wlan') { $out.link_type = 'wifi' }
elseif ($medium -match '802\.3|ethernet|gigabit|gbe|lan') { $out.link_type = 'ethernet' }

$pm = Get-NetAdapterPowerManagement -Name $adapter.Name -ErrorAction SilentlyContinue
if ($pm) {
  $out.magic_packet_supported = ($null -ne $pm.WakeOnMagicPacket)
  $out.magic_packet_enabled = Is-EnabledValue $pm.WakeOnMagicPacket
  if ($out.magic_packet_supported -and -not $out.magic_packet_enabled) {
    try {
      Set-NetAdapterPowerManagement -Name $adapter.Name -WakeOnMagicPacket Enabled -ErrorAction Stop | Out-Null
      $out.auto_configured = $true
    } catch {}
  }
}

$advanced = Get-NetAdapterAdvancedProperty -Name $adapter.Name -AllProperties -ErrorAction SilentlyContinue
$magicProp = $advanced | Where-Object { ([string]$_.RegistryKeyword) -match '(?i)Wake.*Magic|\*WakeOnMagicPacket' } | Select-Object -First 1
if ($magicProp) {
  $out.magic_packet_supported = $true
  if (([string]$magicProp.RegistryValue) -match '^(1|Enabled|True)$' -or ([string]$magicProp.DisplayValue) -match '(?i)enabled|ativado|ligado') {
    $out.magic_packet_enabled = $true
  }
}
$s5Prop = $advanced | Where-Object {
  (([string]$_.RegistryKeyword) + ' ' + ([string]$_.DisplayName)) -match '(?i)(S5|Shutdown|PowerOff).*(Wake|WOL)|(Wake|WOL).*(S5|Shutdown|PowerOff)'
} | Select-Object -First 1
if ($s5Prop) {
  $valueText = ([string]$s5Prop.RegistryValue) + ' ' + ([string]$s5Prop.DisplayValue)
  if ($valueText -match '(?i)(^|\s)(1|Enabled|True|On|Ativado|Ligado)(\s|$)') { $out.s5_driver_hint = $true }
}

$candidates = @($adapter.InterfaceDescription, $adapter.Name)
$programmable = & powercfg.exe /devicequery wake_programmable 2>$null
$armed = & powercfg.exe /devicequery wake_armed 2>$null
$out.wake_programmable = Has-Device $programmable $candidates
$out.wake_armed = Has-Device $armed $candidates

if ($out.wake_programmable -and -not $out.wake_armed) {
  $target = [string]$adapter.InterfaceDescription
  if (-not $target) { $target = [string]$adapter.Name }
  if ($target) {
    & powercfg.exe /deviceenablewake $target 2>$null | Out-Null
    $armed = & powercfg.exe /devicequery wake_armed 2>$null
    $out.wake_armed = Has-Device $armed $candidates
    if ($out.wake_armed) { $out.auto_configured = $true }
  }
}

# Intel AMT/ME: é apenas detecção local. A Central só considera uma rota
# realmente segura depois que o método de energia também estiver provisionado.
try {
  $me = Get-CimInstance -Namespace root\Intel_ME -ClassName ME_System -ErrorAction Stop | Select-Object -First 1
  if ($me) { $out.intel_amt_detected = $true }
} catch {}
if (-not $out.intel_amt_detected) {
  $lms = Get-Service -Name LMS -ErrorAction SilentlyContinue
  if ($lms) {
    $amtNic = Get-NetAdapter -IncludeHidden | Where-Object { ([string]$_.InterfaceDescription) -match '(?i)Intel.*(LM|AMT|vPro)' } | Select-Object -First 1
    if ($amtNic) { $out.intel_amt_detected = $true }
  }
}

# Releia a configuração de energia depois das tentativas automáticas.
$pm2 = Get-NetAdapterPowerManagement -Name $adapter.Name -ErrorAction SilentlyContinue
if ($pm2 -and (Is-EnabledValue $pm2.WakeOnMagicPacket)) { $out.magic_packet_enabled = $true }

$out.windows_prepared = [bool]($out.magic_packet_supported -and $out.magic_packet_enabled -and $out.wake_armed)
if ($out.windows_prepared) {
  if ($out.link_type -eq 'wifi') {
    $out.reason = 'Windows preparado para wake, mas Wake-on-WLAN após desligamento total depende do hardware/firmware.'
  } elseif ($out.s5_driver_hint) {
    $out.reason = 'Windows e driver estão preparados para Magic Packet; o driver também anuncia recurso relacionado a wake após desligamento.'
  } else {
    $out.reason = 'Windows preparado para Magic Packet. O suporte após desligamento total ainda depende da BIOS/UEFI e da entrega do pacote pela rede.'
  }
} elseif (-not $out.magic_packet_supported) {
  $out.reason = 'O driver desta placa não expôs suporte a Wake on Magic Packet para o Windows.'
} elseif (-not $out.magic_packet_enabled) {
  $out.reason = 'Wake on Magic Packet não ficou habilitado no driver da placa.'
} elseif (-not $out.wake_armed) {
  $out.reason = 'A placa não ficou armada pelo Windows para acordar o computador.'
}

$out | ConvertTo-Json -Compress -Depth 5
`

	var raw struct {
		Checked              bool   `json:"checked"`
		AdapterName          string `json:"adapter_name"`
		InterfaceDescription string `json:"interface_description"`
		MACAddress           string `json:"mac_address"`
		LinkType             string `json:"link_type"`
		MagicPacketSupported bool   `json:"magic_packet_supported"`
		MagicPacketEnabled   bool   `json:"magic_packet_enabled"`
		WakeProgrammable     bool   `json:"wake_programmable"`
		WakeArmed            bool   `json:"wake_armed"`
		S5DriverHint         bool   `json:"s5_driver_hint"`
		IntelAMTDetected     bool   `json:"intel_amt_detected"`
		AutoConfigured       bool   `json:"auto_configured"`
		WindowsPrepared      bool   `json:"windows_prepared"`
		FirmwareNeedsCheck   bool   `json:"firmware_needs_check"`
		Reason               string `json:"reason"`
		Error                string `json:"error"`
	}
	if err := runPowerShellJSON(ctx, script, &raw); err != nil {
		result.Error = err.Error()
		result.Reason = "Não foi possível concluir o diagnóstico automático de Wake-on-LAN neste ciclo."
		return result
	}

	result.Checked = raw.Checked
	result.AdapterName = strings.TrimSpace(raw.AdapterName)
	result.InterfaceDescription = strings.TrimSpace(raw.InterfaceDescription)
	if strings.TrimSpace(raw.MACAddress) != "" {
		result.MACAddress = strings.ToLower(strings.TrimSpace(raw.MACAddress))
	}
	result.LinkType = strings.TrimSpace(raw.LinkType)
	result.MagicPacketSupported = raw.MagicPacketSupported
	result.MagicPacketEnabled = raw.MagicPacketEnabled
	result.WakeProgrammable = raw.WakeProgrammable
	result.WakeArmed = raw.WakeArmed
	result.S5DriverHint = raw.S5DriverHint
	result.IntelAMTDetected = raw.IntelAMTDetected
	result.AutoConfigured = raw.AutoConfigured
	result.WindowsPrepared = raw.WindowsPrepared
	result.FirmwareNeedsCheck = raw.FirmwareNeedsCheck
	result.Reason = strings.TrimSpace(raw.Reason)
	result.Error = strings.TrimSpace(raw.Error)
	return result
}

func powerShellSingleQuoted(value string) string {
	return strings.ReplaceAll(value, "'", "''")
}
