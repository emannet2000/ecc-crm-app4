'use strict';
(() => {
 let state={enabled:false,jobs:[]},pending=false;
 const host=document.createElement('details');host.id='offline-work';host.hidden=true;
 const summary=document.createElement('summary');summary.textContent='Offline work';host.append(summary);
 const panel=document.createElement('section');panel.setAttribute('aria-label','Offline work on this device');host.append(panel);document.body.append(host);
 function node(tag,text){const el=document.createElement(tag);if(text)el.textContent=text;return el}
 function button(label,action){const el=node('button',label);el.type='button';el.onclick=async()=>{el.disabled=true;try{await action()}catch(error){message.textContent=error.message}finally{el.disabled=false}};return el}
 const message=node('p');message.setAttribute('role','status');
 async function command(action,extra={}){
  if(!navigator.serviceWorker?.controller)throw new Error('Offline work needs HTTPS and an active service worker. Reload once after installation.');
  return new Promise((resolve,reject)=>{const channel=new MessageChannel(),timeout=setTimeout(()=>reject(new Error('Offline operation timed out. Try again.')),30000);channel.port1.onmessage=event=>{clearTimeout(timeout);channel.port1.close();event.data.error?reject(new Error(event.data.error)):resolve(event.data.result)};navigator.serviceWorker.controller.postMessage({type:'ecc-offline',action,...extra},[channel.port2])});
 }
 window.crmOffline={status:()=>state,command};
 function render(){
  summary.textContent=`Offline work${state.jobs.length?' · '+state.jobs.length+' pending':''}`;
  panel.replaceChildren(node('h2','Offline work on this device'),node('p',state.enabled?'Opened record views are saved on this device. Existing record edits can be queued offline; server validation runs during sync.':'Enable offline work on a device you control to save opened CRM views and queue edits.'),message);message.textContent='';
  if(!state.enabled){panel.append(button('Enable offline work',async()=>{state=await command('enable');render();message.textContent='Enabled. Open the views and records you need while connected.';window.dispatchEvent(new CustomEvent('crm:record-changed',{detail:''}));window.dispatchEvent(new Event('crm:offline-status'))}));return}
  panel.append(node('p','Uploads, new records, deletion, bulk changes, payments and administration need a connection. Pending edits are saved locally until the server accepts them.'));
  panel.append(button('Sync pending edits',async()=>{state=await command('sync');render();message.textContent=state.jobs.length?'Some edits need review below.':'All pending edits synced.';window.dispatchEvent(new CustomEvent('crm:record-changed',{detail:''}))}));
  panel.append(button('Disable and clear device data',async()=>{if(state.jobs.length&&!confirm('Discard pending edits and remove saved CRM data from this device?'))return;state=await command('disable');render();window.dispatchEvent(new Event('crm:offline-status'))}));
  for(const job of state.jobs){
   const row=node('section');row.className='offline-job';row.append(node('h3',job.entity+' · '+job.id),node('p',job.status+' · '+new Date(job.createdAt).toLocaleString()));if(job.error)row.append(node('p',job.error));
   row.append(node('pre',JSON.stringify(job.draft,null,2)));
   row.append(button('Discard this edit',async()=>{if(!confirm('Discard this pending edit?'))return;state=await command('discard',{key:job.key});render();window.dispatchEvent(new CustomEvent('crm:record-changed',{detail:job.id}))}));
   if(job.status!=='queued')row.append(button('Compare with saved record',async()=>{
    const review=await command('review',{key:job.key});row.querySelector('.offline-comparison')?.remove();const comparison=node('section');comparison.className='offline-comparison';comparison.append(node('h3','Current server record'),node('pre',JSON.stringify(review.record,null,2)));
    comparison.append(button('Keep my draft and retry',async()=>{if(!confirm('Apply your draft to the saved record shown above? Review the fields carefully before retrying.'))return;await command('retry',{key:job.key,revision:review.record._revision,confirmed:true});state=await command('sync');render();window.dispatchEvent(new CustomEvent('crm:record-changed',{detail:job.id}))}));row.append(comparison);
   }));panel.append(row);
  }
 }
 async function refresh(){if(pending||!navigator.serviceWorker?.controller)return;pending=true;try{state=await command('status');render();window.dispatchEvent(new Event('crm:offline-status'))}catch(error){message.textContent=error.message}finally{pending=false}}
 const updateVisibility=()=>{host.hidden=!document.querySelector('.app-shell')||!!document.querySelector('[role=dialog],.modal,.ecc-modal,.modal-panel');host.dataset.theme=localStorage.getItem('theme')||'light'};new MutationObserver(updateVisibility).observe(document.body,{childList:true,subtree:true});window.addEventListener('crm:theme-changed',updateVisibility);updateVisibility();render();
 navigator.serviceWorker?.addEventListener('message',event=>{if(event.data?.type==='ecc-offline-changed')refresh()});
 navigator.serviceWorker?.addEventListener('controllerchange',refresh);
 window.addEventListener('online',async()=>{try{if(state.enabled){state=await command('sync');render();window.dispatchEvent(new CustomEvent('crm:record-changed',{detail:''}))}else await refresh()}catch(error){message.textContent=error.message}});
 window.addEventListener('load',refresh);
})();
