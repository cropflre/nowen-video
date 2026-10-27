const { chromium } = require(process.env.PLAYWRIGHT_MODULE || 'playwright');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');
const { execFileSync } = require('node:child_process');
const outputDir = process.env.FAMILY_QA_DIR || fs.mkdtempSync(path.join(os.tmpdir(), 'nowen-family-qa-'));
fs.mkdirSync(outputDir, { recursive: true });
const fixture = path.join(outputDir, 'fixture.mp4');
execFileSync('ffmpeg', ['-loglevel','error','-f','lavfi','-i','color=c=navy:s=320x180:r=10','-t','120','-c:v','libx264','-pix_fmt','yuv420p','-movflags','+faststart','-y',fixture]);
const video = fs.readFileSync(fixture);
const base = process.env.FAMILY_BASE_URL || 'http://127.0.0.1:28917';
const movie = { id:'movie-1', library_id:'lib-1', title:'家庭示例电影', orig_title:'', media_type:'movie', duration:120, runtime:2, year:2026, rating:8.2, overview:'一家人的温暖故事。', genres:'家庭', poster_path:'fixture.jpg', backdrop_path:'fixture.jpg', video_codec:'h264', audio_codec:'', file_path:'/media/demo.mp4', file_size:12345, season_num:1, episode_num:2, subtitle_paths:'', trailer_url:'' };
const items = Array.from({length:50}, (_,i)=>({type:'movie', media:{...movie,id:i?'movie-'+(i+1):movie.id,title:i?'家庭影片 '+(i+1):movie.title}}));
const svg = Buffer.from('<svg xmlns="http://www.w3.org/2000/svg" width="640" height="360"><rect width="640" height="360" fill="#294c5b"/><circle cx="360" cy="130" r="65" fill="#f4c78e"/><path d="M0 360L240 180L440 360" fill="#528279"/></svg>');
const checks=[];
let browser;
(async()=>{
 const librarySource=await (await fetch(base+'/src/components/LibraryManager.tsx')).text();
 const mainSource=await (await fetch(base+'/src/main.tsx')).text();
 const appSource=await (await fetch(base+'/src/App.tsx')).text();
 const modulePath=(source,pattern)=>{const match=source.match(pattern);if(!match)throw new Error('Missing transformed test module');return match[1]};
 const toastURL=modulePath(librarySource,/from "([^"]*\/Toast\.tsx[^"]*)"/);
 const dialogURL=modulePath(librarySource,/from "([^"]*\/Dialog\.tsx[^"]*)"/);
 const reactURL=modulePath(librarySource,/from "([^"]*\/react\.js[^"]*)"/);
 const reactDOMURL=modulePath(mainSource,/from "([^"]*react-dom_client\.js[^"]*)"/);
 const authURL=modulePath(appSource,/from "([^"]*\/stores\/auth\.ts[^"]*)"/);
 browser=await chromium.launch({headless:true});
 async function setup({admin=false,child=false,mobile=false,empty=false,failHome=false,failBrowse=false,failInfo=false,unauthenticated=false,pairingState}={}) {
  const state={admin,child,mobile,empty,failHome,failBrowse,failInfo,policy:{child_mode:false,allowed_libraries:'',max_rating_level:'NC-17',daily_time_limit:0},rating:'',preferences:{large_text:false,subtitle_language:'auto',auto_play_next:true},invites:[],libraryCreates:0,scans:0};
  state.pairing=pairingState||{};state.devices=[{id:'device-current',name:'此浏览器',last_seen_at:new Date().toISOString()},{id:'device-living',name:'客厅设备',last_seen_at:new Date().toISOString()}];state.currentDevice='device-current';
  const ctx=await browser.newContext({viewport:mobile?{width:390,height:844}:{width:1440,height:1000},isMobile:mobile,hasTouch:mobile,locale:'zh-CN'});
  await ctx.addInitScript(({admin,child,unauthenticated})=>{
   if(!localStorage.getItem('nowen-auth'))localStorage.setItem('nowen-auth',JSON.stringify({state:unauthenticated?{token:null,isAuthenticated:false,user:null}:{token:'fixture-token',isAuthenticated:true,user:{id:admin?'admin':'viewer',username:'家人',role:admin?'admin':'user',child_mode:child}},version:0}));
   localStorage.setItem('nowen-i18n',JSON.stringify({state:{locale:'zh-CN'},version:0}));
  },{admin,child,unauthenticated});
  await ctx.route('**/api/**',async route=>{
   const request=route.request(); const url=new URL(request.url()); const p=url.pathname; if(!p.startsWith('/api/')) return route.continue();
   const json=(data,status=200)=>route.fulfill({status,contentType:'application/json',body:JSON.stringify(data)});
   if(p==='/api/ws') return route.abort();
   const fixtureUser=()=>request.headers()['authorization']==='Bearer paired-token'&&state.pairing.approvedUser?state.pairing.approvedUser:{id:admin?'admin':'viewer',username:'家人',role:admin?'admin':'user',child_mode:child,viewing_preferences:state.preferences};
   if(p==='/api/auth/login')return json({token:'fixture-token',user:fixtureUser()});
   if(p==='/api/auth/refresh')return json({error:'device revoked'},401);
   if(p==='/api/users/me/device-status')return state.forceDeviceRevoke?json({error:'device revoked'},401):json({data:{current_id:state.currentDevice}});
   if(p==='/api/users/me/devices')return json({data:state.devices,current_id:state.currentDevice});
   if(p==='/api/users/me/devices/revoke-all'){state.revokeAllCount=(state.revokeAllCount||0)+1;state.devices=[];return json({message:'revoked'});}
   if(p.startsWith('/api/users/me/devices/')&&request.method()==='DELETE'){
    if(state.failDeviceDelete)return json({error:'fixture failure'},503);
    state.deletedDevice=p.split('/').at(-1);state.devices=state.devices.filter(device=>device.id!==state.deletedDevice);return json({message:'revoked'});
   }
   if(p==='/api/auth/logout'){state.logoutCount=(state.logoutCount||0)+1;return json({message:'logout',server_revoked:true});}
   if(p==='/api/auth/device-pairing'){
    state.pairing.created=(state.pairing.created||0)+1;state.pairing.cancelled=false;state.pairing.approved=false;state.pairing.consumed=false;
    state.pairing.request={id:'pair-'+state.pairing.created,secret:'requester-secret-'+state.pairing.created,code:'ABCD-'+(2344+state.pairing.created),device_name:request.postDataJSON().device_name,expires_in_ms:300000};return json({data:state.pairing.request},201);
   }
   if(p==='/api/auth/device-pairing/cancel'){if(!state.pairing.consumed&&state.pairing.request?.id===request.postDataJSON().id)state.pairing.cancelled=true;return json({message:'cancelled'});}
   if(p==='/api/users/me/device-pairing/inspect')return json({data:{device_name:state.pairing.request?.device_name||'客厅平板',expires_at:new Date(Date.now()+300000).toISOString()}});
   if(p==='/api/users/me/device-pairing/approve'){state.pairing.approved=true;state.pairing.approvedUser=fixtureUser();return json({message:'approved'});}
   if(p==='/api/auth/device-pairing/status'){
    if(state.pairing.cancelled)return json({error:'expired'},404);
    if(state.pairing.consumed)return json({error:'consumed'},410);
    if(state.pairing.approved){state.pairing.consumed=true;return json({data:{status:'authorized',authorization:{token:'paired-token',user:state.pairing.approvedUser}}});}
    return json({data:{status:'pending'}});
   }
   const familyStatus=()=>({child_mode:child,allow_auto_play_next:Boolean(state.policy.allow_auto_play_next),day:'2026-10-27',timezone:'Asia/Shanghai',daily_limit_seconds:state.familyLimit||0,used_seconds:0,remaining_seconds:state.familyRemaining??state.familyLimit??0});
   if(p==='/api/users/me/family-controls') return json({data:familyStatus()});
   if(p==='/api/users/me/viewing/start') {
    state.viewingStarts=(state.viewingStarts||0)+1;
    if(state.denyStart) return json({error:'今天的观看时间已用完，休息一下吧',code:'FAMILY_TIME_LIMIT'},403);
    return json({data:{...familyStatus(),lease_id:'lease-'+state.viewingStarts,expires_in_ms:Math.min(15,state.familyRemaining??15)*1000}});
   }
   if(p==='/api/users/me/viewing/renew') {
    state.viewingRenews=(state.viewingRenews||0)+1;
    if(state.failRenew) return json({error:'暂时无法确认观看时间，请稍后重试'},503);
    return json({data:{...familyStatus(),lease_id:request.postDataJSON().lease_id,expires_in_ms:15000}});
   }
   if(p==='/api/users/me/viewing/stop') {state.viewingStops=(state.viewingStops||0)+1;return json({data:familyStatus()});}
   if(state.failHome && ['/api/media/recent/mixed','/api/media/continue','/api/recommend'].includes(p)) return json({error:'fixture failure'},503);
   if(state.failBrowse && p==='/api/media/mixed') return json({error:'fixture failure'},503);
   if(state.failInfo && /\/stream\/[^/]+\/info$/.test(p)) return json({error:'fixture failure'},503);
   if(p==='/api/users/me') {
    if(request.method()==='GET' && state.profileDelay) await new Promise(resolve=>setTimeout(resolve,state.profileDelay));
    if(request.method()==='PUT' && state.failPreferences) return json({error:'fixture preferences failure'},503);
    if(request.method()==='PUT') state.preferences=request.postDataJSON().viewing_preferences || state.preferences;
    return json({data:fixtureUser()});
   }
   if(p==='/api/admin/permissions/child-test') {
    if(request.method()==='PUT') state.policy=request.postDataJSON();
    return json({data:{id:'permission-1',user_id:'child-test',...state.policy}});
   }
   if(p==='/api/admin/users' && request.method()==='POST') {state.createdUser=request.postDataJSON();return json({data:{id:'new-child',...state.createdUser,created_at:new Date().toISOString()}},201);}
   if(p==='/api/admin/rating/movie-1') {
    if(request.method()==='PUT') {if(state.failRating) return json({error:'fixture rating failure'},503);state.rating=request.postDataJSON().level;}
    return json({data:{media_id:'movie-1',level:state.rating}});
   }
   if(p==='/api/admin/series/review-show/rating' && request.method()==='PUT') {state.seriesRating=request.postDataJSON().level;return json({count:2});}
   if(p==='/api/auth/status') return json({data:{initialized:true,registration_open:false,invite_required:true}});
   if(p==='/api/admin/invite-codes') {
    if(request.method()==='POST') { const invite={id:'invite-1',code:'family-once',max_uses:1,used_count:0,note:'家庭邀请',expires_at:new Date(Date.now()+86400000).toISOString()};state.invites.push(invite);return json({data:invite},201); }
    return json({data:state.invites});
   }
   if(p==='/api/admin/invite-codes/invite-1' && request.method()==='DELETE') {state.invites=[];return json({message:'revoked'});}
   if(p==='/api/capabilities') return json({data:{schema_version:1,profile:'lite',capabilities:{}}});
   if(p==='/api/health') return json({status:'ok',version:'test',server_name:'Family fixture'});
   if(p==='/api/media/recent/mixed') return json({data:state.empty?[]:items.slice(0,12)});
   if(p==='/api/media/continue') return json({data:state.empty?[]:[{id:'history-1',media_id:movie.id,media:movie,position:30,duration:120,completed:false}]});
   if(p==='/api/recommend') return json({data:state.empty?[]:items.slice(0,3).map(i=>({media:i.media,reason:'最近添加',score:1}))});
   if(p==='/api/libraries') {
    if(request.method()==='POST') {state.libraryCreates++;state.libraryBody=request.postDataJSON();return json({data:{id:'created-library',...state.libraryBody,path:state.libraryBody.paths[0],created_at:new Date().toISOString()}},201);}
    return json({data:state.libraryCreates?[{id:'created-library',...state.libraryBody,path:state.libraryBody.paths[0],created_at:new Date().toISOString()}]:state.empty?[]:[{id:'lib-1',name:'家里的影片',type:'movie',media_count:50}]});
   }
   if(p==='/api/libraries/created-library/scan') {state.scans++;return state.failScan?json({error:'fixture scan failure'},503):json({message:'started'});}

   if(p==='/api/media/mixed') return json({data:state.empty?[]:items,total:state.empty?0:50,movie_count:50,series_count:0});
   if(p==='/api/series') return json({data:[],total:0});
   if(child && state.revokeMedia && /^\/api\/media\/[^/]+$/.test(p)) return json({error:'content no longer approved'},404);
   if(/^\/api\/media\/[^/]+$/.test(p)) return json({data:{...movie,id:p.split('/').at(-1),...(state.episode?{media_type:'episode',series_id:'series-1',series:{id:'series-1',title:'家庭剧场'}}:{})}});
   if(p.endsWith('/enhanced')) return json({data:{tech_specs:null,file_info:null,library:null,playback_stats:null}});
   if(p.endsWith('/highlights')) return json({data:{items:[],highlights:[],status:'pending'}});
   if(p.endsWith('/tracks')) return json({data:state.episode?{embedded:[{index:0,language:'eng',bitmap:false}],external:[{path:'/subs/family.zh.vtt',filename:'family.zh.vtt',language:'zh'}]}:{embedded:[],external:[]}});
   if(p==='/api/series/series-1/next') return json({data:{...movie,id:'movie-2',media_type:'episode',series_id:'series-1',episode_num:3}});
   if(p==='/api/subtitle/external' || /\/extract\/\d+$/.test(p)) {state.subtitleFetches=(state.subtitleFetches||0)+1;return route.fulfill({contentType:'text/vtt',body:'WEBVTT\n\n00:00:29.000 --> 00:01:30.000\n中文字幕示例\n'});}

   if(p.endsWith('/check')) return json({data:true});
   if(p==='/api/users/me/favorites') {
    const title=state.favoriteTitle || movie.title;
    state.favoriteRequests=(state.favoriteRequests||0)+1;
    if(state.delayFavorites && state.favoriteRequests===1) await new Promise(resolve=>setTimeout(resolve,600));
    return json({data:[{id:'fav-1',media:{...movie,title},media_id:movie.id}],total:1});
   }
   if(p.includes('/users/me/progress/')) return json({data:{position:30,duration:120,completed:false}});
   if(/\/stream\/[^/]+\/info$/.test(p)) return json({data:{duration:120,can_direct_play:true,video_codec:'h264',audio_codec:'',file_ext:'.mp4',playback_plan:{method:'direct',url:'/api/stream/movie-1/direct',session_required:false}}});
   if(p.endsWith('/direct')) {
    const range=request.headers()['range'];let start=0,end=video.length-1;
    if(range){const m=range.match(/bytes=(\d+)-(\d*)/);if(m){start=+m[1];if(m[2])end=Math.min(+m[2],end);}}
    return route.fulfill({status:range?206:200,contentType:'video/mp4',headers:{'Accept-Ranges':'bytes',...(range?{'Content-Range':`bytes ${start}-${end}/${video.length}`}:{})},body:video.subarray(start,end+1)});
   }
   if(/\/(poster|backdrop)$/.test(p)) return route.fulfill({contentType:'image/svg+xml',body:svg});
   return json({data:[],total:0});
  });
  const page=await ctx.newPage();const errors=[];page.on('pageerror',e=>{errors.push(e.message);console.error('PAGEERROR',e.message)});
  return {ctx,page,state,errors};
 }
 async function visible(page,text){await page.getByText(text,{exact:true}).first().waitFor({state:'visible',timeout:10000});}
 let env,page;
 if(!process.env.FAMILY_CHILDREN_ONLY&&!process.env.FAMILY_DEVICES_ONLY){
 env=await setup();page=env.page;
 await page.goto(base);await visible(page,'继续观看');
 assert.deepEqual(await page.locator('.nv-rail-section-links .nv-rail-label').allTextContents(),['首页','找片','想看','我的']);
 const continueBox=await page.getByText('继续观看',{exact:true}).first().boundingBox();const heroBox=await page.locator('.nv-hero-carousel').boundingBox();assert(continueBox.y<heroBox.y);
 assert.equal(await page.locator('.nv-home-library-section').count(),0);
 await page.screenshot({path:path.join(outputDir, 'desktop.png'),fullPage:true});
 const firstTitle=await page.locator('.nv-hero-carousel').getAttribute('aria-label');
 await page.waitForTimeout(7500);assert.equal(await page.locator('.nv-hero-carousel').getAttribute('aria-label'),firstTitle);
 await page.getByRole('textbox',{name:'搜索片名'}).fill('家庭');await page.keyboard.press('ArrowLeft');assert.equal(await page.locator('.nv-hero-carousel').getAttribute('aria-label'),firstTitle);
 checks.push('desktop navigation, resume-first layout, stationary recommendations, search arrow keys');
 await page.getByRole('button',{name:'帮我选一部',exact:true}).click();const chooser=page.getByRole('dialog',{name:'试试这一部'});await chooser.waitFor();assert(!page.url().includes('/play/'));assert.equal(await page.locator('video').count(),0);
 const choice=await chooser.locator('h2').last().textContent();await chooser.getByRole('button',{name:'换一部',exact:true}).click();assert.notEqual(await chooser.locator('h2').last().textContent(),choice);
 await page.keyboard.press('Escape');assert(await page.getByRole('button',{name:'帮我选一部',exact:true}).evaluate(e=>e===document.activeElement));checks.push('pick-one suggests without autoplay, changes candidates, and restores keyboard focus');

 await page.getByRole('link',{name:'继续播放 家庭示例电影',exact:true}).click();
 await page.waitForFunction(()=>document.querySelector('video')?.currentTime>=30);assert((await page.locator('video').evaluate(v=>v.currentTime))<40);
 assert.equal(await page.getByText('播放信息',{exact:true}).count(),0);
 checks.push('real MP4 resumes saved position; viewer sees no playback diagnostics');
 async function failVideo() {
  await page.locator('video').evaluate(v=>{
   Object.defineProperty(v,'error',{configurable:true,value:{code:2,MEDIA_ERR_NETWORK:2,MEDIA_ERR_ABORTED:1,MEDIA_ERR_DECODE:3,MEDIA_ERR_SRC_NOT_SUPPORTED:4,message:'fixture network failure'}});
   v.dispatchEvent(new Event('error'));
  });
 }
 await failVideo();await page.waitForFunction(()=>{const v=document.querySelector('video');return v&&v.readyState>=2&&!v.error&&v.currentTime>=30});
 await failVideo();await visible(page,'这部影片暂时无法播放');
 await page.waitForTimeout(800);await visible(page,'这部影片暂时无法播放');
 await page.getByRole('button',{name:'重试',exact:true}).click();await page.waitForFunction(()=>document.querySelector('video')?.currentTime>=30);
 await failVideo();await page.waitForFunction(()=>{const v=document.querySelector('video');return v&&v.readyState>=2&&!v.error&&v.currentTime>=30});
 checks.push('network auto-retry is bounded, preserves position, and works again after manual retry');

 await page.goto(base+'/play/movie-1?start=0');await page.waitForFunction(()=>document.querySelector('video')?.readyState>=2);assert((await page.locator('video').evaluate(v=>v.currentTime))<5);
 checks.push('explicit restart begins at zero');
 await page.goto(base+'/media/movie-1');await page.getByRole('link',{name:'继续播放 家庭示例电影'}).waitFor();
 await page.getByRole('tab',{name:'简介',exact:true}).waitFor(); assert.equal(await page.getByRole('tab').count(),4);assert.equal(await page.getByRole('tab',{name:'技术规格'}).count(),0);
 await page.getByRole('button',{name:'更多操作'}).click();await visible(page,'从头播放');checks.push('detail has four viewing tabs and a restart action');await page.keyboard.press('Escape');await page.screenshot({path:path.join(outputDir, 'detail.png'),fullPage:true});
 await page.goto(base+'/favorites');await visible(page,'家庭示例电影');checks.push('existing favorite appears in watchlist');
 env.state.failInfo=true;await page.goto(base+'/play/movie-1');await visible(page,'暂时无法准备这部影片');assert(page.url().includes('/play/movie-1'));
 env.state.failInfo=false;await page.getByRole('button',{name:'重试',exact:true}).click();await page.waitForFunction(()=>document.querySelector('video')?.currentTime>=30);checks.push('playback setup failure stays in place and retries successfully');
 assert.deepEqual(env.errors,[]);await env.ctx.close();
 env=await setup({empty:true});page=env.page;await page.goto(base);await visible(page,'还没有可看的影片');assert.equal(await page.getByRole('link',{name:'添加影片文件夹',exact:true}).count(),0);checks.push('empty viewer home does not direct users to admin');await env.ctx.close();
 env=await setup({admin:true,empty:true});page=env.page;await page.goto(base);await page.getByRole('link',{name:'添加影片文件夹',exact:true}).waitFor();assert.equal(await page.getByRole('link',{name:'添加影片文件夹',exact:true}).getAttribute('href'),'/admin#library');checks.push('empty admin home points to the actual library settings tab');await env.ctx.close();
 env=await setup({failHome:true});page=env.page;await page.goto(base);await visible(page,'暂时无法加载影片');assert.equal(await page.getByText('还没有可看的影片',{exact:true}).count(),0);env.state.failHome=false;await page.getByRole('button',{name:'重试',exact:true}).last().click();await visible(page,'继续观看');checks.push('home network failure is distinct from empty content and can recover');await env.ctx.close();
 env=await setup({failBrowse:true});page=env.page;await page.goto(base+'/browse');await visible(page,'暂时无法加载影片');env.state.failBrowse=false;await page.getByRole('button',{name:'重试',exact:true}).click();await page.locator('.nv-media-card').first().waitFor();checks.push('browse request failure has a working retry instead of infinite skeleton');await env.ctx.close();
 env=await setup({mobile:true});page=env.page;await page.goto(base);await visible(page,'继续观看');assert.deepEqual(await page.locator('.nv-mobile-nav-label').allTextContents(),['首页','找片','想看','我的']);
 await page.screenshot({path:path.join(outputDir, 'mobile.png'),fullPage:true});
 assert(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth));
 await page.goto(base+'/browse');await page.locator('.nv-media-card').first().waitFor();assert.equal(await page.locator('.nv-media-card-play').first().evaluate(e=>getComputedStyle(e).opacity),'1');
 await page.locator('#main-scroll-container').evaluate(e=>e.scrollTop=700);await page.waitForTimeout(200);
 const before=await page.locator('#main-scroll-container').evaluate(e=>e.scrollTop);
 await page.locator('.nv-media-card-play').nth(8).click();await page.waitForFunction(()=>document.querySelector('video')?.readyState>=2);await page.goBack();await page.locator('.nv-media-card').first().waitFor();
 await page.waitForTimeout(300);const after=await page.locator('#main-scroll-container').evaluate(e=>e.scrollTop);assert(after>0,`scroll restore lost position: ${before} -> ${after}`);
 await page.setViewportSize({width:320,height:720}); assert(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth)); assert.deepEqual(env.errors,[]);checks.push('mobile navigation, no horizontal overflow, visible touch playback and return scroll restore');await env.ctx.close();
 env=await setup({mobile:true});page=env.page;await page.goto(base+'/my');await page.getByRole('switch',{name:'大字模式',exact:true}).waitFor();
 env.state.failPreferences=true;await page.getByRole('switch',{name:'大字模式',exact:true}).click();await visible(page,'偏好未能保存，请重试');assert.equal(env.state.preferences.large_text,false);assert.equal(await page.getByRole('switch',{name:'大字模式',exact:true}).isChecked(),false);env.state.failPreferences=false;
 await page.getByRole('switch',{name:'大字模式',exact:true}).check();await page.waitForFunction(()=>document.documentElement.dataset.largeText==='true');assert.equal(env.state.preferences.large_text,true);
 await page.reload();await page.waitForFunction(()=>document.documentElement.dataset.largeText==='true');
 await page.getByRole('switch',{name:'自动播放下一集',exact:true}).uncheck();await page.waitForFunction(()=>!document.querySelector('[aria-label="自动播放下一集"]').checked);assert.equal(env.state.preferences.auto_play_next,false);
 await page.getByRole('combobox',{name:'默认字幕',exact:true}).selectOption('off');await page.waitForTimeout(100);assert.equal(env.state.preferences.subtitle_language,'off');
 await page.setViewportSize({width:320,height:720});assert(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth));await page.screenshot({path:path.join(outputDir,'large-text.png'),fullPage:true});assert.deepEqual(env.errors,[]);checks.push('large text, subtitle and autoplay preferences persist at 320px; failed saves restore the previous state');
 env.state.episode=true;await page.goto(base+'/play/movie-1');await page.waitForFunction(()=>document.querySelector('video')?.currentTime>=30);await page.waitForTimeout(150);assert.equal(env.state.subtitleFetches||0,0);
 await page.locator('video').evaluate(v=>v.dispatchEvent(new Event('ended')));await page.waitForTimeout(5500);assert(page.url().endsWith('/play/movie-1'));
 env.state.preferences.subtitle_language='zh';env.state.profileDelay=500;await page.reload();await visible(page,'中文字幕示例');assert(env.state.subtitleFetches>0);checks.push('player honors subtitles Off, preferred Chinese subtitles and disabled automatic next episode');await env.ctx.close();
 env=await setup({admin:true});page=env.page;await page.goto(base+'/my');await page.getByRole('button',{name:'创建家庭邀请',exact:true}).waitFor();await page.getByRole('button',{name:'创建家庭邀请',exact:true}).click();
 await page.getByRole('textbox',{name:'家庭邀请链接',exact:true}).waitFor();const invitationURL=await page.getByRole('textbox',{name:'家庭邀请链接',exact:true}).inputValue();assert(invitationURL.endsWith('/login#invite=family-once'));assert.equal(env.state.invites.length,1);
 await page.getByRole('button',{name:'撤销邀请',exact:true}).click();await page.getByRole('textbox',{name:'家庭邀请链接',exact:true}).waitFor({state:'hidden'});assert.equal(env.state.invites.length,0);
 await page.goto(invitationURL);await visible(page,'家人邀请你加入。设置账号和密码后，就能开始观看。');assert.equal(await page.getByRole('textbox',{name:'邀请码',exact:true}).inputValue(),'family-once');assert.deepEqual(env.errors,[]);checks.push('family invite creates a one-use link, revokes it, and pre-fills registration with public signups closed');await env.ctx.close();
 env=await setup({admin:true,empty:true});page=env.page;
 await page.route('**/family-library-test',route=>route.fulfill({contentType:'text/html',body:`<!doctype html><html><head><meta charset="UTF-8"></head><body><div id="root"></div><script type="module">
 import RefreshRuntime from '/@react-refresh';RefreshRuntime.injectIntoGlobalHook(window);window.$RefreshReg$=()=>{};window.$RefreshSig$=()=>type=>type;window.__vite_plugin_react_preamble_installed__=true;
 </script><script type="module">
 import React from '${reactURL}';const {useState}=React;
 import ReactDOM from '${reactDOMURL}';const {createRoot}=ReactDOM;
 import LibraryManager from '/src/components/LibraryManager.tsx';
 import {ToastProvider} from '${toastURL}';
 import {DialogProvider} from '${dialogURL}';
 import '/src/styles/global.css';import '/src/styles/fluent.css';import '/src/styles/design-system.css';import '/src/styles/admin-design-system.css';import '/src/styles/ui.css';
 function Harness(){const [libraries,setLibraries]=useState([]);const [scanning,setScanning]=useState(new Set());return React.createElement(LibraryManager,{libraries,setLibraries,scanning,setScanning,scanProgress:{},scrapeProgress:{},scanPhase:{}})}
 createRoot(document.getElementById('root')).render(React.createElement(ToastProvider,null,React.createElement(DialogProvider,null,React.createElement(Harness))));
 </script></body></html>`}));
 await page.goto(base+'/family-library-test');await page.getByRole('button',{name:'新增媒体库',exact:true}).first().click();
 const libraryModal=page.getByRole('dialog',{name:'添加影片文件夹',exact:true});await libraryModal.waitFor();assert.equal(await libraryModal.getByPlaceholder('请输入媒体库名称').inputValue(),'电影');
 await libraryModal.getByRole('button',{name:/^电视节目/}).click();assert.equal(await libraryModal.getByPlaceholder('请输入媒体库名称').inputValue(),'电视节目');
 await libraryModal.getByPlaceholder('请输入媒体库名称').fill('家里的故事');await libraryModal.getByRole('button',{name:/^混合影片/}).click();assert.equal(await libraryModal.getByPlaceholder('请输入媒体库名称').inputValue(),'家里的故事');
 await libraryModal.getByPlaceholder(/主路径/).fill('/media/family');env.state.failScan=true;
 await libraryModal.getByRole('button',{name:'添加并查找影片',exact:true}).evaluate(button=>{button.click();button.click()});
 await libraryModal.waitFor({state:'hidden'});assert.equal(env.state.libraryCreates,1);assert.equal(env.state.scans,1);assert.equal(env.state.libraryBody.organize_output_dir,'');assert.equal(env.state.libraryBody.auto_organize_mode,'rule_only');
 await visible(page,'文件夹已添加，但未能开始查找影片。请在文件夹菜单中重试扫描。');assert.deepEqual(env.errors,[]);checks.push('folder setup suggests names, preserves edits, prevents duplicate creation, starts scanning and retains success after scan failure');await env.ctx.close();
 env=await setup();page=env.page;env.state.favoriteTitle='上个账号的影片';env.state.delayFavorites=true;
 await page.goto(base+'/favorites');for(let tries=0;!env.state.favoriteRequests&&tries<500;tries++) await new Promise(resolve=>setTimeout(resolve,10));assert(env.state.favoriteRequests,'favorites request did not start');env.state.favoriteTitle='当前账号的影片';
 await page.evaluate(async(authURL)=>{const {useAuthStore}=await import(authURL);useAuthStore.getState().setAuth('second-token',{id:'second-viewer',username:'第二位家人',role:'user'})},authURL);
 await visible(page,'当前账号的影片');await page.waitForTimeout(800);assert.equal(await page.getByText('上个账号的影片',{exact:true}).count(),0);
 env.state.favoriteTitle='重新登录后的影片';await page.evaluate(async(authURL)=>{const {useAuthStore}=await import(authURL);useAuthStore.getState().setAuth('fixture-token',{id:'viewer',username:'家人',role:'user'})},authURL);
 await visible(page,'重新登录后的影片');assert.equal(await page.getByText('当前账号的影片',{exact:true}).count(),0);assert.deepEqual(env.errors,[]);checks.push('switching accounts isolates cached favorites and rejects delayed responses from the previous account');await env.ctx.close();
 }
 if(!process.env.FAMILY_DEVICES_ONLY){
 env=await setup({child:true,mobile:true});page=env.page;let childSockets=0;page.on('websocket',socket=>{if(new URL(socket.url()).pathname==='/api/ws')childSockets++});
 await page.goto(base+'/my');await page.locator('p').filter({hasText:/^儿童空间$/}).waitFor();assert.equal(await page.getByRole('switch',{name:'自动播放下一集',exact:true}).isDisabled(),true);await visible(page,'今天的观看时间');assert.equal(await page.getByRole('link',{name:/观影统计/}).count(),0);assert.equal(await page.getByRole('link',{name:/播放列表/}).count(),0);assert.equal(await page.getByRole('button',{name:'创建家庭邀请',exact:true}).count(),0);
 await page.goto(base+'/admin');await visible(page,'继续观看');assert.equal(new URL(page.url()).pathname,'/');assert.equal(childSockets,0);assert.deepEqual(env.errors,[]);checks.push('child account shows its own space, hides adult management and unsupported menus, and does not connect to global events');
 await page.clock.install();await page.goto(base+'/play/movie-1');await page.waitForFunction(()=>document.querySelector('video')?.currentTime>=30);env.state.revokeMedia=true;await page.clock.fastForward(15_000);await visible(page,'这部影片暂未向儿童账号开放');assert.equal(await page.locator('video').count(),0);assert.deepEqual(env.errors,[]);checks.push('web player revalidates child visibility and stops after a parent withdraws approval');await env.ctx.close();
 env=await setup({admin:true});page=env.page;
 await page.route('**/family-children-test',route=>route.fulfill({contentType:'text/html',body:`<!doctype html><html><head><meta charset="UTF-8"></head><body><div id="root"></div><script type="module">
 import RefreshRuntime from '/@react-refresh';RefreshRuntime.injectIntoGlobalHook(window);window.$RefreshReg$=()=>{};window.$RefreshSig$=()=>type=>type;window.__vite_plugin_react_preamble_installed__=true;
 </script><script type="module">
 import React from '${reactURL}';const {useState}=React;import ReactDOM from '${reactDOMURL}';const {createRoot}=ReactDOM;
 import UsersTab from '/src/components/admin/UsersTabContent.tsx';import {ToastProvider} from '${toastURL}';import {DialogProvider} from '${dialogURL}';
 import '/src/styles/global.css';import '/src/styles/fluent.css';import '/src/styles/design-system.css';import '/src/styles/admin-design-system.css';import '/src/styles/ui.css';
 function Harness(){const [users,setUsers]=useState([{id:'child-test',username:'小朋友',role:'user',created_at:'2026-10-26T00:00:00Z'}]);return React.createElement(UsersTab,{users,setUsers})}
 createRoot(document.getElementById('root')).render(React.createElement(ToastProvider,null,React.createElement(DialogProvider,null,React.createElement(Harness))));
 </script></body></html>`}));
 await page.goto(base+'/family-children-test');await page.getByRole('button',{name:'权限',exact:true}).click();await page.getByRole('switch',{name:'儿童空间',exact:true}).check();
 assert.equal(await page.locator('select').inputValue(),'G');assert.equal(await page.locator('select option[value="NC-17"]').count(),0);
 await page.getByRole('button',{name:'家里的影片',exact:true}).click();await page.locator('select').selectOption('PG');await page.getByRole('switch',{name:'允许儿童自动连播',exact:true}).check();await page.screenshot({path:path.join(outputDir,'child-permissions.png'),fullPage:true});await page.getByRole('button',{name:'保存权限',exact:true}).click();await visible(page,'权限已保存，该账号需要重新登录');assert.equal(env.state.policy.child_mode,true);assert.equal(env.state.policy.allow_auto_play_next,true);assert.equal(env.state.policy.allowed_libraries,'lib-1');assert.equal(env.state.policy.max_rating_level,'PG');
 checks.push('parent enables child policy with explicit libraries and a child-appropriate rating; adult ratings are unavailable');
 await page.getByRole('button',{name:'新建用户',exact:true}).click();const newChild=page.getByRole('dialog',{name:'创建新用户',exact:true});await newChild.waitFor();await newChild.getByPlaceholder('至少 3 位',{exact:true}).fill('new-child');await newChild.getByPlaceholder('至少 6 位，首次登录须修改',{exact:true}).fill('child-password');await newChild.getByRole('switch',{name:'创建为儿童账号',exact:true}).check();await newChild.getByRole('button',{name:'创建',exact:true}).click();await newChild.waitFor({state:'hidden'});assert.equal(env.state.createdUser.child_mode,true);assert.equal(env.state.createdUser.role,'user');assert.deepEqual(env.errors,[]);checks.push('parent can create a child account that starts with no permitted library');await env.ctx.close();
 env=await setup({admin:true});page=env.page;await page.goto(base+'/media/movie-1');await page.getByText('更多信息与片段',{exact:true}).click();await page.getByRole('combobox',{name:'家庭内容分级',exact:true}).selectOption('G');env.state.failRating=true;await page.getByRole('button',{name:'保存分级',exact:true}).click();await visible(page,'分级未能保存，请重试');assert.equal(env.state.rating,'');env.state.failRating=false;await page.getByRole('button',{name:'保存分级',exact:true}).click();await visible(page,'家庭分级已更新');assert.equal(env.state.rating,'G');await page.getByRole('combobox',{name:'家庭内容分级',exact:true}).selectOption('');await page.getByRole('button',{name:'保存分级',exact:true}).click();await page.waitForTimeout(150);assert.equal(env.state.rating,'');assert.deepEqual(env.errors,[]);checks.push('parent reviews or withdraws movie ratings and failed saves do not claim success');await env.ctx.close();
 env=await setup({child:true});page=env.page;env.state.familyLimit=60;env.state.familyRemaining=2;
 await page.clock.install();await page.goto(base+'/play/movie-1');await page.waitForFunction(()=>document.querySelector('video')?.currentTime>=30);await page.clock.fastForward(2100);await visible(page,'今天的观看时间已用完，休息一下吧');assert.equal(await page.locator('video').count(),0);assert.equal(env.state.viewingStarts,1);assert.deepEqual(env.errors,[]);checks.push('limited child stops at the final server grant deadline even with buffered video');await page.screenshot({path:path.join(outputDir,'daily-limit.png'),fullPage:true});await env.ctx.close();
 env=await setup({child:true});page=env.page;env.state.familyLimit=60;await page.goto(base+'/play/movie-1');await page.waitForFunction(()=>document.querySelector('video')?.currentTime>=30);await page.locator('video').evaluate(v=>v.pause());await page.waitForFunction(()=>document.querySelector('video')?.paused);for(let tries=0;!env.state.viewingStops&&tries<100;tries++) await new Promise(resolve=>setTimeout(resolve,10));assert.equal(env.state.viewingStops,1);await page.locator('video').evaluate(v=>v.play());await page.waitForFunction(()=>!document.querySelector('video')?.paused);for(let tries=0;env.state.viewingStarts<2&&tries<100;tries++) await new Promise(resolve=>setTimeout(resolve,10));assert.equal(env.state.viewingStarts,2);await page.locator('video').evaluate(v=>v.pause());for(let tries=0;env.state.viewingStops<2&&tries<100;tries++) await new Promise(resolve=>setTimeout(resolve,10));assert.equal(env.state.viewingStops,2);await page.keyboard.press('ArrowRight');await page.waitForFunction(()=>document.querySelector('video')?.currentTime>=40&&!document.querySelector('video')?.paused);assert.equal(env.state.viewingStarts,3);assert.deepEqual(env.errors,[]);checks.push('pause releases time; resume and paused seeking each obtain a new grant before playback');await env.ctx.close();
 env=await setup({child:true});page=env.page;env.state.familyLimit=60;env.state.denyStart=true;await page.goto(base+'/play/movie-1');await visible(page,'今天的观看时间已用完，休息一下吧');assert.equal(await page.locator('video').count(),0);checks.push('exhausted child budget prevents creating a player before any media loads');await env.ctx.close();
 env=await setup({child:true});page=env.page;env.state.familyLimit=60;env.state.failRenew=true;await page.clock.install();await page.goto(base+'/play/movie-1');await page.waitForFunction(()=>document.querySelector('video')?.currentTime>=30);await page.clock.fastForward(5100);await visible(page,'暂时无法确认观看时间，请稍后重试');assert.equal(await page.locator('video').count(),0);assert.equal(env.state.viewingRenews,1);checks.push('failed authorization renewal stops the limited child player without silently continuing');await env.ctx.close();
 }
 if(!process.env.FAMILY_CHILDREN_ONLY){
 const paired={};let requester=await setup({unauthenticated:true,pairingState:paired,mobile:true});page=requester.page;await page.goto(base+'/login');await page.getByRole('button',{name:'用已登录设备授权',exact:true}).click();await page.getByRole('textbox',{name:'设备名称',exact:true}).fill('客厅平板');await page.getByRole('button',{name:'生成配对码',exact:true}).click();await visible(page,'ABCD-2345');
 assert((await page.getByRole('img',{name:'用已登录手机扫描的设备配对二维码',exact:true}).getAttribute('src')).startsWith('data:image/svg+xml'));assert.equal(paired.approved,false);assert(!page.url().includes(paired.request.secret));await page.screenshot({path:path.join(outputDir,'device-pairing.png'),fullPage:true});
 const approver=await setup({admin:true,pairingState:paired});const approval=approver.page;await approval.goto(base+'/pair#code=ABCD-2345');await visible(approval,'ABCD-2345');await visible(approval,'客厅平板');assert.equal(paired.approved,false);await visible(approval,'这会让新设备拥有管理员权限。给家人使用时，请先登录对应家人的账号。');await approval.getByRole('button',{name:'确认授权此账号',exact:true}).click();await visible(approval,'已确认，请回到新设备。配对只可使用一次。');
 await requester.page.waitForURL(base+'/');assert.equal(paired.consumed,true);assert.equal(await requester.page.evaluate(()=>JSON.parse(localStorage.getItem('nowen-auth')).state.token),'paired-token');assert.equal(await requester.page.evaluate(()=>JSON.parse(localStorage.getItem('nowen-auth')).state.user.role),'admin');assert.deepEqual(requester.errors,[]);assert.deepEqual(approver.errors,[]);checks.push('new device renders a local QR code; a second logged-in device reviews identity and explicitly authorizes one-time login');await requester.ctx.close();await approver.ctx.close();
 env=await setup({unauthenticated:true});page=env.page;await page.clock.install();await page.goto(base+'/login');await page.getByRole('button',{name:'用已登录设备授权',exact:true}).click();await page.getByRole('button',{name:'生成配对码',exact:true}).click();await visible(page,'ABCD-2345');await page.clock.fastForward(300100);await visible(page,'配对码已失效，请重新生成');await page.getByRole('button',{name:'重新生成配对码',exact:true}).click();await visible(page,'ABCD-2346');assert.equal(env.state.pairing.created,2);await page.getByRole('button',{name:'返回账号登录',exact:true}).click();await page.getByRole('textbox',{name:'用户名',exact:true}).waitFor();for(let tries=0;!env.state.pairing.cancelled&&tries<100;tries++)await new Promise(resolve=>setTimeout(resolve,10));assert.equal(env.state.pairing.cancelled,true);assert.deepEqual(env.errors,[]);checks.push('expired pairing stops polling, regenerates a fresh request and cancels when returning to password login');await env.ctx.close();
 env=await setup();page=env.page;await page.goto(base+'/my');await visible(page,'客厅设备');await page.getByRole('button',{name:'退出 客厅设备',exact:true}).click();await page.getByRole('button',{name:'取消',exact:true}).click();assert.equal(env.state.deletedDevice,undefined);env.state.failDeviceDelete=true;await page.getByRole('button',{name:'退出 客厅设备',exact:true}).click();await page.getByRole('button',{name:'确认退出',exact:true}).click();await visible(page,'设备未能退出，请重试');await visible(page,'客厅设备');env.state.failDeviceDelete=false;await page.getByRole('button',{name:'退出 客厅设备',exact:true}).click();await page.getByRole('button',{name:'确认退出',exact:true}).click();await visible(page,'设备已退出');assert.equal(env.state.deletedDevice,'device-living');assert.equal(env.state.devices.length,1);assert.equal(await page.evaluate(()=>JSON.parse(localStorage.getItem('nowen-auth')).state.isAuthenticated),true);await page.screenshot({path:path.join(outputDir,'login-devices.png'),fullPage:true});await page.getByRole('button',{name:'退出全部设备',exact:true}).click();await page.getByRole('button',{name:'确认退出',exact:true}).click();await page.waitForURL(base+'/login');assert.equal(env.state.revokeAllCount,1);assert.equal(await page.evaluate(()=>JSON.parse(localStorage.getItem('nowen-auth')).state.isAuthenticated),false);assert.deepEqual(env.errors,[]);checks.push('device removal confirms intent, preserves state on failure, leaves other sessions active and signs out after all-device revocation');await env.ctx.close();
 env=await setup();page=env.page;await page.goto(base+'/my');await visible(page,'登录设备');env.state.forceDeviceRevoke=true;await page.evaluate(()=>window.dispatchEvent(new Event('focus')));await page.waitForURL(base+'/login');await page.getByRole('button',{name:'用已登录设备授权',exact:true}).waitFor();assert.equal(await page.evaluate(()=>JSON.parse(localStorage.getItem('nowen-auth')).state.isAuthenticated),false);assert.deepEqual(env.errors,[]);checks.push('an already-open page detects device revocation on focus and cannot restore access through refresh');await env.ctx.close();
 env=await setup({unauthenticated:true});page=env.page;await page.goto(base+'/pair#code=ABCD-2345');await page.getByRole('link',{name:'登录后确认',exact:true}).click();await page.getByRole('textbox',{name:'用户名',exact:true}).fill('family');await page.getByLabel('密码',{exact:true}).fill('family-password');await page.getByRole('button',{name:'进入深空',exact:true}).click();await visible(page,'ABCD-2345');await page.getByRole('button',{name:'确认授权此账号',exact:true}).waitFor();assert.equal(env.state.pairing.approved,undefined);assert.deepEqual(env.errors,[]);checks.push('QR approval survives password login and still requires a separate explicit review and confirmation');await env.ctx.close();
 }
 const report = {passed:checks.length,checks,outputDir};
 fs.writeFileSync(path.join(outputDir, 'report.json'), JSON.stringify(report,null,2));
 console.log(JSON.stringify(report,null,2));
})().catch(e=>{console.error(e);process.exitCode=1}).finally(async()=>{await browser?.close()});
