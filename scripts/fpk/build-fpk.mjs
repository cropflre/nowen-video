#!/usr/bin/env node
import { execFileSync } from 'node:child_process'
import { createHash } from 'node:crypto'
import { cpSync, chmodSync, existsSync, mkdirSync, readFileSync, readdirSync, rmSync, statSync, writeFileSync } from 'node:fs'
import { dirname, join, resolve } from 'node:path'
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

if (!/^\d+\.\d+\.\d+$/.test(VERSION)) {
  console.error(`[fpk] FPK_VERSION 必须是纯 X.Y.Z，当前: ${VERSION}`)
  process.exit(1)
}

function replaceTokens(path, replacements) {
  let content = readFileSync(path, 'utf8')
  for (const [key, value] of Object.entries(replacements)) {
    content = content.replaceAll(`{{${key}}}`, value)
  }
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

function findFnpack() {
  if (process.env.FNPACK_BIN && existsSync(process.env.FNPACK_BIN)) return resolve(process.env.FNPACK_BIN)
  const entries = readdirSync(ROOT).filter((name) => name.toLowerCase().startsWith('fnpack'))
  const platform = process.platform === 'win32' ? 'windows' : process.platform === 'darwin' ? 'darwin' : 'linux'
  const arch = process.arch === 'x64' ? 'amd64' : process.arch
  return [
    entries.find((name) => name.toLowerCase().includes(platform) && name.toLowerCase().includes(arch)),
    entries.find((name) => name.toLowerCase().includes(platform)),
    entries[0],
  ].filter(Boolean).map((name) => join(ROOT, name))[0] || null
}

function fpkFiles(dir) {
  if (!existsSync(dir)) return []
  return readdirSync(dir)
    .filter((name) => name.toLowerCase().endsWith('.fpk'))
    .map((name) => join(dir, name))
}

mkdirSync(OUT, { recursive: true })
const work = join(OUT, `nowen-video-${VERSION}-work`)
rmSync(work, { recursive: true, force: true })
mkdirSync(work, { recursive: true })
cpSync(TEMPLATE, work, { recursive: true })

replaceTokens(join(work, 'manifest'), { VERSION })
replaceTokens(join(work, 'app', 'docker', 'docker-compose.yaml'), {
  DOCKERHUB_REPO,
  IMAGE_TAG,
})

const uiImages = join(work, 'app', 'ui', 'images')
mkdirSync(uiImages, { recursive: true })
writeBrandIcon(join(work, 'ICON.PNG'), 64)
writeBrandIcon(join(work, 'ICON_256.PNG'), 256)
writeBrandIcon(join(uiImages, 'icon_64.png'), 64)
writeBrandIcon(join(uiImages, 'icon_256.png'), 256)
for (const file of readdirSync(join(work, 'cmd'))) chmodSync(join(work, 'cmd', file), 0o755)

const fnpack = findFnpack()
if (!fnpack) {
  console.error('[fpk] 找不到 fnpack。请将 fnpack-* 放在仓库根目录，或设置 FNPACK_BIN。')
  process.exit(1)
}
try { chmodSync(fnpack, 0o755) } catch { /* Windows */ }

console.log(`[fpk] version: ${VERSION}`)
console.log(`[fpk] image:   ${DOCKERHUB_REPO}:${IMAGE_TAG}`)
console.log(`[fpk] fnpack:  ${fnpack}`)
const startedAt = Date.now() - 3000
execFileSync(fnpack, ['build', '-d', work], { cwd: OUT, stdio: 'inherit' })

const candidates = [...fpkFiles(OUT), ...fpkFiles(work), ...fpkFiles(ROOT)]
  .filter((path) => statSync(path).mtimeMs >= startedAt)
  .sort((a, b) => statSync(b).mtimeMs - statSync(a).mtimeMs)
if (candidates.length === 0) throw new Error('[fpk] fnpack 返回成功，但没有找到新生成的 .fpk')

const target = join(OUT, `nowen-video-${VERSION}.fpk`)
if (resolve(candidates[0]) !== resolve(target)) cpSync(candidates[0], target)
const sha256 = createHash('sha256').update(readFileSync(target)).digest('hex')
writeFileSync(join(OUT, 'SHA256SUMS.txt'), `${sha256}  nowen-video-${VERSION}.fpk\n`)
rmSync(work, { recursive: true, force: true })

console.log(`[fpk] 完成: ${target}`)
console.log(`[fpk] SHA256: ${sha256}`)
