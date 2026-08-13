import { readdir, readFile } from 'node:fs/promises'
import path from 'node:path'

const root = path.resolve(process.cwd(), 'src')
const extensions = new Set(['.ts', '.tsx', '.js', '.jsx', '.css'])
const rules = [
  ['strong-blur', /\bbackdrop-blur-(?:md|lg|xl|2xl|3xl)\b/g],
  ['framer-motion', /from\s+['"]framer-motion['"]/g],
  ['surface-jsx', /<Surface\b/g],
]

async function files(dir) {
  const out = []
  for (const entry of await readdir(dir, { withFileTypes: true })) {
    const file = path.join(dir, entry.name)
    if (entry.isDirectory()) out.push(...await files(file))
    else if (entry.isFile() && extensions.has(path.extname(entry.name))) out.push(file)
  }
  return out
}

function lineAt(text, offset) {
  return text.slice(0, offset).split('\n').length
}

console.log('[ui-audit] residual report')
for (const [name, regex] of rules) {
  const hits = []
  for (const file of await files(root)) {
    const text = await readFile(file, 'utf8')
    regex.lastIndex = 0
    let match
    while ((match = regex.exec(text)) !== null) {
      hits.push(`${path.relative(root, file)}:${lineAt(text, match.index)}`)
    }
  }
  console.log(`[ui-audit] ${name}: ${hits.length}`)
  for (const hit of hits) console.log(`- src/${hit}`)
}
console.log('[ui-audit] report only')
