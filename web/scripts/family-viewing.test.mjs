import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { test } from 'node:test'
import { build } from 'esbuild'

const bundle = await build({
  entryPoints: [new URL('../src/utils/viewing.ts', import.meta.url).pathname],
  bundle: true, platform: 'node', format: 'esm', write: false,
})
const { episodeLabel, remainingMinutes, resumePosition, subtitleMatchesLanguage } = await import(`data:text/javascript;base64,${Buffer.from(bundle.outputFiles[0].contents).toString('base64')}`)

const familyBundle = await build({
  entryPoints: [new URL('../src/utils/familyRating.ts', import.meta.url).pathname],
  bundle: true, platform: 'node', format: 'esm', write: false,
})
const { FAMILY_RATING_AGE_KEY, suggestFamilyRating, mpaaToRatingLevel, isMpaaAllowedForChild, remainingMinutesFromSeconds, isLowRemaining } = await import(`data:text/javascript;base64,${Buffer.from(familyBundle.outputFiles[0].contents).toString('base64')}`)

const formatBundle = await build({
  entryPoints: [new URL('../src/utils/format.ts', import.meta.url).pathname],
  bundle: true, platform: 'node', format: 'esm', write: false,
})
const { getDisplayDurationSeconds } = await import(`data:text/javascript;base64,${Buffer.from(formatBundle.outputFiles[0].contents).toString('base64')}`)

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


test('family rating age bands keep the backend enum values', () => {
  assert.equal(FAMILY_RATING_AGE_KEY.G, 'family.ratingAgeG')
  assert.equal(FAMILY_RATING_AGE_KEY.PG, 'family.ratingAgePG')
  assert.equal(FAMILY_RATING_AGE_KEY['PG-13'], 'family.ratingAgePG13')
  assert.equal(FAMILY_RATING_AGE_KEY.R, 'family.ratingAgeR')
  assert.equal(FAMILY_RATING_AGE_KEY['NC-17'], 'family.ratingAgeNC17')
})

test('mpaa strings normalize to the family rating enum', () => {
  assert.equal(mpaaToRatingLevel('G'), 'G')
  assert.equal(mpaaToRatingLevel('PG'), 'PG')
  assert.equal(mpaaToRatingLevel('PG-13'), 'PG-13')
  assert.equal(mpaaToRatingLevel('R'), 'R')
  assert.equal(mpaaToRatingLevel('NC-17'), 'NC-17')
  assert.equal(mpaaToRatingLevel('TV-MA'), 'R')
  assert.equal(mpaaToRatingLevel('TV-Y7'), 'PG')
  assert.equal(mpaaToRatingLevel(''), '')
  assert.equal(mpaaToRatingLevel(undefined), '')
  assert.equal(mpaaToRatingLevel('Unrated'), '')
})

test('only explicit and non-conflicting source ratings become suggestions', () => {
  const source = (entries) => JSON.stringify({ results: entries.map(([country, ...ratings]) => ({ iso_3166_1: country, release_dates: ratings.map(certification => ({ certification })) })) })
  assert.equal(suggestFamilyRating('PG-13')?.level, 'PG-13')
  assert.equal(suggestFamilyRating('Rubbish'), null)
  assert.equal(suggestFamilyRating('JP-18+'), null)
  assert.equal(suggestFamilyRating('', source([['US', 'R']]))?.level, 'R')
  assert.equal(suggestFamilyRating('', source([['US', 'R', 'PG-13']])), null)
  assert.equal(suggestFamilyRating('', source([['US', 'R'], ['GB', '15']])), null)
  assert.equal(suggestFamilyRating('G', source([['US', 'R']])), null)
  assert.equal(suggestFamilyRating('G', '{bad json'), null)
  assert.equal(suggestFamilyRating('', source([['US', 'NR']])), null)
  const tv = JSON.stringify({ results: [{ iso_3166_1: 'US', rating: 'TV-MA' }] })
  assert.equal(suggestFamilyRating('', tv, true)?.level, 'NC-17')
  assert.equal(suggestFamilyRating('', tv), null)
  assert.equal(suggestFamilyRating('', JSON.stringify({ results: [{ iso_3166_1: 'US', rating: 'TV-14' }, { iso_3166_1: 'GB', rating: '15' }] }), true), null)
})

