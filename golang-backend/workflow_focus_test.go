package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestManagerExceptionQueuesRespectWorkspaceAndRole(t *testing.T) {
	setupStore(t)
	u, _ := findUserByEmail("demo@northwind.dev")
	member := addPermissionUser(t, "member", u.OrgID, "exception-member")
	scope := RecordScope{OrgID: u.OrgID, OwnerID: u.ID, Visibility: "organization"}
	foreign := scope
	foreign.OrgID = "exception-foreign"
	mu.Lock()
	cases = []Case{{RecordScope: scope, ID: "exception-app", CaseNumber: "APP-EXCEPTION", CurrentStage: "Assessment", DateOpened: "2000-01-01"}, {RecordScope: foreign, ID: "hidden-app", CaseNumber: "HIDDEN", CurrentStage: "Assessment", DateOpened: "2000-01-01"}, {RecordScope: scope, ID: "closed-app", CurrentStage: "Closed", DateOpened: "2000-01-01"}}
	leads = []Lead{}
	deals = []Deal{}
	students = []Student{}
	mu.Unlock()
	if err := saveStore(); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec("UPDATE case_stage_events SET entered_at='2000-01-01T00:00:00Z' WHERE case_id='exception-app'"); err != nil {
		t.Fatal(err)
	}
	handoff := ApplicationHandoff{CaseID: "exception-app", Status: "pending", DueDate: "2000-01-01", FromName: u.Name, ToName: "Receiving adviser"}
	_, err := database.Exec("INSERT INTO workspace_entries VALUES(?,?,'application_handoff',?,?,?, ?,?)", "exception-handoff", u.OrgID, u.ID, "exception-app", mustJSON(handoff), utcNow(), utcNow())
	if err != nil {
		t.Fatal(err)
	}
	h := leadIntakeMux()
	for _, queue := range []string{"unassigned", "stalled", "overdueHandoffs"} {
		w := rawUserRequest(t, h, u, "GET", "/api/workspace-priorities?queue="+queue, nil)
		assertStatus(t, w, 200)
		var out struct {
			Items     []PriorityRow
			Total     int
			CanManage bool
		}
		if json.Unmarshal(w.Body.Bytes(), &out) != nil || out.Total != 1 || !out.CanManage || out.Items[0].Title != "APP-EXCEPTION" {
			t.Fatal(w.Body.String())
		}
		if strings.Contains(w.Body.String(), "HIDDEN") {
			t.Fatal("foreign record leaked")
		}
		assertStatus(t, rawUserRequest(t, h, member, "GET", "/api/workspace-priorities?queue="+queue, nil), 400)
	}
	// A recent transition removes an old application's stalled classification.
	_, err = database.Exec("INSERT INTO case_stage_events VALUES(?,?,?,?,?)", "recent-stage", u.OrgID, "exception-app", "Assessment", time.Now().UTC().Format(time.RFC3339Nano))
	if err != nil {
		t.Fatal(err)
	}
	w := rawUserRequest(t, h, u, "GET", "/api/workspace-priorities?queue=stalled", nil)
	assertStatus(t, w, 200)
	var out struct{ Total int }
	json.Unmarshal(w.Body.Bytes(), &out)
	if out.Total != 0 {
		t.Fatal(w.Body.String())
	}
}

