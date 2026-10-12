'use strict';

(() => {
  const element = (tag, text, attrs = {}) => {
    const node = document.createElement(tag);
    if (text != null) node.textContent = String(text);
    for (const [key, value] of Object.entries(attrs)) node.setAttribute(key, value);
    return node;
  };
  const labels = {contacts: 'Client', leads: 'Inquiry', students: 'Student', cases: 'Application', invoices: 'Invoice'};
  class ClientWorkspace extends HTMLElement {
    static get observedAttributes() { return ['entity', 'record-id']; }
    constructor() {
      super();
      this.attachShadow({mode: 'open'});
      this.generation = 0;
      this.tab = 'actions';
      this.selected = {}; this.context = null;
    }
    connectedCallback() {
      this.themeChanged = () => {
        this.dataset.theme = localStorage.getItem('theme') || 'light';
        for (const host of this.content?.querySelectorAll('crm-record-tools,crm-work-plan,crm-application-blockers,crm-client-history') || []) host.dataset.theme = this.dataset.theme;
      };
      this.changed = () => this.load(false);
      this.hashChanged = () => {
        if (!this.data || !this.canLeave()) return;
        this.tab = this.hashTab(); this.render();
      };
      this.selectTab = event => {
        const tab = event.detail?.tab;
        if (!tab || !this.data || !this.canLeave()) return;
        this.tab = ({documents: 'files', conversations: 'messages', communication: 'messages', email: 'messages', history: 'history', workflows: 'actions'})[tab] || 'actions';
        this.render();
      };
      this.addEventListener('crm:select-tab', this.selectTab);
      window.addEventListener('crm:record-changed', this.changed);
      window.addEventListener('hashchange', this.hashChanged);
      window.addEventListener('crm:theme-changed', this.themeChanged);
      this.mount();
      this.themeChanged();
    }
    attributeChangedCallback() { if (this.isConnected) this.mount(); }
    disconnectedCallback() {
      this.generation++;
      this.controller?.abort();
      window.removeEventListener('crm:record-changed', this.changed);
      window.removeEventListener('hashchange', this.hashChanged);
      window.removeEventListener('crm:theme-changed', this.themeChanged);
      this.removeEventListener('crm:select-tab', this.selectTab);
    }
    mount() {
      this.tab = this.hashTab(); this.selected = {}; this.context = null;
      this.shadowRoot.replaceChildren();
      for (const file of ['center.css', 'components.css']) this.shadowRoot.append(element('link', null, {rel: 'stylesheet', href: '/public/' + file}));
      const section = element('section', null, {class: 'tools client-workspace', 'aria-label': 'Client workspace'});
      this.heading = element('h1', 'Client workspace');
      this.notice = element('p', 'Loading linked records…', {role: 'status', 'aria-live': 'polite'});
      const refresh = element('button', 'Refresh', {type: 'button', class: 'secondary'});
      refresh.onclick = () => this.load(true);
      const header = element('header'); header.append(this.heading, refresh);
      this.navigation = element('nav', null, {'aria-label': 'Client work'});
      this.content = element('div', null, {class: 'client-workspace__content'});
      section.append(header, this.notice, this.navigation, this.content);
      this.shadowRoot.append(section);
      this.load(true);
    }
    hasUnsavedChanges() {
      return [...this.content.querySelectorAll('crm-work-plan,crm-record-tools,crm-application-blockers')].some(host => host.dirty || host.busy || host.hasUnsavedChanges?.() || host.shadowRoot?.querySelector('form[data-dirty="true"]'));
    }
    hashTab() {
      const params = new URLSearchParams(location.hash.split('?')[1] || '');
      return ({documents: 'files', conversations: 'messages', communication: 'messages', history: 'history', workflows: 'actions'})[params.get('tab')] || 'actions';
    }
    canLeave() {
      if (this.hasUnsavedChanges()) {
        this.notice.hidden = false; this.notice.textContent = 'Save or discard your open entries before switching work. Your draft is still here.';
        return false;
      }
      return true;
    }
    async load(explicit) {
      if (this.hasUnsavedChanges()) {
        if (explicit) { this.notice.hidden = false; this.notice.textContent = 'Your entries are kept. Save or discard them before refreshing.'; }
        return;
      }
      const generation = ++this.generation;
      this.controller?.abort(); this.controller = new AbortController();
      try {
        let response = await fetch('/api/records/' + encodeURIComponent(this.getAttribute('entity')) + '/' + encodeURIComponent(this.getAttribute('record-id')) + '/workspace', {credentials: 'same-origin', signal: this.controller.signal});
        if (response.status === 401) {
          const refresh = await fetch('/api/session/refresh', {method: 'POST', credentials: 'same-origin', signal: this.controller.signal});
          if (refresh.ok) response = await fetch(response.url, {credentials: 'same-origin', signal: this.controller.signal});
        }
        const result = await response.json();
        if (!response.ok) throw new Error(result.error || 'Could not load this client workspace.');
        if (!this.isConnected || generation !== this.generation) return;
        // A user can start entering a draft while this refresh is in flight.
        if (this.hasUnsavedChanges()) return;
        this.data = result;
        const client = (result.entity === 'contacts' ? result.anchor : null) || (result.records.contacts || [])[0] || (result.records.students || [])[0] || (result.records.leads || [])[0];
        this.heading.textContent = client?.name || result.anchor.name || result.label || 'Client workspace';
        this.notice.textContent = '';
        this.notice.hidden = true;
        this.render();
      } catch (error) {
        if (generation === this.generation && error.name !== 'AbortError') { this.notice.hidden = false; this.notice.textContent = error.message + ' Use Refresh to retry.'; }
      }
    }
    records(entities) {
      return entities.flatMap(entity => (this.data.records[entity] || []).map(record => ({entity, record})));
    }
    title({entity, record}) {
      return labels[entity] + ' · ' + (record.caseNumber || record.invoiceNumber || record.leadNumber || record.name || record.id);
    }
    render() {
      this.notice.hidden = true;
      const tabs = [['actions', 'Next actions'], ['files', 'Files'], ['messages', 'Communication'], ['finance', 'Payments'], ['history', 'History']];
      for (const [key, label] of tabs) {
        const button = this.navigation.querySelector('[data-work-tab="' + key + '"]') || element('button', label, {type: 'button', class: 'secondary', 'data-work-tab': key});
        button.setAttribute('aria-pressed', String(this.tab === key));
        button.onclick = () => { if (this.tab !== key && this.canLeave()) { this.tab = key; this.render(); } };
        if (!button.parentNode) this.navigation.append(button);
      }
      this.content.replaceChildren();
      const contexts = this.records(['cases', 'leads', 'students', 'contacts']);
      const current = contexts.find(item => item.entity + '/' + item.record.id === this.context) || contexts.find(item => item.entity === this.getAttribute('entity') && item.entity !== 'contacts' && item.record.id === this.getAttribute('record-id')) || contexts[0];
      if (!current) { this.content.append(element('p', 'No accessible client records.')); return; }
      this.context = current.entity + '/' + current.record.id;
      const label = element('label', 'Application or client inquiry', {class: 'client-workspace__picker'});
      const picker = element('select', null, {'aria-label': 'Application or client inquiry'});
      for (const item of contexts) {
        const option = element('option', this.title(item), {value: item.entity + '/' + item.record.id});
        option.selected = item === current; picker.append(option);
      }
      picker.onchange = () => {
        if (!this.canLeave()) { picker.value = this.context; return; }
        this.context = picker.value; this.render();
      };
      label.append(picker); this.content.append(label);
      let selected = current;
      if (this.tab === 'finance') {
        const invoices = this.records(['invoices']).filter(item => current.entity === 'cases' ? item.record.caseId === current.record.id : current.entity === 'contacts' ? item.record.clientId === current.record.id : true);
        if (current.entity === 'cases' || current.entity === 'contacts') {
          if (current.record._editable) {
            const add = element('button', 'Add invoice', {type:'button', class:'secondary'});
            add.onclick = () => window.dispatchEvent(new CustomEvent('crm:workspace-action', {detail:{entity:current.entity, id:current.record.id, action:'invoice'}}));
            this.content.append(add);
          }
        }
        if (!invoices.length) { this.content.append(element('p', 'No invoices linked to ' + this.title(current) + '.')); return; }
        selected = invoices.find(item => item.record.id === this.selected.finance) || invoices[0];
        const invoicesLabel = element('label', 'Invoice for ' + this.title(current), {class:'client-workspace__picker'});
        const invoicesPicker = element('select', null, {'aria-label':'Invoice'});
        for (const item of invoices) { const option = element('option', this.title(item), {value:item.record.id}); option.selected = item === selected; invoicesPicker.append(option); }
        invoicesPicker.onchange = () => { if (!this.canLeave()) { invoicesPicker.value = selected.record.id; return; } this.selected.finance = invoicesPicker.value; this.render(); };
        invoicesLabel.append(invoicesPicker); this.content.append(invoicesLabel);
      }
      if (this.tab === 'files' && !['cases','leads'].includes(current.entity)) {
        this.content.append(element('p', 'Select an application or inquiry above to review its files.'));
        if (current.record._editable && ['contacts','students'].includes(current.entity)) {
          const add = element('button', 'Add application', {type:'button', class:'secondary'});
          add.onclick = () => window.dispatchEvent(new CustomEvent('crm:workspace-action', {detail:{entity:current.entity, id:current.record.id, action:'application'}}));
          this.content.append(add);
        }
        return;
      }
      if (['messages','history'].includes(this.tab) && current.entity === 'leads') {
        this.content.append(element('p', 'Communication and client history become available after this inquiry is linked to a client or application.'));
        const link = element('button', 'Review inquiry and conversion', {type:'button', class:'secondary'});
        link.onclick = () => { if (this.canLeave()) { this.tab = 'actions'; this.render(); } };
        this.content.append(link); return;
      }
      const {entity, record} = selected;
      const summary = element('section', null, {class: 'card client-workspace__summary'});
      summary.append(element('p', record.currentStage || record.applicationStage || record.status || record.stage || record.paymentMilestone || 'Client profile'));
      const owner = element('p', 'Responsible staff: ');
      owner.append(element('crm-staff-name', null, {value: record.assignedOfficer || record.assignedTo || record.owner || record.ownerId || ''}));
      summary.append(owner);
      if (record._editable) {
        const edit = element('button', 'Edit ' + labels[entity].toLowerCase(), {type: 'button', class: 'secondary'});
        edit.onclick = () => window.dispatchEvent(new CustomEvent('crm:workspace-action', {detail: {entity, id: record.id, action: 'edit'}}));
        summary.append(edit);
      } else summary.append(element('p', 'Read-only record'));
      if (['contacts','students'].includes(entity) && record._editable) {
        const add = element('button', 'Add application', {type:'button', class:'secondary'});
        add.onclick = () => window.dispatchEvent(new CustomEvent('crm:workspace-action', {detail:{entity, id:record.id, action:'application'}}));
        summary.append(add);
      }
      this.content.append(summary);
      if (this.tab === 'actions') {
        const documents = (this.data.records.documents || []).filter(document => document.caseId === record.id);
        const missing = documents.filter(document => !document.filePath).length;
        const deadline = record.nextDeadline || record.followUpDate || '';
        const overdue = /^\d{4}-\d{2}-\d{2}/.test(deadline) && deadline.slice(0,10) < [new Date().getFullYear(), String(new Date().getMonth()+1).padStart(2,'0'), String(new Date().getDate()).padStart(2,'0')].join('-');
        if (missing || overdue) {
          const attention = element('section', null, {class:'client-workspace__attention', 'aria-label':'Needs attention'});
          attention.append(element('strong', 'Needs attention'));
          if (overdue) attention.append(element('p', 'Follow-up overdue · ' + deadline));
          if (missing) {
            const review = element('button', missing + ' missing files · Review requirements', {type:'button', class:'secondary'});
            review.onclick = () => { const details = this.content.querySelector('.journey-section'); if (details) { details.open = true; details.scrollIntoView({block:'start', behavior:'smooth'}); } };
            attention.append(review);
          }
          this.content.append(attention);
        }
        if (['cases', 'leads', 'students'].includes(entity)) this.content.append(element('crm-work-plan', null, {entity, 'record-id': record.id, compact: ''}));
        if (entity === 'cases') {
          const documents = (this.data.records.documents || []).filter(document => document.caseId === record.id);
          const requirements = element('details', null, {class: 'journey-section'});
          requirements.append(element('summary', 'Application requirements · ' + documents.length + ' documents · ' + documents.filter(document => !document.filePath).length + ' missing files'));
          requirements.addEventListener('toggle', () => {
            if (requirements.open && !requirements.querySelector('crm-application-blockers')) {
              const blockers = element('crm-application-blockers', null, {'record-id': record.id, 'data-theme': this.dataset.theme || 'light'});
              requirements.append(blockers);
            }
          });
          this.content.append(requirements);
        }
        if (entity === 'leads') this.content.append(element('crm-record-tools', null, {entity, 'record-id': record.id, 'initial-tab': 'documents'}));
        if (entity === 'students') this.content.append(element('crm-record-tools', null, {entity, 'record-id': record.id, 'initial-tab': 'workflows'}));
        if (entity === 'contacts') this.content.append(element('p', 'Select an application above to plan the next step, or add an application for this client.'));
      } else if (this.tab === 'history') {
        this.content.append(element('crm-client-history', null, {entity, 'record-id': record.id}));
      } else {
        const tools = element('crm-record-tools', null, {entity, 'record-id': record.id, 'initial-tab': this.tab === 'files' ? 'documents' : this.tab === 'messages' ? 'communication' : 'workflows'});
        this.content.append(tools);
      }
      if (Object.values(this.data.hasMore).some(Boolean)) this.content.append(element('p', 'Showing the first 100 linked records per type. Use All modules to find additional records.'));
      this.themeChanged();
    }
  }
  customElements.define('crm-client-workspace', ClientWorkspace);
})();