test('child suggestions only drop clearly adult mpaa labels', () => {
  assert.equal(isMpaaAllowedForChild('R'), false)
  assert.equal(isMpaaAllowedForChild('NC-17'), false)
  assert.equal(isMpaaAllowedForChild('TV-MA'), false)
  assert.equal(isMpaaAllowedForChild('PG-13'), true)
  assert.equal(isMpaaAllowedForChild('PG'), true)
  assert.equal(isMpaaAllowedForChild('G'), true)
  assert.equal(isMpaaAllowedForChild(''), true)
  assert.equal(isMpaaAllowedForChild(undefined), true)
})

test('remaining seconds convert to display minutes and low-time detection', () => {
  assert.equal(remainingMinutesFromSeconds(3600), 60)
  assert.equal(remainingMinutesFromSeconds(61), 2)
  assert.equal(remainingMinutesFromSeconds(60), 1)
  assert.equal(remainingMinutesFromSeconds(30), 1)
  assert.equal(remainingMinutesFromSeconds(0), null)
  assert.equal(remainingMinutesFromSeconds(null), null)
  assert.equal(isLowRemaining(300), true)
  assert.equal(isLowRemaining(301), false)
  assert.equal(isLowRemaining(0), false)
  assert.equal(isLowRemaining(null), false)
})

test('tertiary and secondary text meet WCAG AA 4.5:1 on canvas in both themes', () => {
  // Reads the frozen token source (ui-theme.css) so a later token regression fails here.
  const css = readFileSync(new URL('../src/styles/ui-theme.css', import.meta.url), 'utf8')
  const block = (theme) => {
    const match = css.match(new RegExp(`\\[data-theme="${theme}"\\]\\s*\\{([\\s\\S]*?)\\n\\}`))
    assert.ok(match, `${theme} theme block missing`)
    return match[1]
  }
  const token = (text, name) => {
    const match = text.match(new RegExp(`--nv-${name}:\\s*([^;]+);`))
    assert.ok(match, `--nv-${name} missing`)
    return match[1].trim()
  }
  const parseColor = (value) => {
    const rgba = value.match(/rgba?\(\s*([\d.]+)\s*,\s*([\d.]+)\s*,\s*([\d.]+)\s*(?:,\s*([\d.]+))?\s*\)/)
    if (rgba) return { rgb: rgba.slice(1, 4).map(Number), alpha: rgba[4] === undefined ? 1 : Number(rgba[4]) }
    const hex = value.match(/#([0-9a-f]{6})/i)
    assert.ok(hex, `unparsable color ${value}`)
    return { rgb: [0, 2, 4].map((i) => parseInt(hex[1].slice(i, i + 2), 16)), alpha: 1 }
  }
  const luminance = (rgb) => {
    const linear = rgb.map((c) => {
      const s = c / 255
      return s <= 0.04045 ? s / 12.92 : ((s + 0.055) / 1.055) ** 2.4
    })
    return 0.2126 * linear[0] + 0.7152 * linear[1] + 0.0722 * linear[2]
  }
  for (const theme of ['dark', 'light']) {
    const text = block(theme)
    const canvas = parseColor(token(text, 'bg-canvas')).rgb
    for (const name of ['text-tertiary', 'text-secondary']) {
      const fg = parseColor(token(text, name))
      const blended = fg.rgb.map((v, i) => fg.alpha * v + (1 - fg.alpha) * canvas[i])
      const ratio = (Math.max(luminance(blended), luminance(canvas)) + 0.05) / (Math.min(luminance(blended), luminance(canvas)) + 0.05)
      assert.ok(ratio >= 4.5, `${theme} --nv-${name} contrast is ${ratio.toFixed(2)}:1, below WCAG AA 4.5:1`)
    }
  }
})

test('subtitle preference matches language names and codes without accepting unrelated words', () => {
  for (const label of ['zho', 'chi', 'zh-CN', '中文', 'Chinese', 'movie.zh.srt']) assert.equal(subtitleMatchesLanguage(label, 'zh'), true, label)
  assert.equal(subtitleMatchesLanguage('English', 'en'), true)
  assert.equal(subtitleMatchesLanguage('jpn', 'ja'), true)
  assert.equal(subtitleMatchesLanguage('French', 'en'), false)
  assert.equal(subtitleMatchesLanguage('undefined', 'en'), false)
  assert.equal(subtitleMatchesLanguage('', 'zh'), false)
})

test('display duration prefers the probed file duration over scraped runtime', () => {
  assert.equal(getDisplayDurationSeconds(5160, 148), 5160)
  assert.equal(getDisplayDurationSeconds(0, 148), 8880)
  assert.equal(getDisplayDurationSeconds(0, 0), 0)
  assert.equal(getDisplayDurationSeconds(-1, -2), 0)
})
