package main

import (
	"encoding/json"
	"fmt"
	"testing"
)

func consolidatedMember(t *testing.T) (User, RecordScope, RecordScope) {
	t.Helper()
	setupStore(t)
	u, _ := findUserByEmail("demo@northwind.dev")
	u.Role = "member"
	users[u.Email] = u
	if err := saveStore(); err != nil {
		t.Fatal(err)
	}
	return u, RecordScope{OrgID: u.OrgID, OwnerID: u.ID, Visibility: "organization"}, RecordScope{OrgID: u.OrgID, OwnerID: "other", Visibility: "private"}
}

func TestCommunicationWorkspaceChannelsPermissionsAndPaging(t *testing.T) {
	u, visible, hidden := consolidatedMember(t)
	insertSearchFixture(t, "contacts", "hub-client", visible, map[string]any{"name": "Hub client"})
	insertSearchFixture(t, "contacts", "hub-hidden-client", hidden, map[string]any{"name": "Hidden client"})
	insertSearchFixture(t, "cases", "hub-case", visible, map[string]any{"caseNumber": "HUB-001"})
	insertSearchFixture(t, "students", "hub-student", visible, map[string]any{"name": "Hub student"})
	insertSearchFixture(t, "students", "hub-hidden-student", hidden, map[string]any{"name": "Hidden student"})
	if _, err := database.Exec("UPDATE cases SET data=json_set(data,'$.studentId','hub-student') WHERE id='hub-case'"); err != nil {
		t.Fatal(err)
	}
	insertSearchFixture(t, "cases", "hub-hidden-case", hidden, map[string]any{"caseNumber": "PRIVATE-001"})
	insertSearchFixture(t, "invoices", "hub-hidden-invoice", hidden, map[string]any{"invoiceNumber": "PRIVATE-INVOICE"})
	add := func(id, contact, application, channel, org string) {
		t.Helper()
		if _, err := database.Exec("INSERT INTO conversations VALUES(?,?,?,?,?,?,?,?,?,?,?)", id, org, contact, application, channel, "inbound", id, "Hub message", "Body", "received", "2026-10-11T09:00:00Z"); err != nil {
			t.Fatal(err)
		}
	}
	for i := 0; i < 28; i++ {
		add(fmt.Sprintf("hub-%02d", i), "hub-client", "hub-case", []string{"email", "whatsapp", "portal"}[i%3], u.OrgID)
	}
	add("hub-hidden-contact", "hub-hidden-client", "hub-case", "email", u.OrgID)
	add("hub-hidden-case", "hub-client", "hub-hidden-case", "portal", u.OrgID)
	add("hub-unmatched", "", "", "email", u.OrgID)
	add("hub-foreign", "hub-client", "hub-case", "email", "other-workspace")
	add("hub-student-response", "", "hub-case", "portal", u.OrgID)
	r := authorizedRequest(t, featureMux(), "GET", "/api/communication?limit=25&contactId=hub-client", "")
	var out struct {
		Messages []map[string]any
		Total    int
	}
	if r.Code != 200 || json.Unmarshal(r.Body.Bytes(), &out) != nil || out.Total != 28 || len(out.Messages) != 25 {
		t.Fatalf("history: %d %s", r.Code, r.Body.String())
	}
	for _, message := range out.Messages {
		if message["recordRoute"] != "/cases/hub-case" {
			t.Fatalf("wrong link: %+v", message)
		}
	}
	r = authorizedRequest(t, featureMux(), "GET", "/api/communication?contactId=hub-client&offset=25", "")
	json.Unmarshal(r.Body.Bytes(), &out)
	if len(out.Messages) != 3 {
		t.Fatal(r.Body.String())
	}
	r = authorizedRequest(t, featureMux(), "GET", "/api/communication?channel=portal&caseId=hub-case", "")
	json.Unmarshal(r.Body.Bytes(), &out)
	if out.Total != 10 {
		t.Fatal(r.Body.String())
	}
	r = authorizedRequest(t, featureMux(), "GET", "/api/communication?studentId=hub-student&channel=portal", "")
	json.Unmarshal(r.Body.Bytes(), &out)
	if r.Code != 200 || out.Total != 10 {
		t.Fatal(r.Body.String())
	}
	r = authorizedRequest(t, featureMux(), "GET", "/api/communication?studentId=hub-hidden-student", "")
	if r.Code != 404 {
		t.Fatal("exposed inaccessible student communication")
	}
	r = authorizedRequest(t, featureMux(), "GET", "/api/communication?unmatched=true", "")
	json.Unmarshal(r.Body.Bytes(), &out)
	if out.Total != 0 {
		t.Fatal("member saw unmatched messages")
	}
	// Outbox rows must retain invoice access checks and exclude account mail.
	for _, item := range []struct{ kind, invoice string }{{"client", ""}, {"invoice", "hub-hidden-invoice"}, {"password_reset", ""}, {"invite", ""}} {
		if _, err := database.Exec("INSERT INTO mail_outbox(id,org_id,user_id,contact_id,invoice_id,kind,recipient,subject,body,status,created_at) VALUES(?,?,?,?,?,?,?,?,?,'queued',?)", newID("hub-mail"), u.OrgID, u.ID, "hub-client", item.invoice, item.kind, "client@example.com", "Hub mail", "Body", utcNow()); err != nil {
			t.Fatal(err)
		}
	}
	r = authorizedRequest(t, featureMux(), "GET", "/api/communication?contactId=hub-client&channel=email&q=Hub%20mail", "")
	json.Unmarshal(r.Body.Bytes(), &out)
	if out.Total != 1 || out.Messages[0]["source"] != "mail" || out.Messages[0]["recipient"] != "client@example.com" {
		t.Fatal(r.Body.String())
	}
	// A manager can resolve unmatched messages, but never another workspace's.
	u.Role = "manager"
	if _, err := database.Exec("UPDATE users SET role='manager' WHERE id=?", u.ID); err != nil {
		t.Fatal(err)
	}
	r = authorizedRequest(t, featureMux(), "GET", "/api/communication?unmatched=true", "")
	json.Unmarshal(r.Body.Bytes(), &out)
	if r.Code != 200 || out.Total != 1 || out.Messages[0]["id"] != "hub-unmatched" {
		t.Fatal(r.Body.String())
	}
}

