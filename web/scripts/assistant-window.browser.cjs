/* Playwright check for the non-modal 问问助手 window.
 *
 * Needs an isolated backend + Vite frontend, described by a manifest:
 *   ASSISTANT_WINDOW_MANIFEST=/abs/path/manifest.json node scripts/assistant-window.browser.cjs
 * manifest: { "isolated": true, "backend", "frontend", "db", "password" }
 */
const { chromium } = require('playwright')
const assert = require('node:assert/strict')
const fs = require('node:fs')
const path = require('node:path')

if (!process.env.ASSISTANT_WINDOW_MANIFEST) throw new Error('ASSISTANT_WINDOW_MANIFEST is required')
const manifestPath = path.resolve(process.env.ASSISTANT_WINDOW_MANIFEST)
const input = JSON.parse(fs.readFileSync(manifestPath, 'utf8'))
const output = path.dirname(manifestPath)
if (!input.isolated || !path.resolve(input.db).startsWith(output + path.sep)) throw new Error('An isolated database inside the manifest directory is required')
for (const endpoint of [input.backend, input.frontend]) {
  const url = new URL(endpoint)
  if (url.hostname !== '127.0.0.1' || ['28888', '28889'].includes(url.port)) throw new Error('Refusing non-isolated endpoints')
}

const checks = []
const pageErrors = []
const PASSWORD = input.password
const OWNER = 'window-owner'
const OTHER = 'window-other'
const USER_KEY = `nowen-assistant-window:v1:${OWNER}`
const CHAT = '帮我找一部喜剧'

let browser
const redact = value => String(value).replaceAll(PASSWORD, '[REDACTED]')

async function login(context, username) {
  const page = await context.newPage()
  page.on('pageerror', error => pageErrors.push(redact(error.message)))
  page.setDefaultTimeout(15000)
  await page.goto(input.frontend + '/login')
  await page.getByRole('textbox', { name: '用户名', exact: true }).fill(username)
  await page.getByLabel('密码', { exact: true }).fill(PASSWORD)
  await page.getByRole('button', { name: '进入深空', exact: true }).click()
  await page.waitForURL(input.frontend + '/')
  return page
}

const launcher = page => page.getByRole('button', { name: '打开观影助手', exact: true })
const dialog = page => page.getByRole('dialog', { name: '观影助手', exact: true })
const log = page => page.getByRole('log', { name: '观影对话', exact: true })

async function openAssistant(page) {
  await launcher(page).click()
  await dialog(page).waitFor({ state: 'visible' })
}

async function box(page) {
  const value = await dialog(page).boundingBox()
  assert(value, 'assistant window has no layout box')
  return value
}

async function dragFrom(page, x, y, dx, dy, steps = 12) {
  await page.mouse.move(x, y)
  await page.mouse.down()
  for (let step = 1; step <= steps; step += 1) await page.mouse.move(x + (dx * step) / steps, y + (dy * step) / steps)
  await page.mouse.up()
  // Let the committed position re-render before measuring.
  await page.waitForTimeout(150)
}

async function dragWindow(page, dx, dy) {
  const start = await box(page)
  await dragFrom(page, start.x + Math.min(120, start.width / 3), start.y + 24, dx, dy)
  return box(page)
}

async function storedPreferences(page) {
  return page.evaluate(key => localStorage.getItem(key), USER_KEY)
}

async function noHorizontalScroll(page) {
  const overflow = await page.evaluate(() => document.documentElement.scrollWidth - window.innerWidth)
  assert(overflow <= 0, `horizontal overflow of ${overflow}px`)
}

async function touchDrag(page, dx, dy) {
  const context = page.context()
  const cdp = await context.newCDPSession(page)
  const start = await box(page)
  const x = Math.round(start.x + Math.min(110, start.width / 3))
  const y = Math.round(start.y + 24)
  const point = (px, py) => [{ x: Math.round(px), y: Math.round(py), id: 1, radiusX: 2, radiusY: 2, force: 1 }]
  await cdp.send('Input.dispatchTouchEvent', { type: 'touchStart', touchPoints: point(x, y) })
  for (let step = 1; step <= 10; step += 1) {
    await cdp.send('Input.dispatchTouchEvent', { type: 'touchMove', touchPoints: point(x + (dx * step) / 10, y + (dy * step) / 10) })
  }
  await cdp.send('Input.dispatchTouchEvent', { type: 'touchEnd', touchPoints: [] })
  return box(page)
}

