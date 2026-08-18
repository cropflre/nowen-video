# Nowen Video Desktop 2.0 Windows 开发环境启动器。
#
# 用法：
#   pwsh desktop/scripts/dev.ps1
#   pwsh desktop/scripts/dev.ps1 -RebuildSidecar

param(
    [switch]$RebuildSidecar = $false
)

$ErrorActionPreference = "Stop"

$ScriptRoot = Split-Path -Parent $MyInvocation.MyCommand.Path
$DesktopRoot = Split-Path -Parent $ScriptRoot
$ProjectRoot = Split-Path -Parent $DesktopRoot
$TauriRoot = Join-Path $DesktopRoot "src-tauri"
$DevWebPort = 28889

function Normalize-Version([string]$Raw) {
    if ([string]::IsNullOrWhiteSpace($Raw)) { return $null }
    $value = $Raw.Trim() -replace '^refs/tags/', '' -replace '^v', ''
    if ($value -match '^[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z.-]+)?$') { return $value }
    return $null
}

function Resolve-AppVersion {
    foreach ($candidate in @($env:NOWEN_VERSION, $env:APP_VERSION, $env:GITHUB_REF_NAME)) {
        $normalized = Normalize-Version $candidate
        if ($normalized) { return $normalized }
    }
    $tag = (& git -C $ProjectRoot describe --tags --abbrev=0 --match "v[0-9]*" 2>$null)
    if ($LASTEXITCODE -eq 0) {
        $normalized = Normalize-Version $tag
        if ($normalized) { return $normalized }
    }
    return "0.1.0"
}

$AppVersion = Resolve-AppVersion
$env:NOWEN_VERSION = $AppVersion
$env:APP_VERSION = $AppVersion
$env:VITE_APP_VERSION = $AppVersion
$env:WEB_PORT = "$DevWebPort"

Write-Host "============================================" -ForegroundColor Cyan
Write-Host " Nowen Video Desktop 2.0 开发环境" -ForegroundColor Cyan
Write-Host "============================================" -ForegroundColor Cyan
Write-Host "Version: $AppVersion" -ForegroundColor DarkGray
Write-Host "Vite port: $DevWebPort" -ForegroundColor DarkGray

$BinDir = Join-Path $TauriRoot "bin"
$SidecarExe = Join-Path $BinDir "nowen-video-server.exe"

if ($RebuildSidecar -or -not (Test-Path $SidecarExe)) {
    Write-Host "`n[1/3] 构建 Go Media Core..." -ForegroundColor Yellow
    $pwshCmd = if (Get-Command pwsh -ErrorAction SilentlyContinue) { "pwsh" } else { "powershell" }
    & $pwshCmd -ExecutionPolicy Bypass -File (Join-Path $ScriptRoot "build-sidecar.ps1")
    if ($LASTEXITCODE -ne 0) { throw "Desktop Media Core 构建失败" }
} else {
    Write-Host "`n[1/3] Media Core 已存在，跳过构建（-RebuildSidecar 可强制重建）" -ForegroundColor Green
}

Write-Host "`n[2/3] 启动 Vite 开发服务器..." -ForegroundColor Yellow
$WebRoot = Join-Path $ProjectRoot "web"
if (-not (Test-Path (Join-Path $WebRoot "node_modules"))) {
    Push-Location $WebRoot
    try { npm install } finally { Pop-Location }
}

$viteJob = Start-Job -ArgumentList $WebRoot, $AppVersion, $DevWebPort -ScriptBlock {
    param($web, $version, $port)
    Set-Location $web
    $env:VITE_APP_VERSION = $version
    $env:WEB_PORT = "$port"
    npm run dev -- --port $port --strictPort
}

$DevWebUrl = "http://localhost:$DevWebPort"
$ready = $false
for ($i = 0; $i -lt 30; $i++) {
    Start-Sleep -Seconds 1
    try {
        $response = Invoke-WebRequest -Uri $DevWebUrl -TimeoutSec 1 -UseBasicParsing -ErrorAction Stop
        if ($response.StatusCode -eq 200) { $ready = $true; break }
    } catch { }
}
if (-not $ready) { Write-Host "[WARN] Vite 尚未就绪" -ForegroundColor Yellow }

Write-Host "`n[3/3] 启动 Tauri Desktop 2.0..." -ForegroundColor Yellow
try {
    Push-Location $TauriRoot
    & cargo tauri dev
} finally {
    Pop-Location
    Stop-Job $viteJob -ErrorAction SilentlyContinue
    Remove-Job $viteJob -ErrorAction SilentlyContinue
}
