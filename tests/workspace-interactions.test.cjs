const {test}=require('node:test');
const assert=require('node:assert/strict');
const {parseHTML}=require('linkedom');
const vm=require('node:vm');
const fs=require('node:fs');
const path=require('node:path');
function mount(html){
 const {window}=parseHTML('<html><body>'+html+'</body></html>'),document=window.document;
 let active=null;Object.defineProperty(document,'activeElement',{get:()=>active});
 window.HTMLElement.prototype.focus=function(){active=this};
 Object.defineProperty(window.HTMLElement.prototype,'offsetParent',{configurable:true,get(){return this.hidden?null:document.body}});
 window.location={hash:''};
 const context=vm.createContext({window,MutationObserver:window.MutationObserver,requestAnimationFrame:fn=>fn()});
 vm.runInContext(fs.readFileSync(path.join(__dirname,'../public/workspace-interactions.js'),'utf8'),context);
 const controller=new window.CRMWorkspaceInteractions(document);
 return {window,document,controller};
}
function key(window,target,name,shiftKey=false){const event=new window.Event('keydown',{bubbles:true,cancelable:true});event.key=name;event.shiftKey=shiftKey;target.dispatchEvent(event);return event}
const tick=()=>new Promise(resolve=>setTimeout(resolve,0));
test('workspace keyboard controller delegates row activation once and removes listeners on stop',()=>{
 const {window,document,controller}=mount('<main class="content"><table><tbody><tr><td>Application</td></tr><tr><td><button>Edit</button></td></tr></tbody></table></main>');
 let clicks=0;const rows=document.querySelectorAll('tr');rows[0].onclick=()=>clicks++;
 controller.start();controller.start();
 assert.equal(rows[0].getAttribute('role'),'button');assert.equal(rows[1].hasAttribute('data-keyboard'),false);
 assert.equal(key(window,rows[0],'Enter').defaultPrevented,true);assert.equal(clicks,1);
 controller.stop();key(window,rows[0],'Enter');assert.equal(clicks,1);
 controller.start();key(window,rows[0],' ');assert.equal(clicks,2);controller.stop();
});
test('workspace controller traps modal focus, restores its trigger and supports deep links without inferring permissions from labels',async()=>{
 const {window,document,controller}=mount('<div class="app-shell--viewer"><button id="update" disabled>Update action, owner or due date</button><button id="trigger">View</button></div><details id="handoff"><summary>Handoffs</summary></details>');
 const trigger=document.getElementById('trigger');trigger.focus();controller.start();
 assert.equal(document.getElementById('update').disabled,true);assert.equal(trigger.disabled,false);
 const unrelated=document.createElement("button");unrelated.textContent="Edit";document.querySelector(".app-shell--viewer").append(unrelated);controller.refresh();assert.equal(unrelated.disabled,false);
 window.location.hash='#handoff';window.dispatchEvent(new window.Event('hashchange'));assert.equal(document.getElementById('handoff').open,true);
 const dialog=document.createElement('div');dialog.className='modal';dialog.innerHTML='<h2>Edit application</h2><input id="first"><button id="last">Save</button>';document.body.append(dialog);await tick();
 assert.equal(dialog.getAttribute('aria-label'),'Edit application');assert.equal(document.activeElement.id,'first');
 document.getElementById('last').focus();key(window,document.activeElement,'Tab');assert.equal(document.activeElement.id,'first');
 key(window,document.activeElement,'Tab',true);assert.equal(document.activeElement.id,'last');
 dialog.remove();await tick();assert.equal(document.activeElement,trigger);controller.stop();
});
