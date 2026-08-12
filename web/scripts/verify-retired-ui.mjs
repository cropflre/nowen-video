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

// index.css 是最后一份待物理清理的历史全局定义文件。
// 这里先锁死其它 live 源码的 caller，确保旧视觉不会继续扩散；
// index.css 清空后即可移除这个唯一豁免。
const sourceAllowlist = new Set(['index.css'])
const retiredSourcePatterns = [
  { name: '旧 Neon/Glass utility', regex: /\b(?:text-neon(?:-blue)?|glass-panel(?:-strong)?|btn-ghost|badge-neon)\b/g },
  { name: '旧 Neon CSS token', regex: /var\(--neon-[A-Za-z0-9-]+\)/g },
  { name: '旧 Glass CSS token', regex: /var\(--glass-[A-Za-z0-9-]+\)/g },
  { name: '旧背景 CSS token', regex: /var\(--bg-[A-Za-z0-9-]+\)/g },
  { name: '旧文字 CSS token', regex: /var\(--text-[A-Za-z0-9-]+\)/g },
  { name: '旧边框 CSS token', regex: /var\(--border-[A-Za-z0-9-]+\)/g },
  { name: '旧阴影 CSS token', regex: /var\(--shadow-[A-Za-z0-9-]+\)/g },
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
    if (entry.isFile() && extensions.has(path.extname(entry.name).toLowerCase())) {
      files.push(absolutePath)
    }
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
    if (sourceAllowlist.has(relative)) continue
    const content = await readFile(file, 'utf8')
    violations.push(...findViolations(content, retiredSourcePatterns, relative))
  }

  if (violations.length > 0) {
    console.error('[retired-ui] 检测到旧 Design System 的 live caller：')
    for (const violation of violations) {
      console.error(`- src/${violation.file}: ${violation.name} (${JSON.stringify(violation.token)})`)
    }
    process.exit(1)
  }

  console.log('[retired-ui] source contains no legacy Design System callers outside index.css')
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
    for (const violation of violations) {
      console.error(`- ${violation.file}: ${violation.name} (${JSON.stringify(violation.token)})`)
    }
    process.exit(1)
  }

  console.log('[retired-ui] production assets contain no retired Pulse UI')
}

await verifyRetiredSourceCallers()
await verifyRetiredProductionUI()
