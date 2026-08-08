import { readdir, readFile, stat } from 'node:fs/promises'
import path from 'node:path'
import process from 'node:process'

const distDir = path.resolve(process.cwd(), 'dist')
const textExtensions = new Set(['.html', '.js', '.css', '.json', '.map', '.txt'])
const retiredPatterns = [
  { name: '中文 Pulse 标题', regex: /Pulse 数据中心/g },
  { name: '英文 Pulse 标题', regex: /Pulse (?:Data )?Center/g },
  { name: '日文 Pulse 标题', regex: /Pulse データセンター/g },
  { name: 'Pulse 导航翻译键', regex: /nav\.pulse/g },
  { name: 'Pulse 页面翻译键', regex: /(?:^|["'])pulse\.[A-Za-z0-9_]+/g },
  { name: 'Pulse 客户端路由', regex: /["']\/pulse(?:\/|["'])/g },
]

async function collectFiles(directory) {
  const entries = await readdir(directory, { withFileTypes: true })
  const files = []

  for (const entry of entries) {
    const absolutePath = path.join(directory, entry.name)
    if (entry.isDirectory()) {
      files.push(...await collectFiles(absolutePath))
      continue
    }
    if (entry.isFile() && textExtensions.has(path.extname(entry.name).toLowerCase())) {
      files.push(absolutePath)
    }
  }

  return files
}

async function main() {
  try {
    const info = await stat(distDir)
    if (!info.isDirectory()) throw new Error('dist 不是目录')
  } catch {
    console.error(`[retired-ui] 未找到生产目录：${distDir}`)
    console.error('[retired-ui] 请先执行 npm run build。')
    process.exit(1)
  }

  const violations = []
  for (const file of await collectFiles(distDir)) {
    const content = await readFile(file, 'utf8')
    for (const pattern of retiredPatterns) {
      pattern.regex.lastIndex = 0
      const match = pattern.regex.exec(content)
      if (!match) continue
      violations.push({
        file: path.relative(distDir, file),
        name: pattern.name,
        token: match[0],
      })
    }
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

await main()