func TestUnifiedCalendarParentsTimezoneAndPagination(t *testing.T) {
	u, visible, hidden := consolidatedMember(t)
	if _, err := database.Exec("UPDATE organizations SET timezone='Asia/Manila' WHERE id=?", u.OrgID); err != nil {
		t.Fatal(err)
	}
	insertSearchFixture(t, "contacts", "calendar-client", visible, map[string]any{"name": "Client"})
	insertSearchFixture(t, "contacts", "calendar-hidden", hidden, map[string]any{"name": "Private"})
	insertSearchFixture(t, "cases", "calendar-case", visible, map[string]any{"caseNumber": "Calendar case", "nextDeadline": "2026-11-02", "currentStage": "Assessment"})
	insertSearchFixture(t, "cases", "calendar-hidden-case", hidden, map[string]any{"caseNumber": "Hidden", "nextDeadline": "2026-11-02"})
	insertSearchFixture(t, "documents", "calendar-doc", visible, map[string]any{"docName": "Passport", "caseId": "calendar-case", "dueDate": "2026-11-03", "expiryDate": "2026-11-04"})
	insertSearchFixture(t, "documents", "calendar-hidden-doc", visible, map[string]any{"docName": "Hidden passport", "caseId": "calendar-hidden-case", "dueDate": "2026-11-03"})
	for i := 0; i < 27; i++ {
		insertSearchFixture(t, "tasks", fmt.Sprintf("calendar-task-%02d", i), visible, map[string]any{"title": "Calendar task", "dueDate": "2026-11-05", "status": "todo"})
	}
	insertSearchFixture(t, "tasks", "calendar-done", visible, map[string]any{"title": "Done", "dueDate": "2026-11-05", "status": "done"})
	insertSearchFixture(t, "invoices", "calendar-invoice", visible, map[string]any{"invoiceNumber": "Due invoice", "dueDate": "2026-11-06", "balance": 50})
	insertSearchFixture(t, "invoices", "calendar-paid", visible, map[string]any{"invoiceNumber": "Paid invoice", "dueDate": "2026-11-06", "balance": 0})
	for _, client := range []string{"calendar-client", "calendar-hidden"} {
		data := mustJSON(map[string]any{"contactId": client, "title": "Consultation", "occurredAt": "2026-10-31T20:30:00Z"})
		if _, err := database.Exec("INSERT INTO workspace_entries VALUES(?,?,'calendar_event',?,?,?,?,?)", "meeting-"+client, u.OrgID, u.ID, client, data, utcNow(), utcNow()); err != nil {
			t.Fatal(err)
		}
	}
	var out struct {
		Events   []map[string]any
		Total    int
		Timezone string
	}
	r := authorizedRequest(t, featureMux(), "GET", "/api/workspace-calendar?from=2026-11-01&to=2026-11-30", "")
	if r.Code != 200 || json.Unmarshal(r.Body.Bytes(), &out) != nil || out.Total != 32 || len(out.Events) != 25 || out.Timezone != "Asia/Manila" {
		t.Fatalf("calendar: %d %s", r.Code, r.Body.String())
	}
	if out.Events[0]["kind"] != "consultation" || out.Events[0]["date"] != "2026-11-01" {
		t.Fatal("consultation was not converted to workspace timezone")
	}
	r = authorizedRequest(t, featureMux(), "GET", "/api/workspace-calendar?from=2026-11-01&to=2026-11-30&kind=task&offset=25", "")
	json.Unmarshal(r.Body.Bytes(), &out)
	if out.Total != 27 || len(out.Events) != 2 {
		t.Fatal(r.Body.String())
	}
	r = authorizedRequest(t, featureMux(), "GET", "/api/workspace-calendar?from=2026-10-01&to=2026-10-31&kind=consultation", "")
	json.Unmarshal(r.Body.Bytes(), &out)
	if out.Total != 0 {
		t.Fatal("consultation appeared on UTC date instead of workspace date")
	}
	for _, query := range []string{"from=bad&to=2026-11-30", "from=2026-11-30&to=2026-11-01", "from=2025-01-01&to=2026-11-30"} {
		if r = authorizedRequest(t, featureMux(), "GET", "/api/workspace-calendar?"+query, ""); r.Code != 400 {
			t.Fatal(r.Body.String())
		}
	}
}

