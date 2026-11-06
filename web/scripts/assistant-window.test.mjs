import assert from 'node:assert/strict'
import { test } from 'node:test'
import { build } from 'esbuild'

const bundle = await build({
  entryPoints: [new URL('../src/utils/assistantWindow.ts', import.meta.url).pathname],
  bundle: true, platform: 'node', format: 'esm', write: false,
})
const {
  ASSISTANT_LAUNCHER_HEIGHT,
  ASSISTANT_LAUNCHER_WIDTH,
  assistantPreferencesKey,
  clampAssistantLauncher,
  clampAssistantPosition,
  clampAssistantSize,
  defaultAssistantLauncherPosition,
  defaultAssistantPreferences,
  defaultAssistantPosition,
  defaultAssistantSize,
  moveAssistant,
  normalizeAssistantPreferences,
  isAssistantPlayerPath,
  readAssistantPreferences,
  resizeAssistant,
  shouldCollapseForRoute,
  writeAssistantPreferences,
} = await import(`data:text/javascript;base64,${Buffer.from(bundle.outputFiles[0].contents).toString('base64')}`)

const desktop = { width: 1280, height: 900 }
const phone = { width: 320, height: 720 }

function memoryStorage(initial = {}) {
  const data = new Map(Object.entries(initial))
  return {
    data,
    getItem: (key) => (data.has(key) ? data.get(key) : null),
    setItem: (key, value) => { data.set(key, value) },
  }
}

test('default window sits at the bottom-right and stays inside the viewport', () => {
  const preferences = defaultAssistantPreferences(desktop)
  assert.equal(preferences.visibility, 'minimized')
  assert.deepEqual(preferences.size, { width: 420, height: 620 })
  assert.deepEqual(preferences.position, { x: 848, y: 268 })
  assert.deepEqual(preferences.launcher, { x: 1216, y: 844 })
})

test('a 320px phone gets a panel narrow enough to avoid horizontal scrolling', () => {
  const size = defaultAssistantSize(phone)
  assert.equal(size.width, phone.width - 24)
  assert.deepEqual(defaultAssistantPosition(size, phone), { x: 12, y: 18 })
  // The launcher keeps clear of the bottom navigation.
  assert.equal(defaultAssistantLauncherPosition(phone).y, phone.height - 82 - ASSISTANT_LAUNCHER_HEIGHT)
})

test('a notch safe area pushes the phone panel and launcher higher', () => {
  const notch = { width: 390, height: 844, safeBottom: 34 }
  const size = defaultAssistantSize(notch)
  assert.deepEqual(defaultAssistantPosition(size, notch), { x: 12, y: 844 - 116 - size.height })
  assert.equal(defaultAssistantLauncherPosition(notch).y, 844 - 116 - ASSISTANT_LAUNCHER_HEIGHT)
  // Desktop ignores the phone bottom inset.
  assert.deepEqual(defaultAssistantPosition({ width: 420, height: 620 }, desktop), { x: 848, y: 268 })
})

test('size clamps to 320x300 and never exceeds the available viewport', () => {
  assert.deepEqual(clampAssistantSize({ width: 100, height: 100 }, desktop), { width: 320, height: 300 })
  assert.deepEqual(clampAssistantSize({ width: 5000, height: 5000 }, desktop), { width: 720, height: 876 })
  const narrow = clampAssistantSize({ width: 5000, height: 5000 }, { width: 300, height: 400 })
  assert.deepEqual(narrow, { width: 276, height: 306 })
  assert.deepEqual(clampAssistantSize({ width: Number.NaN, height: Number.NaN }, desktop), { width: 420, height: 620 })
})

test('dragging can never push the window fully off screen', () => {
  const size = { width: 420, height: 620 }
  assert.deepEqual(clampAssistantPosition({ x: 5000, y: 5000 }, size, desktop), { x: 1172, y: 792 })
  assert.deepEqual(clampAssistantPosition({ x: -5000, y: -5000 }, size, desktop), { x: -312, y: 12 })
  assert.deepEqual(clampAssistantPosition({ x: 100, y: 120 }, size, desktop), { x: 100, y: 120 })
  assert.deepEqual(moveAssistant({ x: 800, y: 300 }, { x: 10, y: -20 }, size, desktop), { x: 810, y: 280 })
  // 96px of the title bar always stays reachable.
  const pushed = clampAssistantPosition({ x: 4000, y: 4000 }, size, desktop)
  assert.equal(desktop.width - pushed.x >= 96, true)
  assert.equal(desktop.height - pushed.y >= 96, true)
})