func TestCommunicationHistoryIncludesQueuedMailWithoutLeakingPrivateApplications(t *testing.T) {
	setupStore(t)
	u, _ := findUserByEmail("demo@northwind.dev")
	member := addPermissionUser(t, "member", u.OrgID, "communication-member")
	scope := RecordScope{OrgID: u.OrgID, OwnerID: member.ID, Visibility: "organization"}
	private := RecordScope{OrgID: u.OrgID, OwnerID: u.ID, Visibility: "private"}
	mu.Lock()
	contacts = []Contact{{RecordScope: scope, ID: "communication-client", Name: "Client"}}
	cases = []Case{{RecordScope: scope, ID: "communication-app", CaseNumber: "VISIBLE-APP", ClientID: "communication-client"}, {RecordScope: private, ID: "private-app", CaseNumber: "PRIVATE", ClientID: "communication-client"}}
	invoices = []Invoice{}
	mu.Unlock()
	if err := saveStore(); err != nil {
		t.Fatal(err)
	}
	for _, row := range []struct{ id, app string }{{"incoming", "communication-app"}, {"hidden", "private-app"}} {
		_, err := database.Exec("INSERT INTO conversations VALUES(?,?,?,?,?,?,?,?,?,?,?)", row.id, u.OrgID, "communication-client", row.app, "email", "inbound", row.id, row.id, "Body", "received", "2026-10-10")
		if err != nil {
			t.Fatal(err)
		}
	}
	_, err := database.Exec("INSERT INTO mail_outbox(id,org_id,user_id,contact_id,kind,recipient,subject,body,status,created_at) VALUES(?,?,?,?,'compose','test@example.com','Queued test','Body','delivery_unknown','2026-10-11')", "queued-test", u.OrgID, member.ID, "communication-client")
	if err != nil {
		t.Fatal(err)
	}
	h := leadIntakeMux()
	w := rawUserRequest(t, h, member, "GET", "/api/contacts/communication-client/conversations?limit=1", nil)
	assertStatus(t, w, http.StatusOK)
	var out struct {
		Total    int
		Messages []map[string]any
	}
	json.Unmarshal(w.Body.Bytes(), &out)
	if out.Total != 2 || len(out.Messages) != 1 || out.Messages[0]["source"] != "mail" || !strings.Contains(fmtString(out.Messages[0]["deliveryExplanation"]), "before retrying") {
		t.Fatal(w.Body.String())
	}
	w = rawUserRequest(t, h, member, "GET", "/api/contacts/communication-client/conversations?limit=1&offset=1", nil)
	assertStatus(t, w, 200)
	if strings.Contains(w.Body.String(), "private-app") || !strings.Contains(w.Body.String(), "VISIBLE-APP") {
		t.Fatal(w.Body.String())
	}
	if strings.Contains(deliveryExplanation("email", "sent"), "confirmed delivery") {
		t.Fatal("SMTP acceptance claimed delivery")
	}
}

func TestServiceTemplatesPreserveDocumentsAndExistingPlans(t *testing.T) {
	setupStore(t)
	u, _ := findUserByEmail("demo@northwind.dev")
	scope := RecordScope{OrgID: u.OrgID, OwnerID: u.ID, Visibility: "organization"}
	mu.Lock()
	cases = []Case{{RecordScope: scope, ID: "template-app", CaseNumber: "APP-TEMPLATE", ServiceCategory: "Study", CurrentStage: "Assessment", NextAction: "Existing plan", NextDeadline: "2099-01-01"}}
	documents = []Document{{RecordScope: scope, ID: "existing-passport", CaseID: "template-app", DocName: "Passport", FilePath: "private-evidence", LatestVersion: 1}}
	mu.Unlock()
	if err := saveStore(); err != nil {
		t.Fatal(err)
	}
	h := leadIntakeMux()
	body := map[string]any{"name": "Study starter", "service": "Study", "documents": "Passport\nCV\nCV", "stages": "Assessment\nDocuments Pending", "nextAction": "Review CV", "deadlineDays": 7}
	w := rawUserRequest(t, h, u, "POST", "/api/workflows/case-template", body)
	assertStatus(t, w, 201)
	var template struct{ ID string }
	json.Unmarshal(w.Body.Bytes(), &template)
	save := func() {
		t.Helper()
		w := rawUserRequest(t, h, u, "GET", "/api/records/cases/template-app", nil)
		var record struct{ Record map[string]any }
		json.Unmarshal(w.Body.Bytes(), &record)
		req := map[string]any{"id": "template-app", "templateId": template.ID, "applyTemplatePlan": true}
		assertStatus(t, rawUserRequestWithRevision(t, h, u, "POST", "/api/workflows/case", req, fmtString(record.Record["_revision"])), 200)
	}
	save()
	save()
	w = rawUserRequest(t, h, u, "GET", "/api/records/cases/template-app", nil)
	assertStatus(t, w, 200)
	if !strings.Contains(w.Body.String(), "Existing plan") || !strings.Contains(w.Body.String(), "2099-01-01") {
		t.Fatal(w.Body.String())
	}
	var count int
	database.QueryRow("SELECT count(*) FROM documents WHERE json_extract(data,'$.caseId')='template-app'").Scan(&count)
	if count != 2 {
		t.Fatalf("expected existing passport and one CV; got %d", count)
	}
	body["stages"] = "Invented stage"
	assertStatus(t, rawUserRequest(t, h, u, "POST", "/api/workflows/case-template", body), 400)
}

