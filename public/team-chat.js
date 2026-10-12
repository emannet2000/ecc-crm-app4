(() => {
 'use strict';
 const draftStores=new Map();
 const node=(tag,text,attrs={})=>{const element=document.createElement(tag);if(text!=null)element.textContent=text;for(const[key,value]of Object.entries(attrs))element.setAttribute(key,value);return element;};
 const roleName=role=>({admin:'Administrator',manager:'Manager',member:'Team member',viewer:'Viewer'})[role]||'Workspace colleague';
 const initials=name=>name.trim().split(/\s+/).slice(0,2).map(word=>word[0]).join('').toUpperCase();
 class TeamChat extends HTMLElement {
  connectedCallback(){
   if(!this.shadowRoot)this.mount();
   this.generation++;this.rosterRequest++;this.peopleLoading=false;
   this.online=()=>{this.updateComposer();this.$('.status').textContent=navigator.onLine===false?'Offline. Your draft is kept. Reconnect before sending.':'';if(navigator.onLine!==false){this.loadPeople();if(this.peer)this.loadMessages();}};
   this.visible=()=>{if(!document.hidden)this.online();};
   window.addEventListener('online',this.online);window.addEventListener('offline',this.online);document.addEventListener('visibilitychange',this.visible);
   window.addEventListener('resize',this.resize);window.visualViewport?.addEventListener('resize',this.resize);window.visualViewport?.addEventListener('scroll',this.resize);
   requestAnimationFrame(this.resize);this.loadPeople();if(this.peer)this.loadMessages();
   this.timer=setInterval(()=>{if(!document.hidden&&navigator.onLine!==false){this.loadPeople();if(this.peer)this.loadMessages();}},5000);
  }
  mount(){
   this.attachShadow({mode:'open'});this.$=selector=>this.shadowRoot.querySelector(selector);
   const focus=document.querySelector('crm-nav-focus');this.userKey=(focus?.getAttribute('org-id')||'')+':'+this.getAttribute('user-id');
   if(!draftStores.has(this.userKey))draftStores.set(this.userKey,new Map());
   this.drafts=draftStores.get(this.userKey);this.peer=null;this.generation=0;this.rosterRequest=0;this.controllers=new Set();this.pendingSends=new Map();this.messagesRequest=0;
   this.shadowRoot.innerHTML=`<link rel="stylesheet" href="/public/team-chat.css"><div class="layout"><section class="people" aria-label="Colleagues"><h2>Team chat</h2><p class="people-intro">Direct messages with your workspace colleagues.</p><input class="search" type="search" placeholder="Find a colleague" aria-label="Find a colleague" maxlength="200"><p class="roster-status" role="status"></p><button class="roster-retry" type="button" hidden>Retry loading colleagues</button><div class="list"></div></section><section class="conversation" aria-label="Conversation"><header class="heading"><button class="back" type="button" aria-label="Back to colleagues">←</button><span class="avatar peer-avatar" aria-hidden="true" hidden></span><div><h2 tabindex="-1">Choose a colleague</h2><p class="peer-role">Workspace team chat</p></div></header><div class="messages" role="log" aria-label="Messages" aria-live="polite" aria-relevant="additions"><p class="empty">Choose a colleague to open a direct conversation.</p></div><form class="composer" hidden><textarea rows="2" maxlength="4000" aria-label="Message" placeholder="Write a message…" required></textarea><button class="send" type="submit">Send</button></form><div class="composer-help" hidden><span class="shortcut">Enter to send · Shift+Enter for a new line</span><span class="count">0 / 4,000</span></div><p class="status" role="status" aria-live="polite"></p><button class="retry" type="button" hidden>Retry loading messages</button></section></div>`;
   this.$('.roster-retry').onclick=()=>this.loadPeople();this.$('.retry').onclick=()=>this.loadMessages();
   this.$('.search').oninput=()=>{clearTimeout(this.searchTimer);this.rosterRequest++;this.peopleLoading=false;this.searchTimer=setTimeout(()=>{this.people=[];this.loadPeople();},200);};
   this.$('.back').onclick=()=>{this.saveDraft();this.peer=null;this.generation++;this.$('.layout').classList.remove('open');this.$('.search').focus();this.loadPeople();};
   this.$('.composer').onsubmit=event=>{event.preventDefault();this.send();};
   this.$('textarea').oninput=()=>{this.saveDraft();this.updateComposer();};
   this.$('textarea').onkeydown=event=>{if(event.key==='Enter'&&!event.isComposing&&!event.shiftKey&&(!matchMedia('(max-width:700px)').matches||event.ctrlKey||event.metaKey)){event.preventDefault();this.send();}};
   this.resize=()=>{if(!this.isConnected||!matchMedia('(max-width:700px)').matches)return;const viewport=window.visualViewport,bottom=(viewport?.height||innerHeight)+(viewport?.offsetTop||0),nav=document.querySelector('.mobile-nav')?.getBoundingClientRect(),limit=nav?.height&&nav.top<bottom?nav.top:bottom;this.style.setProperty('--chat-height',Math.max(120,limit-this.getBoundingClientRect().top-8)+'px');};
  }
  disconnectedCallback(){this.saveDraft();clearTimeout(this.searchTimer);clearInterval(this.timer);this.generation++;this.rosterRequest++;this.messagesRequest++;for(const controller of this.controllers||[])controller.abort();window.removeEventListener('online',this.online);window.removeEventListener('offline',this.online);document.removeEventListener('visibilitychange',this.visible);window.removeEventListener('resize',this.resize);window.visualViewport?.removeEventListener('resize',this.resize);window.visualViewport?.removeEventListener('scroll',this.resize);}
  async request(path,data){
   const controller=new AbortController();this.controllers.add(controller);const timeout=setTimeout(()=>controller.abort(),15000);
   const options={credentials:'same-origin',cache:'no-store',signal:controller.signal,headers:data?{'Content-Type':'application/json'}:{},method:data?'POST':'GET',body:data?JSON.stringify(data):undefined};
   try{
    let response=await fetch('/api/team-chat/'+path,options);
    if(response.status===401){if(!this.sessionRefresh)this.sessionRefresh=fetch('/api/session/refresh',{method:'POST',credentials:'same-origin',signal:controller.signal}).finally(()=>{this.sessionRefresh=null;});if((await this.sessionRefresh).ok)response=await fetch('/api/team-chat/'+path,options);}
    const result=await response.json().catch(()=>({}));if(!response.ok){const error=new Error(response.status===401?'Sign in again to continue.':result.error||'Unable to connect. Please try again.');error.status=response.status;throw error;}if(!data&&!Array.isArray(result))throw new Error('Unable to load this conversation. Please retry.');return result;
   }catch(error){if(error.name==='AbortError')throw new Error('Connection timed out. Please try again.');throw error;}finally{clearTimeout(timeout);this.controllers.delete(controller);}
  }
  async loadPeople(more=false){
   const query=this.$('.search').value,offset=more?(this.people||[]).length:0;
   if(this.peopleLoading&&!more&&this.peopleQuery===query)return;
   const request=++this.rosterRequest;this.peopleLoading=true;this.peopleQuery=query;
   try{if(!this.people?.length)this.$('.roster-status').textContent='Loading colleagues…';const pages=more?1:Math.max(1,Math.ceil((this.people||[]).length/200));const batches=await Promise.all(Array.from({length:pages},(_,index)=>this.request('people?q='+encodeURIComponent(query)+'&offset='+(offset+index*200))));if(!this.isConnected||request!==this.rosterRequest||query!==this.$('.search').value)return;this.people=more?[...(this.people||[]),...batches.flat()]:batches.flat();this.hasMore=batches.at(-1).length===200;this.$('.roster-status').textContent='';this.$('.roster-retry').hidden=true;this.renderPeople();}catch(error){if(this.isConnected&&request===this.rosterRequest){this.$('.roster-status').textContent=error.message;this.$('.roster-retry').hidden=false;this.renderPeople();}}finally{if(request===this.rosterRequest)this.peopleLoading=false;}
  }
  renderPeople(){
   const signature=JSON.stringify([this.people,this.peer?.id,this.$('.search').value,this.hasMore]);if(signature===this.rosterSignature)return;this.rosterSignature=signature;
   const list=this.$('.list');list.replaceChildren();const search=this.$('.search').value.toLowerCase();const people=(this.people||[]).filter(person=>person.name.toLowerCase().includes(search));
   for(const person of people){const button=node('button',null,{type:'button',class:'person','aria-pressed':String(this.peer?.id===person.id)}),avatar=node('span',initials(person.name),{class:'avatar','aria-hidden':'true'}),label=node('span',null,{class:'person-label'});label.append(node('span',person.name,{class:'person-name'}),node('span',person.lastMessage||roleName(person.role),{class:person.lastMessage?'preview':'role'}));button.append(avatar,label);if(person.unread)button.append(node('span',person.unread,{class:'badge','aria-label':person.unread+' unread messages'}));button.onclick=()=>this.select(person);list.append(button);}
   if(this.hasMore){const more=node('button','Load more colleagues',{type:'button'});more.onclick=()=>{more.disabled=true;this.loadPeople(true);};list.append(more);}
   if(!people.length&&!this.peopleLoading)list.append(node('p',this.people?.length?'No matching colleagues.':'No other active workspace members yet.'));
  }
  draftKey(peer){return 'ecc:team-chat:v2:'+this.userKey+':'+peer;}
  readStored(peer){try{return JSON.parse(sessionStorage.getItem(this.draftKey(peer)))||{};}catch{return {};}}
  store(peer,value){this.drafts.set(peer,value);const saved=this.readStored(peer);try{if(!value&&!saved.pending)sessionStorage.removeItem(this.draftKey(peer));else sessionStorage.setItem(this.draftKey(peer),JSON.stringify({...saved,body:value}));}catch{}}
  saveDraft(){if(this.peer)this.store(this.peer.id,this.$('textarea').value);}
  updateComposer(){const input=this.$('textarea');this.$('.count').textContent=Array.from(input.value).length.toLocaleString()+' / 4,000';this.$('.send').disabled=!!this.sending||!!this.peerUnavailable||navigator.onLine===false||!input.value.trim();this.$('.send').textContent=this.sending?'Sending…':'Send';input.style.height='auto';input.style.height=Math.min(140,Math.max(48,input.scrollHeight))+'px';}
  select(peer){this.saveDraft();this.peer=peer;this.generation++;this.lastSignature=null;this.lastMessageId=0;this.latest=[];this.messagesRequest++;this.readSeq=0;this.history=[];this.historyDone=false;this.deliveryError=false;this.peerUnavailable=false;this.$('.layout').classList.add('open');this.$('.heading h2').textContent=peer.name;this.$('.peer-role').textContent='Direct message · '+roleName(peer.role);this.$('.peer-avatar').hidden=false;this.$('.peer-avatar').textContent=initials(peer.name);this.$('.composer').hidden=false;this.$('.composer-help').hidden=false;this.$('textarea').value=this.drafts.get(peer.id)??this.readStored(peer.id).body??'';this.$('.messages').replaceChildren();this.$('.status').textContent='Loading conversation…';this.$('.retry').hidden=true;this.updateComposer();this.renderPeople();this.loadMessages();(matchMedia('(max-width:700px)').matches?this.$('.back'):this.$('.heading h2')).focus();}
  renderMessages(messages){
   const log=this.$('.messages'),nearBottom=log.scrollHeight-log.scrollTop-log.clientHeight<100,scrollTop=log.scrollTop,first=this.lastSignature===null;
   const combined=[...new Map([...(this.history||[]),...messages].map(message=>[message.id,message])).values()].sort((a,b)=>a.id-b.id);
   const hasNew=combined.some(message=>message.id>(this.lastMessageId||0));this.lastMessageId=combined.at(-1)?.id||0;
   const signature=JSON.stringify(combined);if(signature===this.lastSignature)return nearBottom;
   // Keep existing bubbles in place. Polling must not announce the whole thread
   // again or disturb someone selecting text and reading earlier messages.
   const existing=new Map([...log.children].map(child=>[child.dataset.key,child])),wanted=[];
   if(messages.length===100&&!this.historyDone){const older=existing.get('older')||node('button','Load earlier messages',{type:'button'});older.dataset.key='older';older.onclick=()=>this.loadOlder(older);wanted.push(older);}
   let day='';for(const message of combined){
    const date=new Date(message.createdAt),label=date.toLocaleDateString(undefined,{weekday:'short',month:'short',day:'numeric',year:'numeric'});
    if(label!==day){day=label;const separator=existing.get('day:'+day)||node('div',day,{class:'day'});separator.dataset.key='day:'+day;wanted.push(separator);}
    const mine=message.senderId===this.getAttribute('user-id'),key='message:'+message.id;
    let bubble=existing.get(key);
    if(!bubble){bubble=node('div',null,{class:'message'+(mine?' mine':'')});bubble.dataset.key=key;bubble.append(node('div',message.body,{class:'body'}));const meta=node('div',null,{class:'message-meta'});meta.append(node('time',date.toLocaleTimeString(undefined,{hour:'numeric',minute:'2-digit'}),{datetime:message.createdAt}));if(mine)meta.append(node('span',message.readAt?'Read':'Sent'));bubble.append(meta);}
    const receipt=bubble.querySelector('.message-meta span');if(receipt&&receipt.textContent!==(message.readAt?'Read':'Sent'))receipt.textContent=message.readAt?'Read':'Sent';
    wanted.push(bubble);
   }
   if(!nearBottom&&!first&&!hasNew&&existing.get('jump'))wanted.push(existing.get('jump'));
   if(!combined.length){const empty=existing.get('empty')||node('p','Start a conversation with '+this.peer.name+'.',{class:'empty'});empty.dataset.key='empty';wanted.push(empty);}
   for(const child of [...log.children])if(!wanted.includes(child))child.remove();
   wanted.forEach((child,index)=>{if(log.children[index]!==child)log.insertBefore(child,log.children[index]||null);});
   if(nearBottom||first)log.scrollTop=log.scrollHeight;
   else{log.scrollTop=scrollTop;if(hasNew){const jump=node('button','New messages · Jump to latest',{type:'button',class:'new-messages'});jump.dataset.key='jump';jump.onclick=()=>{log.scrollTop=log.scrollHeight;jump.remove();this.loadMessages();};log.append(jump);}}
   this.lastSignature=signature;return nearBottom||first;
  }
  async loadMessages(){const peer=this.peer,generation=this.generation;if(!peer||this.messagesLoading===generation)return;this.messagesLoading=generation;const request=++this.messagesRequest;
   try{const messages=await this.request(encodeURIComponent(peer.id));if(!this.isConnected||generation!==this.generation||request!==this.messagesRequest)return;this.peerUnavailable=false;this.updateComposer();this.latest=messages;const atBottom=this.renderMessages(messages);this.$('.retry').hidden=true;if(!this.sending&&!this.deliveryError)this.$('.status').textContent='';const incoming=messages.filter(message=>message.senderId!==this.getAttribute('user-id')).at(-1);if(incoming&&atBottom&&!this.loadingOlder&&!document.hidden&&this.readSeq!==incoming.id){await this.request(encodeURIComponent(peer.id),{readId:incoming.id});if(generation===this.generation){this.readSeq=incoming.id;this.loadPeople();}}}catch(error){if(this.isConnected&&generation===this.generation&&request===this.messagesRequest){this.$('.status').textContent=error.message;if(error.status===404){this.peerUnavailable=true;this.updateComposer();}this.$('.retry').hidden=false;}}finally{if(this.messagesLoading===generation&&request===this.messagesRequest)this.messagesLoading=null;}}
  async loadOlder(button){const peer=this.peer,generation=this.generation;if(!peer||this.loadingOlder)return;this.loadingOlder=true;button.disabled=true;try{const log=this.$('.messages'),first=this.history?.[0]?.id||this.latest?.[0]?.id||JSON.parse(this.lastSignature)[0]?.id;const older=await this.request(encodeURIComponent(peer.id)+'?before='+first);if(!this.isConnected||generation!==this.generation)return;const height=log.scrollHeight;this.historyDone=older.length<100;this.history=[...new Map([...older,...(this.history||[]),...(this.latest||[])].map(message=>[message.id,message])).values()].sort((a,b)=>a.id-b.id);this.lastSignature=null;this.messagesRequest++;this.messagesLoading=null;await this.loadMessages();log.scrollTop=log.scrollHeight-height;}catch(error){if(this.isConnected&&generation===this.generation){this.$('.status').textContent=error.message;button.disabled=false;}}finally{this.loadingOlder=false;}}
  async send(){if(!this.peer||this.sending||this.peerUnavailable||navigator.onLine===false)return;const input=this.$('textarea'),body=input.value.trim();if(!body)return;const peer=this.peer,generation=this.generation,saved=this.readStored(peer.id),prior=this.pendingSends.get(peer.id)||saved.pending,pending=prior?.body===body?prior:{body,id:crypto.randomUUID()};this.pendingSends.set(peer.id,pending);try{sessionStorage.setItem(this.draftKey(peer.id),JSON.stringify({...saved,body:input.value,pending}));}catch{}
   this.sending=true;this.deliveryError=false;this.updateComposer();this.$('.status').textContent='Sending…';
   try{await this.request(encodeURIComponent(peer.id),{body,clientId:pending.id});this.pendingSends.delete(peer.id);const current=this.readStored(peer.id);if(this.peer?.id===peer.id&&input.value.trim()===body)input.value='';const draft=this.peer?.id===peer.id?input.value:(this.drafts.get(peer.id)||current.body||'');const keep=draft.trim()===body?'':draft;this.drafts.set(peer.id,keep);try{if(keep)sessionStorage.setItem(this.draftKey(peer.id),JSON.stringify({body:keep}));else sessionStorage.removeItem(this.draftKey(peer.id));}catch{}if(this.isConnected&&generation===this.generation){this.lastSignature=null;this.messagesRequest++;this.messagesLoading=null;await this.loadMessages();this.$('.status').textContent=this.$('.retry').hidden?'Message sent.':'Message sent. Retry loading to update the conversation.';}}
   catch(error){if(this.isConnected&&generation===this.generation){this.deliveryError=true;this.$('.status').textContent=error.message+' Your draft is kept; tap Send to retry.';}}
   finally{this.sending=false;if(this.isConnected)this.updateComposer();}
  }
 }
 customElements.define('crm-team-chat',TeamChat);
})();
