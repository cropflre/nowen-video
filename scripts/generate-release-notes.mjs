#!/usr/bin/env node
import { execFileSync } from 'node:child_process'
import { mkdirSync, writeFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'

const args = process.argv.slice(2)
let version = ''
let since = ''
let output = ''
let hasAndroid = true
let hasFpk = true
let hasDesktop = false

for (let i = 0; i < args.length; i += 1) {
  switch (args[i]) {
    case '-v':
    case '--version': version = String(args[++i] || '').replace(/^v/, ''); break
    case '--since': since = args[++i] || ''; break
    case '--output': output = args[++i] || ''; break
    case '--no-android': hasAndroid = false; break
    case '--no-fpk': hasFpk = false; break
    case '--desktop': hasDesktop = true; break
    default: throw new Error(`未知参数: ${args[i]}`)
  }
}

if (!/^\d+\.\d+\.\d+(?:-[0-9A-Za-z.-]+)?$/.test(version)) {
  throw new Error('请通过 --version X.Y.Z 指定合法版本号')
}

function git(argv) {
  return execFileSync('git', argv, { encoding: 'utf8', stdio: ['ignore', 'pipe', 'ignore'] }).trim()
}

function previousTag() {
  if (since) return since
  const tags = git(['tag', '--list', 'v*', '--sort=-v:refname']).split(/\r?\n/).filter(Boolean)
  return tags.find((tag) => tag !== `v${version}`) || ''
}

const groups = [
  ['feat', '✨ 新增功能'],
  ['fix', '🐛 问题修复'],
  ['perf', '⚡ 性能优化'],
  ['style', '🎨 界面与体验'],
  ['refactor', '♻️ 重构与能力升级'],
  ['docs', '📝 文档'],
  ['build', '📦 构建与发布'],
  ['ci', '🤖 CI / 发布工程'],
  ['test', '✅ 测试'],
  ['chore', '🔧 其他'],
  ['other', '📌 其他变更'],
]
const titles = Object.fromEntries(groups)
const order = Object.fromEntries(groups.map(([type], index) => [type, index]))
const conventional = /^(feat|fix|perf|style|refactor|docs|build|ci|test|chore)(?:\(([^)]+)\))?(!)?:\s*(.+)$/i

const start = previousTag()
const range = start ? `${start}..HEAD` : 'HEAD'
const log = git(['log', range, '--no-merges', '--pretty=format:%H%x1f%s%x1e'])
const commits = log
  .split('\x1e')
  .map((record) => record.trim())
  .filter(Boolean)
  .map((record) => {
    const [hash, subject] = record.split('\x1f')
    const match = conventional.exec(subject || '')
    if (!match) return { type: 'other', scope: '', subject: subject || '', hash }
    return {
      type: match[1].toLowerCase(),
      scope: (match[2] || '').trim(),
      subject: (match[4] || '').trim(),
      hash,
    }
  })
  .filter((item) => !/^release[:(]/i.test(item.subject) && !/^merge\b/i.test(item.subject))

const seen = new Set()
const deduped = commits.filter((item) => {
  const key = `${item.type}|${item.scope}|${item.subject}`
  if (seen.has(key)) return false
  seen.add(key)
  return true
})

const grouped = new Map()
for (const item of deduped) {
  if (!grouped.has(item.type)) grouped.set(item.type, [])
  grouped.get(item.type).push(item)
}

const today = new Date().toISOString().slice(0, 10)
const lines = [
  `# 🎬 Nowen Video v${version} 发布公告`,
  '',
  `发布日期：${today}`,
  `源码：\`${git(['rev-parse', '--short=12', 'HEAD'])}\`${start ? ` · 变更范围 \`${start}..HEAD\`` : ''}`,
  '',
  '本版本继续围绕媒体管理、播放体验、详情页信息架构、移动端与发布稳定性进行完善。以下内容由本次版本的真实提交自动汇总。',
  '',
]

for (const [type] of groups.sort((a, b) => order[a[0]] - order[b[0]])) {
  const items = grouped.get(type) || []
  if (items.length === 0) continue
  const limit = ['test', 'ci', 'build', 'chore', 'docs'].includes(type) ? 12 : 24
  lines.push(`## ${titles[type]}`, '')
  for (const item of items.slice(0, limit)) {
    const scope = item.scope ? `**${item.scope}**：` : ''
    lines.push(`- ${scope}${item.subject} \`${item.hash.slice(0, 7)}\``)
  }
  if (items.length > limit) lines.push(`- 其余 ${items.length - limit} 项工程变更已省略，可在 GitHub 提交记录中查看。`)
  lines.push('')
}

lines.push('## 📦 本次发布渠道', '')
lines.push(`- Docker：\`cropflre/nowen-video:v${version}\``)
if (!version.includes('-')) lines.push('- Docker stable：`cropflre/nowen-video:latest`')
if (hasAndroid) {
  lines.push(`- Android APK：\`nowen-video-android-${version}.apk\``)
  lines.push(`- Android AAB：\`nowen-video-android-${version}.aab\``)
}
if (hasFpk) lines.push(`- 飞牛 fnOS：\`nowen-video-${version}.fpk\``)
if (hasDesktop) lines.push('- Windows Desktop：GitHub Release 中对应安装包')
lines.push('', '## ⬆️ 升级说明', '')
lines.push('- Docker 用户建议固定版本标签升级，确认运行正常后再跟随 `latest`。')
if (hasFpk) lines.push('- 飞牛 fnOS 用户可在应用中心使用本版本 `.fpk` 覆盖安装，持久化数据目录不会随应用包替换。')
if (hasAndroid) lines.push('- Android 用户直接安装本版本 APK；AAB 用于应用商店或后续渠道分发。')
lines.push('- 升级前建议备份 `/data`，尤其是跨多个版本升级时。', '')
lines.push('## ✅ 发布完整性', '')
lines.push('本 Release 只有在 Server CI、Release Contract、客户端正式候选构建、Docker 远端 manifest、Git tag、GitHub Release 资产以及各渠道校验全部通过后才会被发布脚本判定为成功。', '')

const markdown = `${lines.join('\n').replace(/\n{3,}/g, '\n\n')}\n`
if (output) {
  const target = resolve(output)
  mkdirSync(dirname(target), { recursive: true })
  writeFileSync(target, markdown, 'utf8')
  console.log(`[release-notes] ${target}`)
} else {
  process.stdout.write(markdown)
}
