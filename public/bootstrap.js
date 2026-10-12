
    (function () {
      localStorage.removeItem("token");
      var token = "cookie-session";
      var now = new Date();
      var today = [now.getFullYear(), String(now.getMonth() + 1).padStart(2, "0"), String(now.getDate()).padStart(2, "0")].join("-");

      var app = Elm.Main.init({
        node: document.getElementById("app"),
        flags: {
          token: token,
          today: today,
          theme: localStorage.getItem("theme")
        }
      });
      window.connectDraftStorage(app);
      app.ports.recordSaved.subscribe(function (id) { window.dispatchEvent(new CustomEvent('crm:record-changed', {detail: id})); });
      window.addEventListener('crm:workspace-action', function (event) { app.ports.workspaceAction.send(event.detail); });
      app.ports.storeToken.subscribe(function (token) {
        localStorage.removeItem("token");
        if (!token) fetch("/api/session/logout", {method:"POST",credentials:"same-origin"});
      });
      app.ports.storeTheme.subscribe(function (theme) { localStorage.setItem("theme", theme); window.dispatchEvent(new Event("crm:theme-changed")); });
      window.addEventListener("crm:record-changed", function(event) { app.ports.recordChanged.send(event.detail || ""); });
      app.ports.downloadFile.subscribe(function (file) {
        var url = URL.createObjectURL(new Blob([file.content], { type: file.mime }));
        var link = document.createElement("a");
        link.href = url;
        link.download = file.filename;
        document.body.appendChild(link);
        link.click();
        link.remove();
        setTimeout(function () { URL.revokeObjectURL(url); }, 1000);
      });
    })();

window.crmWorkspaceInteractions = new window.CRMWorkspaceInteractions(document);
window.crmWorkspaceInteractions.start();
