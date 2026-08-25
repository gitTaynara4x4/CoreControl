$ErrorActionPreference = 'Stop'

$root = Split-Path -Parent $PSScriptRoot
$hostSrc = Join-Path $PSScriptRoot 'host\src'
$extensionDir = Join-Path $PSScriptRoot 'extension'
$extensionManifest = Join-Path $extensionDir 'manifest.json'
$installDir = Join-Path $env:LOCALAPPDATA 'Programs\CoreControl'
$hostPath = Join-Path $installDir 'CoreControlBrowserHost.exe'
$manifestPath = Join-Path $installDir 'com.corecontrol.browser.json'
$tabsFile = Join-Path $env:LOCALAPPDATA 'CoreTuner\Browser\browser-tabs.json'
$extensionId = 'ndljalddjljcojeekmcbbophpdfpfpoj'

Write-Host ''
Write-Host 'CoreControl Browser Bridge DEV' -ForegroundColor Cyan
Write-Host '==============================' -ForegroundColor DarkGray

if (-not (Test-Path $extensionManifest)) {
    throw "Extensao incompleta: manifest.json nao encontrado em $extensionManifest"
}
if (-not (Test-Path $hostSrc)) {
    throw "Browser Host nao encontrado em $hostSrc"
}

New-Item -ItemType Directory -Force -Path $installDir | Out-Null

Write-Host '[1/5] Compilando Browser Host...' -ForegroundColor Cyan
Push-Location $hostSrc
try {
    $env:GOOS = 'windows'
    $env:GOARCH = 'amd64'
    $env:CGO_ENABLED = '0'
    go build -trimpath -ldflags '-s -w' -o $hostPath .
    if ($LASTEXITCODE -ne 0) { throw 'Falha ao compilar CoreControlBrowserHost.exe' }
}
finally {
    Pop-Location
}

Write-Host '[2/5] Registrando integracao nativa...' -ForegroundColor Cyan
$nativeManifest = [ordered]@{
    name = 'com.corecontrol.browser'
    description = 'CoreControl Browser Integration'
    path = $hostPath
    type = 'stdio'
    allowed_origins = @("chrome-extension://$extensionId/")
} | ConvertTo-Json -Depth 4
[System.IO.File]::WriteAllText($manifestPath, $nativeManifest, (New-Object System.Text.UTF8Encoding($false)))

$keys = @(
    'HKCU\Software\Google\Chrome\NativeMessagingHosts\com.corecontrol.browser',
    'HKCU\Software\Microsoft\Edge\NativeMessagingHosts\com.corecontrol.browser',
    'HKCU\Software\Opera Software\NativeMessagingHosts\com.corecontrol.browser'
)
foreach ($key in $keys) {
    & reg.exe add $key /ve /t REG_SZ /d $manifestPath /f | Out-Null
}

Write-Host '[3/5] Localizando Opera...' -ForegroundColor Cyan
$possibleOpera = @(
    (Join-Path $env:LOCALAPPDATA 'Programs\Opera GX\opera.exe'),
    (Join-Path $env:LOCALAPPDATA 'Programs\Opera\opera.exe'),
    (Join-Path $env:ProgramFiles 'Opera\opera.exe')
)
if (${env:ProgramFiles(x86)}) {
    $possibleOpera += (Join-Path ${env:ProgramFiles(x86)} 'Opera\opera.exe')
}
$opera = $possibleOpera | Where-Object { $_ -and (Test-Path $_) } | Select-Object -First 1
if (-not $opera) {
    $opera = Get-ChildItem (Join-Path $env:LOCALAPPDATA 'Programs') -Filter opera.exe -Recurse -ErrorAction SilentlyContinue |
        Select-Object -First 1 -ExpandProperty FullName
}
if (-not $opera) {
    throw 'Opera nao encontrado neste computador.'
}
Write-Host "Opera: $opera" -ForegroundColor DarkGray

Write-Host '[4/5] Reiniciando Opera com a extensao CoreControl...' -ForegroundColor Cyan
Get-Process opera -ErrorAction SilentlyContinue | Stop-Process -Force
Start-Sleep -Seconds 2

# Remove o snapshot antigo para o teste comprovar uma sincronizacao NOVA.
Remove-Item $tabsFile -Force -ErrorAction SilentlyContinue

# IMPORTANTe: ProcessStartInfo.Arguments preserva as aspas do caminho "User 01".
$psi = New-Object System.Diagnostics.ProcessStartInfo
$psi.FileName = $opera
$psi.Arguments = "--load-extension=`"$extensionDir`" opera://extensions"
$psi.UseShellExecute = $true
[System.Diagnostics.Process]::Start($psi) | Out-Null

Write-Host '[5/5] Aguardando primeira sincronizacao...' -ForegroundColor Cyan
$ok = $false
for ($i = 1; $i -le 35; $i++) {
    Start-Sleep -Seconds 1
    if (Test-Path $tabsFile) {
        $ok = $true
        break
    }
}

Write-Host ''
if ($ok) {
    Write-Host 'SUCESSO: o CoreControl recebeu as abas do Opera.' -ForegroundColor Green
    Write-Host "Arquivo: $tabsFile" -ForegroundColor DarkGray
    try {
        $state = Get-Content $tabsFile -Raw | ConvertFrom-Json
        $operaState = $state.browsers.opera
        if ($operaState) {
            $count = @($operaState.tabs).Count
            Write-Host "Abas recebidas: $count" -ForegroundColor Green
            @($operaState.tabs) | Select-Object -First 8 title, domain, active | Format-Table -AutoSize
        }
    }
    catch {
        Write-Host 'Snapshot criado; nao foi possivel resumir o JSON no terminal.' -ForegroundColor Yellow
    }
} else {
    Write-Host 'A extensao foi iniciada, mas nenhuma aba chegou ao Browser Host.' -ForegroundColor Yellow
    Write-Host ''
    Write-Host 'Diagnostico automatico:' -ForegroundColor Cyan
    Write-Host "  manifest.json da extensao: $(Test-Path $extensionManifest)"
    Write-Host "  Browser Host:              $(Test-Path $hostPath)"
    Write-Host "  Manifest nativo:           $(Test-Path $manifestPath)"
    Write-Host "  Snapshot de abas:          $(Test-Path $tabsFile)"
    Write-Host "  Extensao ID esperada:      $extensionId"
    Write-Host ''
    Write-Host 'Se o Opera mostrar um erro, mande o print e estas ultimas linhas do PowerShell.' -ForegroundColor Yellow
}
