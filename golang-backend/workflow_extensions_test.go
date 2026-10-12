package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"
)

func workflowMux() http.Handler {
	m := http.NewServeMux()
	registerWorkspaceRoutes(m)
	m.HandleFunc("GET /api/deals", authMiddleware(listDeals))
	m.HandleFunc("PUT /api/cases/{id}/stage", authMiddleware(updateCaseStageHandler))
	return persistMutations(m)
}
func gatePreview(t *testing.T, h http.Handler, id, stage string) (string, string, []string) {
	t.Helper()
	w := expansionRequest(t, h, "GET", "/api/cases/"+id+"/gates", nil, "")
	assertStatus(t, w, 200)
	var out struct {
		Revision string
		Stages   []struct {
			Rule        StageGateRule
			Fingerprint string
			Issues      []string
		}
	}
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	for _, p := range out.Stages {
		if p.Rule.Stage == stage {
			return out.Revision, p.Fingerprint, p.Issues
		}
	}
	t.Fatal("missing stage preview")
	return "", "", nil
}
func configureTestGate(t *testing.T, h http.Handler, rule StageGateRule) {
	t.Helper()
	w := expansionRequest(t, h, "GET", "/api/application-gates", nil, "")
	assertStatus(t, w, 200)
	var out struct{ Rules []StageGateRule }
	json.Unmarshal(w.Body.Bytes(), &out)
	for _, r := range out.Rules {
		if r.Stage == rule.Stage {
			rule.Revision = r.Revision
		}
	}
	assertStatus(t, expansionRequest(t, h, "PUT", "/api/application-gates", rule, ""), 200)
}
func TestStageGateEnforcementApprovalAndAuditedOverride(t *testing.T) {
	setupStore(t)
	h := workflowMux()
	c := cases[0]
	c.AutoAdvance = false
	c.CurrentStage = "Assessment"
	cases[0] = c
	stage := "Submitted"
	configureTestGate(t, h, StageGateRule{Stage: stage, Enabled: true, DocumentNames: []string{"Passport"}, Uploaded: true, Reviewed: true, Verified: true, ManagerApproval: true})
	rev, fp, issues := gatePreview(t, h, c.ID, stage)
	if len(issues) < 2 {
		t.Fatal("missing document and approval blockers")
	}
	// Existing forms must not bypass configured gates.
	assertStatus(t, expansionRequest(t, h, "PUT", "/api/cases/"+c.ID+"/stage", map[string]any{"currentStage": stage}, rev), 409)
	if cases[0].CurrentStage != c.CurrentStage {
		t.Fatal("blocked transition persisted")
	}
	assertStatus(t, expansionRequest(t, h, "POST", "/api/cases/"+c.ID+"/transition", map[string]any{"stage": stage}, rev), 409)
	assertStatus(t, expansionRequest(t, h, "POST", "/api/cases/"+c.ID+"/gate-approval", map[string]any{"stage": stage, "fingerprint": fp, "reason": "Reviewed application"}, rev), 200)
	_, _, issues = gatePreview(t, h, c.ID, stage)
	if strings.Contains(strings.Join(issues, ";"), "manager must approve") {
		t.Fatal("approval not recognized")
	}
	// Changing policy invalidates approval tied to the reviewed versions.
	configureTestGate(t, h, StageGateRule{Stage: stage, Enabled: true, ManagerApproval: true})
	rev, fp, issues = gatePreview(t, h, c.ID, stage)
	if len(issues) != 1 {
		t.Fatalf("policy change did not invalidate approval: %v", issues)
	}
	assertStatus(t, expansionRequest(t, h, "POST", "/api/cases/"+c.ID+"/gate-approval", map[string]any{"stage": stage, "fingerprint": fp, "reason": "Reviewed revised policy"}, rev), 200)
	assertStatus(t, expansionRequest(t, h, "POST", "/api/cases/"+c.ID+"/transition", map[string]any{"stage": stage}, rev), 200)
	// Another blocked stage permits only an explained manager override.
	configureTestGate(t, h, StageGateRule{Stage: "Approved", Enabled: true, ManagerApproval: true})
	rev, _, _ = gatePreview(t, h, c.ID, "Approved")
	assertStatus(t, expansionRequest(t, h, "POST", "/api/cases/"+c.ID+"/transition", map[string]any{"stage": "Approved", "override": true}, rev), 400)
	assertStatus(t, expansionRequest(t, h, "POST", "/api/cases/"+c.ID+"/transition", map[string]any{"stage": "Approved", "override": true, "reason": "Exceptional approval documented"}, rev), 200)
	var count int
	if err := database.QueryRow("SELECT count(*) FROM workspace_entries WHERE category='application_transition' AND json_extract(data,'$.override')=1 AND json_extract(data,'$.reason')='Exceptional approval documented'").Scan(&count); err != nil || count != 1 {
		t.Fatalf("override audit missing: %d %v", count, err)
	}
	u, _ := findUserByEmail("demo@northwind.dev")
	u.Role = "member"
	users[u.Email] = u
	assertStatus(t, expansionRequest(t, h, "GET", "/api/application-gates", nil, ""), 403)
	rev, _, _ = gatePreview(t, h, c.ID, stage)
	assertStatus(t, expansionRequest(t, h, "POST", "/api/cases/"+c.ID+"/transition", map[string]any{"stage": stage, "override": true, "reason": "Bypass"}, rev), 403)
}
func TestStageGateDocumentVersionsAndAutomaticAdvance(t *testing.T) {
	setupStore(t)
	h := workflowMux()
	c := cases[0]
	c.AutoAdvance = false
	c.CurrentStage = "Assessment"
	cases[0] = c
	d := documents[0]
	d.ID = "gate_passport"
	d.CaseID = c.ID
	d.OrgID = c.OrgID
	d.DocName = "Passport"
	d.ExpiryDate = ""
	d.Required = true
	d.Status = "Verified"
	d.LatestVersion = 1
	d.FilePath = "fixture"
	d.ReviewedVersion = 1
	d.ReviewedBy = "Reviewer"
	d.ReviewedAt = utcNow()
	documents = []Document{d}
	if _, err := database.Exec("INSERT INTO file_versions VALUES(?,?,?,?,?,?,?,?,?,?,?)", "gate_file", c.OrgID, d.ID, 1, "passport.pdf", "fixture", "application/pdf", 1, "hash", "reviewer", utcNow()); err != nil {
		t.Fatal(err)
	}
	configureTestGate(t, h, StageGateRule{Stage: "Submitted", Enabled: true, RequiredDocuments: true, Uploaded: true, Reviewed: true, Verified: true})
	_, _, issues := gatePreview(t, h, c.ID, "Submitted")
	if len(issues) != 0 {
		t.Fatalf("valid document blocked: %v", issues)
	}
	documents[0].LatestVersion = 2
	_, _, issues = gatePreview(t, h, c.ID, "Submitted")
	if len(issues) == 0 {
		t.Fatal("untracked current upload passed gate")
	}

	// Approvals expire when an otherwise valid document is replaced and reviewed again.
	documents[0].LatestVersion = 1
	configureTestGate(t, h, StageGateRule{Stage: "Submitted", Enabled: true, RequiredDocuments: true, Uploaded: true, Reviewed: true, Verified: true, ManagerApproval: true})
	rev, fp, _ := gatePreview(t, h, c.ID, "Submitted")
	assertStatus(t, expansionRequest(t, h, "POST", "/api/cases/"+c.ID+"/gate-approval", map[string]any{"stage": "Submitted", "fingerprint": fp, "reason": "Reviewed passport"}, rev), 200)
	documents[0].LatestVersion = 2
	documents[0].ReviewedVersion = 2
	if _, err := database.Exec("INSERT INTO file_versions VALUES(?,?,?,?,?,?,?,?,?,?,?)", "gate_file_new", c.OrgID, d.ID, 2, "passport-new.pdf", "fixture", "application/pdf", 1, "new_hash", "reviewer", utcNow()); err != nil {
		t.Fatal(err)
	}
	_, _, issues = gatePreview(t, h, c.ID, "Submitted")
	if len(issues) != 1 || !strings.Contains(issues[0], "manager must approve") {
		t.Fatalf("replacement did not invalidate approval: %v", issues)
	}
	// Worker or document updates retain their changes while holding automatic stage advancement.
	before := cloneStore()
	after := cloneStore()
	after.Cases[0].AutoAdvance = true
	after.Cases[0].CurrentStage = "Submitted"
	u, _ := findUserByEmail("demo@northwind.dev")
	r := aiUserRequest(context.Background(), u)
	r.URL = &url.URL{Path: "/api/documents/" + d.ID}
	if err := enforceApplicationGates(r, before, &after); err != nil {
		t.Fatal(err)
	}
	if after.Cases[0].CurrentStage != before.Cases[0].CurrentStage || after.Documents[0].LatestVersion != 2 {
		t.Fatal("automatic transition was not held independently")
	}
}
func TestProgressiveDealBoardTotalsFiltersAndSnapshot(t *testing.T) {
	setupStore(t)
	h := workflowMux()
	base := deals[0]
	deals = nil
	for i := 0; i < 125; i++ {
		d := base
		d.ID = fmt.Sprintf("progress_%03d", i)
		d.Title = d.ID
		d.Owner = "Board owner"
		d.Value = 10
		d.Currency = "USD"
		d.Stage = "Lead"
		d.CloseDate = "2026-10-15"
		d.Probability = nil
		deals = append(deals, d)
	}
	hidden := base
	hidden.ID = "hidden_board"
	hidden.OrgID = "another_workspace"
	hidden.Value = 99999
	deals = append(deals, hidden)
	read := func(path string) struct {
		Deals    []Deal
		Total    int
		Summary  BoardSummary
		Snapshot string
		Owners   []string
	} {
		w := expansionRequest(t, h, "GET", path, nil, "")
		assertStatus(t, w, 200)
		var out struct {
			Deals    []Deal
			Total    int
			Summary  BoardSummary
			Snapshot string
			Owners   []string
		}
		if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
			t.Fatal(err)
		}
		return out
	}
	out := read("/api/deals?board=page&limit=50&owner=Board%20owner&from=2026-10-01&to=2026-10-31")
	if len(out.Deals) != 50 || out.Total != 125 || out.Summary.Count != 125 || out.Summary.Currencies["USD"].Open != 1250 || out.Summary.Currencies["USD"].Weighted != 125 {
		t.Fatalf("incorrect workspace totals: %+v", out.Summary)
	}
	path := "/api/deals?board=page&limit=50&offset=50&owner=Board%20owner&from=2026-10-01&to=2026-10-31&snapshot=" + out.Snapshot
	next := read(path)
	if len(next.Deals) != 50 || next.Deals[0].ID == out.Deals[0].ID {
		t.Fatal("pagination repeated page")
	}
	if read("/api/deals?board=page&from=2026-11-01").Total != 0 || read("/api/deals?board=page&q=progress_124").Total != 1 {
		t.Fatal("server filters failed")
	}
	for i := range deals {
		if deals[i].ID == "progress_000" {
			deals[i].Value = 20
		}
	}
	assertStatus(t, expansionRequest(t, h, "GET", path, nil, ""), 409)
	assertStatus(t, expansionRequest(t, h, "GET", "/api/deals?board=page&from=bad", nil, ""), 400)
	assertStatus(t, expansionRequest(t, h, "GET", "/api/deals?board=page&from=2026-11-01&to=2026-10-01", nil, ""), 400)
}
func TestManagerEscalationsDeduplicateResolveAndRespectWorkspace(t *testing.T) {
	setupStore(t)
	u, _ := findUserByEmail("demo@northwind.dev")
	c := cases[0]
	c.CurrentStage = "Assessment"
	c.NextDeadline = "2026-10-01"
	cases = []Case{c}
	commitFixtureEdits()
	rule := EscalationRule{ID: "rule_one", Name: "Overdue applications", Kind: "application", AfterDays: 3, ManagerID: u.ID, Enabled: true}
	if _, err := database.Exec("INSERT INTO escalation_rules VALUES(?,?,?)", rule.ID, u.OrgID, mustJSON(rule)); err != nil {
		t.Fatal(err)
	}
	day := func(_ string, offset int) string {
		return time.Date(2026, 10, 11, 0, 0, 0, 0, time.UTC).AddDate(0, 0, offset).Format("2006-01-02")
	}
	for i := 0; i < 2; i++ {
		if err := runEscalations(database, cloneStore(), day); err != nil {
			t.Fatal(err)
		}
	}
	var count int
	database.QueryRow("SELECT count(*) FROM notifications WHERE kind='escalation' AND user_id=?", u.ID).Scan(&count)
	if count != 1 {
		t.Fatalf("duplicate or missing escalation: %d", count)
	}
	cases[0].CurrentStage = "Closed"
	if err := runEscalations(database, cloneStore(), day); err != nil {
		t.Fatal(err)
	}
	database.QueryRow("SELECT count(*) FROM notifications WHERE kind='escalation' AND read_at IS NULL").Scan(&count)
	if count != 0 {
		t.Fatal("resolved work remained escalated")
	}
	rule.ID = "other_workspace_rule"
	database.Exec("INSERT INTO escalation_rules VALUES(?,?,?)", rule.ID, "different_org", mustJSON(rule))
	cases[0].CurrentStage = "Assessment"
	if err := runEscalations(database, cloneStore(), day); err != nil {
		t.Fatal(err)
	}
	database.QueryRow("SELECT count(*) FROM escalation_deliveries WHERE rule_id=?", rule.ID).Scan(&count)
	if count != 0 {
		t.Fatal("escalation crossed workspace")
	}
}
func TestUnifiedClientTimelineLinkedRootsEventsAndPermissions(t *testing.T) {
	setupStore(t)
	h := workflowMux()
	c := cases[0]
	c.ClientID = contacts[0].ID
	c.StudentID = students[0].ID
	cases[0] = c
	for _, category := range []string{"application_handoff", "application_gate_approval", "application_transition"} {
		if _, err := database.Exec("INSERT INTO workspace_entries VALUES(?,?,?,?,?,?,?,?)", category, c.OrgID, category, "manager", c.ID, mustJSON(map[string]any{"stage": "Submitted", "reason": "Timeline review marker", "status": "pending"}), utcNow(), utcNow()); err != nil {
			t.Fatal(err)
		}
	}
	for _, root := range []string{"contacts/" + c.ClientID, "cases/" + c.ID, "students/" + c.StudentID} {
		route := "/api/client-history/" + root + "?kind=approval&q=Timeline%20review&from=2000-01-01&to=2099-01-01"
		w := expansionRequest(t, h, "GET", route, nil, "")
		assertStatus(t, w, 200)
		var out struct {
			Items     []ClientHistoryItem
			Total     int
			ContactID string
		}
		json.Unmarshal(w.Body.Bytes(), &out)
		if out.Total != 1 || len(out.Items) != 1 || out.Items[0].Kind != "approval" || out.ContactID != c.ClientID {
			t.Fatalf("linked timeline incorrect: %s", w.Body.String())
		}
	}
	assertStatus(t, expansionRequest(t, h, "GET", "/api/client-history/cases/"+c.ID+"?contactId=unlinked", nil, ""), 404)
	invoices[0].ClientID = c.ClientID
	invoices[0].Currency = "PHP"
	payments = append(payments, Payment{RecordScope: invoices[0].RecordScope, ID: "timeline_refund", InvoiceID: invoices[0].ID, Reference: "Refund currency fixture", Amount: -10, Status: "refund", PaidOn: "2026-10-11"})
	finance := expansionRequest(t, h, "GET", "/api/contacts/"+c.ClientID+"/history?kind=payment&q=Refund%20currency%20fixture", nil, "")
	assertStatus(t, finance, 200)
	var financeOut struct{ Items []ClientHistoryItem }
	json.Unmarshal(finance.Body.Bytes(), &financeOut)
	if len(financeOut.Items) != 1 || !strings.HasPrefix(financeOut.Items[0].Detail, "PHP ") {
		t.Fatalf("refund did not use invoice currency: %s", finance.Body.String())
	}

	// A member cannot obtain events for an application owned privately by another person.
	u, _ := findUserByEmail("demo@northwind.dev")
	u.Role = "member"
	users[u.Email] = u
	cases[0].Visibility = "private"
	cases[0].OwnerID = "someone_else"
	w := expansionRequest(t, h, "GET", "/api/contacts/"+c.ClientID+"/history?kind=approval", nil, "")
	assertStatus(t, w, 200)
	var out struct{ Total int }
	json.Unmarshal(w.Body.Bytes(), &out)
	if out.Total != 0 {
		t.Fatal("timeline leaked inaccessible application events")
	}
	assertStatus(t, expansionRequest(t, h, "GET", "/api/client-history/cases/"+c.ID, nil, ""), 404)
}

