
# 一键启动 nowen-video 桌面端开发环境
#
# 用法:
#   pwsh desktop/scripts/dev.ps1
#
# 该脚本会:
#   1. 构建 Go sidecar（首次或强制重建时）
#   2. 启动前端 Vite dev server（后台）
#   3. 启动 Tauri dev 模式（前台）
#
# 要求:
#   - Rust / cargo ≥ 1.77
#   - Node.js ≥ 18
#   - Go ≥ 1.22

param(
    [switch]$RebuildSidecar = $false
)

$ErrorActionPreference = "Stop"

$ScriptRoot  = Split-Path -Parent $MyInvocation.MyCommand.Path
$DesktopRoot = Split-Path -Parent $ScriptRoot
$ProjectRoot = Split-Path -Parent $DesktopRoot

Write-Host "============================================" -ForegroundColor Cyan
Write-Host " nowen-video Desktop 开发环境启动"            -ForegroundColor Cyan
Write-Host "============================================" -ForegroundColor Cyan

# Step 1: 构建 Go sidecar（若不存在或强制）
$BinDir = Join-Path $DesktopRoot "bin"
$SidecarExe = Join-Path $BinDir "nowen-video.exe"

if ($RebuildSidecar -or -not (Test-Path $SidecarExe)) {
    Write-Host "`n[1/3] 构建 Go sidecar..." -ForegroundColor Yellow
    & pwsh (Join-Path $ScriptRoot "build-sidecar.ps1")
    if ($LASTEXITCODE -ne 0) {
        Write-Host "❌ sidecar 构建失败" -ForegroundColor Red
        exit 1
    }
} else {
    Write-Host "`n[1/3] ✅ sidecar 已存在，跳过构建（使用 -RebuildSidecar 强制重建）" -ForegroundColor Green
}

# Step 2: 启动前端 dev server（后台）
Write-Host "`n[2/3] 启动前端 Vite dev server（后台）..." -ForegroundColor Yellow
$WebRoot = Join-Path $ProjectRoot "web"
if (-not (Test-Path (Join-Path $WebRoot "node_modules"))) {
    Write-Host "  首次运行，安装依赖..." -ForegroundColor DarkGray
    Push-Location $WebRoot
    npm install
    Pop-Location
}

$viteJob = Start-Job -ScriptBlock {
    Set-Location $using:WebRoot
    npm run dev
}
Write-Host "  Vite 已启动 (Job ID: $($viteJob.Id))" -ForegroundColor DarkGray

# 等待 vite 就绪
Write-Host "  等待 http://localhost:3000 就绪..." -ForegroundColor DarkGray
$ready = $false
for ($i = 0; $i -lt 30; $i++) {
    Start-Sleep -Seconds 1
    try {
        $resp = Invoke-WebRequest -Uri "http://localhost:3000" -TimeoutSec 1 -UseBasicParsing -ErrorAction Stop
        if ($resp.StatusCode -eq 200) { $ready = $true; break }
    } catch { }
}
if (-not $ready) {
    Write-Host "⚠️  Vite 未能就绪，请手动检查" -ForegroundColor Yellow
} else {
    Write-Host "  ✅ Vite ready" -ForegroundColor Green
}

# Step 3: 启动 Tauri dev
Write-Host "`n[3/3] 启动 Tauri 桌面壳..." -ForegroundColor Yellow
Write-Host "  （关闭桌面应用后会自动停止 Vite）" -ForegroundColor DarkGray

try {
    Push-Location (Join-Path $DesktopRoot "src-tauri")
    & cargo tauri dev
} finally {
    Pop-Location
    Write-Host "`n清理 Vite 后台进程..." -ForegroundColor Yellow
    Stop-Job $viteJob -ErrorAction SilentlyContinue
    Remove-Job $viteJob -ErrorAction SilentlyContinue
}
