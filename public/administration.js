'use strict';
(() => {
  const node = (tag, text, attrs = {}) => {
    const element = document.createElement(tag);
    if (text !== null && text !== undefined) element.textContent = String(text);
    for (const [key, value] of Object.entries(attrs)) element.setAttribute(key, value);
    return element;
  };
  const button = (label, run, secondary = true) => {
    const element = node('button', label, {type: 'button', class: secondary ? 'secondary' : ''});
    element.onclick = run;
    return element;
  };
  const card = (title, description) => {
    const element = node('section', null, {class: 'card'});
    element.append(node('h3', title));
    if (description) element.append(node('p', description));
    return element;
  };
  const field = (name, label, value = '', options) => {
    const wrapper = node('label', label);
    const input = node(options ? 'select' : 'input', null, {name});
    if (options) for (const option of options) input.append(node('option', option.label, {value: option.value}));
    input.value = value ?? '';
    wrapper.append(input);
    return wrapper;
  };
  const table = (rows, columns, actions) => {
    const wrap = node('div', null, {class: 'table-wrap'});
    if (!rows.length) { wrap.append(node('p', 'No matching entries.')); return wrap; }
    const element = node('table'), head = node('thead'), labels = node('tr');
    for (const [, label] of columns) labels.append(node('th', label, {scope: 'col'}));
    if (actions) labels.append(node('th', 'Actions', {scope: 'col'}));
    head.append(labels); element.append(head);
    const body = node('tbody');
    for (const row of rows) {
      const tr = node('tr');
      for (const [key] of columns) tr.append(node('td', row[key] ?? '—'));
      if (actions) { const td = node('td'); actions(row, td); tr.append(td); }
      body.append(tr);
    }
    element.append(body); wrap.append(element); return wrap;
  };
  async function request(path, options = {}) {
    let response = await fetch(path, {credentials: 'same-origin', ...options});
    if (response.status === 401) {
      const refresh = await fetch('/api/session/refresh', {method: 'POST', credentials: 'same-origin'});
      if (refresh.ok) response = await fetch(path, {credentials: 'same-origin', ...options});
    }
    const data = await response.json();
    if (!response.ok) {
      const error = new Error(data.error || data.message || 'Could not complete this request.');
      error.status = response.status;
      throw error;
    }
    return data;
  }

  class Administration extends HTMLElement {
    constructor() {
      super(); this.attachShadow({mode: 'open'});
      this.tab = 'users'; this.search = ''; this.roleFilter = ''; this.statusFilter = '';
      this.generation = 0; this.busy = false;
    }
    connectedCallback() {
      this.dataset.theme = localStorage.getItem('theme') || 'light';
      this.shadowRoot.replaceChildren();
      for (const href of ['/public/center.css', '/public/components.css', '/public/administration.css', '/public/mobile-tables.css']) {
        this.shadowRoot.append(node('link', null, {rel: 'stylesheet', href}));
      }
      this.root = node('section', null, {class: 'tools administration'});
      const header = node('header'), title = node('div');
      title.append(node('h2', 'People and access'), node('p', 'Administration applies to your workspace.'));
      this.refresh = button('Refresh', () => this.load()); header.append(title, this.refresh);
      this.notice = node('div', null, {id: 'notice', role: 'status', 'aria-live': 'polite'});
      this.confirmation = node('div'); this.content = node('div');
      this.root.append(header, this.notice, this.confirmation, this.content);
      this.shadowRoot.append(this.root); this.load();
    }
    disconnectedCallback() { this.generation++; }
    tell(message, error = false) {
      this.notice.textContent = message; this.notice.className = error ? 'error' : '';
    }
    async load() {
      const generation = ++this.generation;
      this.refresh.disabled = true;
      this.content.replaceChildren(node('p', 'Loading administration…'));
      try {
        const data = await request('/api/admin/system');
        if (!this.isConnected || generation !== this.generation) return;
        this.data = data; this.render();
      } catch (error) {
        if (!this.isConnected || generation !== this.generation) return;
        this.data = null;
        this.content.replaceChildren(node('p', error.status === 403 ? 'System administration requires Administrator access.' : error.message));
      } finally { if (generation === this.generation) this.refresh.disabled = false; }
    }
    async mutate(path, payload = {}, method = 'POST', user) {
      if (this.busy) return;
      this.busy = true; this.root.setAttribute('aria-busy', 'true');
      for (const control of this.root.querySelectorAll('button,input,select')) control.disabled = true;
      this.confirmation.replaceChildren(); this.tell('Saving…');
      try {
        const headers = {'Content-Type': 'application/json'};
        if (user?.accessVersion) headers['If-Match'] = user.accessVersion;
        const result = await request(path, {method, headers, body: JSON.stringify(payload)});
        if (!this.isConnected) return;
        if (user?.id === this.data.currentUserId && (method === 'PATCH' || path.endsWith('/revoke-sessions'))) {
          window.location.assign('/'); return;
        }
        this.editId = null; this.showCreate = false;
        this.tell(result.message || 'Changes saved.');
        if (result.inviteLink) {
          this.notice.append(node('p', 'Share this private invitation with the intended user. It expires in 72 hours.'));
          const link = node('input', null, {type: 'text', readonly: '', 'aria-label': 'Private invitation link'});
          link.value = result.inviteLink; link.onclick = () => link.select(); this.notice.append(link);
        }
        await this.load();
      } catch (error) {
        if (this.isConnected) this.tell(error.message, true);
      } finally {
        this.busy = false; this.root.removeAttribute('aria-busy');
        for (const control of this.root.querySelectorAll('button,input,select')) control.disabled = false;
      }
    }
    confirm(message, run) {
      const panel = card('Confirm account change', message);
      panel.setAttribute('role', 'alert');
      panel.append(button('Confirm', run, false), button('Cancel', () => this.confirmation.replaceChildren()));
      this.confirmation.replaceChildren(panel);
      panel.querySelector('button').focus();
    }
    render() {
      this.content.replaceChildren(); this.confirmation.replaceChildren();
      const {users} = this.data, metrics = node('div', null, {class: 'admin-metrics'});
      const counts = [
        ['Users', users.length], ['Active', users.filter(u => !u.disabled).length],
        ['Administrators', users.filter(u => !u.disabled && u.role === 'admin').length],
        ['Awaiting invitation', users.filter(u => u.disabled && u.invitationExpiresAt).length]
      ];
      for (const [label, count] of counts) {
        const metric = card(label); metric.append(node('strong', count, {class: 'metric'})); metrics.append(metric);
      }
      const nav = node('nav', null, {'aria-label': 'Administration sections'});
      for (const [key, label] of [['users','Users'], ['levels','Access levels'], ['teams','Teams'], ['security','Security'], ['settings','Workspace settings']]) {
        const tab = button(label, () => { this.tab = key; this.editId = null; this.showCreate = false; this.render(); });
        if (key === this.tab) tab.setAttribute('aria-current', 'page'); nav.append(tab);
      }
      this.content.append(metrics, nav); this.main = node('div'); this.content.append(this.main);
      ({users: () => this.renderUsers(), levels: () => this.renderLevels(), teams: () => this.renderTeams(), security: () => this.renderSecurity(), settings: () => this.renderSettings()})[this.tab]();
    }
    roleOptions() { return this.data.accessLevels.map(level => ({value: level.id, label: level.name})); }
    teamOptions() { return this.data.teams.map(team => ({value: team.id, label: team.name})); }
    roleName(id) { return this.data.accessLevels.find(level => level.id === id)?.name || id; }
    form(title, description, fields, label, save) {
      const section = card(title, description), form = node('form'), grid = node('div', null, {class: 'fields'});
      grid.append(...fields); const submit = node('button', label, {type: 'submit'});
      form.append(grid, submit);
      form.onsubmit = event => { event.preventDefault(); save(Object.fromEntries(new FormData(form))); };
      section.append(form); return section;
    }
    renderUsers() {
      const toolbar = node('div', null, {class: 'toolbar'});
      const search = field('search', 'Search users', this.search);
      search.querySelector('input').type = 'search';
      const role = field('roleFilter', 'Access level', this.roleFilter, [{value: '', label: 'All levels'}, ...this.roleOptions()]);
      const status = field('statusFilter', 'Status', this.statusFilter, ['', 'Active', 'Invited', 'Invitation expired', 'Disabled'].map(value => ({value, label: value || 'All statuses'})));
      toolbar.append(search, role, status, button('Add user', () => { this.showCreate = true; this.editId = null; this.render(); }, false));
      const list = node('div');
      const show = () => {
        const needle = this.search.trim().toLowerCase();
        const users = this.data.users.filter(u => (!needle || (u.name+' '+u.email).toLowerCase().includes(needle)) && (!this.roleFilter || u.role === this.roleFilter) && (!this.statusFilter || u.status === this.statusFilter));
        list.replaceChildren(node('p', users.length+' of '+this.data.users.length+' users'), table(users.map(u => ({...u, level: this.roleName(u.role), team: this.data.teams.find(t => t.id === u.teamId)?.name || 'No team', twoFactor: u.twoFactorEnabled ? 'Enabled' : 'Off'})), [['name','Name'],['email','Email'],['level','Access level'],['team','Team'],['status','Status'],['twoFactor','2FA'],['activeSessions','Sessions']], (u, cell) => {
          const manage = button('Manage', () => { this.editId = u.id; this.showCreate = false; this.render(); this.main.querySelector('[name=name]')?.focus(); });
          manage.setAttribute('aria-label', 'Manage '+u.name); cell.append(manage);
        }));
      };
      search.querySelector('input').oninput = event => { this.search = event.target.value; show(); };
      role.querySelector('select').onchange = event => { this.roleFilter = event.target.value; show(); };
      status.querySelector('select').onchange = event => { this.statusFilter = event.target.value; show(); };
      this.main.append(toolbar);
      if (this.showCreate) this.renderCreate();
      const user = this.data.users.find(u => u.id === this.editId);
      if (user) this.renderUser(user);
      this.main.append(list); show();
    }
    renderCreate() {
      const name = field('name', 'Full name'), email = field('email', 'Email'), password = field('password', 'Initial password (optional)');
      name.querySelector('input').required = true; name.querySelector('input').maxLength = 100;
      email.querySelector('input').type = 'email'; email.querySelector('input').required = true;
      password.querySelector('input').type = 'password'; password.querySelector('input').minLength = 12;
      password.querySelector('input').autocomplete = 'new-password';
      this.main.append(this.form('Add a user', 'Leave the password empty for a single-use invitation lasting 72 hours. Email delivery requires SMTP; the invitation can also be shared manually.', [name, email, field('role','Access level','member',this.roleOptions()), field('teamId','Team',this.data.teams[0]?.id,this.teamOptions()), password], 'Create user', data => this.mutate('/api/admin/users', data)));
    }
    renderUser(user) {
      const name = field('name', 'Full name', user.name); name.querySelector('input').required = true; name.querySelector('input').maxLength = 100;
      const section = this.form('Manage '+user.name, 'Saving access changes signs this user out on all devices. Keep at least one active Administrator.', [name, field('role','Access level',user.role,this.roleOptions()), field('teamId','Team',user.teamId,[{value:'',label:'No team'},...this.teamOptions()])], 'Save access', data => {
        const save = () => this.mutate('/api/admin/users/'+user.id, data, 'PATCH', user);
        if (data.role !== user.role || user.id === this.data.currentUserId) this.confirm('Save '+this.roleName(data.role)+' access for '+user.name+' and sign out their existing sessions?', save);
        else save();
      });
      const actions = node('div', null, {class: 'admin-actions'}), base = '/api/admin/users/'+user.id;
      if (user.disabled && user.invitationExpiresAt) {
        actions.append(button('Renew invitation', () => this.confirm('Replace the previous invitation for '+user.email+' with a new 72-hour link?', () => this.mutate(base+'/invitation', {}, 'POST', user))));
      } else {
        actions.append(button(user.disabled ? 'Enable account' : 'Disable account', () => this.confirm((user.disabled ? 'Enable sign-in for ' : 'Disable sign-in and revoke sessions and access links for ')+user.name+'?', () => this.mutate(base, {disabled: !user.disabled}, 'PATCH', user))));
      }
      if (user.disabled && user.invitationExpiresAt) actions.append(button('Cancel invitation', () => this.confirm('Cancel the invitation and keep this account disabled?', () => this.mutate(base, {disabled: true}, 'PATCH', user))));
      if (!user.disabled) {
        actions.append(button('Sign out all devices', () => this.confirm('Sign out all devices for '+user.name+'?', () => this.mutate(base+'/revoke-sessions', {}, 'POST', user))));
        actions.append(button('Send password recovery', () => this.confirm('Queue a password recovery email to '+user.email+'? The link expires in 30 minutes.', () => this.mutate(base+'/password-reset', {}, 'POST', user))));
      }
      if (user.twoFactorEnabled) actions.append(button('Reset two-factor authentication', () => this.confirm('Remove two-factor authentication and recovery codes for '+user.name+' and sign out all their devices?', () => this.mutate(base, {resetTwoFactor: true}, 'PATCH', user))));
      actions.append(button('Close', () => { this.editId = null; this.render(); }));
      section.append(actions); this.main.append(section);
    }
    renderLevels() {
      const rows = this.data.accessLevels.map(level => ({...level, recoveryText: level.recovery ? 'Allowed' : 'No', usersText: level.users ? 'Allowed' : 'No', settingsText: level.settings ? 'Allowed' : 'No'}));
      const section = card('Access levels', 'These four levels are enforced by the server. Reports and exports contain only records the user can read.');
      section.append(table(rows, [['name','Level'],['read','Read records'],['write','Create / edit / delete'],['recoveryText','Client invitations / trash'],['usersText','Manage users'],['settingsText','Workspace settings']]));
      const visibility = card('Record visibility', 'Administrator and Manager roles can access all records in their workspace. Member and Viewer access follows record ownership and visibility.');
      visibility.append(table([{name:'Private',access:'Owner, Administrators and Managers'}, {name:'Team',access:'Owner, matching team, Administrators and Managers'}, {name:'Organization',access:'All users in this workspace'}], [['name','Visibility'],['access','Who can read']]));
      visibility.append(node('a', 'Manage record ownership and visibility', {href:'/workspace?tab=team'}));
      this.main.append(section, visibility);
    }
    renderTeams() {
      const section = card('Teams', 'Team assignment controls which team-shared records a user can read. Change assignments from Users.');
      section.append(table(this.data.teams.map(team => ({...team, members:this.data.users.filter(u => u.teamId === team.id).length})), [['name','Team'],['members','Users']]));
      const name = field('name','Team name'); name.querySelector('input').required = true; name.querySelector('input').maxLength = 100;
      this.main.append(section, this.form('Create a team', '', [name], 'Create team', data => this.mutate('/api/admin/teams', data)));
    }
    renderSecurity() {
      const sessions = card('Active sessions', 'The 100 most recently active sessions in this workspace. Account access changes revoke all previous sessions.');
      sessions.append(table(this.data.sessions, [['name','User'],['userAgent','Browser / device'],['ip','IP address'],['lastSeen','Last active']], (row, cell) => {
        const user = this.data.users.find(u => u.id === row.userId);
        if (user) cell.append(button('Sign out all devices', () => this.confirm('Sign out all devices for '+user.name+'?', () => this.mutate('/api/admin/users/'+user.id+'/revoke-sessions', {}, 'POST', user))));
      }));
      const changes = card('User access history', 'The 100 most recent user changes, with the administrator who made each change.');
      changes.append(table(this.data.changes, [['actor','Changed by'],['label','User'],['action','Action'],['occurredAt','When']]));
      const events = card('Login and security history', 'The 100 most recent security events in this workspace.');
      events.append(table(this.data.security, [['name','User / actor'],['event','Event'],['ip','IP address'],['occurredAt','When']]));
      this.main.append(sessions, changes, events);
    }
    renderSettings() {
      const org = this.data.organization[0], name = field('name','Workspace name',org.name), timezone = field('timezone','Timezone',org.timezone);
      name.querySelector('input').required = true; timezone.querySelector('input').required = true;
      const currencies = ['USD','EUR','GBP','CAD','AUD','NZD','PHP','SGD','INR','JPY','CNY','AED','HKD','CHF','KRW','MYR','THB','IDR'].map(value => ({value,label:value}));
      this.main.append(this.form('Workspace settings', 'Use an IANA timezone, such as Asia/Manila. Leave allowed networks empty for unrestricted access; an allowlist must include your current network.', [name, timezone, field('currency','Reporting currency',org.currency,currencies), field('ipAllowlist','Allowed IP addresses / CIDRs',org.ipAllowlist)], 'Save settings', data => this.mutate('/api/admin/organization', data, 'PUT')));
      const connections = card('Integrations', 'Configure email delivery, AI assistance and webhooks.');
      connections.append(node('a','Open integration settings',{href:'/workspace?tab=integrations'})); this.main.append(connections);
      const readiness = card('Workspace readiness', 'Review setup, delivery failures, backup status and isolated restore verification.');
      readiness.append(node('a','Getting started & readiness',{href:'/workspace?tab=readiness'})); this.main.append(readiness);
    }
  }
  customElements.define('crm-administration', Administration);
})();
