const {test} = require('node:test');
const assert=require('node:assert/strict');
const {parseHTML}=require('linkedom');
const vm=require('node:vm');
const fs=require('node:fs');
const path=require('node:path');

async function mount(entity='contacts',id='c_client',options={}) {
 const {window}=parseHTML('<html><body><div id="app"></div></body></html>');
 const document=window.document,errors=[],calls=[],timers=[];
 const contact={id:'c_client',name:'Client One',email:'client@example.com',phone:'+12025550123',_revision:'contact-version'};
 const student={id:'s_client',name:'Student One',applicationStage:'Applying',intake:'January 2027',_revision:'student-version'};
 const flow={contacts:[contact,{...contact,id:'c_other',name:'Other Client'}],students:[student],deals:[],cases:[],documents:[],invoices:[],schools:[],leads:[],agents:[],partners:[],tasks:[],templates:[],ledger:[]};
 const center={role:'manager',organization:[{name:'Test workspace'}],features:{ai:false,smtp:options.smtp!==false},records:{contacts:flow.contacts,students:flow.students},mail:[],entries:[],notifications:[]};
 // Linkedom omits several browser form APIs. Supply their standard behavior only
 // in this test harness; browser tests exercise the same code without shims.
 Object.defineProperty(window.HTMLElement.prototype,'elements',{configurable:true,get(){return {namedItem:name=>this.querySelector('[name="'+name+'"]')}}});
 Object.defineProperty(window.HTMLSelectElement.prototype,'value',{configurable:true,get(){const selected=this.querySelector('option[selected]')||this.querySelector('option');return selected?.getAttribute('value')||''},set(value){for(const o of this.querySelectorAll('option')){if(o.getAttribute('value')===String(value))o.setAttribute('selected','');else o.removeAttribute('selected')}}});
 window.HTMLElement.prototype.scrollIntoView=()=>{};
 const sandbox={window,document,history:{replaceState(){}},location:{href:'http://crm.test/',origin:'http://crm.test',pathname:'/',search:''},localStorage:{getItem:()=>null},URLSearchParams,URL,FormData:class {constructor(form){this.items=[...form.querySelectorAll("[name]")].map(e=>[e.name,e.value])} [Symbol.iterator](){return this.items[Symbol.iterator]()}},Intl,console,CustomEvent:window.CustomEvent,Event:window.Event,MutationObserver:window.MutationObserver,HTMLElement:window.HTMLElement,customElements:window.customElements,navigator:{},setTimeout,clearTimeout,setInterval:(fn,ms)=>{const timer=setInterval(fn,ms);timers.push(timer);return timer},clearInterval};
 sandbox.fetch=async(url,opts={})=>{calls.push({url,opts});let data={status:'ok'};if(url.startsWith('/api/records/'))data={record:contact,editable:true};else if(url==='/api/workflows')data=flow;else if(url==='/api/control-center')data=center;else if(url.startsWith('/api/assistant/jobs'))data={jobs:[]};else if(url==='/api/portal/invitations')data={invitations:[]};else if(url.startsWith('/api/communication'))data={messages:[{channel:'email',direction:'inbound',subject:'Reply',body:'<img src=x onerror=alert(1)>',status:'received'}]};else if(url.startsWith('/api/insights'))data={sources:[{source:'Website',leads:501,converted:100,rate:19.96}],enrollment:[],agents:[],stages:[],nextActions:[],cohort:'All accessible records'};return {status:200,ok:true,json:async()=>data}};
 const context=vm.createContext(sandbox);
 for(const file of ['pickers.js','everyday.js','consolidation.js','center.js','components.js'])vm.runInContext(fs.readFileSync(path.join(__dirname,'../public',file),'utf8'),context,{filename:file});
 const element=document.createElement(entity==='workspace'?'crm-workspace':entity==='insights'?'crm-insights':'crm-record-tools');
 if(!['workspace','insights'].includes(entity)){element.setAttribute('entity',entity);element.setAttribute('record-id',id)}
 document.getElementById('app').append(element);
 await new Promise(resolve=>setTimeout(resolve,30));
 return {window,document,element,calls,flow,errors,cleanup(){element.remove();for(const timer of timers)clearInterval(timer)}};
}

