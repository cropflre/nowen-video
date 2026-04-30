#Requires -Version 5.1
<#
.SYNOPSIS
  nowen-video 桌面端 C 档资源一键抓取脚本

.DESCRIPTION
  按需下载以下资源到 desktop/src-tauri/resources/（被 .gitignore 忽略）：
    - mpv/             libmpv-2.dll + 同级 DLL（Windows 嵌入模式必需）
    - yt-dlp.exe       在线流嗅探（可选，~13 MB）
    - shaders/         Anime4K v4 完整着色器（~1 MB）
    - fonts/           思源黑体 CN 子集（~10 MB，用于 ASS 字幕兜底）

.PARAMETER Skip
  跳过某些资源：mpv / yt-dlp / shaders / fonts 的任意组合（分号分隔）

.EXAMPLE
  powershell -ExecutionPolicy Bypass -File scripts/fetch-assets.ps1
  powershell -ExecutionPolicy Bypass -File scripts/fetch-assets.ps1 -Skip "fonts;yt-dlp"
#>

param(
  [string]$Skip = ""
)

$ErrorActionPreference = "Stop"
$ProgressPreference = "Continue"

$RepoRoot = Resolve-Path (Join-Path $PSScriptRoot "..")
$ResourceRoot = Join-Path $RepoRoot "desktop\src-tauri\resources"
if (-not (Test-Path $ResourceRoot)) { New-Item -ItemType Directory -Path $ResourceRoot | Out-Null }

$SkipSet = @{}
if ($Skip) {
  foreach ($s in ($Skip -split ';')) {
    if ($s.Trim()) { $SkipSet[$s.Trim().ToLowerInvariant()] = $true }
  }
}

function Write-Step($msg) { Write-Host ""; Write-Host ">>> $msg" -ForegroundColor Cyan }
function Write-Done($msg) { Write-Host "    OK - $msg" -ForegroundColor Green }
function Write-Skip($msg) { Write-Host "    ~~ skip: $msg" -ForegroundColor DarkGray }

# 统一下载（含简单重试）
function Invoke-Download($url, $dest, $desc) {
  if (Test-Path $dest) {
    Write-Host "    = $desc 已存在，跳过下载" -ForegroundColor DarkGray
    return
  }
  $dir = Split-Path $dest -Parent
  if (-not (Test-Path $dir)) { New-Item -ItemType Directory -Path $dir | Out-Null }
  for ($i = 1; $i -le 3; $i++) {
    try {
      Write-Host "    下载 $desc ... ($i/3)"
      Invoke-WebRequest -Uri $url -OutFile $dest -UseBasicParsing -TimeoutSec 600
      return
    } catch {
      Write-Warning "    第 $i 次失败: $_"
      Start-Sleep -Seconds 3
    }
  }
  throw "下载 $desc 失败: $url"
}

function Expand-7z($archive, $targetDir) {
  # 优先用 7z（若在 PATH），否则用内置 Expand-Archive 兼容 zip
  if ($archive -match '\.zip$') {
    Expand-Archive -Path $archive -DestinationPath $targetDir -Force
    return
  }
  $sevenZip = (Get-Command 7z -ErrorAction SilentlyContinue)
  if (-not $sevenZip) {
    $sevenZip = (Get-Command 7z.exe -ErrorAction SilentlyContinue)
  }
  if (-not $sevenZip) {
    throw "需要 7z.exe 来解压 .7z，请安装 7-Zip 并加入 PATH"
  }
  & $sevenZip.Source x $archive "-o$targetDir" -y | Out-Null
}

