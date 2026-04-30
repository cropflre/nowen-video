
# 构建 Go sidecar 二进制供 Tauri 桌面端使用
#
# 用法:
#   pwsh desktop/scripts/build-sidecar.ps1             # dev 构建（当前平台）
#   pwsh desktop/scripts/build-sidecar.ps1 -Production # release 构建（当前平台）
#
# 产物：desktop/bin/nowen-video-<target>.exe

param(
    [switch]$Production = $false
)

$ErrorActionPreference = "Stop"

# 定位项目根目录（脚本父父目录）
$ScriptRoot = Split-Path -Parent $MyInvocation.MyCommand.Path
$DesktopRoot = Split-Path -Parent $ScriptRoot
$ProjectRoot = Split-Path -Parent $DesktopRoot

Write-Host "=====================================" -ForegroundColor Cyan
Write-Host " 构建 nowen-video Go sidecar"          -ForegroundColor Cyan
Write-Host "=====================================" -ForegroundColor Cyan
Write-Host "项目根: $ProjectRoot"
Write-Host "产物目录: $DesktopRoot\bin"
Write-Host ""

# 确保 bin 目录存在
$BinDir = Join-Path $DesktopRoot "bin"
New-Item -ItemType Directory -Force -Path $BinDir | Out-Null

# 探测 Go 架构/平台，用于命名（Tauri externalBin 要求 <name>-<target-triple>）
$GoArch = (go env GOARCH).Trim()
$GoOs   = (go env GOOS).Trim()

$TripleMap = @{
    "windows/amd64" = "x86_64-pc-windows-msvc"
    "windows/arm64" = "aarch64-pc-windows-msvc"
    "darwin/amd64"  = "x86_64-apple-darwin"
    "darwin/arm64"  = "aarch64-apple-darwin"
    "linux/amd64"   = "x86_64-unknown-linux-gnu"
    "linux/arm64"   = "aarch64-unknown-linux-gnu"
}

$Key = "$GoOs/$GoArch"
$Triple = $TripleMap[$Key]
if (-not $Triple) {
    Write-Host "[ERROR] Unknown platform: $Key" -ForegroundColor Red
    exit 1
}

$Ext = ""
if ($GoOs -eq "windows") { $Ext = ".exe" }
$OutName = "nowen-video-$Triple$Ext"
$OutPath = Join-Path $BinDir $OutName

# 构建参数
$BuildArgs = @("build", "-o", $OutPath)
if ($Production) {
    $BuildArgs += @("-ldflags", "-s -w -X main.Version=desktop-$(Get-Date -Format yyyyMMdd)")
    $BuildArgs += @("-trimpath")
}
$BuildArgs += "./cmd/server"

Write-Host "go $($BuildArgs -join ' ')" -ForegroundColor Yellow

Push-Location $ProjectRoot
try {
    & go @BuildArgs
    if ($LASTEXITCODE -ne 0) {
        throw "Go build failed (exit code $LASTEXITCODE)"
    }
} finally {
    Pop-Location
}

# 同时复制一份不带 triple 后缀的版本（dev 模式方便使用）
$DevCopy = Join-Path $BinDir "nowen-video$Ext"
Copy-Item -Path $OutPath -Destination $DevCopy -Force

# 复制 config.example.yaml 作为默认配置（若 bin 下不存在 config.yaml）
$ConfigTarget = Join-Path $BinDir "config.yaml"
$ConfigExample = Join-Path $ProjectRoot "config.example.yaml"
if ((Test-Path $ConfigExample) -and -not (Test-Path $ConfigTarget)) {
    Copy-Item -Path $ConfigExample -Destination $ConfigTarget -Force
    Write-Host "  Copied default config: $ConfigTarget" -ForegroundColor DarkGray
}

Write-Host ""
Write-Host "[OK] Build complete" -ForegroundColor Green
Write-Host "  $OutPath"
Write-Host "  $DevCopy"
$size = [math]::Round((Get-Item $OutPath).Length / 1MB, 2)
Write-Host "  Size: $size MB"
