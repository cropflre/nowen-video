import { readdir, readFile, stat } from 'node:fs/promises'
import path from 'node:path'
import process from 'node:process'

const rootDir = process.cwd()
const distDir = path.resolve(rootDir, 'dist')
const srcDir = path.resolve(rootDir, 'src')
const distTextExtensions = new Set(['.html', '.js', '.css', '.json', '.map', '.txt'])
const sourceTextExtensions = new Set(['.ts', '.tsx', '.js', '.jsx', '.css'])

const retiredDistPatterns = [
  { name: '中文 Pulse 标题', regex: /Pulse 数据中心/g },
  { name: '英文 Pulse 标题', regex: /Pulse (?:Data )?Center/g },
  { name: '日文 Pulse 标题', regex: /Pulse データセンター/g },
  { name: 'Pulse 导航翻译键', regex: /nav\.pulse/g },
  { name: 'Pulse 页面翻译键', regex: /(?:^|["'])pulse\.[A-Za-z0-9_]+/g },
  { name: 'Pulse 客户端路由', regex: /["']\/pulse(?:\/|["'])/g },
]

// The retirement gate validates visual dependencies, not implementation hooks.
// Stable DOM hooks such as player-controls/progress-bar may remain while the
// player business component is kept intact; their appearance must come only
// from the current --nv-player-* semantic boundary.
const retiredSourcePatterns = [
  { name: '旧 Neon/Glass utility', regex: /\b(?:text-neon(?:-blue)?|glass-panel(?:-strong)?|btn-ghost|badge-neon)\b/g },
  { name: '旧 Neon Tailwind utility', regex: /\b(?:text|bg|border|ring|shadow)-neon-[A-Za-z0-9-]+\b/g },
  { name: '旧 Neon gradient utility', regex: /\bbg-neon-gradient(?:-[hv])?\b/g },
  { name: '旧 Neon animation utility', regex: /\banimate-(?:neon-breathe|glow-pulse|energy-flow)\b/g },
  { name: '旧 Glass/shadow utility', regex: /\bshadow-(?:glass|inner-glow|neon-glow)\b/g },
  { name: '旧 Theme Tailwind utility', regex: /\b(?:text|bg|border)-theme-[A-Za-z0-9-]+\b/g },
  { name: '旧 Surface Tailwind utility', regex: /\b(?:text|bg|border)-surface-(?:50|100|200|300|400|500|600|700|800|900|950)\b/g },
  { name: '旧 Cyan Tailwind chrome', regex: /\b(?:text|bg|border|ring|shadow)-(?:cyan|sky)-(?:300|400|500|600)(?:\/[0-9]+)?\b/g },
  { name: '旧全局导航/开关 class', regex: /(?<![A-Za-z0-9_-])(?:nav-item|theme-toggle-btn|toggle-switch(?:-thumb|-lg|-sm)?|storage-input)(?![A-Za-z0-9_-])/g },
  { name: '旧全局弹层/管理 class', regex: /(?<![A-Za-z0-9_-])(?:modal-overlay|modal-panel|btn-close-ghost|admin-tab|tab-content-enter)(?![A-Za-z0-9_-])/g },
  { name: '旧 Neon CSS token', regex: /var\(--neon-[A-Za-z0-9-]+\)/g },
  { name: '旧 Glass CSS token', regex: /var\(--glass-[A-Za-z0-9-]+\)/g },
  { name: '旧背景 CSS token', regex: /var\(--bg-[A-Za-z0-9-]+\)/g },
  { name: '旧文字 CSS token', regex: /var\(--text-[A-Za-z0-9-]+\)/g },
  { name: '旧边框 CSS token', regex: /var\(--border-[A-Za-z0-9-]+\)/g },
  { name: '旧阴影 CSS token', regex: /var\(--shadow-[A-Za-z0-9-]+\)/g },
  { name: '旧 Surface CSS token', regex: /var\(--surface-(?:50|100|200|300|400|500|600|700|800|900|950)\b/g },
  { name: '旧播放器 Cyan token', regex: /--nv-player-(?:accent|focus)\s*:\s*#(?:00e5ff|00f0ff|22d3ee|06b6d4)\b/gi },
  { name: '旧播放器 Glow token', regex: /--nv-player-[A-Za-z0-9-]*glow\s*:/g },
]

async function collectFiles(directory, extensions) {
  const entries = await readdir(directory, { withFileTypes: true })
  const files = []
  for (const entry of entries) {
    const absolutePath = path.join(directory, entry.name)
    if (entry.isDirectory()) {
      files.push(...await collectFiles(absolutePath, extensions))
      continue
    }
    if (entry.isFile() && extensions.has(path.extname(entry.name).toLowerCase())) files.push(absolutePath)
  }
  return files
}

function findViolations(content, patterns, file) {
  const violations = []
  for (const pattern of patterns) {
    pattern.regex.lastIndex = 0
    const match = pattern.regex.exec(content)
    if (!match) continue
    violations.push({ file, name: pattern.name, token: match[0] })
  }
  return violations
}

async function verifyRetiredSourceCallers() {
  const violations = []
  for (const file of await collectFiles(srcDir, sourceTextExtensions)) {
    const relative = path.relative(srcDir, file)
    const content = await readFile(file, 'utf8')
    violations.push(...findViolations(content, retiredSourcePatterns, relative))
  }
  if (violations.length > 0) {
    console.error('[retired-ui] 检测到旧 Design System 的 live caller：')
    for (const violation of violations) console.error(`- src/${violation.file}: ${violation.name} (${JSON.stringify(violation.token)})`)
    process.exit(1)
  }
  console.log('[retired-ui] source contains no retired visual-system callers')
}

async function verifyRetiredProductionUI() {
  try {
    const info = await stat(distDir)
    if (!info.isDirectory()) throw new Error('dist 不是目录')
  } catch {
    console.error(`[retired-ui] 未找到生产目录：${distDir}`)
    console.error('[retired-ui] 请先执行 npm run build。')
    process.exit(1)
  }

  const violations = []
  for (const file of await collectFiles(distDir, distTextExtensions)) {
    const content = await readFile(file, 'utf8')
    violations.push(...findViolations(content, retiredDistPatterns, path.relative(distDir, file)))
  }
  if (violations.length > 0) {
    console.error('[retired-ui] 检测到已经退役的 Pulse 前端内容：')
    for (const violation of violations) console.error(`- ${violation.file}: ${violation.name} (${JSON.stringify(violation.token)})`)
    process.exit(1)
  }
  console.log('[retired-ui] production assets contain no retired Pulse UI')
}

await verifyRetiredSourceCallers()
await verifyRetiredProductionUI()
