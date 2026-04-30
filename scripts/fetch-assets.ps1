#Requires -Version 5.1
<#
.SYNOPSIS
  nowen-video 桌面端 C 档资源一键抓取脚本（v2，单一资源失败不影响其余）

.DESCRIPTION
  按需下载：
    - mpv/             libmpv-2.dll（Windows 嵌入模式必需，~40 MB）
    - yt-dlp.exe       在线流嗅探（可选，~13 MB）
    - shaders/         Anime4K v4 完整 GLSL 着色器（~1 MB）
    - fonts/           思源黑体 CN（~30 MB，用于 ASS 字幕兜底）

.PARAMETER Skip
  跳过某些资源：mpv / yt-dlp / shaders / fonts 的任意组合（分号分隔）

.EXAMPLE
  powershell -ExecutionPolicy Bypass -File scripts/fetch-assets.ps1
  powershell -ExecutionPolicy Bypass -File scripts/fetch-assets.ps1 -Skip "fonts;yt-dlp"
#>

param(
  [string]$Skip = ""
)

# 关键：不能用 "Stop"，否则任何资源段失败都会整脚本退出
$ErrorActionPreference = "Continue"
$ProgressPreference = "SilentlyContinue"

# 强制 UTF-8 输出，解决中文乱码
try {
  [Console]::OutputEncoding = New-Object System.Text.UTF8Encoding
  $OutputEncoding = New-Object System.Text.UTF8Encoding
} catch { }

# TLS 1.2（部分 GitHub/SF 需要）
[System.Net.ServicePointManager]::SecurityProtocol = [System.Net.SecurityProtocolType]::Tls12 -bor [System.Net.SecurityProtocolType]::Tls13

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
function Write-Done($msg) { Write-Host "    [OK] $msg" -ForegroundColor Green }
function Write-Skip($msg) { Write-Host "    [skip] $msg" -ForegroundColor DarkGray }
function Write-Warn2($msg) { Write-Host "    [warn] $msg" -ForegroundColor Yellow }

# 下载单个文件（按顺序尝试多个镜像）
function Invoke-SmartDownload {
  param(
    [string[]]$Urls,
    [string]$Dest,
    [string]$Desc,
    [int]$MinSizeBytes = 0
  )
  if (Test-Path $Dest) {
    $existSize = (Get-Item $Dest).Length
    if ($MinSizeBytes -le 0 -or $existSize -ge $MinSizeBytes) {
      Write-Host "    = $Desc 已存在 ($([math]::Round($existSize/1MB,2)) MB)，跳过下载" -ForegroundColor DarkGray
      return $true
    } else {
      Write-Warn2 "$Desc 已存在但尺寸异常（$existSize < $MinSizeBytes），删除并重下"
      Remove-Item $Dest -Force -ErrorAction SilentlyContinue
    }
  }
  $dir = Split-Path $Dest -Parent
  if (-not (Test-Path $dir)) { New-Item -ItemType Directory -Path $dir -Force | Out-Null }

  foreach ($url in $Urls) {
    for ($i = 1; $i -le 2; $i++) {
      Write-Host "    下载 $Desc ... ($i/2)  $url"
      try {
        $tmp = "$Dest.part"
        if (Test-Path $tmp) { Remove-Item $tmp -Force }
        Invoke-WebRequest -Uri $url -OutFile $tmp -UseBasicParsing -TimeoutSec 600 -ErrorAction Stop -UserAgent "nowen-video-fetch/1.0"
        $size = (Get-Item $tmp).Length
        if ($MinSizeBytes -gt 0 -and $size -lt $MinSizeBytes) {
          Write-Warn2 "下载结果过小 ($size < $MinSizeBytes)，可能是中转 HTML 页"
          Remove-Item $tmp -Force -ErrorAction SilentlyContinue
          continue
        }
        Move-Item -Path $tmp -Destination $Dest -Force
        Write-Host "    下载完成: $([math]::Round($size/1MB,2)) MB" -ForegroundColor DarkGray
        return $true
      } catch {
        Write-Warn2 "第 $i 次失败: $($_.Exception.Message)"
        Start-Sleep -Seconds 2
      }
    }
  }
  Write-Warn2 "所有镜像均失败：$Desc"
  return $false
}