func rawUserRequestWithRevision(t *testing.T, h http.Handler, u User, method, path string, body any, revision string) *httptest.ResponseRecorder {
	t.Helper()
	token, err := issueToken(u)
	if err != nil {
		t.Fatal(err)
	}
	r := httptest.NewRequest(method, path, strings.NewReader(mustJSON(body)))
	r.Header.Set("Authorization", "Bearer "+token)
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("If-Match", revision)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}

func TestOperationalAttentionIsScopedAndDoesNotExposeBackupPaths(t *testing.T) {
	setupStore(t)
	u, _ := findUserByEmail("demo@northwind.dev")
	member := addPermissionUser(t, "member", u.OrgID, "operations-member")
	manager := addPermissionUser(t, "manager", u.OrgID, "operations-manager")
	scope := RecordScope{OrgID: u.OrgID, OwnerID: u.ID, Visibility: "organization"}
	mu.Lock()
	contacts = append(contacts, Contact{RecordScope: scope, ID: "operation-client", Name: "Operations client"})
	mu.Unlock()
	if err := saveStore(); err != nil {
		t.Fatal(err)
	}
	for _, row := range []struct{ id, org, status string }{{"own-uncertain", u.OrgID, "delivery_unknown"}, {"foreign-uncertain", "foreign-org", "delivery_unknown"}} {
		_, err := database.Exec("INSERT INTO mail_outbox(id,org_id,user_id,contact_id,kind,recipient,subject,body,status,created_at) VALUES(?,?,?,'operation-client','compose','test@example.com','Test','Body',?,?)", row.id, row.org, u.ID, row.status, utcNow())
		if err != nil {
			t.Fatal(err)
		}
	}
	raw := `{"state":"ok","checkedAt":"2000-01-01T00:00:00Z","destination":"private-backup-path","restoreVerification":{"state":"verified","archive":"private-backup-path"}}`
	if err := os.WriteFile(filepath.Join(filepath.Dir(dataPath), "backup-status.json"), []byte(raw), 0600); err != nil {
		t.Fatal(err)
	}
	h := leadIntakeMux()
	assertStatus(t, rawUserRequest(t, h, member, "GET", "/api/operations/attention", nil), 403)
	w := rawUserRequest(t, h, manager, "GET", "/api/operations/attention", nil)
	assertStatus(t, w, 200)
	var out struct {
		Counts     map[string]int
		Deliveries []map[string]any
		Backup     map[string]any
	}
	json.Unmarshal(w.Body.Bytes(), &out)
	if len(out.Deliveries) != 1 || out.Deliveries[0]["route"] != "/contacts/operation-client#record-tools?tab=conversations" || out.Counts["emailUncertain"] != 1 || out.Backup["state"] != "stale" || out.Backup["restoreState"] != "verified" || strings.Contains(w.Body.String(), "private-backup-path") {
		t.Fatal(w.Body.String())
	}
}
