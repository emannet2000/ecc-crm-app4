'use strict';
// Owns keyboard access, modal focus, read-only affordances and handoff deep links
// for the Elm workspace. One observer and delegated listeners have one lifecycle.
(() => {
  class WorkspaceInteractions {
    constructor(document) {
      this.document = document;
      this.activeDialog = null;
      this.returnFocus = null;
      this.refresh = this.refresh.bind(this);
      this.onKeydown = this.onKeydown.bind(this);
      this.onClick = this.onClick.bind(this);
    }

    start() {
      if (this.observer) return;
      this.observer = new MutationObserver(this.refresh);
      this.observer.observe(this.document.body, {childList: true, subtree: true});
      this.document.addEventListener('keydown', this.onKeydown);
      this.document.addEventListener('click', this.onClick);
      window.addEventListener('hashchange', this.refresh);
      this.refresh();
    }

    stop() {
      this.observer?.disconnect();
      this.observer = null;
      this.document.removeEventListener('keydown', this.onKeydown);
      this.document.removeEventListener('click', this.onClick);
      window.removeEventListener('hashchange', this.refresh);
      this.activeDialog = null;
      this.returnFocus = null;
    }

    refresh() {
      const main = this.document.querySelector('main.content, .content__body');
      if (main) {
        if (main.id !== 'workspace-content') main.id = 'workspace-content';
        if (main.getAttribute('tabindex') !== '-1') main.setAttribute('tabindex', '-1');
      }
      for (const row of this.document.querySelectorAll('tbody tr')) {
        if (row.querySelector('a,button,input') || row.dataset.keyboard) continue;
        row.dataset.keyboard = 'true';
        row.tabIndex = 0;
        row.setAttribute('role', 'button');
      }
      const dialog = this.document.querySelector('[role=dialog], .modal, .ecc-modal, .modal-panel');
      if (dialog !== this.activeDialog) {
        if (dialog) {
          this.returnFocus = this.document.activeElement;
          if (!dialog.hasAttribute('role')) dialog.setAttribute('role', 'dialog');
          dialog.setAttribute('aria-modal', 'true');
          if (!dialog.getAttribute('aria-label') && !dialog.getAttribute('aria-labelledby')) {
            dialog.setAttribute('aria-label', dialog.querySelector('h2,h3')?.textContent || 'Edit record');
          }
          this.activeDialog = dialog;
          requestAnimationFrame(() => {
            if (this.activeDialog === dialog && dialog.isConnected) {
              dialog.querySelector('input,select,textarea,button,[tabindex]')?.focus();
            }
          });
        } else {
          this.activeDialog = null;
          if (this.returnFocus?.isConnected) this.returnFocus.focus();
        }
      }
      if (window.location.hash === '#handoff') {
        const panel = this.document.getElementById('handoff');
        if (panel?.tagName === 'DETAILS' && !panel.open) panel.open = true;
      }
      if (['#handoff', '#next-action', '#record-details'].includes(window.location.hash)) {
        const details = this.document.querySelector('.client-record-details');
        if (details && !details.open) details.open = true;
      }
    }

    onKeydown(event) {
      const row = event.target.closest?.('tr[data-keyboard="true"]');
      if (row && event.target === row && ['Enter', ' '].includes(event.key)) {
        event.preventDefault();
        row.click();
        return;
      }
      if (event.key !== 'Tab' || !this.activeDialog) return;
      const focusable = [...this.activeDialog.querySelectorAll('a[href],button:not([disabled]),input:not([disabled]),select:not([disabled]),textarea:not([disabled]),[tabindex="0"]')]
        .filter(element => element.offsetParent !== null);
      if (!focusable.length) return;
      const first = focusable[0], last = focusable.at(-1);
      if (event.shiftKey && this.document.activeElement === first) {
        event.preventDefault();
        last.focus();
      } else if (!event.shiftKey && this.document.activeElement === last) {
        event.preventDefault();
        first.focus();
      }
    }

    onClick(event) {
      if (event.target.closest?.('.skip-link')) this.document.getElementById('workspace-content')?.focus();
    }
  }
  window.CRMWorkspaceInteractions = WorkspaceInteractions;
})();