func TestManagerHandoffEscalationDelayAndResolution(t *testing.T) {
	setupStore(t)
	h := workflowMux()
	u, _ := findUserByEmail("demo@northwind.dev")
	c := cases[0]
	assertStatus(t, expansionRequest(t, h, "POST", "/api/escalation-rules", EscalationRule{Name: "Unanswered handoff", Kind: "handoff", AfterDays: 3, ManagerID: u.ID, Enabled: true}, ""), 200)
	handoff := ApplicationHandoff{ID: "waiting_handoff", CaseID: c.ID, Status: "pending", CreatedAt: time.Now().Add(-2 * 24 * time.Hour).UTC().Format(time.RFC3339)}
	if _, err := database.Exec("INSERT INTO workspace_entries VALUES(?,?,?,?,?,?,?,?)", handoff.ID, c.OrgID, "application_handoff", u.ID, c.ID, mustJSON(handoff), handoff.CreatedAt, handoff.CreatedAt); err != nil {
		t.Fatal(err)
	}
	day := func(org string, offset int) string { return time.Now().AddDate(0, 0, offset).Format("2006-01-02") }
	if err := runEscalations(database, cloneStore(), day); err != nil {
		t.Fatal(err)
	}
	var count int
	database.QueryRow("SELECT count(*) FROM notifications WHERE kind='escalation'").Scan(&count)
	if count != 0 {
		t.Fatal("handoff escalated before configured delay")
	}
	handoff.CreatedAt = time.Now().Add(-4 * 24 * time.Hour).UTC().Format(time.RFC3339)
	database.Exec("UPDATE workspace_entries SET data=? WHERE id=?", mustJSON(handoff), handoff.ID)
	// Exercise the real worker transaction and its deduplication.
	generateReminders()
	generateReminders()
	database.QueryRow("SELECT count(*) FROM notifications WHERE kind='escalation'").Scan(&count)
	if count != 1 {
		t.Fatalf("worker did not deliver exactly once: %d", count)
	}
	handoff.Status = "accepted"
	database.Exec("UPDATE workspace_entries SET data=? WHERE id=?", mustJSON(handoff), handoff.ID)
	generateReminders()
	database.QueryRow("SELECT count(*) FROM notifications WHERE kind='escalation' AND read_at IS NULL").Scan(&count)
	if count != 0 {
		t.Fatal("accepted handoff remained escalated")
	}
}
