# 构建 Desktop 2.0 Go Media Core sidecar。
#
# 用法：
#   pwsh desktop/scripts/build-sidecar.ps1
#   pwsh desktop/scripts/build-sidecar.ps1 -Production
#
# Tauri externalBin 要求产物位于 src-tauri/bin，并命名为
# nowen-video-server-<target-triple>[.exe]。

param(
    [switch]$Production = $false
)

$ErrorActionPreference = "Stop"

$ScriptRoot = Split-Path -Parent $MyInvocation.MyCommand.Path
$DesktopRoot = Split-Path -Parent $ScriptRoot
$ProjectRoot = Split-Path -Parent $DesktopRoot
$TauriRoot = Join-Path $DesktopRoot "src-tauri"
$BinDir = Join-Path $TauriRoot "bin"

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

$GoArch = (go env GOARCH).Trim()
$GoOs = (go env GOOS).Trim()
$TripleMap = @{
    "windows/amd64" = "x86_64-pc-windows-msvc"
    "windows/arm64" = "aarch64-pc-windows-msvc"
    "darwin/amd64"  = "x86_64-apple-darwin"
    "darwin/arm64"  = "aarch64-apple-darwin"
    "linux/amd64"   = "x86_64-unknown-linux-gnu"
    "linux/arm64"   = "aarch64-unknown-linux-gnu"
}

$Triple = $TripleMap["$GoOs/$GoArch"]
if (-not $Triple) { throw "不支持的平台: $GoOs/$GoArch" }

$Ext = if ($GoOs -eq "windows") { ".exe" } else { "" }
$OutName = "nowen-video-server-$Triple$Ext"
$OutPath = Join-Path $BinDir $OutName
$DevCopy = Join-Path $BinDir "nowen-video-server$Ext"

New-Item -ItemType Directory -Force -Path $BinDir | Out-Null

$VersionPackage = "github.com/nowen-video/nowen-video/internal/version.Version"
$BuildArgs = @("build", "-ldflags", "-s -w -X $VersionPackage=$AppVersion", "-o", $OutPath)
if ($Production) { $BuildArgs += "-trimpath" }
$BuildArgs += "./cmd/server-lite"

Write-Host "=====================================" -ForegroundColor Cyan
Write-Host " Nowen Video Desktop 2.0 Media Core" -ForegroundColor Cyan
Write-Host "=====================================" -ForegroundColor Cyan
Write-Host "版本: $AppVersion"
Write-Host "目标: $Triple"
Write-Host "产物: $OutPath"
Write-Host "go $($BuildArgs -join ' ')" -ForegroundColor Yellow

Push-Location $ProjectRoot
try {
    & go @BuildArgs
    if ($LASTEXITCODE -ne 0) { throw "Go Media Core 编译失败，退出码 $LASTEXITCODE" }
} finally {
    Pop-Location
}

Copy-Item -Path $OutPath -Destination $DevCopy -Force

$ConfigTarget = Join-Path $BinDir "config.yaml"
$ConfigExample = Join-Path $ProjectRoot "config.example.yaml"
if ((Test-Path $ConfigExample) -and -not (Test-Path $ConfigTarget)) {
    Copy-Item -Path $ConfigExample -Destination $ConfigTarget -Force
}

$Size = [math]::Round((Get-Item $OutPath).Length / 1MB, 2)
Write-Host "[OK] Desktop Media Core 构建完成：$Size MB" -ForegroundColor Green
