const { chromium } = require('playwright');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const crypto = require('node:crypto');
const { execFileSync } = require('node:child_process');
if (!process.env.AGENT_SCAN_MANIFEST) throw new Error('AGENT_SCAN_MANIFEST is required');
const input = JSON.parse(fs.readFileSync(process.env.AGENT_SCAN_MANIFEST, 'utf8'));
const output = path.dirname(process.env.AGENT_SCAN_MANIFEST);
if (!input.isolated) throw new Error('An explicitly isolated environment is required');
for (const endpoint of [input.backend, input.frontend]) if (new URL(endpoint).hostname !== '127.0.0.1' || ['28888','28889'].includes(new URL(endpoint).port)) throw new Error('Refusing production development ports');
const checks=[];const pageErrors=[];let browser;
const redact=value=>String(value).replaceAll(input.password,'[REDACTED]');
async function api(method,route,token,body,status=200){
 const response=await fetch(input.backend+'/api'+route,{method,headers:{...(token?{Authorization:'Bearer '+token}:{}),...(body!==undefined?{'Content-Type':'application/json'}:{})},...(body!==undefined?{body:JSON.stringify(body)}:{}),signal:AbortSignal.timeout(10000)});
 assert.equal(response.status,status,`${method} ${route}: unexpected status`);return response.json();
}
function sql(query,params=[]){return JSON.parse(execFileSync('python3',['-c','import sqlite3,json,sys;c=sqlite3.connect(sys.argv[1]);print(json.dumps(c.execute(sys.argv[2],json.loads(sys.argv[3])).fetchall(),ensure_ascii=False))',input.db,query,JSON.stringify(params)],{encoding:'utf8'}));}
async function login(username){const context=await browser.newContext({viewport:{width:1280,height:900},locale:'zh-CN'});await context.addInitScript(()=>localStorage.setItem('nowen-i18n',JSON.stringify({state:{locale:'zh-CN'},version:0})));const page=await context.newPage();page.on('pageerror',error=>pageErrors.push(redact(error.message)));page.setDefaultTimeout(15000);await page.goto(input.frontend+'/login');await page.getByRole('textbox',{name:'用户名',exact:true}).fill(username);await page.getByLabel('密码',{exact:true}).fill(input.password);await page.getByRole('button',{name:'进入深空',exact:true}).click();await page.waitForURL(input.frontend+'/');const token=await page.evaluate(()=>JSON.parse(localStorage.getItem('nowen-auth')).state.token);return{context,page,token};}
async function open(page){await page.getByRole('button',{name:'打开观影助手',exact:true}).click();await page.getByRole('dialog',{name:'观影助手',exact:true}).waitFor();}
async function say(page,text){await page.getByRole('textbox',{name:'告诉助手你想做什么',exact:true}).fill(text);await page.getByRole('button',{name:'发送消息',exact:true}).click();await page.getByRole('status').filter({hasText:'正在处理'}).waitFor({state:'hidden'});}
async function completed(page,count){await page.getByText('任务预览与回执',{exact:true}).waitFor();const expand=page.getByRole('button',{name:'展开任务记录',exact:true});if(await expand.isVisible())await expand.click();await page.getByText(`查找完成，新增登记 ${count} 部电影。已有记录和原文件保持原状。`,{exact:true}).first().waitFor({state:'visible',timeout:20000});}
(async()=>{
 browser=await chromium.launch({headless:true});
 const owner=await login('scan-owner'),other=await login('scan-other'),viewer=await login('scan-viewer'),child=await login('scan-child');
 for(const account of [viewer,child]){await api('GET','/admin/agent/runs',account.token,undefined,403);await api('POST','/admin/agent/runs',account.token,{library_id:input.library_id},403);await api('POST','/agent/chat',account.token,{message:'查找新影片'},403);}
 checks.push('ordinary and child accounts cannot list, prepare or chat-trigger management tasks');
 const page=owner.page;await open(page);await say(page,'查找新影片');await page.getByRole('button',{name:'确认查找新影片',exact:true}).waitFor();assert.equal(sql('select count(*) from media where deleted_at is null')[0][0],2);
 let runs=(await api('GET','/admin/agent/runs',owner.token)).data;let draft=runs.find(run=>run.status==='draft');assert(draft);assert.deepEqual(draft.paths,[input.media_dir]);assert.equal((await api('GET','/admin/agent/runs',other.token)).data.length,0);
 await api('POST',`/admin/agent/runs/${draft.id}/confirm`,other.token,{approval_digest:draft.approval_digest},404);await api('POST',`/admin/agent/runs/${draft.id}/confirm`,owner.token,{approval_digest:'0'.repeat(64)},409);
 checks.push('chat produces a scoped preview without scanning; owner isolation and approval digest are enforced');
 await page.getByRole('button',{name:'取消预览',exact:true}).click();await page.getByText('预览已取消，未启动任务',{exact:true}).waitFor();await api('POST',`/admin/agent/runs/${draft.id}/confirm`,owner.token,{approval_digest:draft.approval_digest});assert.equal(sql('select count(*) from media where deleted_at is null')[0][0],2);
 checks.push('cancelled previews do not execute even when an old confirm request is replayed');
 await say(page,'查找新影片');await page.getByRole('button',{name:'确认查找新影片',exact:true}).waitFor();await page.setViewportSize({width:320,height:720});await page.screenshot({path:path.join(output,'scan-preview-mobile.png'),fullPage:true});assert(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth));
 draft=(await api('GET','/admin/agent/runs',owner.token)).data.find(run=>run.status==='draft');await page.getByRole('button',{name:'确认查找新影片',exact:true}).click();await api('POST',`/admin/agent/runs/${draft.id}/confirm`,owner.token,{approval_digest:draft.approval_digest});await completed(page,1);
 assert.equal(sql('select count(*) from media where deleted_at is null')[0][0],3);assert.deepEqual(sql('select title,file_size,rating from media where id=?',['scan-existing'])[0],['保留标题',123,9]);assert.equal(sql('select count(*) from media where id=? and deleted_at is null',['scan-offline'])[0][0],1);
 checks.push('explicit confirmation indexes a real MP4 once; existing metadata and offline records are preserved at 320px');
 await page.reload();await open(page);await completed(page,1);await page.screenshot({path:path.join(output,'scan-persisted-receipt.png'),fullPage:true});
 await say(page,'查找新影片');await page.getByRole('button',{name:'确认查找新影片',exact:true}).click();await completed(page,0);assert.equal(sql('select count(*) from media where deleted_at is null')[0][0],3);
 checks.push('receipts survive a page reload; a fresh repeat scan adds zero duplicate records');
 await say(page,'查找新影片');await page.getByRole('button',{name:'确认查找新影片',exact:true}).waitFor();fs.renameSync(input.media_dir,input.media_dir+'.offline');
 try{await page.getByRole('button',{name:'确认查找新影片',exact:true}).click();await page.getByText(/查找未完成，已登记 0 部电影/).first().waitFor();}finally{fs.renameSync(input.media_dir+'.offline',input.media_dir);}
 assert.equal(sql('select count(*) from media where deleted_at is null')[0][0],3);await page.getByRole('button',{name:'重新预览',exact:true}).first().click();await page.getByRole('button',{name:'确认查找新影片',exact:true}).waitFor();await page.getByRole('button',{name:'取消预览',exact:true}).click();
 checks.push('an unavailable folder produces a durable failure rather than success, and can be previewed again after reconnecting');
 assert.equal(crypto.createHash('sha256').update(fs.readFileSync(path.join(input.media_dir,'New Movie (2026).mp4'))).digest('hex'),input.source_sha256);
 checks.push('source media bytes remain unchanged by discovery and retries');
 assert.deepEqual(pageErrors,[]);
 for(const account of [owner,other,viewer,child])await account.context.close();
})().catch(error=>{console.error(redact(error.stack||error.message));process.exitCode=1}).finally(async()=>{await browser?.close();const report={passed:checks.length,checks,uses_real_backend:true,api_mocking:false};fs.writeFileSync(path.join(output,'report.json'),JSON.stringify(report,null,2),{mode:0o600});console.log(JSON.stringify(report,null,2));});