test('resizing stops at the minimum size and the viewport edge', () => {
  assert.deepEqual(resizeAssistant({ width: 420, height: 620 }, { x: -500, y: -500 }, desktop), { width: 320, height: 300 })
  assert.deepEqual(resizeAssistant({ width: 420, height: 620 }, { x: 1000, y: 1000 }, desktop), { width: 720, height: 876 })
})

test('the launcher stays on screen and keeps a default spot', () => {
  assert.deepEqual(clampAssistantLauncher({ x: -80, y: -80 }, desktop), { x: 12, y: 12 })
  assert.deepEqual(clampAssistantLauncher({ x: 9999, y: 9999 }, desktop), { x: 1216, y: 844 })
  assert.deepEqual(clampAssistantLauncher({ x: Number.NaN, y: Number.NaN }, desktop), { x: 1216, y: 844 })
  // A measured, content-sized pill must fit inside the right edge too.
  const wide = clampAssistantLauncher({ x: 9999, y: 9999 }, desktop, { width: 132, height: 44 })
  assert.equal(wide.x, 1280 - 12 - 132)
  assert.equal(wide.y, 900 - 12 - 44)
})

test('stored state is validated field by field instead of trusted', () => {
  const normalized = normalizeAssistantPreferences({
    visibility: 'hidden',
    size: { width: 'wide', height: null },
    position: { x: 5000, y: 5000 },
    launcher: { x: -100, y: -100 },
  }, desktop)
  assert.equal(normalized.visibility, 'hidden')
  assert.deepEqual(normalized.size, { width: 420, height: 620 })
  // A restored window is pulled fully inside the viewport, not just kept reachable.
  assert.deepEqual(normalized.position, { x: 848, y: 268 })
  assert.deepEqual(normalized.launcher, { x: 12, y: 12 })

  assert.equal(normalizeAssistantPreferences({ visibility: 'bogus' }, desktop).visibility, 'minimized')
  assert.deepEqual(normalizeAssistantPreferences('not an object', desktop), defaultAssistantPreferences(desktop))
})

test('a smaller viewport pulls a saved window back into reach', () => {
  const saved = { visibility: 'expanded', size: { width: 700, height: 800 }, position: { x: 900, y: 700 }, launcher: { x: 1200, y: 800 } }
  const normalized = normalizeAssistantPreferences(saved, { width: 800, height: 600 })
  assert.equal(normalized.visibility, 'expanded')
  assert.equal(normalized.size.width <= 776, true)
  assert.equal(normalized.size.height <= 576, true)
  assert.equal(normalized.position.x + normalized.size.width <= 800, true)
  assert.equal(normalized.position.y + 96 <= 600, true)
  assert.equal(normalized.launcher.x + ASSISTANT_LAUNCHER_WIDTH <= 800, true)
})

test('only the player route collapses an expanded assistant', () => {
  assert.equal(isAssistantPlayerPath('/play/abc'), true)
  assert.equal(isAssistantPlayerPath('/play'), false)
  assert.equal(isAssistantPlayerPath('/play/a/b'), false)
  assert.equal(isAssistantPlayerPath('/player/abc'), false)
  assert.equal(shouldCollapseForRoute('/play/abc', 'expanded'), true)
  assert.equal(shouldCollapseForRoute('/play/abc', 'minimized'), false)
  assert.equal(shouldCollapseForRoute('/play/abc', 'hidden'), false)
  assert.equal(shouldCollapseForRoute('/media/abc', 'expanded'), false)
  assert.equal(shouldCollapseForRoute('/', 'expanded'), false)
})

test('preferences are per user and storage failures never break the window', () => {
  const storage = memoryStorage()
  const preferences = { ...defaultAssistantPreferences(desktop), visibility: 'expanded' }
  writeAssistantPreferences(storage, 'user-a', preferences)
  assert.deepEqual(readAssistantPreferences(storage, 'user-a', desktop), preferences)
  assert.notEqual(assistantPreferencesKey('user-a'), assistantPreferencesKey('user-b'))
  assert.deepEqual(readAssistantPreferences(storage, 'user-b', desktop), defaultAssistantPreferences(desktop))

  storage.setItem(assistantPreferencesKey('user-c'), '{ broken json')
  assert.deepEqual(readAssistantPreferences(storage, 'user-c', desktop), defaultAssistantPreferences(desktop))

  const throwing = { getItem: () => { throw new Error('blocked') }, setItem: () => { throw new Error('quota') } }
  assert.doesNotThrow(() => writeAssistantPreferences(throwing, 'user-a', preferences))
  assert.deepEqual(readAssistantPreferences(throwing, 'user-a', desktop), defaultAssistantPreferences(desktop))
  assert.deepEqual(readAssistantPreferences(null, 'user-a', desktop), defaultAssistantPreferences(desktop))
})