# ====== 1. libmpv ======
if (-not $SkipSet.ContainsKey('mpv')) {
  Write-Step "抓取 libmpv (Windows x64, shinchiro build)"
  $mpvDir = Join-Path $ResourceRoot "mpv"
  if (-not (Test-Path (Join-Path $mpvDir "libmpv-2.dll"))) {
    # shinchiro 维护的 mpv-dev 发布，每两周更新
    # 用 GitHub API 查最新 release 的 mpv-dev-x86_64-v3-<date>-git-<hash>.7z
    $api = "https://api.github.com/repos/shinchiro/mpv-winbuild-cmake/releases/latest"
    try {
      $rel = Invoke-RestMethod -Uri $api -Headers @{ "User-Agent" = "nowen-video" } -TimeoutSec 60
      $asset = $rel.assets | Where-Object { $_.name -like "mpv-dev-x86_64-v3-*.7z" } | Select-Object -First 1
      if (-not $asset) {
        $asset = $rel.assets | Where-Object { $_.name -like "mpv-dev-x86_64-*.7z" -and $_.name -notlike "*v3*" } | Select-Object -First 1
      }
      if (-not $asset) { throw "未找到 mpv-dev-x86_64*.7z" }

      $tmp7z = Join-Path $env:TEMP $asset.name
      Invoke-Download $asset.browser_download_url $tmp7z "libmpv $($asset.name)"

      Write-Host "    解压 libmpv ..."
      $tmpDir = Join-Path $env:TEMP ("mpv-extract-" + (Get-Random))
      New-Item -ItemType Directory -Path $tmpDir | Out-Null
      Expand-7z $tmp7z $tmpDir

      if (-not (Test-Path $mpvDir)) { New-Item -ItemType Directory -Path $mpvDir | Out-Null }
      # libmpv-dev 包里关键文件：libmpv-2.dll / libmpv.dll.a / include/mpv/*
      foreach ($pat in @("libmpv-2.dll", "libmpv.dll.a", "mpv.def")) {
        Get-ChildItem -Path $tmpDir -Filter $pat -Recurse -File -ErrorAction SilentlyContinue |
          ForEach-Object { Copy-Item $_.FullName (Join-Path $mpvDir $_.Name) -Force }
      }
      Remove-Item $tmpDir -Recurse -Force
      Remove-Item $tmp7z -Force
      Write-Done "libmpv -> $mpvDir"
    } catch {
      Write-Warning "libmpv 自动下载失败：$_"
      Write-Warning "请手动从 https://sourceforge.net/projects/mpv-player-windows/files/libmpv/ 下载 mpv-dev-x86_64-*.7z"
      Write-Warning "解压后把 libmpv-2.dll 放到 $mpvDir"
    }
  } else {
    Write-Done "libmpv 已就绪"
  }
} else { Write-Skip "libmpv" }

# ====== 2. yt-dlp ======
if (-not $SkipSet.ContainsKey('yt-dlp')) {
  Write-Step "抓取 yt-dlp"
  $ytdlp = Join-Path $ResourceRoot "yt-dlp.exe"
  Invoke-Download "https://github.com/yt-dlp/yt-dlp/releases/latest/download/yt-dlp.exe" $ytdlp "yt-dlp.exe"
  Write-Done "yt-dlp -> $ytdlp"
} else { Write-Skip "yt-dlp" }

# ====== 3. Anime4K v4 shaders ======
if (-not $SkipSet.ContainsKey('shaders')) {
  Write-Step "抓取 Anime4K v4.0.1 shaders"
  $shaderDir = Join-Path $ResourceRoot "shaders"
  if (-not (Test-Path (Join-Path $shaderDir "Anime4K_Upscale_CNN_x2_VL.glsl"))) {
    $zip = Join-Path $env:TEMP "anime4k.zip"
    Invoke-Download "https://github.com/bloc97/Anime4K/releases/download/v4.0.1/Anime4K_v4.0.zip" $zip "Anime4K v4.0"
    if (-not (Test-Path $shaderDir)) { New-Item -ItemType Directory -Path $shaderDir | Out-Null }
    $tmpDir = Join-Path $env:TEMP ("a4k-extract-" + (Get-Random))
    Expand-Archive -Path $zip -DestinationPath $tmpDir -Force
    # 只挑 GLSL 着色器
    Get-ChildItem -Path $tmpDir -Filter "*.glsl" -Recurse -File |
      ForEach-Object { Copy-Item $_.FullName (Join-Path $shaderDir $_.Name) -Force }
    Remove-Item $tmpDir -Recurse -Force
    Remove-Item $zip -Force
    Write-Done "shaders -> $shaderDir"
  } else {
    Write-Done "Anime4K 已就绪"
  }
} else { Write-Skip "Anime4K shaders" }

# ====== 4. 字幕兜底字体 ======
if (-not $SkipSet.ContainsKey('fonts')) {
  Write-Step "抓取思源黑体 CN (Light/Normal/Regular/Bold) 作为 ASS/PGS 兜底"
  $fontDir = Join-Path $ResourceRoot "fonts"
  if (-not (Test-Path $fontDir)) { New-Item -ItemType Directory -Path $fontDir | Out-Null }
  $fontsMap = @{
    "SourceHanSansCN-Normal.otf"  = "https://github.com/adobe-fonts/source-han-sans/raw/release/OTF/SimplifiedChinese/SourceHanSansCN-Normal.otf"
    "SourceHanSansCN-Regular.otf" = "https://github.com/adobe-fonts/source-han-sans/raw/release/OTF/SimplifiedChinese/SourceHanSansCN-Regular.otf"
    "SourceHanSansCN-Bold.otf"    = "https://github.com/adobe-fonts/source-han-sans/raw/release/OTF/SimplifiedChinese/SourceHanSansCN-Bold.otf"
  }
  foreach ($name in $fontsMap.Keys) {
    Invoke-Download $fontsMap[$name] (Join-Path $fontDir $name) $name
  }
  Write-Done "fonts -> $fontDir"
} else { Write-Skip "fonts" }

Write-Host ""
Write-Host "=== 资源抓取完成 ===" -ForegroundColor Green
Write-Host "资源根目录: $ResourceRoot"
