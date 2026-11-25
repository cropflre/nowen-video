#!/usr/bin/env node
import { execFileSync } from 'node:child_process'
import { createHash } from 'node:crypto'
import {
  cpSync,
  chmodSync,
  existsSync,
  mkdirSync,
  mkdtempSync,
  readFileSync,
  readdirSync,
  renameSync,
  rmSync,
  writeFileSync,
} from 'node:fs'
import { homedir } from 'node:os'
import { delimiter, dirname, join, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

const __dirname = dirname(fileURLToPath(import.meta.url))
const ROOT = resolve(__dirname, '..', '..')
const TEMPLATE = join(__dirname, 'template')
const pkg = JSON.parse(readFileSync(join(ROOT, 'package.json'), 'utf8'))
const VERSION = process.env.FPK_VERSION || pkg.version
const IMAGE_TAG = process.env.FPK_IMAGE_TAG || `v${VERSION}`
const DOCKERHUB_REPO = process.env.DOCKERHUB_REPO || 'cropflre/nowen-video'
const OUT = resolve(ROOT, process.env.FPK_OUT_DIR || 'dist-fpk')

// Keep this pinned to the version published by the official fnOS developer docs.
// Override only for emergency/manual testing when the official docs publish a newer tool.
const FNPACK_TOOL_VERSION = process.env.FNPACK_TOOL_VERSION || '1.2.3'
const FNPACK_OFFICIAL_BASE_URL = 'https://static2.fnnas.com/fnpack'

if (!/^\d+\.\d+\.\d+(?:-[0-9A-Za-z.-]+)?$/.test(VERSION)) {
  console.error(`[fpk] FPK_VERSION 必须为 X.Y.Z 或 X.Y.Z-预发布标识，当前: ${VERSION}`)
  process.exit(1)
}

function replaceTokens(path, replacements) {
  let content = readFileSync(path, 'utf8')
  for (const [key, value] of Object.entries(replacements)) content = content.replaceAll(`{{${key}}}`, value)
  if (/\{\{[^}]+\}\}/.test(content)) throw new Error(`[fpk] ${path} 仍有未解析模板变量`)
  writeFileSync(path, content)
}

function currentFnpackTarget() {
  const platform = process.platform === 'win32'
    ? 'windows'
    : process.platform === 'darwin'
      ? 'darwin'
      : process.platform === 'linux'
        ? 'linux'
        : null
  const arch = process.arch === 'x64' ? 'amd64' : process.arch === 'arm64' ? 'arm64' : null

  if (!platform || !arch || (platform === 'windows' && arch !== 'amd64')) {
    throw new Error(
      `[fpk] 飞牛官方 fnpack 文档没有提供当前平台组合: ${process.platform}/${process.arch}。` +
      '请通过 FNPACK_BIN 显式指定可用的 fnpack。',
    )
  }
  return { platform, arch }
}

function fnpackCacheDir() {
  if (process.env.FNPACK_CACHE_DIR) return resolve(process.env.FNPACK_CACHE_DIR)
  if (process.platform === 'win32') {
    return join(process.env.LOCALAPPDATA || join(homedir(), 'AppData', 'Local'), 'NowenVideo', 'fnpack')
  }
  if (process.platform === 'darwin') return join(homedir(), 'Library', 'Caches', 'nowen-video', 'fnpack')
  return join(process.env.XDG_CACHE_HOME || join(homedir(), '.cache'), 'nowen-video', 'fnpack')
}

function fnpackFileName(target) {
  const base = `fnpack-${FNPACK_TOOL_VERSION}-${target.platform}-${target.arch}`
  return process.platform === 'win32' ? `${base}.exe` : base
}

function findFnpackOnPath() {
  const executable = process.platform === 'win32' ? 'fnpack.exe' : 'fnpack'
  for (const entry of (process.env.PATH || '').split(delimiter).filter(Boolean)) {
    const candidate = join(entry.replace(/^"|"$/g, ''), executable)
    if (existsSync(candidate)) return candidate
  }
  return null
}

function findBundledFnpack(target) {
  const entries = readdirSync(ROOT).filter((name) => name.toLowerCase().startsWith('fnpack'))
  const exact = entries.find((name) => {
    const lower = name.toLowerCase()
    return lower.includes(target.platform) && lower.includes(target.arch)
  })
  return exact ? join(ROOT, exact) : null
}

function probeFnpack(path) {
  try { chmodSync(path, 0o755) } catch { /* Windows */ }
  execFileSync(path, ['--help'], { stdio: 'ignore', timeout: 15_000 })
  return path
}

