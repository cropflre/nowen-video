// Real backend + real NAS acceptance. No request mocking or production writes.
// Prepare an isolated database/config and pass its private manifest explicitly.
const { chromium } = require('playwright');
const fs = require('node:fs');
const path = require('node:path');
const assert = require('node:assert/strict');
const manifestPath = process.env.FAMILY_REAL_MANIFEST;
if (!manifestPath) throw new Error('FAMILY_REAL_MANIFEST must point to an isolated QA manifest');
const manifest = JSON.parse(fs.readFileSync(manifestPath, 'utf8'));
const output = path.dirname(manifestPath);
for (const base of [manifest.backend, manifest.frontend]) {
  const url = new URL(base);
  if (url.hostname !== '127.0.0.1' || url.protocol !== 'http:') throw new Error('Acceptance requires isolated loopback services');
}
const reports = [];
let browser;
const redact = text => manifest.users.reduce((result, user) => result.replaceAll(user.password, '[REDACTED]'), String(text)).replace(/sk-(?:ws-)?[A-Za-z0-9._-]{12,}/g, '[REDACTED]');
async function api(method, route, token, data, expected = 200) {
  const response = await fetch(manifest.backend + '/api' + route, { method, headers: { ...(token ? { Authorization: 'Bearer ' + token } : {}), ...(data !== undefined ? { 'Content-Type': 'application/json' } : {}) }, ...(data !== undefined ? { body: JSON.stringify(data) } : {}), signal: AbortSignal.timeout(20000) });
  assert((Array.isArray(expected) ? expected : [expected]).includes(response.status), `${method} ${route}: unexpected HTTP status ${response.status}`);
  if (!(response.headers.get('content-type') || '').includes('application/json')) throw new Error(`${method} ${route}: response is not JSON`);
  return response.json();
}
async function step(name, fn) {
  const started = Date.now();
  try { const details = await fn(); reports.push({ name, pass: true, milliseconds: Date.now() - started, ...details }); }
  catch (error) {
    const diagnostics=[];
    for (const [index,context] of (browser?.contexts() || []).entries()) for (const page of context.pages()) {
      try {
        diagnostics.push(await page.evaluate(() => ({ path: location.pathname, text: document.body.innerText.slice(-1800), video: Array.from(document.querySelectorAll('video')).map(video => ({ time: video.currentTime, paused: video.paused, ended: video.ended, ready: video.readyState, error: video.error?.code })) })));
        await page.screenshot({ path: path.join(output, `failure-${reports.length}-${index}.png`), fullPage: true });
      } catch { /* Preserve the original failure if a page is already closing. */ }
    }
    reports.push({ name, pass: false, milliseconds: Date.now() - started, error: redact(error.message), diagnostics: JSON.parse(redact(JSON.stringify(diagnostics))) });
  }
  console.log(JSON.stringify(reports.at(-1)));
}
async function login(role) {
  const user = manifest.users.find(user => user.id === 'qa-real-' + role);
  const context = await browser.newContext({ viewport: { width: 1440, height: 900 }, locale: 'zh-CN' });
  await context.addInitScript(() => localStorage.setItem('nowen-i18n', JSON.stringify({ state: { locale: 'zh-CN' }, version: 0 })));
  const page = await context.newPage();
  page.setDefaultTimeout(20000);
  const errors = [];
  page.on('pageerror', error => errors.push(redact(error.message)));
  await page.goto(manifest.frontend + '/login');
  await page.getByRole('textbox', { name: '用户名', exact: true }).fill(user.username);
  await page.getByLabel('密码', { exact: true }).fill(user.password);
  await page.getByRole('button', { name: '进入深空', exact: true }).click();
  await page.waitForURL(manifest.frontend + '/');
  const token = await page.evaluate(() => JSON.parse(localStorage.getItem('nowen-auth')).state.token);
  assert(token);
  return { context, page, token, errors };
}
async function visible(page, text) { await page.getByText(text, { exact: true }).last().waitFor({ state: 'visible' }); }