func TestCombinedDirectoryPreservesTypesAndAccess(t *testing.T) {
	_, visible, hidden := consolidatedMember(t)
	insertSearchFixture(t, "agents", "directory-agent", visible, map[string]any{"name": "Consolidation agency", "agentCode": "CODE-42", "commissionRatePercent": 12, "contractStatus": "Signed"})
	insertSearchFixture(t, "partners", "directory-partner", visible, map[string]any{"legalCompanyName": "Consolidation partner", "type": "School", "commissionRatePercent": 8, "agreementExpiry": "2027-01-01"})
	insertSearchFixture(t, "partners", "directory-hidden", hidden, map[string]any{"legalCompanyName": "Consolidation hidden"})
	foreign := visible
	foreign.OrgID = "other-workspace"
	insertSearchFixture(t, "agents", "directory-foreign", foreign, map[string]any{"name": "Consolidation foreign"})
	var out struct {
		Items []map[string]any
		Total int
	}
	r := authorizedRequest(t, featureMux(), "GET", "/api/referral-directory?q=Consolidation", "")
	if r.Code != 200 || json.Unmarshal(r.Body.Bytes(), &out) != nil || out.Total != 2 {
		t.Fatal(r.Body.String())
	}
	if out.Items[0]["route"] != "/agents/directory-agent" || out.Items[1]["route"] != "/partners/directory-partner" || out.Items[1]["type"] != "Partner · School" || out.Items[1]["agreementExpiry"] != "2027-01-01" {
		t.Fatal(r.Body.String())
	}
	r = authorizedRequest(t, featureMux(), "GET", "/api/referral-directory?entity=agents&q=CODE-42", "")
	json.Unmarshal(r.Body.Bytes(), &out)
	if out.Total != 1 || out.Items[0]["commissionRate"] != float64(12) {
		t.Fatal(r.Body.String())
	}
	if r = authorizedRequest(t, featureMux(), "GET", "/api/referral-directory?entity=contacts", ""); r.Code != 400 {
		t.Fatal("accepted unsupported directory type")
	}
}
