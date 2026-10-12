const {test}=require('node:test');
const assert=require('node:assert/strict');
const fs=require('node:fs'),path=require('node:path'),os=require('node:os');
const {spawn}=require('node:child_process');
const puppeteer=require('puppeteer');
const root=path.join(__dirname,'..');
const sleep=ms=>new Promise(resolve=>setTimeout(resolve,ms));
test('Deals recovers from navigation during loading and keeps cards during refresh', {timeout:120000},async()=>{
 const directory=fs.mkdtempSync(path.join(os.tmpdir(),'ecc-deals-browser-')),base='http://127.0.0.1:17982';
 const server=spawn(path.join(root,'build/ecc-crm'),[],{cwd:root,env:{...process.env,PORT:'17982',DATABASE_PATH:path.join(directory,'crm.sqlite3'),JWT_SECRET:'deals-browser-only',APP_URL:base,ALLOW_SIGNUP:'false',OPENAI_API_KEY:'',SMTP_HOST:''},stdio:'pipe'});
 let log='';server.stdout.on('data',chunk=>log+=chunk);server.stderr.on('data',chunk=>log+=chunk);let browser;
 try{
  for(let i=0;i<100;i++){try{if((await fetch(base+'/api/health')).ok)break}catch{}if(server.exitCode!==null)throw Error(log);await sleep(100)}
  browser=await puppeteer.launch({executablePath:process.env.CHROME_PATH||path.join(os.homedir(),'.cache/ms-playwright/chromium-1234/chrome-linux64/chrome'),headless:true,pipe:true,args:['--no-sandbox','--disable-dev-shm-usage']});
  const page=await browser.newPage();page.setDefaultTimeout(30000);await page.setViewport({width:390,height:844});
  let boardRequests=0,delay=1200,failNext=false;const requests=[];
  await page.setRequestInterception(true);page.on('request',request=>{
   const url=new URL(request.url());
   if(url.pathname==='/api/deals'&&url.searchParams.get('board')==='page'){
    boardRequests++;requests.push(request.url());
    const fail=failNext;failNext=false;
    setTimeout(()=>{if(fail)request.respond({status:503,contentType:'application/json',body:JSON.stringify({error:'Temporary test failure'})}).catch(()=>{});else request.continue().catch(()=>{})},delay);
   }else request.continue().catch(()=>{});
  });
  await page.goto(base,{waitUntil:"domcontentloaded"});await page.waitForSelector('#email');
  await page.type('#email','demo@northwind.dev');await page.type('#password','Demo1234');await page.click('button[type=submit]');await page.waitForSelector('.app-shell');
  // Home starts a board request. Changing route used to discard its response
  // while retaining Loading, preventing the new Deals route from requesting data.
  await page.waitForFunction(()=>document.querySelector('.app-shell'));
  for(let i=0;i<100&&!boardRequests;i++)await sleep(10);
  assert.ok(boardRequests>0,'Home did not start a board request');
  await page.evaluate(()=>document.querySelector('[data-nav-label="Deals"]').click());
  await page.waitForFunction(()=>location.pathname==='/deals');await page.waitForSelector('.deal-card');
  assert.ok(boardRequests>=2,'Deals did not replace the invalidated request');
  const initialCount=await page.$$eval('.deal-card',cards=>cards.length);assert.ok(initialCount>0);
  const refresh=()=>page.evaluate(()=>[...document.querySelectorAll('button')].find(button=>button.textContent.trim()==='Refresh board').click());
  delay=500;await refresh();await page.waitForSelector('[aria-busy="true"]');
  assert.equal(await page.$$eval('.deal-card',cards=>cards.length),initialCount,'Refresh hid the board');
  await page.waitForSelector('[aria-busy="false"]');
  failNext=true;await refresh();await page.waitForSelector('[aria-busy="true"]');await page.waitForSelector('[aria-busy="false"]');
  assert.equal(await page.$$eval('.deal-card',cards=>cards.length),initialCount,'Failed refresh discarded cards');
  await refresh();await page.waitForSelector('[aria-busy="true"]');await page.waitForSelector('[aria-busy="false"]');
  await page.select('select[aria-label="Show deal stage"]','Won');
  assert.equal(await page.$$eval('.pipeline-col',cols=>cols.length),1);assert.equal(await page.$eval('.pipeline-col__title',el=>el.textContent),'Won');
  assert.ok(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth+1));
  assert.ok(await page.$eval('.deal-card',card=>card.querySelector('.deal-card__checkbox').getBoundingClientRect().bottom<=card.querySelector('.deal-card__value').getBoundingClientRect().top),'Selection overlaps deal value');
  await page.focus('.deal-card__checkbox');await page.keyboard.press('Enter');
  assert.equal(new URL(page.url()).pathname,'/deals','Selecting a deal opened its detail page');
  await page.waitForSelector('.deal-card__checkbox[aria-checked="true"]');
  await page.focus('.deal-card__checkbox');await page.keyboard.press('Enter');

  // Search stays visibly busy until the matching response arrives.
  await page.select('select[aria-label="Show deal stage"]','');await page.type('input[aria-label="Search deals"]','absent-deal-999');
  await page.waitForSelector('[aria-busy="true"]');await page.waitForSelector('[aria-busy="false"]');
  await page.waitForFunction(()=>document.querySelector('.empty-state__title')?.textContent==='No deals match your filters');
  assert.ok(requests.some(url=>new URL(url).searchParams.get('q')==='absent-deal-999'));
  await page.$eval('input[aria-label="Search deals"]',input=>{input.value='';input.dispatchEvent(new Event('input',{bubbles:true}))});await page.waitForSelector('.deal-card');
  const artifacts=path.join(root,'tests/artifacts');fs.mkdirSync(artifacts,{recursive:true});await page.screenshot({path:path.join(artifacts,'deals-phone.png'),fullPage:true});
  // A small viewport uses the same drawer focus rules as a phone.
  await page.click('.topbar__menu-btn');await page.waitForFunction(()=>document.activeElement?.classList.contains('sidebar__close'));await page.click('.sidebar__close');await page.waitForFunction(()=>!document.querySelector('.app-main').inert);
 }catch(error){const pages=await browser?.pages();const page=pages?.at(-1);if(page)console.error(await page.evaluate(()=>{const close=document.querySelector('.sidebar__close');const css=close&&getComputedStyle(close);const before=document.activeElement?.outerHTML.slice(0,100);close?.focus();return JSON.stringify({before,after:document.activeElement?.outerHTML.slice(0,100),display:css?.display,visibility:css?.visibility,inert:[...document.querySelectorAll('[inert]')].map(e=>e.className),closeTab:close?.tabIndex,sidebar:document.querySelector('.sidebar')?.className,focused:document.hasFocus()})}));throw error;
 }finally{await browser?.close();if(server.exitCode===null){server.kill('SIGTERM');await new Promise(resolve=>server.once('exit',resolve))}fs.rmSync(directory,{recursive:true,force:true})}
});
