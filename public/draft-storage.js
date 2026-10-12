'use strict';

// Elm owns form values, recovery, and record revisions. This adapter only stores
// explicitly supplied drafts; it never reads the DOM or intercepts requests.
window.connectDraftStorage = function (app) {
  app.ports.draftStorage.subscribe(({action, key, fields}) => {
    const storageKey = 'ecc:form-draft:v3:' + key;
    try {
      if (action === 'read') {
        const saved = sessionStorage.getItem(storageKey);
        app.ports.storedDraft.send({key, fields: saved ? JSON.parse(saved) : null});
      } else if (action === 'write') {
        sessionStorage.setItem(storageKey, JSON.stringify(fields));
      } else if (action === 'remove') {
        sessionStorage.removeItem(storageKey);
      }
    } catch {
      if (action === 'read') app.ports.storedDraft.send({key, fields: null});
    }
  });
};
