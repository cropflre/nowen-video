# One-click dev launcher for nowen-video desktop
#
# Usage:
#   powershell -ExecutionPolicy Bypass -File desktop\scripts\dev.ps1
#   pwsh desktop/scripts/dev.ps1
#
# Steps:
#   1. Build Go sidecar (first time or when -RebuildSidecar)
#   2. Start Vite dev server (background job)
#   3. Start Tauri dev (foreground)
#
# Requirements: Rust >= 1.77, Node.js >= 18, Go >= 1.22

param(
    [switch]$RebuildSidecar = $false
)

$ErrorActionPreference = "Stop"

$ScriptRoot  = Split-Path -Parent $MyInvocation.MyCommand.Path
$DesktopRoot = Split-Path -Parent $ScriptRoot
$ProjectRoot = Split-Path -Parent $DesktopRoot

Write-Host "============================================" -ForegroundColor Cyan
Write-Host " nowen-video Desktop dev launcher"           -ForegroundColor Cyan
Write-Host "============================================" -ForegroundColor Cyan

# Step 1: build Go sidecar if missing or forced
$BinDir = Join-Path $DesktopRoot "bin"
$SidecarExe = Join-Path $BinDir "nowen-video.exe"

if ($RebuildSidecar -or -not (Test-Path $SidecarExe)) {
    Write-Host "`n[1/3] Building Go sidecar..." -ForegroundColor Yellow
    if (Get-Command pwsh -ErrorAction SilentlyContinue) {
        $pwshCmd = "pwsh"
    } else {
        $pwshCmd = "powershell"
    }
    & $pwshCmd -ExecutionPolicy Bypass -File (Join-Path $ScriptRoot "build-sidecar.ps1")
    if ($LASTEXITCODE -ne 0) {
        Write-Host "[ERROR] sidecar build failed" -ForegroundColor Red
        exit 1
    }
} else {
    Write-Host "`n[1/3] sidecar exists, skip build (use -RebuildSidecar to force)" -ForegroundColor Green
}

# Step 2: start Vite dev server in background
Write-Host "`n[2/3] Starting Vite dev server (background)..." -ForegroundColor Yellow
$WebRoot = Join-Path $ProjectRoot "web"
if (-not (Test-Path (Join-Path $WebRoot "node_modules"))) {
    Write-Host "  First run, installing npm deps..." -ForegroundColor DarkGray
    Push-Location $WebRoot
    npm install
    Pop-Location
}

$viteJob = Start-Job -ArgumentList $WebRoot -ScriptBlock {
    param($web)
    Set-Location $web
    npm run dev
}
Write-Host "  Vite job started (Job ID: $($viteJob.Id))" -ForegroundColor DarkGray

# wait for vite to be ready
Write-Host "  Waiting for http://localhost:3000 ..." -ForegroundColor DarkGray
$ready = $false
for ($i = 0; $i -lt 30; $i++) {
    Start-Sleep -Seconds 1
    try {
        $resp = Invoke-WebRequest -Uri "http://localhost:3000" -TimeoutSec 1 -UseBasicParsing -ErrorAction Stop
        if ($resp.StatusCode -eq 200) { $ready = $true; break }
    } catch { }
}
if (-not $ready) {
    Write-Host "[WARN] Vite not ready yet, please check manually" -ForegroundColor Yellow
} else {
    Write-Host "  Vite ready" -ForegroundColor Green
}

# Step 3: run Tauri dev
Write-Host "`n[3/3] Launching Tauri desktop shell..." -ForegroundColor Yellow
Write-Host "  (Vite job will be cleaned up when the app exits)" -ForegroundColor DarkGray

try {
    Push-Location (Join-Path $DesktopRoot "src-tauri")
    & cargo tauri dev
} finally {
    Pop-Location
    Write-Host "`nCleaning up Vite background job..." -ForegroundColor Yellow
    Stop-Job $viteJob -ErrorAction SilentlyContinue
    Remove-Job $viteJob -ErrorAction SilentlyContinue
}