async function downloadOfficialFnpack(target) {
  const cacheDir = fnpackCacheDir()
  mkdirSync(cacheDir, { recursive: true })
  const targetPath = join(cacheDir, fnpackFileName(target))
  const officialName = `fnpack-${FNPACK_TOOL_VERSION}-${target.platform}-${target.arch}`
  const url = `${FNPACK_OFFICIAL_BASE_URL}/${officialName}`

  if (existsSync(targetPath)) {
    try {
      probeFnpack(targetPath)
      console.log(`[fpk] 使用缓存的官方 fnpack: ${targetPath}`)
      return targetPath
    } catch {
      console.warn(`[fpk] 缓存 fnpack 无法执行，将重新下载: ${targetPath}`)
      rmSync(targetPath, { force: true })
    }
  }

  console.log(`[fpk] 本机未找到 fnpack，自动从飞牛官方 CDN 下载 ${officialName}`)
  console.log(`[fpk] download: ${url}`)
  let response
  try {
    response = await fetch(url, {
      redirect: 'follow',
      signal: AbortSignal.timeout(60_000),
      headers: { 'user-agent': 'nowen-video-release/fnpack-bootstrap' },
    })
  } catch (error) {
    throw new Error(`[fpk] 下载官方 fnpack 失败: ${error.message}`)
  }
  if (!response.ok) {
    throw new Error(
      `[fpk] 下载官方 fnpack 失败: HTTP ${response.status} ${response.statusText} (${url})。` +
      '可从飞牛开发者文档手动下载后设置 FNPACK_BIN。',
    )
  }

  const bytes = Buffer.from(await response.arrayBuffer())
  if (bytes.length < 1024) throw new Error(`[fpk] 官方 fnpack 下载内容异常，仅 ${bytes.length} bytes`)
  writeFileSync(targetPath, bytes, { mode: 0o755 })

  try {
    probeFnpack(targetPath)
  } catch (error) {
    rmSync(targetPath, { force: true })
    throw new Error(`[fpk] 下载后的 fnpack 无法执行: ${error.message}`)
  }

  console.log(`[fpk] 官方 fnpack 已缓存: ${targetPath}`)
  return targetPath
}

async function resolveFnpack() {
  const explicit = process.env.FNPACK_BIN
  if (explicit) {
    const path = resolve(explicit)
    if (!existsSync(path)) throw new Error(`[fpk] FNPACK_BIN 指向的文件不存在: ${path}`)
    probeFnpack(path)
    console.log(`[fpk] 使用 FNPACK_BIN: ${path}`)
    return path
  }

  const target = currentFnpackTarget()
  const pathFnpack = findFnpackOnPath()
  if (pathFnpack) {
    probeFnpack(pathFnpack)
    console.log(`[fpk] 使用 PATH 中的 fnpack: ${pathFnpack}`)
    return pathFnpack
  }

  const bundled = findBundledFnpack(target)
  if (bundled) {
    probeFnpack(bundled)
    console.log(`[fpk] 使用仓库根目录 fnpack: ${bundled}`)
    return bundled
  }

  return downloadOfficialFnpack(target)
}

mkdirSync(OUT, { recursive: true })
const fnpack = await resolveFnpack()
const staging = mkdtempSync(join(OUT, '.fnpack-'))

try {
  const work = join(staging, 'package')
  cpSync(TEMPLATE, work, { recursive: true })
  replaceTokens(join(work, 'manifest'), { VERSION })
  replaceTokens(join(work, 'app', 'docker', 'docker-compose.yaml'), { DOCKERHUB_REPO, IMAGE_TAG })

  const uiImages = join(work, 'app', 'ui', 'images')
  mkdirSync(uiImages, { recursive: true })
  cpSync(join(work, 'ICON.PNG'), join(uiImages, 'icon_64.png'))
  cpSync(join(work, 'ICON_256.PNG'), join(uiImages, 'icon_256.png'))
  for (const file of readdirSync(join(work, 'cmd'))) chmodSync(join(work, 'cmd', file), 0o755)

  console.log(`[fpk] version: ${VERSION}`)
  console.log(`[fpk] image:   ${DOCKERHUB_REPO}:${IMAGE_TAG}`)
  console.log(`[fpk] fnpack:  ${fnpack}`)
  execFileSync(fnpack, ['build', '--directory', work], { cwd: staging, stdio: 'inherit' })

  const packed = join(staging, 'nowen-video.fpk')
  const filename = `弄文影视-${VERSION}.fpk`
  const target = join(OUT, filename)
  const sha256 = createHash('sha256').update(readFileSync(packed)).digest('hex')
  renameSync(packed, target)
  writeFileSync(join(OUT, 'SHA256SUMS-fpk.txt'), `${sha256}  ${filename}\n`)
  console.log(`[fpk] 完成: ${target}`)
  console.log(`[fpk] SHA256: ${sha256}`)
} finally {
  rmSync(staging, { recursive: true, force: true })
}
