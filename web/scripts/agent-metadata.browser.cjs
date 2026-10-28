const { chromium } = require('playwright');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const crypto = require('node:crypto');
const { execFileSync } = require('node:child_process');
if (!process.env.AGENT_METADATA_MANIFEST) throw new Error('AGENT_METADATA_MANIFEST is required');
const manifest = path.resolve(process.env.AGENT_METADATA_MANIFEST);
const input = JSON.parse(fs.readFileSync(manifest, 'utf8'));
const output = path.dirname(manifest);
if (!input.isolated || !path.resolve(input.db).startsWith(output + path.sep)) throw new Error('An isolated database inside the manifest directory is required');
for (const endpoint of [input.backend, input.frontend, input.source]) {
 const u = new URL(endpoint);
 if (u.hostname !== '127.0.0.1' || ['28888', '28889'].includes(u.port)) throw new Error('Refusing non-isolated endpoints');
}
const checks = [], pageErrors = []; let browser;
const redact = value => String(value).replaceAll(input.password, '[REDACTED]');
async function api(method, route, token, body, status = 200) {
 const response = await fetch(input.backend + '/api' + route, { method, headers: { ...(token ? { Authorization: 'Bearer ' + token } : {}), ...(body !== undefined ? { 'Content-Type': 'application/json' } : {}) }, ...(body !== undefined ? { body: JSON.stringify(body) } : {}), signal: AbortSignal.timeout(15000) });
 assert.equal(response.status, status, `${method} ${route}: unexpected status`); return response.json();
}
function sql(query, params = []) { return JSON.parse(execFileSync('python3', ['-c', 'import sqlite3,json,sys;c=sqlite3.connect(sys.argv[1]);r=c.execute(sys.argv[2],json.loads(sys.argv[3]));rows=r.fetchall();c.commit();print(json.dumps(rows,ensure_ascii=False))', input.db, query, JSON.stringify(params)], { encoding: 'utf8' })); }
async function sourceCalls() { return (await (await fetch(input.source + '/stats')).json()).calls; }
async function login(username) {
 const context = await browser.newContext({ viewport: { width: 1280, height: 900 }, locale: 'zh-CN' });
 await context.addInitScript(() => localStorage.setItem('nowen-i18n', JSON.stringify({ state: { locale: 'zh-CN' }, version: 0 })));
 const page = await context.newPage(); page.on('pageerror', e => pageErrors.push(redact(e.message))); page.setDefaultTimeout(15000);
 await page.goto(input.frontend + '/login'); await page.getByRole('textbox', { name: '用户名', exact: true }).fill(username); await page.getByLabel('密码', { exact: true }).fill(input.password); await page.getByRole('button', { name: '进入深空', exact: true }).click(); await page.waitForURL(input.frontend + '/');
 const token = await page.evaluate(() => JSON.parse(localStorage.getItem('nowen-auth')).state.token); return { context, page, token };
}
async function open(page) { await page.getByRole('button', { name: '打开观影助手', exact: true }).click(); await page.getByRole('dialog', { name: '观影助手', exact: true }).waitFor(); }
async function say(page, text) { await page.getByRole('textbox', { name: '告诉助手你想做什么', exact: true }).fill(text); await page.getByRole('button', { name: '发送消息', exact: true }).click(); await page.getByRole('status').filter({ hasText: '正在处理' }).waitFor({ state: 'hidden' }); }
async function newest(token) { return (await api('GET', '/admin/agent/runs', token)).data.find(r => r.status === 'draft'); }
const base = '/admin/agent/runs';
(async () => {
 browser = await chromium.launch({ headless: true });
 const owner = await login('meta-owner'), other = await login('meta-other'), viewer = await login('meta-viewer'), child = await login('meta-child');
 const startCalls = await sourceCalls();
 for (const a of [viewer, child]) {
  await api('POST', base + '/metadata', a.token, { media_id: 'movie' }, 403);
  await api('POST', '/agent/chat', a.token, { message: '补全资料' }, 403);
  await api('POST', base + '/unknown/undo', a.token, { approval_digest: '0'.repeat(64) }, 403);
 }
 assert.equal(await sourceCalls(), startCalls); checks.push('ordinary and child accounts cannot fetch or change metadata');
 const reply = (await api('POST', '/agent/chat', owner.token, { message: '补全资料' })).data;
 assert.equal(reply.metadata_candidates, true); assert(reply.cards.some(c => c.id === 'movie')); assert(!reply.cards.some(c => ['unmatched', 'full'].includes(c.id)));
 const at = reply.cards.findIndex(c => c.id === 'movie');
 const ordinal = ['第一', '第二', '第三', '第四', '第五', '第六'][at];
 const selected = (await api('POST', '/agent/chat', owner.token, { message: `补全${ordinal}部资料`, context: reply.cards.map(({ id, kind }) => ({ id, kind })) })).data;
 assert.equal(selected.action?.kind, 'prepare_metadata'); assert.equal(selected.action.target.id, 'movie');
 checks.push('candidate list and ordinal command select linked movies with missing text');
 const page = owner.page; await open(page); await say(page, '补全资料');
 const card = page.getByText(/^\d+\. 手工片名$/).locator('..');
 await card.getByRole('button', { name: '预览资料补全', exact: true }).click();
 await page.getByRole('button', { name: '确认补全资料', exact: true }).waitFor(); let p = await newest(owner.token);
 assert.equal(p.media_title, '手工片名'); assert.equal(p.source_title, '来源片名'); assert.equal(p.changes.length, 3); assert(!p.changes.some(c => c.field === 'orig_title'));
 assert.equal(sql('select overview from media where id=?', ['movie'])[0][0], '  ');
 await api('POST', `${base}/${p.id}/confirm`, other.token, { approval_digest: p.approval_digest }, 404);
 await api('POST', `${base}/${p.id}/confirm`, owner.token, { approval_digest: '0'.repeat(64) }, 409);
 await page.getByRole('button', { name: '取消预览', exact: true }).click();
 assert.equal((await api('POST', `${base}/${p.id}/confirm`, owner.token, { approval_digest: p.approval_digest })).data.status, 'cancelled');
 checks.push('preview shows exact source and blank fields; cancellation and owner binding prevent writes');
 await card.getByRole('button', { name: '预览资料补全', exact: true }).click(); await page.getByRole('button', { name: '确认补全资料', exact: true }).waitFor(); p = await newest(owner.token);
 await page.setViewportSize({ width: 320, height: 720 }); await page.screenshot({ path: path.join(output, 'metadata-preview-mobile.png'), fullPage: true });
 assert(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth));
 const inputBox = await page.getByRole('textbox', { name: '告诉助手你想做什么', exact: true }).boundingBox(); assert(inputBox && inputBox.y + inputBox.height <= 720);
 const callsBefore = await sourceCalls(); await page.getByRole('button', { name: '确认补全资料', exact: true }).click();
 await page.getByText(/已补全《手工片名》的 3 项空缺资料/).first().waitFor();
 assert.equal((await api('POST', `${base}/${p.id}/confirm`, owner.token, { approval_digest: p.approval_digest })).data.status, 'completed');
 assert.equal(await sourceCalls(), callsBefore);
 assert.deepEqual(sql('select title,orig_title,rating,scrape_status,overview,genres,premiered from media where id=?', ['movie'])[0], ['手工片名', '手工原名', 9.1, 'manual', '新的简介：来自隔离资料源。', '剧情', '2020-02-02']);
 checks.push('320px preview and explicit confirmation persist only approved fields; replay does not fetch again');
 await page.reload(); await open(page); await page.getByText('任务预览与回执', { exact: true }).waitFor();
 const expand = page.getByRole('button', { name: '展开任务记录', exact: true }); if (await expand.isVisible()) await expand.click();
 await page.getByRole('button', { name: '撤销本次补全', exact: true }).click(); await page.getByText('本次补全已撤销，恢复了预览前的空缺字段。', { exact: true }).waitFor();
 assert.equal((await api('POST', `${base}/${p.id}/undo`, owner.token, { approval_digest: p.approval_digest })).data.status, 'reverted');
 assert.deepEqual(sql('select overview,genres,premiered from media where id=?', ['movie'])[0], ['  ', '', '']);
 checks.push('reload preserves receipt; explicit undo restores exact blanks and is replay safe');
 p = (await api('POST', base + '/metadata', owner.token, { media_id: 'movie' })).data;
 sql('update media set overview=? where id=?', ['其他管理员的新简介', 'movie']);
 await api('POST', `${base}/${p.id}/confirm`, owner.token, { approval_digest: p.approval_digest }, 409);
 assert.equal(sql('select overview from media where id=?', ['movie'])[0][0], '其他管理员的新简介');
 await api('POST', `${base}/${p.id}/cancel`, owner.token, {});
 checks.push('stale preview refuses to overwrite later edits');
 p = (await api('POST', base + '/metadata', owner.token, { media_id: 'movie' })).data;
 await api('POST', `${base}/${p.id}/confirm`, owner.token, { approval_digest: p.approval_digest });
 sql('update media set genres=? where id=?', ['后续手工编辑', 'movie']);
 await api('POST', `${base}/${p.id}/undo`, owner.token, { approval_digest: p.approval_digest }, 409);
 assert.deepEqual(sql('select genres,premiered from media where id=?', ['movie'])[0], ['后续手工编辑', '2020-02-02']);
 checks.push('undo refuses later field edits without partially reverting other fields');
 const countBefore = sql('select count(*) from agent_runs')[0][0];
 await api('POST', base + '/metadata', owner.token, { media_id: 'failed-source' }, 503);
 await api('POST', base + '/metadata', owner.token, { media_id: 'unmatched' }, 400);
 assert.equal(sql('select count(*) from agent_runs')[0][0], countBefore);
 checks.push('provider failure and unmatched films create no approval or mutation');
 assert.equal(sql('select level from content_ratings where media_id=?', ['movie'])[0][0], 'R');
 for (const [file, expected] of [['original.mp4', 'unchanged-source-bytes'], ['original.nfo', '<movie><title>manual title</title></movie>']]) assert.equal(crypto.createHash('sha256').update(fs.readFileSync(path.join(input.media_dir, file))).digest('hex'), crypto.createHash('sha256').update(expected).digest('hex'));
 assert.deepEqual(pageErrors, []); checks.push('source and NFO bytes and child rating remain unchanged; no uncaught page errors');
 await page.screenshot({ path: path.join(output, 'metadata-result-mobile.png'), fullPage: true });
})().catch(error => { console.error(redact(error.stack || error.message)); process.exitCode = 1 }).finally(async () => {
 await browser?.close(); const report = { passed: checks.length, checks, uses_real_backend: true, api_mocking: false, metadata_source: 'isolated HTTP fixture; not live TMDb' };
 fs.writeFileSync(path.join(output, 'report.json'), JSON.stringify(report, null, 2), { mode: 0o600 }); console.log(JSON.stringify(report, null, 2));
});
