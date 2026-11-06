const { chromium } = require('playwright');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const crypto = require('node:crypto');
const { execFileSync } = require('node:child_process');
if (!process.env.AGENT_MATCH_MANIFEST) throw new Error('AGENT_MATCH_MANIFEST is required');
const manifest = path.resolve(process.env.AGENT_MATCH_MANIFEST);
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
async function login(username) {
 const context = await browser.newContext({ viewport: { width: 1280, height: 900 }, locale: 'zh-CN' });
 await context.addInitScript(() => localStorage.setItem('nowen-i18n', JSON.stringify({ state: { locale: 'zh-CN' }, version: 0 })));
 const page = await context.newPage(); page.on('pageerror', e => pageErrors.push(redact(e.message))); page.setDefaultTimeout(15000);
 await page.goto(input.frontend + '/login'); await page.getByRole('textbox', { name: '用户名', exact: true }).fill(username); await page.getByLabel('密码', { exact: true }).fill(input.password); await page.getByRole('button', { name: '进入深空', exact: true }).click(); await page.waitForURL(input.frontend + '/');
 const token = await page.evaluate(() => JSON.parse(localStorage.getItem('nowen-auth')).state.token); return { context, page, token };
}
async function open(page) { await page.getByRole('button', { name: '打开观影助手', exact: true }).click(); await page.getByRole('dialog', { name: '观影助手', exact: true }).waitFor(); }
async function say(page, text) { await page.getByRole('textbox', { name: '告诉助手你想做什么', exact: true }).fill(text); await page.getByRole('button', { name: '发送消息', exact: true }).click(); await page.getByRole('status').filter({ hasText: '正在处理' }).waitFor({ state: 'hidden' }); }
async function newest(token) { return (await api('GET', '/admin/agent/runs', token)).data.find(r => r.kind === 'match_movie' && r.status === 'draft'); }
const base = '/admin/agent/runs';
async function stats() { return (await (await fetch(input.source + '/stats')).json()); }
(async () => {
 browser=await chromium.launch({headless:true});
 const owner=await login('match-owner'),other=await login('match-other'),viewer=await login('match-viewer'),child=await login('match-child');
 const initial=await stats();
 for(const u of [viewer,child]) {
  await api('POST',base+'/matches',u.token,{media_id:'movie'},403);
  await api('POST',base+'/unknown/select',u.token,{candidate_id:42,approval_digest:'0'.repeat(64)},403);
  await api('POST','/agent/chat',u.token,{message:'匹配影片'},403);
 }
 assert.deepEqual(await stats(),initial);checks.push('ordinary and child accounts cannot search or select candidates');
 const listed=(await api('POST','/agent/chat',owner.token,{message:'匹配影片'})).data;
 assert.equal(listed.match_candidates,true);assert(listed.cards.some(c=>c.id==='movie'));
 const at=listed.cards.findIndex(c=>c.id==='movie'),ordinal=['第一','第二','第三','第四','第五','第六'][at];
 const target=(await api('POST','/agent/chat',owner.token,{message:`匹配${ordinal}部`,context:listed.cards.map(({id,kind})=>({id,kind}))})).data;
 assert.equal(target.action?.kind,'prepare_match');assert.equal(target.action.target.id,'movie');
 checks.push('unlinked movies and ordinal commands enter matching without an LLM');
 const page=owner.page;await open(page);await say(page,'匹配影片');
 const card=page.getByText(/^\d+\. 手工片名$/).locator('..');await card.getByRole('button',{name:'查找影片候选',exact:true}).click();
 await page.getByRole('button',{name:'选择此候选',exact:true}).first().waitFor();
 let p=await newest(owner.token);
 let row=page.getByRole('article',{name:'手工片名影片匹配任务',exact:true});
 assert.equal(p.selected_match,undefined);assert.equal(sql('select tm_db_id from media where id=?',['movie'])[0][0],0);
 assert.equal(await row.getByRole('button',{name:'确认匹配此影片',exact:true}).count(),0);
 assert(p.match_candidates[1].evidence.some(s=>s.includes('年份不同')));
 await api('POST',`${base}/${p.id}/confirm`,owner.token,{approval_digest:p.approval_digest},400);
 await api('POST',`${base}/${p.id}/select`,other.token,{candidate_id:42,approval_digest:p.approval_digest},404);
 await api('POST',`${base}/${p.id}/select`,owner.token,{candidate_id:12345,approval_digest:p.approval_digest},400);
 checks.push('same-name candidates expose year differences; no default selection or forged IDs');
 await page.setViewportSize({width:320,height:720});await page.screenshot({path:path.join(output,'match-candidates-mobile.png'),fullPage:true});
 assert(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth));
 const box=await page.getByRole('textbox',{name:'告诉助手你想做什么',exact:true}).boundingBox();const viewport=page.viewportSize();assert(box && viewport && box.y+box.height<=viewport.height && box.x+box.width<=viewport.width);
 const oldDigest=p.approval_digest;
 await row.locator('[aria-label="匹配候选 手工片名 2020"]').getByRole('button',{name:'选择此候选',exact:true}).click();
 await row.getByRole('button',{name:'确认匹配此影片',exact:true}).waitFor();p=await newest(owner.token);
 assert.notEqual(p.approval_digest,oldDigest);assert.equal(sql('select tm_db_id from media where id=?',['movie'])[0][0],0);
 await api('POST',`${base}/${p.id}/confirm`,owner.token,{approval_digest:oldDigest},409);
 await row.getByRole('button',{name:'修改片名或年份',exact:true}).click();
 await row.getByRole('textbox',{name:'影片匹配关键词',exact:true}).fill('修改过的名称');
 assert(await row.getByRole('button',{name:'确认匹配此影片',exact:true}).isDisabled());
 await row.getByRole('button',{name:'取消修改',exact:true}).click();
 checks.push('320px layout, explicit selection, rotated approval and dirty-query protection');
 const beforeConfirm=await stats();await row.getByRole('button',{name:'确认匹配此影片',exact:true}).click();
 await page.getByText('影片关联已保存，现有资料保持原状。可以继续预览资料补全。',{exact:true}).waitFor();
 await api('POST',`${base}/${p.id}/confirm`,owner.token,{approval_digest:p.approval_digest});assert.deepEqual(await stats(),beforeConfirm);
 assert.deepEqual(sql('select tm_db_id,title,orig_title,rating,overview from media where id=?',['movie'])[0],[42,'手工片名','手工原名',9.1,'  ']);
 checks.push('only confirmation saves association; replay does not search or fill fields');
 await page.reload();await open(page);await page.getByText('任务预览与回执',{exact:true}).waitFor();
 const expand=page.getByRole('button',{name:'展开任务记录',exact:true});if(await expand.isVisible())await expand.click();
 await page.getByRole('button',{name:'预览资料补全',exact:true}).click();await page.getByRole('button',{name:'确认补全资料',exact:true}).click();
 await page.getByText(/已补全《手工片名》的 3 项空缺资料/).first().waitFor();
 await page.getByRole('button',{name:'撤销本次匹配',exact:true}).click();
 await page.getByRole('alert').filter({hasText:'影片身份或资料已变化'}).waitFor();
 await page.waitForTimeout(3500);
 assert(await page.getByRole('alert').filter({hasText:'影片身份或资料已变化'}).isVisible());
 await api('POST',`${base}/${p.id}/undo`,owner.token,{approval_digest:p.approval_digest},409);
 checks.push('receipt survives reload and links to separately approved metadata fill');
 await page.getByRole('button',{name:'撤销本次补全',exact:true}).click();
 await page.getByText('本次补全已撤销，恢复了预览前的空缺字段。',{exact:true}).waitFor();
 await page.getByRole('button',{name:'撤销本次匹配',exact:true}).click();
 await page.getByText('本次影片关联已撤销，原有资料和文件保持原状。',{exact:true}).waitFor();
 assert.deepEqual(sql('select tm_db_id,overview,genres,premiered from media where id=?',['movie'])[0],[0,'  ','','']);
 checks.push('undo enforces dependency order and restores original identity and blanks');
 p=(await api('POST',base+'/matches',owner.token,{media_id:'movie',query:'错误名称',year:0})).data;
 assert.equal((p.match_candidates||[]).length,0);await api('POST',`${base}/${p.id}/confirm`,owner.token,{approval_digest:p.approval_digest},400);
 const count=sql('select count(*) from agent_runs')[0][0];
 await api('POST',base+'/matches',owner.token,{media_id:'movie',query:'错误来源',year:0},503);assert.equal(sql('select count(*) from agent_runs')[0][0],count);
 const old=p;p=(await api('POST',base+'/matches',owner.token,{media_id:'movie',query:'正确名称',year:0})).data;
 assert.equal((await api('POST',`${base}/${old.id}/confirm`,owner.token,{approval_digest:old.approval_digest})).data.status,'cancelled');
 assert.equal(p.match_query,'正确名称');assert.equal(p.match_year,0);await api('POST',`${base}/${p.id}/cancel`,owner.token,{});
 checks.push('empty results, source errors and refined searches never auto-match; new search cancels old draft');
 p=(await api('POST',base+'/matches',owner.token,{media_id:'movie'})).data;
 p=(await api('POST',`${base}/${p.id}/select`,owner.token,{candidate_id:42,approval_digest:p.approval_digest})).data;
 sql('update media set title=? where id=?',['另一位管理员更改片名','movie']);
 await api('POST',`${base}/${p.id}/confirm`,owner.token,{approval_digest:p.approval_digest},409);
 assert.equal(sql('select tm_db_id from media where id=?',['movie'])[0][0],0);await api('POST',`${base}/${p.id}/cancel`,owner.token,{});
 checks.push('stale identity rejects confirmation without changing association');
 for(const[file,expected]of[['original.mp4','unchanged-source-bytes'],['original.nfo','<movie><title>manual title</title></movie>']])
  assert.equal(crypto.createHash('sha256').update(fs.readFileSync(path.join(input.media_dir,file))).digest('hex'),crypto.createHash('sha256').update(expected).digest('hex'));
 assert.equal(sql('select level from content_ratings where media_id=?',['movie'])[0][0],'R');assert.deepEqual(pageErrors,[]);
 checks.push('source and NFO bytes and child rating preserved; no page errors');
})().catch(e=>{console.error(redact(e.stack||e.message));process.exitCode=1}).finally(async()=>{
 await browser?.close();const report={passed:checks.length,checks,uses_real_backend:true,api_mocking:false,metadata_source:'isolated HTTP fixture'};
 fs.writeFileSync(path.join(output,'report.json'),JSON.stringify(report,null,2),{mode:0o600});console.log(JSON.stringify(report,null,2));
});
