import assert from 'node:assert/strict'
import { test } from 'node:test'
import { build } from 'esbuild'

const bundle = await build({
  entryPoints: [new URL('../src/utils/viewing.ts', import.meta.url).pathname],
  bundle: true, platform: 'node', format: 'esm', write: false,
})
const { episodeLabel, remainingMinutes, resumePosition, subtitleMatchesLanguage } = await import(`data:text/javascript;base64,${Buffer.from(bundle.outputFiles[0].contents).toString('base64')}`)

test('saved progress resumes unfinished media, but completed media restarts', () => {
  assert.equal(resumePosition({ position: 450, duration: 1200, completed: false }), 450)
  assert.equal(resumePosition({ position: 450, duration: 1200, completed: true }), 0)
  assert.equal(resumePosition({ position: 1080, duration: 1200 }), 0)
  assert.equal(resumePosition({ position: 1500, duration: 1200 }), 0)
  assert.equal(resumePosition(null), 0)
})

test('missing duration keeps valid progress without inventing a remaining time', () => {
  assert.equal(resumePosition({ position: 42, duration: 0 }), 42)
  assert.equal(remainingMinutes(42, 0), null)
  assert.equal(remainingMinutes(42, Infinity), null)
  assert.equal(remainingMinutes(NaN, 120), null)
})

test('invalid progress cannot seek outside a valid timeline', () => {
  for (const position of [-5, NaN, Infinity, -Infinity]) {
    assert.equal(resumePosition({ position, duration: 1200 }), 0)
  }
  assert.equal(remainingMinutes(1201, 1200), 0)
  assert.equal(remainingMinutes(-5, 120), 2)
  assert.equal(remainingMinutes(61, 120), 1)
})

test('episode labels distinguish specials from regular seasons in supported locales', () => {
  assert.equal(episodeLabel(1, 2), '第 1 季 · 第 2 集')
  assert.equal(episodeLabel(0, 2), '特别篇 · 第 2 集')
  assert.equal(episodeLabel(2, 5, 'en-US'), 'Season 2 · Episode 5')
  assert.equal(episodeLabel(0, 1, 'ja-JP'), '特別編 · 第 1 話')
  assert.equal(episodeLabel(NaN, NaN), '')
})


test('subtitle preference matches language names and codes without accepting unrelated words', () => {
  for (const label of ['zho', 'chi', 'zh-CN', '中文', 'Chinese', 'movie.zh.srt']) assert.equal(subtitleMatchesLanguage(label, 'zh'), true, label)
  assert.equal(subtitleMatchesLanguage('English', 'en'), true)
  assert.equal(subtitleMatchesLanguage('jpn', 'ja'), true)
  assert.equal(subtitleMatchesLanguage('French', 'en'), false)
  assert.equal(subtitleMatchesLanguage('undefined', 'en'), false)
  assert.equal(subtitleMatchesLanguage('', 'zh'), false)
})