;(async () => {
  browser = await chromium.launch({ headless: true })
  const ownerContext = await browser.newContext({ viewport: { width: 1280, height: 900 }, locale: 'zh-CN' })
  const owner = await login(ownerContext, OWNER)

  // 1 · default state: launcher only, no dialog
  await launcher(owner).waitFor({ state: 'visible' })
  assert.equal(await dialog(owner).count(), 0)
  const launcherBox = await launcher(owner).boundingBox()
  assert(launcherBox && launcherBox.width >= 96, `launcher collapsed to ${launcherBox?.width}px`)
  assert(launcherBox.height <= 60, `launcher grew to ${launcherBox.height}px tall`)
  const labelBox = await launcher(owner).locator('span').first().boundingBox()
  assert(labelBox && labelBox.height < 26, `launcher label wrapped into ${labelBox?.height}px`)
  assert(launcherBox.x + launcherBox.width <= 1280 - 11, `launcher clipped by the right edge: ${launcherBox.x + launcherBox.width}`)
  await owner.screenshot({ path: path.join(output, 'assistant-window-launcher.png') })
  checks.push('默认只显示悬浮按钮，入口文案不折行')

  // 2 · open: non-modal, no backdrop, no scroll lock
  const bodyOverflowBefore = await owner.evaluate(() => getComputedStyle(document.body).overflow)
  await openAssistant(owner)
  assert.equal(await dialog(owner).getAttribute('aria-modal'), null, 'assistant must not claim aria-modal')
  assert.equal(await owner.evaluate(() => getComputedStyle(document.body).overflow), bodyOverflowBefore, 'opening the assistant changed body overflow')
  const outside = await owner.evaluate(() => {
    const element = document.elementFromPoint(4, 4)
    return Boolean(element && element.closest('.nv-assistant-window'))
  })
  assert.equal(outside, false, 'the window must not cover the page')
  assert.equal(await owner.locator('.nv-assistant-window').count(), 1)
  const textFits = await owner.evaluate(() => {
    const root = document.querySelector('.nv-assistant-window')
    const empty = root?.querySelector('[role="log"] p')
    const hint = Array.from(root?.querySelectorAll('span') || []).find(element => element.className.includes('min-w-0'))
    const fits = element => !element || element.scrollWidth <= element.clientWidth + 1
    return { empty: fits(empty), hint: fits(hint) }
  })
  assert.deepEqual(textFits, { empty: true, hint: true }, 'assistant text is clipped horizontally')
  await owner.screenshot({ path: path.join(output, 'assistant-window-dark.png') })
  checks.push('点击按钮展开，且不出现全屏遮罩或滚动锁')

  // 3 · drag by the title bar
  const before = await box(owner)
  // The default spot is already the bottom-right corner, so move up-left.
  const moved = await dragWindow(owner, -160, -120)
  assert(Math.abs(moved.x - (before.x - 160)) <= 2, `drag x drift: ${moved.x} vs ${before.x - 160}`)
  assert(Math.abs(moved.y - (before.y - 120)) <= 2, `drag y drift: ${moved.y} vs ${before.y - 120}`)
  checks.push('拖动标题栏后窗口位置改变')

  // 4 · dragging past the corner keeps the window reachable
  const offscreen = await dragWindow(owner, 3000, 3000)
  assert(offscreen.x >= 0 && offscreen.x + offscreen.width <= 1280, `window left the screen horizontally: ${offscreen.x}`)
  assert(offscreen.y >= 0 && offscreen.y + offscreen.height <= 900, `window left the screen vertically: ${offscreen.y}`)
  assert.equal(await owner.getByRole('button', { name: '最小化助手', exact: true }).isVisible(), true)
  checks.push('拖拽不能把窗口完全移出屏幕')

  // 5 · resize from the bottom-right handle
  await owner.getByRole('button', { name: '恢复默认位置和大小', exact: true }).click()
  const sized = await box(owner)
  const handle = await owner.locator('.nv-assistant-window__resize').boundingBox()
  assert(handle, 'resize handle missing on desktop')
  await dragFrom(owner, handle.x + handle.width / 2, handle.y + handle.height / 2, -80, -60)
  const resized = await box(owner)
  assert(Math.abs(resized.width - (sized.width - 80)) <= 4, `width ${resized.width} != ${sized.width - 80}`)
  assert(Math.abs(resized.height - (sized.height - 60)) <= 4, `height ${resized.height} != ${sized.height - 60}`)
  await noHorizontalScroll(owner)
  checks.push('拖动右下角缩放后布局正常且无横向滚动')

  // 6 · chat survives minimise
  await owner.getByRole('textbox', { name: '告诉助手你想做什么', exact: true }).fill(CHAT)
  await owner.getByRole('button', { name: '发送消息', exact: true }).click()
  await owner.getByRole('status').filter({ hasText: '正在处理' }).waitFor({ state: 'hidden' })
  await log(owner).getByText(CHAT, { exact: true }).waitFor()
  await owner.getByRole('button', { name: '最小化助手', exact: true }).click()
  await dialog(owner).waitFor({ state: 'hidden' })
  await launcher(owner).waitFor({ state: 'visible' })
  assert.equal(await owner.evaluate(() => document.activeElement?.getAttribute('aria-label')), '打开观影助手')
  checks.push('最小化后出现悬浮按钮，焦点回到入口')

  // 7 · restore keeps the conversation
  await openAssistant(owner)
  await log(owner).getByText(CHAT, { exact: true }).waitFor()
  checks.push('点击悬浮按钮恢复，对话未丢失')

  // 8 · persistence across reload
  const persisted = await box(owner)
  await owner.reload()
  await dialog(owner).waitFor({ state: 'visible' })
  const restored = await box(owner)
  assert(Math.abs(restored.x - persisted.x) <= 2 && Math.abs(restored.y - persisted.y) <= 2, 'position was not restored')
  assert(Math.abs(restored.width - persisted.width) <= 2 && Math.abs(restored.height - persisted.height) <= 2, 'size was not restored')
  const saved = await storedPreferences(owner)
  assert(saved && saved.includes('expanded'), 'visibility was not persisted')
  assert.equal(saved.includes(CHAT), false, 'chat text must never reach localStorage')
  assert.equal(saved.includes('token'), false, 'assistant preferences must not store tokens')
  checks.push('刷新后恢复显示状态、位置与尺寸，且不保存聊天内容')

  // 9 · hide, then reopen from personal settings
  await owner.getByRole('button', { name: '隐藏助手', exact: true }).click()
  await dialog(owner).waitFor({ state: 'hidden' })
  assert.equal(await launcher(owner).count(), 0, 'hidden assistant must not leave a launcher')
  assert.equal(await owner.locator('.nv-assistant-window').count(), 0)
  checks.push('完全隐藏后页面没有悬浮入口')

  await owner.goto(input.frontend + '/profile')
  const reopen = owner.getByRole('button', { name: '重新开启问问助手', exact: true })
  await reopen.waitFor({ state: 'visible' })
  await reopen.click()
  await dialog(owner).waitFor({ state: 'visible' })
  checks.push('可以从「我的 → 个人设置」重新开启')

  // 10 · no full-page blocking and keyboard escape
  await owner.getByRole('textbox', { name: '告诉助手你想做什么', exact: true }).focus()
  await owner.keyboard.press('Escape')
  await dialog(owner).waitFor({ state: 'hidden' })
  await launcher(owner).waitFor({ state: 'visible' })
  checks.push('Escape 收起窗口，播放器区域不被遮挡')

  // 11 · narrow screen
  await owner.setViewportSize({ width: 320, height: 720 })
  await openAssistant(owner)
  const narrow = await box(owner)
  assert(narrow.width <= 296, `320px viewport produced a ${narrow.width}px wide window`)
  await noHorizontalScroll(owner)
  const inputBox = await owner.getByRole('textbox', { name: '告诉助手你想做什么', exact: true }).boundingBox()
  assert(inputBox && inputBox.y + inputBox.height <= 720, 'input is outside the 320x720 viewport')
  await owner.screenshot({ path: path.join(output, 'assistant-window-320.png') })
  checks.push('320px 窄屏不产生横向滚动，输入框仍可见')

  // 12 · shrink the browser window: window is pulled back inside
  await owner.setViewportSize({ width: 700, height: 500 })
  await owner.waitForTimeout(150)
  const shrunk = await box(owner)
  assert(shrunk.x + shrunk.width <= 700, `right edge ${shrunk.x + shrunk.width} > 700`)
  assert(shrunk.y + shrunk.height <= 500, `bottom edge ${shrunk.y + shrunk.height} > 500`)
  checks.push('浏览器窗口变小时自动校正位置和尺寸')

  // 13 · light theme keeps the panel readable
  await owner.setViewportSize({ width: 1280, height: 900 })
  const previousTheme = await owner.evaluate(() => document.documentElement.dataset.theme || '')
  await owner.evaluate(() => { document.documentElement.dataset.theme = 'light' })
  await owner.waitForTimeout(150)
  const lightSurface = await owner.locator('.nv-assistant-window').evaluate(element => getComputedStyle(element).backgroundColor)
  const scale = lightSurface.startsWith('color(') ? 255 : 1
  const channels = (lightSurface.match(/[\d.]+/g) || []).slice(0, 3).map(value => Number(value) * scale)
  assert(channels.reduce((sum, value) => sum + value, 0) > 600, `light theme surface is not light: ${lightSurface}`)
  await owner.screenshot({ path: path.join(output, 'assistant-window-light.png') })
  await owner.evaluate(theme => { document.documentElement.dataset.theme = theme }, previousTheme || 'dark')
  checks.push('浅色模式下玻璃面板保持浅底深字')

  // 14 · playing a movie hides the panel and its launcher, leaving restores both
  await owner.setViewportSize({ width: 1280, height: 900 })
  await owner.goto(input.frontend + '/profile')
  if (!(await dialog(owner).isVisible())) await owner.getByRole('button', { name: '重新开启问问助手', exact: true }).click()
  await dialog(owner).waitFor({ state: 'visible' })
  await owner.goto(input.frontend + '/play/missing-media')
  await owner.waitForTimeout(300)
  assert.equal(await dialog(owner).count(), 0, 'the panel is still on the player')
  assert.equal(await launcher(owner).count(), 0, 'the launcher is still on the player')
  assert.equal(await owner.locator('.nv-assistant-window').count(), 0)
  await owner.screenshot({ path: path.join(output, 'assistant-hidden-on-player.png') })
  await owner.goto(input.frontend + '/profile')
  await dialog(owner).waitFor({ state: 'visible' })
  checks.push('播放页完全隐藏窗口和入口，离开后恢复展开')

  // 15 · a minimized state comes back minimized, not expanded
  await owner.getByRole('button', { name: '最小化助手', exact: true }).click()
  await launcher(owner).waitFor({ state: 'visible' })
  await owner.goto(input.frontend + '/play/missing-media')
  await owner.waitForTimeout(300)
  assert.equal(await launcher(owner).count(), 0, 'the launcher is still on the player')
  await owner.goto(input.frontend + '/profile')
  await launcher(owner).waitFor({ state: 'visible' })
  assert.equal(await dialog(owner).count(), 0)
  checks.push('播放页隐藏后按原来的最小化状态恢复')

  // 16 · account isolation
  const otherContext = await browser.newContext({ viewport: { width: 1280, height: 900 }, locale: 'zh-CN' })
  const other = await login(otherContext, OTHER)
  await launcher(other).waitFor({ state: 'visible' })
  assert.equal(await dialog(other).count(), 0, 'another account inherited the expanded state')
  const otherSaved = await other.evaluate(() => localStorage.getItem('nowen-assistant-window:v1:window-other'))
  assert(otherSaved, 'the second account should keep its own record')
  const otherPreferences = JSON.parse(otherSaved)
  assert.equal(otherPreferences.visibility, 'minimized', 'another account inherited the expanded state')
  assert.deepEqual(otherPreferences.position, { x: 848, y: 268 }, 'another account inherited a moved position')
  assert.deepEqual(otherPreferences.size, { width: 420, height: 620 }, 'another account inherited a resized window')
  checks.push('切换账号后不复用前一个账号的显示状态')

  // 17 · touch drag on a phone viewport
  const touchContext = await browser.newContext({ viewport: { width: 390, height: 844 }, hasTouch: true, locale: 'zh-CN' })
  const touchPage = await login(touchContext, OWNER)
  await launcher(touchPage).waitFor({ state: 'visible' })
  const touchLauncher = await launcher(touchPage).boundingBox()
  assert(touchLauncher && touchLauncher.x + touchLauncher.width <= 390 - 11, `phone launcher clipped: ${JSON.stringify(touchLauncher)}`)
  assert(touchLauncher.y + touchLauncher.height <= 844 - 82 + 1, 'phone launcher overlaps the bottom navigation')
  await openAssistant(touchPage)
  await touchPage.getByRole('button', { name: '恢复默认位置和大小', exact: true }).click()
  await touchPage.waitForTimeout(120)
  const touchBefore = await box(touchPage)
  // A phone-width panel fills the width, so the touch drag is vertical.
  const touchAfter = await touchDrag(touchPage, 0, -100)
  assert(Math.abs(touchAfter.x - touchBefore.x) <= 3, `touch drag moved horizontally: ${touchAfter.x} vs ${touchBefore.x}`)
  assert(Math.abs(touchAfter.y - (touchBefore.y - 100)) <= 3, `touch drag y drift: ${touchAfter.y} vs ${touchBefore.y - 100}`)
  await noHorizontalScroll(touchPage)
  await touchPage.screenshot({ path: path.join(output, 'assistant-window-touch.png') })
  checks.push('手机触摸拖拽可用')

  assert.deepEqual(pageErrors, [], `page errors: ${pageErrors.join(' | ')}`)
  fs.writeFileSync(path.join(output, 'assistant-window-report.json'), JSON.stringify({ checks, pageErrors }, null, 2))
  console.log(JSON.stringify({ checks, pageErrors }, null, 2))
  await browser.close()
})().catch(async error => {
  fs.writeFileSync(path.join(output, 'assistant-window-report.json'), JSON.stringify({ checks, pageErrors, error: redact(error.stack || error.message) }, null, 2))
  console.error(redact(error.stack || error.message))
  if (browser) await browser.close()
  process.exitCode = 1
})