test('record email tools select the current client and render incoming content as text',async()=>{
 const app=await mount();try{const root=app.element.shadowRoot;assert.equal(root.querySelector('[name=contactId]').value,'c_client');assert.equal(root.querySelector('[name=recipient]').value,'client@example.com');const button=[...root.querySelectorAll('nav button')].find(b=>b.textContent==='Communication');button.onclick();await new Promise(resolve=>setTimeout(resolve,10));assert.ok(root.querySelector(".tools").textContent.includes('<img src=x'));assert.equal(root.querySelectorAll('img').length,0)}finally{app.cleanup()}
});
test('student tools hydrate the existing application and intake',async()=>{
 const app=await mount('students','s_client');try{const root=app.element.shadowRoot;assert.equal(root.querySelector('[name=intake]').value,'January 2027');assert.equal(root.querySelector('[name=applicationStage]').value,'Applying');assert.ok(root.querySelector(".tools").textContent.includes('Create a student application'));assert.ok(!root.querySelector(".tools").textContent.includes('Lead qualification'))}finally{app.cleanup()}
});
test('native workspace offers recoverable deletion and unmatched-message workflows',async()=>{
 const app=await mount('workspace');try{assert.equal(app.document.querySelectorAll('iframe').length,0);const labels=[...app.element.shadowRoot.querySelectorAll('nav button')].map(b=>b.textContent);assert.ok(labels.includes('Trash'));assert.ok(labels.includes('Communication'));assert.ok(!labels.includes('Client portal'));app.element.shadowRoot.querySelectorAll('nav button').forEach(button=>{if(button.textContent==='Communication')button.onclick()});await new Promise(resolve=>setTimeout(resolve,10));assert.ok(app.element.shadowRoot.querySelector('select[aria-label="Communication channel"] option[value="portal"]'))}finally{app.cleanup()}
});
test('performance reports display full server aggregates',async()=>{
 const app=await mount('insights');try{assert.ok(app.element.shadowRoot.querySelector(".tools").textContent.includes('501'));assert.ok(app.calls.some(c=>c.url.startsWith('/api/insights')));assert.ok(app.element.shadowRoot.querySelector(".tools").textContent.includes('Lead conversion by source'))}finally{app.cleanup()}
});

test('draft storage keeps explicitly supplied record keys separate without inspecting forms or requests',()=>{
 const data=new Map(),responses=[];let receive;
 const app={ports:{draftStorage:{subscribe(callback){receive=callback}},storedDraft:{send(value){responses.push(value)}}}};
 const window={};const sessionStorage={getItem:key=>data.get(key)||null,setItem:(key,value)=>data.set(key,value),removeItem:key=>data.delete(key)};
 vm.runInContext(fs.readFileSync(path.join(__dirname,'../public/draft-storage.js'),'utf8'),vm.createContext({window,sessionStorage,JSON}));
 window.connectDraftStorage(app);
 receive({action:'write',key:'user-one/contact-one',fields:{name:'First draft',revision:'version-one'}});
 receive({action:'write',key:'user-one/contact-two',fields:{name:'Second draft',revision:'version-two'}});
 receive({action:'read',key:'user-one/contact-one'});
 assert.equal(responses.at(-1).fields.name,'First draft');assert.equal(responses.at(-1).fields.revision,'version-one');
 receive({action:'read',key:'user-two/contact-one'});assert.equal(responses.at(-1).fields,null);
 receive({action:'remove',key:'user-one/contact-one'});receive({action:'read',key:'user-one/contact-one'});assert.equal(responses.at(-1).fields,null);
 receive({action:'read',key:'user-one/contact-two'});assert.equal(responses.at(-1).fields.name,'Second draft');
});

test('unconfigured email keeps history available without offering a send action',async()=>{
 const app=await mount('contacts','c_client',{smtp:false});try{
  const root=app.element.shadowRoot;
  assert.equal(root.querySelector('.communication-compose button[type=submit]').disabled,true);
  assert.ok(root.querySelector('.tools').textContent.includes('Sending is unavailable'));
  assert.ok([...root.querySelectorAll('nav button')].some(button=>button.textContent==='Communication'));
 }finally{app.cleanup()}
});

test('empty daily work offers assignment recovery and workflow links',async()=>{
 const app=await mount('insights');try{
  const work=app.document.createElement('crm-my-work');app.document.body.append(work);
  await new Promise(resolve=>setTimeout(resolve,20));
  const root=work.shadowRoot;
  assert.equal(root.querySelector('a[href="/leads"]').textContent,'Create your first lead');
  assert.ok(root.querySelector('a[href="/invoices"]'));
  const all=[...root.querySelectorAll('button')].find(b=>b.textContent==='Check all accessible work');all.onclick();
  await new Promise(resolve=>setTimeout(resolve,20));
  assert.ok(app.calls.some(c=>c.url.startsWith('/api/my-work?')&&c.url.includes('scope=all')));
  work.remove();
 }finally{app.cleanup()}
});

test('unconfigured AI work brief remains hidden without offering a provider action',async()=>{
 const app=await mount('insights');try{
  const wrapper=app.document.createElement('details');wrapper.setAttribute('data-ai-brief','');wrapper.hidden=true;
  const brief=app.document.createElement('crm-ai-productivity');wrapper.append(brief);app.document.body.append(wrapper);
  await new Promise(resolve=>setTimeout(resolve,20));
  assert.equal(wrapper.hidden,true);assert.equal(brief.shadowRoot.querySelector('button'),null);
  wrapper.remove();
 }finally{app.cleanup()}
});