(async () => {
  let ready=false;
  for (const deadline=Date.now()+15000;Date.now()<deadline;) {
    try { const response=await fetch(manifest.backend+'/api/health',{signal:AbortSignal.timeout(500)});if(response.ok){ready=true;break} } catch { /* The isolated process may still be starting. */ }
    await new Promise(resolve=>setTimeout(resolve,100));
  }
  assert(ready,'isolated backend did not become ready');
  browser = await chromium.launch({ headless: true, args: ['--autoplay-policy=no-user-gesture-required'] });
  await step('real service health and unknown API contract', async () => {
    const health = await api('GET', '/health'); assert.equal(health.status, 'ok'); assert(health.capabilities);
    await api('GET', '/acceptance-route-that-does-not-exist', null, undefined, 404);
  });
  const admin = await login('admin');
  const viewer = await login('viewer');
  const child = await login('child');
  const media = manifest.media[0];
  const hidden = manifest.media[1];
  await step('actual UI login creates separate authenticated device sessions', async () => {
    for (const account of [admin, viewer, child]) { const status = await api('GET', '/users/me/device-status', account.token); assert(status.data.current_id); }
    const profile = await api('GET', '/users/me', child.token); assert.equal(profile.data.child_mode, true); assert.equal(profile.data.viewing_preferences.auto_play_next, false);
  });
  await step('real catalog returns current NAS media', async () => {
    const catalog = await api('GET', '/media/mixed?size=200', viewer.token); assert(catalog.total >= 100);
    const detail = await api('GET', '/media/' + media.id, viewer.token); assert.equal(detail.data.title, media.title);
    return { accessible_media: catalog.total };
  });
  await step('watchlist changes from the detail page persist through actual APIs and SQLite', async () => {
    await viewer.page.goto(manifest.frontend + '/media/' + media.id);
    await viewer.page.getByRole('button', { name: '加入想看', exact: true }).click();
    const remove = viewer.page.getByRole('button', { name: '移出想看', exact: true });
    await remove.waitFor();
    assert.equal((await api('GET', '/users/me/favorites/' + media.id + '/check', viewer.token)).data, true);
    await remove.click();
    await viewer.page.getByRole('button', { name: '加入想看', exact: true }).waitFor();
    assert.equal((await api('GET', '/users/me/favorites/' + media.id + '/check', viewer.token)).data, false);
  });
  await step('viewing preferences persist on reload and mobile large text stays usable', async () => {
    await viewer.page.goto(manifest.frontend + '/my');
    await viewer.page.getByRole('switch', { name: '大字模式', exact: true }).check();
    await viewer.page.waitForFunction(() => document.documentElement.dataset.largeText === 'true');
    const saved = viewer.page.waitForResponse(response => new URL(response.url()).pathname === '/api/users/me' && response.request().method() === 'PUT' && response.ok());
    await viewer.page.getByRole('combobox', { name: '默认字幕', exact: true }).selectOption('zh');
    await saved;
    await viewer.page.reload();
    await viewer.page.waitForFunction(() => document.documentElement.dataset.largeText === 'true');
    const profile = await api('GET', '/users/me', viewer.token); assert.equal(profile.data.viewing_preferences.subtitle_language, 'zh');
    await viewer.page.setViewportSize({ width: 320, height: 720 });
    assert(await viewer.page.evaluate(() => document.documentElement.scrollWidth <= innerWidth));
    await viewer.page.screenshot({ path: path.join(output, 'real-nas-mobile.png'), fullPage: true });
    await viewer.page.setViewportSize({ width: 1440, height: 900 });
  });
  await step('real SMB MKV plays through automatic browser playback preparation', async () => {
    const info = await api('GET', '/stream/' + media.id + '/info', viewer.token);
    const started = Date.now(); await viewer.page.goto(manifest.frontend + '/play/' + media.id);
    await viewer.page.waitForFunction(() => { const video = document.querySelector('video'); return video && !video.paused && video.currentTime > 2 && video.videoWidth > 0; }, null, { timeout: 60000 });
    const playback = await viewer.page.locator('video').evaluate(video => ({ time: video.currentTime, width: video.videoWidth, height: video.videoHeight, ready: video.readyState }));
    assert(playback.width > 0);
    return { first_decoded_playback_ms: Date.now() - started, method: info.data.playback_plan?.method, ...playback };
  });
  await step('real watching progress survives leaving and re-entering the player', async () => {
    await viewer.page.waitForFunction(() => document.querySelector('video')?.currentTime >= 8, null, { timeout: 30000 });
    await viewer.page.goto(manifest.frontend + '/my');
    let progress;
    for (let i = 0; i < 30; i++) { progress = (await api('GET', '/users/me/progress/' + media.id, viewer.token)).data; if (progress?.position >= 5) break; await new Promise(resolve => setTimeout(resolve, 200)); }
    assert(progress?.position >= 5, 'actual saved progress missing');
    await viewer.page.goto(manifest.frontend + '/play/' + media.id);
    await viewer.page.waitForFunction(expected => { const video = document.querySelector('video'); const slider = document.querySelector('[role="slider"][aria-label="播放进度"]'); return Boolean(video && video.readyState >= 2 && video.videoWidth > 0 && slider && Number(slider.getAttribute('aria-valuenow')) >= expected - 2); }, progress.position, { timeout: 45000 });
    const source = await viewer.page.locator('video').getAttribute('src');
    assert(source);
    await viewer.page.goto(manifest.frontend + '/my');
    return { persisted_seconds: progress.position };
  });
  await step('child real catalog and direct media/stream requests enforce reviewed scope', async () => {
    const list = await api('GET', '/media/mixed?size=200', child.token); assert.equal(list.total, 1);
    for (const route of ['/media/' + hidden.id, '/stream/' + hidden.id + '/info', '/stream/' + hidden.id + '/direct']) await api('GET', route, child.token, undefined, [403, 404]);
    assert.equal((await api('GET', '/users/me/family-controls', child.token)).data.daily_limit_seconds, 60);
  });
  await step('limited child plays real NAS media and server stops it at budget expiry', async () => {
    await child.page.goto(manifest.frontend + '/play/' + media.id);
    await child.page.waitForFunction(() => { const video = document.querySelector('video'); return video && !video.paused && video.currentTime > 1 && video.videoWidth > 0; }, null, { timeout: 60000 });
    const granted = await api('GET', '/users/me/family-controls', child.token); assert(granted.data.remaining_seconds <= 60 && granted.data.remaining_seconds > 0);
    await child.page.waitForFunction(() => document.body.textContent?.includes('今天的观看时间已用完'), null, { timeout: 80000 });
    assert.equal(await child.page.evaluate(() => document.querySelector('video')?.paused !== false), true);
    const ended = await api('GET', '/users/me/family-controls', child.token); assert.equal(ended.data.remaining_seconds, 0);
    await api('GET', '/media/' + media.id, child.token); await api('GET', '/stream/' + media.id + '/direct', child.token, undefined, 403);
    await child.page.screenshot({ path: path.join(output, 'real-child-budget.png'), fullPage: true });
    await child.page.goto(manifest.frontend + '/my');
  });
  await step('parent policy withdrawal invalidates existing child credentials', async () => {
    await api('PUT', '/admin/permissions/qa-real-child', admin.token, { child_mode: true, allowed_libraries: '', max_rating_level: 'G', daily_time_limit: 1, allow_auto_play_next: false });
    await api('GET', '/users/me', child.token, undefined, 401);
    await child.page.reload(); await child.page.waitForURL(manifest.frontend + '/login');
  });
  await step('actual pairing, one-time exchange, device revocation and session isolation', async () => {
    const pairing = (await api('POST', '/auth/device-pairing', null, { device_name: 'Real acceptance paired browser' }, 201)).data;
    await viewer.page.goto(manifest.frontend + '/pair#code=' + pairing.code);
    await viewer.page.getByRole('button', { name: '确认授权此账号', exact: true }).click(); await visible(viewer.page, '已确认，请回到新设备。配对只可使用一次。');
    const exchange = (await api('POST', '/auth/device-pairing/status', null, { id: pairing.id, secret: pairing.secret })).data; assert.equal(exchange.status, 'authorized');
    const pairedToken = exchange.authorization.token;
    assert.equal((await api('GET', '/users/me', pairedToken)).data.id, 'qa-real-viewer');
    await api('POST', '/auth/device-pairing/status', null, { id: pairing.id, secret: pairing.secret }, 410);
    const id = (await api('GET', '/users/me/device-status', pairedToken)).data.current_id;
    await api('DELETE', '/users/me/devices/' + id, viewer.token);
    await api('GET', '/users/me', pairedToken, undefined, 401); await api('GET', '/users/me', viewer.token);
  });
  await step('real browser pages produced no uncaught runtime exceptions', async () => { assert.deepEqual([...admin.errors, ...viewer.errors, ...child.errors], []); });
  await admin.context.close(); await viewer.context.close(); await child.context.close();
})().catch(error => { reports.push({ name: 'fatal acceptance setup', pass: false, error: redact(error.message) }); }).finally(async () => {
  await browser?.close();
  const summary = { passed: reports.filter(report => report.pass).length, failed: reports.filter(report => !report.pass).length, uses_real_backend: true, uses_real_nas: true, isolated_database: true, reports };
  fs.writeFileSync(path.join(output, 'browser-report.json'), JSON.stringify(summary, null, 2), { mode: 0o600 });
  console.log(JSON.stringify({ passed: summary.passed, failed: summary.failed }));
  if (summary.failed) process.exitCode = 1;
});
