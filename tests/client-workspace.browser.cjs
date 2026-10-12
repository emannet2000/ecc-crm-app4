const {test} = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const os = require('node:os');
const {spawn} = require('node:child_process');
const puppeteer = require('puppeteer');
const root = path.join(__dirname, '..');
const sleep = ms => new Promise(resolve => setTimeout(resolve, ms));
async function input(page, selector, value) {
  await page.$eval(selector, (field, value) => {field.value = value; field.dispatchEvent(new Event('input', {bubbles:true}));}, value);
}
async function click(page, text, selector='button') {
  await page.evaluate((text, selector) => {const button=[...document.querySelectorAll(selector)].find(button=>button.textContent.trim()===text);if(!button)throw new Error('Missing '+text);button.click();}, text, selector);
}
async function workspaceButton(page, text) {
  await page.evaluate(text => [...document.querySelector('crm-client-workspace').shadowRoot.querySelectorAll('button')].find(button=>button.textContent.trim()===text).click(), text);
}
async function api(page, url, method='GET', body, revision) {
  return page.evaluate(async (url, method, body, revision) => {
    const response=await fetch(url,{method,headers:{'Content-Type':'application/json',...(revision?{'If-Match':revision}:{})},body:body===undefined?undefined:JSON.stringify(body)});
    const data=await response.json();if(!response.ok)throw new Error(response.status+': '+JSON.stringify(data));return data;
  },url,method,body,revision);
}

