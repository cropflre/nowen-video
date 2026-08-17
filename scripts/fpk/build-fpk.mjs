#!/usr/bin/env node
import { execFileSync } from 'node:child_process'
import { createHash } from 'node:crypto'
import {
  cpSync,
  chmodSync,
  existsSync,
  mkdirSync,
  readFileSync,
  readdirSync,
  rmSync,
  statSync,
  writeFileSync,
} from 'node:fs'
import { homedir } from 'node:os'
import { delimiter, dirname, join, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'
import { deflateSync } from 'node:zlib'

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

if (!/^\d+\.\d+\.\d+$/.test(VERSION)) {
  console.error(`[fpk] FPK_VERSION 必须是纯 X.Y.Z，当前: ${VERSION}`)
  process.exit(1)
}

function replaceTokens(path, replacements) {
  let content = readFileSync(path, 'utf8')
  for (const [key, value] of Object.entries(replacements)) content = content.replaceAll(`{{${key}}}`, value)
  if (/\{\{[^}]+\}\}/.test(content)) throw new Error(`[fpk] ${path} 仍有未解析模板变量`)
  writeFileSync(path, content)
}

const CRC_TABLE = Array.from({ length: 256 }, (_, n) => {
  let c = n
  for (let k = 0; k < 8; k += 1) c = (c & 1) ? (0xedb88320 ^ (c >>> 1)) : (c >>> 1)
  return c >>> 0
})
function crc32(buffer) {
  let c = 0xffffffff
  for (const byte of buffer) c = CRC_TABLE[(c ^ byte) & 0xff] ^ (c >>> 8)
  return (c ^ 0xffffffff) >>> 0
}
function pngChunk(type, data) {
  const typeBuffer = Buffer.from(type)
  const length = Buffer.alloc(4); length.writeUInt32BE(data.length)
  const checksum = Buffer.alloc(4); checksum.writeUInt32BE(crc32(Buffer.concat([typeBuffer, data])))
  return Buffer.concat([length, typeBuffer, data, checksum])
}
function writeBrandIcon(path, size) {
  const stride = 1 + size * 4
  const raw = Buffer.alloc(stride * size)
  const left = Math.floor(size * 0.25)
  const right = Math.floor(size * 0.75)
  const bar = Math.max(3, Math.floor(size * 0.09))
  for (let y = 0; y < size; y += 1) {
    raw[y * stride] = 0
    for (let x = 0; x < size; x += 1) {
      const p = y * stride + 1 + x * 4
      const diagonal = left + ((right - left) * y) / Math.max(1, size - 1)
      const isN = Math.abs(x - left) <= bar || Math.abs(x - right) <= bar || Math.abs(x - diagonal) <= bar
      raw[p] = isN ? 255 : 132
      raw[p + 1] = isN ? 255 : 111
      raw[p + 2] = isN ? 255 : 238
      raw[p + 3] = 255
    }
  }
  const ihdr = Buffer.alloc(13)
  ihdr.writeUInt32BE(size, 0); ihdr.writeUInt32BE(size, 4)
  ihdr[8] = 8; ihdr[9] = 6
  const png = Buffer.concat([
    Buffer.from([137, 80, 78, 71, 13, 10, 26, 10]),
    pngChunk('IHDR', ihdr),
    pngChunk('IDAT', deflateSync(raw, { level: 9 })),
    pngChunk('IEND', Buffer.alloc(0)),
  ])
  writeFileSync(path, png)
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

function fpkFiles(dir) {
  if (!existsSync(dir)) return []
  return readdirSync(dir).filter((name) => name.toLowerCase().endsWith('.fpk')).map((name) => join(dir, name))
}

mkdirSync(OUT, { recursive: true })
const work = join(OUT, `nowen-video-${VERSION}-work`)
rmSync(work, { recursive: true, force: true })
mkdirSync(work, { recursive: true })
cpSync(TEMPLATE, work, { recursive: true })
replaceTokens(join(work, 'manifest'), { VERSION })
replaceTokens(join(work, 'app', 'docker', 'docker-compose.yaml'), { DOCKERHUB_REPO, IMAGE_TAG })

const uiImages = join(work, 'app', 'ui', 'images')
mkdirSync(uiImages, { recursive: true })
writeBrandIcon(join(work, 'ICON.PNG'), 64)
writeBrandIcon(join(work, 'ICON_256.PNG'), 256)
writeBrandIcon(join(uiImages, 'icon_64.png'), 64)
writeBrandIcon(join(uiImages, 'icon_256.png'), 256)
for (const file of readdirSync(join(work, 'cmd'))) chmodSync(join(work, 'cmd', file), 0o755)

const fnpack = await resolveFnpack()
console.log(`[fpk] version: ${VERSION}`)
console.log(`[fpk] image:   ${DOCKERHUB_REPO}:${IMAGE_TAG}`)
console.log(`[fpk] fnpack:  ${fnpack}`)

const startedAt = Date.now() - 3000
// Official fnpack documentation uses: fnpack build --directory <path>
execFileSync(fnpack, ['build', '--directory', work], { cwd: OUT, stdio: 'inherit' })

const candidates = [...fpkFiles(OUT), ...fpkFiles(work), ...fpkFiles(ROOT)]
  .filter((path) => statSync(path).mtimeMs >= startedAt)
  .sort((a, b) => statSync(b).mtimeMs - statSync(a).mtimeMs)
if (candidates.length === 0) throw new Error('[fpk] fnpack 返回成功，但没有找到新生成的 .fpk')

const target = join(OUT, `nowen-video-${VERSION}.fpk`)
if (resolve(candidates[0]) !== resolve(target)) cpSync(candidates[0], target)
const sha256 = createHash('sha256').update(readFileSync(target)).digest('hex')
writeFileSync(join(OUT, 'SHA256SUMS-fpk.txt'), `${sha256}  nowen-video-${VERSION}.fpk\n`)
rmSync(work, { recursive: true, force: true })
console.log(`[fpk] 完成: ${target}`)
console.log(`[fpk] SHA256: ${sha256}`)