function Expand-Archive7z {
  param([string]$Archive, [string]$TargetDir)
  if ($Archive -match '\.zip$') {
    Expand-Archive -Path $Archive -DestinationPath $TargetDir -Force
    return $true
  }
  # 查找 7z.exe
  $sevenZip = (Get-Command 7z -ErrorAction SilentlyContinue)
  if (-not $sevenZip) { $sevenZip = (Get-Command 7z.exe -ErrorAction SilentlyContinue) }
  $sevenZipPath = $null
  if ($sevenZip) { $sevenZipPath = $sevenZip.Source }
  if (-not $sevenZipPath) {
    $candidates = @(
      "$env:ProgramFiles\7-Zip\7z.exe",
      "${env:ProgramFiles(x86)}\7-Zip\7z.exe",
      (Join-Path $PSScriptRoot "7zr.exe"),
      (Join-Path $env:TEMP "7zr.exe")
    )
    foreach ($c in $candidates) {
      if ($c -and (Test-Path $c)) { $sevenZipPath = $c; break }
    }
  }
  # 自动下载 7zr.exe（只能解压 .7z，约 600 KB 单文件便携）
  if (-not $sevenZipPath) {
    Write-Host "    未检测到 7z.exe，尝试自动下载 7zr.exe 便携版..."
    $sevenZipPath = Join-Path $env:TEMP "7zr.exe"
    $ok = Invoke-SmartDownload `
      -Urls @("https://www.7-zip.org/a/7zr.exe") `
      -Dest $sevenZipPath `
      -Desc "7zr.exe"
    if (-not $ok) {
      Write-Warn2 "7zr.exe 下载失败，请手动安装 7-Zip：https://www.7-zip.org/"
      return $false
    }
  }
  & $sevenZipPath x $Archive "-o$TargetDir" -y | Out-Null
  return ($LASTEXITCODE -eq 0)
}

# ====== 1. libmpv ======
function Fetch-Libmpv {
  Write-Step "抓取 libmpv (Windows x64)"
  $mpvDir = Join-Path $ResourceRoot "mpv"
  if (Test-Path (Join-Path $mpvDir "libmpv-2.dll")) {
    Write-Done "libmpv 已就绪"
    return
  }
  if (-not (Test-Path $mpvDir)) { New-Item -ItemType Directory -Path $mpvDir -Force | Out-Null }

  # 候选渠道：① SourceForge RSS 查文件名，GitHub 直链下载 ② GitHub API（有时 403 限频）
  #
  # SF 的 /download 链接会返回 HTML 中转页，不能直接用来下载；
  # 幸运的是 shinchiro 把同名文件同时发到 GitHub Releases，tag = 日期前缀。
  # 所以我们优先"用 SF RSS 拿最新文件名 -> 构造 GitHub 直链"。
  $asset7z = $null
  $downloadUrls = @()

  # ① SourceForge RSS → 解析 tag 和文件名
  try {
    $rssUri = "https://sourceforge.net/projects/mpv-player-windows/rss?path=/libmpv"
    $rss = Invoke-WebRequest -Uri $rssUri -UseBasicParsing -TimeoutSec 30 -ErrorAction Stop
    $xml = [xml]$rss.Content
    $item = $xml.rss.channel.item |
      Where-Object { $_.title.InnerText -match "mpv-dev-x86_64-v3-(\d{8})-git-[0-9a-f]+\.7z$" } |
      Select-Object -First 1
    if ($item) {
      $asset7z = Split-Path -Leaf $item.title.InnerText
      if ($asset7z -match "mpv-dev-x86_64-v3-(\d{8})-git-[0-9a-f]+\.7z$") {
        $tag = $Matches[1]
        # 首选：GitHub Releases 直链
        $downloadUrls += "https://github.com/shinchiro/mpv-winbuild-cmake/releases/download/$tag/$asset7z"
      }
      # 次选：SF 的 master.dl 真实 mirror（跳过中转 HTML）
      $downloadUrls += "https://master.dl.sourceforge.net/project/mpv-player-windows/libmpv/$asset7z"
    }
  } catch {
    Write-Warn2 "SourceForge RSS 查询失败：$($_.Exception.Message)"
  }

  # ② GitHub API（辅助，无 token 时每小时 60 次限频）
  if (-not $asset7z) {
    try {
      $api = "https://api.github.com/repos/shinchiro/mpv-winbuild-cmake/releases/latest"
      $rel = Invoke-RestMethod -Uri $api -Headers @{ "User-Agent" = "nowen-video-fetch" } -TimeoutSec 30 -ErrorAction Stop
      $asset = $rel.assets | Where-Object { $_.name -like "mpv-dev-x86_64-v3-*.7z" } | Select-Object -First 1
      if (-not $asset) {
        $asset = $rel.assets | Where-Object { $_.name -like "mpv-dev-x86_64-*.7z" } | Select-Object -First 1
      }
      if ($asset) {
        $asset7z = $asset.name
        $downloadUrls += $asset.browser_download_url
      }
    } catch {
      Write-Warn2 "GitHub API 查询失败：$($_.Exception.Message)"
    }
  }

  if (-not $asset7z) {
    Write-Warn2 "所有渠道均无法获取 libmpv 最新版本信息"
    Write-Warn2 "请手动从 https://sourceforge.net/projects/mpv-player-windows/files/libmpv/ 下载"
    return
  }

  $tmp7z = Join-Path $env:TEMP $asset7z
  $ok = Invoke-SmartDownload -Urls $downloadUrls -Dest $tmp7z -Desc "libmpv-dev $asset7z" -MinSizeBytes 10000000
  if (-not $ok) {
    Write-Warn2 "libmpv 自动下载失败，请手动下载："
    Write-Warn2 "  https://sourceforge.net/projects/mpv-player-windows/files/libmpv/"
    Write-Warn2 "解压后把 libmpv-2.dll 放到 $mpvDir"
    return
  }

  Write-Host "    解压 libmpv ..."
  $tmpDir = Join-Path $env:TEMP ("mpv-extract-" + (Get-Random))
  New-Item -ItemType Directory -Path $tmpDir -Force | Out-Null
  $extOk = Expand-Archive7z -Archive $tmp7z -TargetDir $tmpDir
  if (-not $extOk) {
    Write-Warn2 "libmpv 解压失败；如未安装 7-Zip 请先安装"
    return
  }

  foreach ($pat in @("libmpv-2.dll", "libmpv.dll.a", "mpv.def", "libmpv-2.dll.lib")) {
    Get-ChildItem -Path $tmpDir -Filter $pat -Recurse -File -ErrorAction SilentlyContinue |
      ForEach-Object { Copy-Item $_.FullName (Join-Path $mpvDir $_.Name) -Force }
  }

  # mpv include 头（给 rust-bindgen 用，虽然 libmpv2 crate 自带 binding 不需要）
  $includeSrc = Join-Path $tmpDir "include"
  if (Test-Path $includeSrc) {
    $includeDst = Join-Path $mpvDir "include"
    if (Test-Path $includeDst) { Remove-Item $includeDst -Recurse -Force }
    Copy-Item $includeSrc $includeDst -Recurse -Force
  }

  Remove-Item $tmpDir -Recurse -Force -ErrorAction SilentlyContinue
  Remove-Item $tmp7z -Force -ErrorAction SilentlyContinue

  if (Test-Path (Join-Path $mpvDir "libmpv-2.dll")) {
    Write-Done "libmpv -> $mpvDir"
  } else {
    Write-Warn2 "libmpv 解压完成但找不到 libmpv-2.dll，请检查归档"
  }
}

# ====== 2. yt-dlp ======
function Fetch-YtDlp {
  Write-Step "抓取 yt-dlp"
  $ytdlp = Join-Path $ResourceRoot "yt-dlp.exe"
  $ok = Invoke-SmartDownload `
    -Urls @("https://github.com/yt-dlp/yt-dlp/releases/latest/download/yt-dlp.exe") `
    -Dest $ytdlp `
    -Desc "yt-dlp.exe"
  if ($ok) { Write-Done "yt-dlp -> $ytdlp" }
}

# ====== 3. Anime4K v4 shaders ======
function Fetch-Anime4K {
  Write-Step "抓取 Anime4K v4.0.1 shaders"
  $shaderDir = Join-Path $ResourceRoot "shaders"
  if (-not (Test-Path $shaderDir)) { New-Item -ItemType Directory -Path $shaderDir -Force | Out-Null }
  if (Test-Path (Join-Path $shaderDir "Anime4K_Upscale_CNN_x2_VL.glsl")) {
    Write-Done "Anime4K 已就绪"
    return
  }
  $zip = Join-Path $env:TEMP "anime4k-v4.zip"
  $ok = Invoke-SmartDownload `
    -Urls @("https://github.com/bloc97/Anime4K/releases/download/v4.0.1/Anime4K_v4.0.zip") `
    -Dest $zip `
    -Desc "Anime4K v4.0.zip"
  if (-not $ok) { return }

  $tmpDir = Join-Path $env:TEMP ("a4k-extract-" + (Get-Random))
  try {
    Expand-Archive -Path $zip -DestinationPath $tmpDir -Force
    Get-ChildItem -Path $tmpDir -Filter "*.glsl" -Recurse -File |
      ForEach-Object { Copy-Item $_.FullName (Join-Path $shaderDir $_.Name) -Force }
    Write-Done "shaders -> $shaderDir"
  } catch {
    Write-Warn2 "Anime4K 解压失败：$($_.Exception.Message)"
  } finally {
    Remove-Item $tmpDir -Recurse -Force -ErrorAction SilentlyContinue
    Remove-Item $zip -Force -ErrorAction SilentlyContinue
  }
}

# ====== 4. 字幕兜底字体 ======
function Fetch-Fonts {
  Write-Step "抓取思源黑体 CN（Normal/Regular/Bold）"
  $fontDir = Join-Path $ResourceRoot "fonts"
  if (-not (Test-Path $fontDir)) { New-Item -ItemType Directory -Path $fontDir -Force | Out-Null }
  $fontsMap = @{
    "SourceHanSansCN-Normal.otf"  = "https://github.com/adobe-fonts/source-han-sans/raw/release/OTF/SimplifiedChinese/SourceHanSansCN-Normal.otf"
    "SourceHanSansCN-Regular.otf" = "https://github.com/adobe-fonts/source-han-sans/raw/release/OTF/SimplifiedChinese/SourceHanSansCN-Regular.otf"
    "SourceHanSansCN-Bold.otf"    = "https://github.com/adobe-fonts/source-han-sans/raw/release/OTF/SimplifiedChinese/SourceHanSansCN-Bold.otf"
  }
  foreach ($name in $fontsMap.Keys) {
    Invoke-SmartDownload -Urls @($fontsMap[$name]) -Dest (Join-Path $fontDir $name) -Desc $name | Out-Null
  }
  Write-Done "fonts -> $fontDir"
}

# ====== 调度 ======
if (-not $SkipSet.ContainsKey('mpv'))     { Fetch-Libmpv }  else { Write-Skip "libmpv" }
if (-not $SkipSet.ContainsKey('yt-dlp'))  { Fetch-YtDlp }   else { Write-Skip "yt-dlp" }
if (-not $SkipSet.ContainsKey('shaders')) { Fetch-Anime4K } else { Write-Skip "shaders" }
if (-not $SkipSet.ContainsKey('fonts'))   { Fetch-Fonts }   else { Write-Skip "fonts" }

Write-Host ""
Write-Host "=== 资源抓取完成 ===" -ForegroundColor Green
Write-Host "资源根目录: $ResourceRoot"

# 汇报缺失项
$missing = @()
if (-not (Test-Path (Join-Path $ResourceRoot "mpv\libmpv-2.dll")) -and -not $SkipSet.ContainsKey('mpv')) { $missing += "libmpv-2.dll" }
if (-not (Test-Path (Join-Path $ResourceRoot "yt-dlp.exe")) -and -not $SkipSet.ContainsKey('yt-dlp'))     { $missing += "yt-dlp.exe" }
if (-not (Test-Path (Join-Path $ResourceRoot "shaders\Anime4K_Upscale_CNN_x2_VL.glsl")) -and -not $SkipSet.ContainsKey('shaders')) { $missing += "Anime4K shaders" }

if ($missing.Count -gt 0) {
  Write-Host ""
  Write-Host "! 以下资源未获取，相关功能将自动降级：" -ForegroundColor Yellow
  $missing | ForEach-Object { Write-Host "  - $_" -ForegroundColor Yellow }
  exit 1
}
exit 0