test('phone login and unified client work preserve drafts, revisions and permissions', {timeout:120000}, async () => {
  const directory=fs.mkdtempSync(path.join(os.tmpdir(),'ecc-client-workspace-'));
  const port=17981, base='http://127.0.0.1:'+port;
  const server=spawn(path.join(root,'build/ecc-crm'),[],{cwd:root,env:{...process.env,PORT:String(port),DATABASE_PATH:path.join(directory,'crm.sqlite3'),JWT_SECRET:'client-workspace-browser-only',APP_URL:base,ALLOW_SIGNUP:'false',OPENAI_API_KEY:'',SMTP_HOST:''},stdio:'pipe'});
  let serverLog='';server.stdout.on('data',chunk=>serverLog+=chunk);server.stderr.on('data',chunk=>serverLog+=chunk);
  let browser;
  try {
    for(let i=0;i<100;i++){try{if((await fetch(base+'/api/health')).ok)break;}catch{}if(server.exitCode!==null)throw new Error(serverLog);await sleep(100);}
    const chrome=path.join(os.homedir(),'.cache/ms-playwright/chromium-1234/chrome-linux64/chrome');
    browser=await puppeteer.launch({executablePath:process.env.CHROME_PATH||(fs.existsSync(chrome)?chrome:puppeteer.executablePath()),headless:true,pipe:true,args:['--no-sandbox','--disable-dev-shm-usage']});
    const page=await browser.newPage();page.setDefaultTimeout(15000);const errors=[];page.on('pageerror',error=>errors.push(error.message));
    const artifacts=path.join(root,'tests/artifacts');fs.mkdirSync(artifacts,{recursive:true});
    await page.goto(base);
    await page.waitForSelector('#email');
    for(const [width,height] of [[320,740],[390,844],[768,1024],[667,375]]){
      await page.setViewport({width,height});
      assert.ok(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth+1),'login overflow '+width);
      assert.equal(await page.$eval('.brand-panel',el=>getComputedStyle(el).display),'none');
      assert.equal(await page.$eval('.auth-mobile-brand',el=>getComputedStyle(el).display),'flex');
      assert.ok(await page.$eval('#email',el=>parseFloat(getComputedStyle(el).fontSize)>=16));
      assert.ok(await page.$eval('.ecc-field__toggle',el=>el.getBoundingClientRect().width>=44&&el.getBoundingClientRect().height>=44));
      assert.ok(await page.$eval('.auth-forgot',el=>el.getBoundingClientRect().height>=44));
      assert.ok(await page.$eval('.auth-form button[type=submit]',el=>el.getBoundingClientRect().height>=52));
      assert.equal(await page.$eval('.auth-form .ecc-field label',el=>getComputedStyle(el).position),'static');
    }
    await page.setViewport({width:390,height:844});
    await page.screenshot({path:path.join(artifacts,'login-phone.png'),fullPage:true});
    await input(page,'#email','demo@northwind.dev');await input(page,'#password','Demo1234');
    await page.click('.ecc-field__toggle');assert.equal(await page.$eval('#password',el=>el.type),'text');await page.click('.ecc-field__toggle');
    await page.click('.auth-form button[type=submit]');await page.waitForSelector('.app-shell');
    assert.equal(await page.$$eval('.mobile-nav > *',items=>items.filter(item=>getComputedStyle(item).display!=='none').length),5);
    const ys=await page.$$eval('.mobile-nav > *',items=>items.filter(item=>getComputedStyle(item).display!=='none').map(item=>Math.round(item.getBoundingClientRect().y)));assert.equal(new Set(ys).size,1);
    await page.goto(base+'/contacts/c_1');await page.waitForFunction(()=>document.querySelector('crm-client-workspace')?.shadowRoot.querySelector('select'));
    assert.equal(await page.$('script[src="/public/record-versions.js"]'),null);assert.equal(await page.$('script[src="/public/draft-recovery.js"]'),null);
    for(const label of ['Next actions','Files','Communication','Payments','History'])assert.ok(await page.$eval('crm-client-workspace',(host,label)=>host.shadowRoot.textContent.includes(label),label));
    assert.equal(await page.$eval('crm-client-workspace',host=>host.shadowRoot.querySelector('h1').textContent),'Ada Lovelace');
    assert.equal(await page.$eval('.content__header',el=>getComputedStyle(el).display),'none');
    assert.ok(await page.$eval('crm-client-workspace',host=>[...host.shadowRoot.querySelectorAll('nav button')].every(button=>{const box=button.getBoundingClientRect();return box.x>=0&&box.right<=innerWidth&&box.height>=44;})));
    const applicationContext=await page.$eval('crm-client-workspace',host=>host.shadowRoot.querySelector('select').value);
    assert.equal(applicationContext,'cases/case_1');
    assert.ok(await page.$eval('crm-client-workspace',host=>host.shadowRoot.querySelector('[aria-label="Needs attention"]').textContent.includes('missing files')));
    await page.evaluate(()=>document.querySelector('crm-client-workspace').shadowRoot.querySelector('[data-work-tab=messages]').focus());
    await workspaceButton(page,'Communication');
    assert.equal(await page.$eval('crm-client-workspace',host=>host.shadowRoot.activeElement?.dataset.workTab),'messages');
    assert.equal(await page.$eval('crm-client-workspace',host=>host.shadowRoot.querySelector('select').value),applicationContext);
    await workspaceButton(page,'Files');
    assert.equal(await page.$eval('crm-client-workspace',host=>host.shadowRoot.querySelector('select').value),applicationContext);
    await workspaceButton(page,'Payments');
    await workspaceButton(page,'Add invoice');await page.waitForSelector('.modal');
    assert.equal(await page.$eval('#inv-caseId',el=>el.getAttribute('value')||el.value),'case_1');
    await page.click('.modal__close');await page.waitForFunction(()=>!document.querySelector('.modal'));
    await page.select('crm-nav-focus select','finance');
    assert.ok(await page.$$eval('.mobile-nav [data-nav-label="Invoices"]',items=>items.some(item=>getComputedStyle(item).display!=='none')));
    assert.equal(await page.$eval('.mobile-nav [data-nav-label="Applications"]',el=>getComputedStyle(el).display),'none');
    await page.select('crm-nav-focus select','adviser');
    await workspaceButton(page,'Payments');await page.waitForFunction(()=>document.querySelector('crm-client-workspace').shadowRoot.querySelector('crm-record-tools')?.shadowRoot.textContent.includes('Workflows'));
    assert.equal(new URL(page.url()).pathname,'/contacts/c_1');
    await workspaceButton(page,'Next actions');
    await page.evaluate(()=>{const select=document.querySelector('crm-client-workspace').shadowRoot.querySelector('select');select.value='contacts/c_1';select.dispatchEvent(new Event('change'));});
    await workspaceButton(page,'Add application');await page.waitForSelector('.modal');
    assert.equal(await page.$eval('#cf-clientId',el=>el.getAttribute('value')||el.value),'c_1');
    await page.click('.modal__close');await page.waitForFunction(()=>!document.querySelector('.modal'));
    await workspaceButton(page,'Edit client');await page.waitForSelector('#cf-name');
    const snapshot=(await api(page,'/api/contacts/c_1')).contact;
    await input(page,'#cf-name','Unsaved phone draft');
    await page.evaluate(()=>window.dispatchEvent(new CustomEvent('crm:record-changed',{detail:'c_1'})));
    await sleep(250);assert.equal(await page.$eval('#cf-name',el=>el.value),'Unsaved phone draft');
    assert.ok(await page.evaluate(()=>Object.keys(sessionStorage).some(key=>key.startsWith('ecc:form-draft:v3:'))));
    await api(page,'/api/contacts/c_1','PUT',{...snapshot,name:'Concurrent staff edit'},snapshot._revision);
    await page.evaluate(()=>window.dispatchEvent(new CustomEvent('crm:record-changed',{detail:'c_1'})));
    await sleep(250);
    const stale=page.waitForResponse(response=>new URL(response.url()).pathname==='/api/contacts/c_1'&&response.request().method()==='PUT');
    await page.$eval('.modal form',form=>form.requestSubmit());
    assert.equal((await stale).status(),409);
    assert.equal(await page.$eval('#cf-name',el=>el.value),'Unsaved phone draft');
    await page.waitForSelector('.draft-recovery');await click(page,'Compare with latest saved record');
    await page.waitForSelector('.draft-comparison');assert.ok(await page.$eval('.draft-comparison',el=>el.textContent.includes('Concurrent staff edit')&&el.textContent.includes('Unsaved phone draft')));
    await page.click('.draft-recovery__review input');
    const reviewRevision=(await api(page,'/api/records/contacts/c_1')).record._revision;
    const storedRevisions=await page.evaluate(()=>Object.entries(sessionStorage).filter(([key])=>key.startsWith('ecc:form-draft:v3:')).map(([key,value])=>JSON.parse(value).revision));
    assert.ok(storedRevisions.includes(reviewRevision),JSON.stringify({storedRevisions,reviewRevision,checked:await page.$eval('.draft-recovery__review input',el=>el.checked)}));
    let writes=0;const count=request=>{if(request.method()==='PUT'&&new URL(request.url()).pathname==='/api/contacts/c_1')writes++;};page.on('request',count);
    const saved=page.waitForResponse(response=>new URL(response.url()).pathname==='/api/contacts/c_1'&&response.request().method()==='PUT');
    await page.$eval('.modal form',form=>{form.requestSubmit();form.requestSubmit();});const savedResponse=await saved;assert.equal(savedResponse.status(),200,JSON.stringify({body:await savedResponse.json(),revision:savedResponse.request().headers()['if-match'],current:(await api(page,'/api/records/contacts/c_1')).record._revision}));
    await page.waitForFunction(()=>!document.querySelector('.modal'));assert.equal(writes,1);page.off('request',count);
    assert.equal((await api(page,'/api/contacts/c_1')).contact.name,'Unsaved phone draft');
    // Recovery after a reload is keyed to the actual record and user.
    await workspaceButton(page,'Edit client');await page.waitForSelector('#cf-name');await input(page,'#cf-name','Draft across reload');await sleep(50);
    await page.reload();await page.waitForFunction(()=>document.querySelector('crm-client-workspace')?.shadowRoot.querySelector('select'));
    await page.evaluate(()=>{const select=document.querySelector('crm-client-workspace').shadowRoot.querySelector('select');select.value='contacts/c_1';select.dispatchEvent(new Event('change'));});
    await workspaceButton(page,'Edit client');await page.waitForSelector('.draft-recovery');await click(page,'Restore draft');assert.equal(await page.$eval('#cf-name',el=>el.value),'Draft across reload');
    await page.screenshot({path:path.join(artifacts,'client-draft-phone.png'),fullPage:true});
    await page.click('.modal__close');await click(page,'Discard');await page.waitForFunction(()=>!document.querySelector('.modal'));
    await page.evaluate(()=>{const select=document.querySelector('crm-client-workspace').shadowRoot.querySelector('select');select.value='cases/case_1';select.dispatchEvent(new Event('change'));});
    await page.waitForFunction(()=>document.querySelector('crm-client-workspace').shadowRoot.querySelector('crm-work-plan')?.shadowRoot.querySelector('textarea'));
    await page.evaluate(()=>{const field=document.querySelector('crm-client-workspace').shadowRoot.querySelector('crm-work-plan').shadowRoot.querySelector('textarea');field.value='Keep this next action';field.dispatchEvent(new Event('input',{bubbles:true}));});
    await workspaceButton(page,'Files');assert.ok(await page.$eval('crm-client-workspace',host=>host.shadowRoot.querySelector('crm-work-plan')),'switch replaced dirty plan');
    await page.reload();await page.waitForFunction(()=>document.querySelector('crm-client-workspace')?.shadowRoot.querySelector('select'));
    await page.screenshot({path:path.join(artifacts,'client-workspace-phone.png'),fullPage:true});
    assert.ok(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth+1));
    await page.goto(base+'/deals');await page.waitForSelector('.deal-card__checkbox');
    const deal=(await api(page,'/api/deals')).deals[0];
    await page.evaluate(title=>[...document.querySelectorAll('.deal-card__checkbox')].find(button=>button.getAttribute('aria-label')===title).click(),deal.title);
    await click(page,'Move to stage…');await page.waitForSelector('.modal');
    await api(page,'/api/deals/'+deal.id,'PUT',{...deal,title:'Concurrent deal change'},deal._revision);
    await page.evaluate(()=>window.dispatchEvent(new CustomEvent('crm:record-changed',{detail:''})));await sleep(200);
    const bulk=page.waitForResponse(response=>new URL(response.url()).pathname==='/api/deals/bulk'&&response.request().method()==='POST');
    await click(page,'Move','.modal button');const bulkResponse=await bulk;
    assert.equal(bulkResponse.status(),409);assert.equal(JSON.parse(bulkResponse.request().postData()).revisions[deal.id],deal._revision);
    assert.equal((await api(page,'/api/deals/'+deal.id)).deal.title,'Concurrent deal change');
    await api(page,'/api/admin/users','POST',{name:'Client Viewer',email:'client-viewer@example.com',password:'ViewerPassword123!',role:'viewer'});
    await page.goto(base);await page.waitForSelector('.app-shell');await click(page,'Sign out');await page.waitForSelector('#email');await input(page,'#email','client-viewer@example.com');await input(page,'#password','ViewerPassword123!');await page.click('.auth-form button[type=submit]');await page.waitForSelector('.app-shell');
    await page.goto(base+'/contacts/c_1');await page.waitForFunction(()=>document.querySelector('crm-client-workspace')?.shadowRoot.textContent.includes('Read-only record'));
    assert.equal(await page.$eval('crm-client-workspace',host=>[...host.shadowRoot.querySelectorAll('button')].some(button=>button.textContent.startsWith('Edit '))),false);
    await page.goto(base+'/contacts');await page.waitForSelector('.page-toolbar');assert.equal(await page.evaluate(()=>[...document.querySelectorAll('button')].find(button=>button.textContent.trim()==='Add contact').disabled),true);
    assert.equal(await page.$$eval('.row-action',buttons=>buttons.length>0&&buttons.every(button=>button.disabled)),true);
    assert.deepEqual(errors,[]);
  } finally {
    await browser?.close();if(server.exitCode===null){server.kill('SIGTERM');await new Promise(resolve=>server.once('exit',resolve));}fs.rmSync(directory,{recursive:true,force:true});
  }
});
