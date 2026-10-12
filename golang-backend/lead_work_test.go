package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func leadWorkRequest(t *testing.T, h http.Handler, cookies []*http.Cookie, method, path string, body any, revision string) *httptest.ResponseRecorder {
	t.Helper()
	raw := ""
	if body != nil {
		raw = mustJSON(body)
	}
	r := httptest.NewRequest(method, path, strings.NewReader(raw))
	r.Header.Set("Content-Type", "application/json")
	if revision != "" {
		r.Header.Set("If-Match", revision)
	}
	for _, c := range cookies {
		r.AddCookie(c)
	}
	w := httptest.NewRecorder()
	serveFixtureRequest(h, w, r)
	return w
}
func leadWorkRevision(t *testing.T, h http.Handler, cookies []*http.Cookie, entity, id string) string {
	t.Helper()
	w := featureRequest(h, "GET", "/api/records/"+entity+"/"+id, nil, cookies)
	assertStatus(t, w, 200)
	var out struct{ Record map[string]any }
	json.Unmarshal(w.Body.Bytes(), &out)
	return fmtString(out.Record["_revision"])
}
func TestLeadDuplicateReviewMatchesPhonesAndContacts(t *testing.T) {
	setupStore(t)
	h := leadIntakeMux()
	cookies := demoCookies(t, h)
	contact := contacts[0]
	before := len(leads)
	body := map[string]any{"name": "Another inquiry", "email": strings.ToUpper(contact.Email), "source": "Referral", "status": "New", "nextAction": "Call client"}
	assertStatus(t, featureRequest(h, "POST", "/api/leads", body, cookies), 400)
	if len(leads) != before {
		t.Fatal("duplicate inquiry saved without review")
	}
	w := featureRequest(h, "POST", "/api/form-quality", formQualityRequest{Entity: "leads", Email: " " + strings.ToUpper(contact.Email) + " "}, cookies)
	assertStatus(t, w, 200)
	if !strings.Contains(w.Body.String(), "/contacts/"+contact.ID) {
		t.Fatal("matching contact has no review link")
	}
	body["duplicateConfirmed"] = true
	body["phone"] = "+639171111111"
	assertStatus(t, featureRequest(h, "POST", "/api/leads", body, cookies), 201)
	body["email"] = "separate@example.com"
	body["phone"] = "0063 917 111 1111"
	body["duplicateConfirmed"] = false
	assertStatus(t, featureRequest(h, "POST", "/api/leads", body, cookies), 400)
	body["duplicateConfirmed"] = true
	assertStatus(t, featureRequest(h, "POST", "/api/leads", body, cookies), 201)
	if len(leads) != before+2 {
		t.Fatal("reviewed separate inquiries not saved")
	}
	admin, _ := findUserByEmail("demo@northwind.dev")
	member := addPermissionUser(t, "member", admin.OrgID, "duplicate-reader")
	mu.Lock()
	for i := range contacts {
		if contacts[i].ID == contact.ID {
			contacts[i].Visibility = "private"
		}
	}
	for i := range leads {
		if leads[i].Email == contact.Email || strings.EqualFold(leads[i].Email, contact.Email) {
			leads[i].Visibility = "private"
		}
	}
	mu.Unlock()
	w = dailyUserRequest(t, h, member, "POST", "/api/form-quality", formQualityRequest{Entity: "leads", Email: contact.Email})
	assertStatus(t, w, 200)
	if strings.Contains(w.Body.String(), contact.Name) || strings.Contains(w.Body.String(), contact.ID) {
		t.Fatal("duplicate hints exposed a private contact")
	}
}
func TestLeadPlanTimelineDailyWorkAndFirstFollowUp(t *testing.T) {
	setupStore(t)
	h := leadIntakeMux()
	cookies := demoCookies(t, h)
	lead := intakeLead(t, h, cookies)
	if lead.NextAction == "" || lead.CreatedAtTime == "" {
		t.Fatal("new inquiry lacks a next action or creation timestamp")
	}
	body := leadMap(lead)
	body["nextAction"] = "Call about missing transcript"
	body["followUpDate"] = "2000-01-01"
	assertStatus(t, leadWorkRequest(t, h, cookies, "PUT", "/api/leads/"+lead.ID, body, leadWorkRevision(t, h, cookies, "leads", lead.ID)), 200)
	w := featureRequest(h, "GET", "/api/my-work?scope=all", nil, cookies)
	assertStatus(t, w, 200)
	var work struct {
		Items  []productivityItem
		Counts map[string]int
	}
	json.Unmarshal(w.Body.Bytes(), &work)
	found := false
	for _, item := range work.Items {
		if item.ID == lead.ID && item.Action == "Call about missing transcript" && item.Priority == "urgent" {
			found = true
		}
	}
	if !found || work.Counts["overdueLeads"] < 1 {
		t.Fatal("lead action missing from overdue daily work: " + w.Body.String())
	}
	revision := leadWorkRevision(t, h, cookies, "leads", lead.ID)
	assertStatus(t, leadWorkRequest(t, h, cookies, "POST", "/api/leads/"+lead.ID+"/timeline", map[string]any{"kind": "note", "body": "Asked Flor to review CV", "followUp": true}, revision), 400)
	assertStatus(t, leadWorkRequest(t, h, cookies, "POST", "/api/leads/"+lead.ID+"/timeline", map[string]any{"kind": "note", "body": "Asked Flor to review CV"}, revision), 201)
	current, _ := getLeadByID(lead.ID)
	if current.FirstFollowUpAt != "" {
		t.Fatal("a note falsely counted as client follow-up")
	}
	assertStatus(t, leadWorkRequest(t, h, cookies, "POST", "/api/leads/"+lead.ID+"/timeline", map[string]any{"kind": "call", "body": "Discussed study plans", "followUp": true}, revision), 201)
	current, _ = getLeadByID(lead.ID)
	first := current.FirstFollowUpAt
	if first == "" {
		t.Fatal("first client follow-up was not recorded")
	}
	assertStatus(t, leadWorkRequest(t, h, cookies, "POST", "/api/leads/"+lead.ID+"/timeline", map[string]string{"kind": "message", "body": "Stale log"}, revision), 409)
	assertStatus(t, leadWorkRequest(t, h, cookies, "POST", "/api/leads/"+lead.ID+"/timeline", map[string]any{"kind": "message", "body": "Sent information separately", "followUp": true}, leadWorkRevision(t, h, cookies, "leads", lead.ID)), 201)
	current, _ = getLeadByID(lead.ID)
	if current.FirstFollowUpAt != first {
		t.Fatal("later activity changed first-follow-up time")
	}
	w = multipartRequest(h, "/api/leads/"+lead.ID+"/cv", "file", "cv.txt", []byte("Timeline CV"), cookies, nil)
	assertStatus(t, w, 201)
	w = featureRequest(h, "GET", "/api/leads/"+lead.ID+"/timeline?limit=2", nil, cookies)
	assertStatus(t, w, 200)
	var page struct {
		Events []LeadTimelineEvent
		Total  int
	}
	json.Unmarshal(w.Body.Bytes(), &page)
	if len(page.Events) != 2 || page.Total < 5 {
		t.Fatal("timeline did not page all record activity")
	}
	w = featureRequest(h, "GET", "/api/leads/"+lead.ID+"/timeline", nil, cookies)
	for _, text := range []string{"Discussed study plans", "Sent information separately", "CV / document uploaded", "Call about missing transcript"} {
		if !strings.Contains(w.Body.String(), text) {
			t.Fatal("missing timeline event: " + text)
		}
	}
	other := featureRequest(h, "POST", "/api/register", map[string]string{"name": "Other", "email": "other-work@example.com", "password": "LongEnoughPassword!"}, nil).Result().Cookies()
	assertStatus(t, featureRequest(h, "GET", "/api/leads/"+lead.ID+"/timeline", nil, other), 404)
}
func TestLeadPipelineReportsOnlyMeasuredFollowUpTimes(t *testing.T) {
	leads := []Lead{{Source: "Referral", Referrer: "Flor", CreatedAt: "2026-10-01", CreatedAtTime: "2026-10-01T08:00:00Z", FirstFollowUpAt: "2026-10-01T10:00:00Z", Status: "Converted"}, {Source: "Referral", Referrer: "Flor", CreatedAt: "2026-10-02", Status: "New", FollowUpDate: "2026-10-03"}, {Source: "Referral", Referrer: "Flor", CreatedAt: "2026-10-02", FirstFollowUpAt: "2026-10-02T10:00:00Z", Status: "Closed/Lost", FollowUpDate: "2026-10-03"}, {Source: "Website", CreatedAt: "2025-10-01", Status: "New"}}
	rows := leadPipelineMetrics(leads, "2026-10-01", "2026-10-10", "2026-10-10")
	if len(rows) != 1 || rows[0].Leads != 3 || rows[0].Converted != 1 || rows[0].Rate != 33.33 || rows[0].Overdue != 1 || rows[0].WithoutLoggedFollowUp != 1 || rows[0].TimedFollowUps != 1 || rows[0].AverageFirstFollowUpHours == nil || *rows[0].AverageFirstFollowUpHours != 2 {
		t.Fatalf("incorrect pipeline metrics: %+v", rows)
	}
	rows = leadPipelineMetrics(leads[1:2], "", "", "2026-10-10")
	if rows[0].AverageFirstFollowUpHours != nil {
		t.Fatal("missing timestamps were reported as zero hours")
	}
}
func TestLeadRemindersDeduplicateAndStopForClosedLeads(t *testing.T) {
	setupStore(t)
	h := leadIntakeMux()
	cookies := demoCookies(t, h)
	lead := intakeLead(t, h, cookies)
	u, _ := findUserByEmail("demo@northwind.dev")
	mu.Lock()
	for i := range leads {
		if leads[i].ID == lead.ID {
			leads[i].AssignedTo = u.Name
			leads[i].FollowUpDate = "2000-01-01"
		}
	}
	mu.Unlock()
	commitFixtureEdits()
	generateReminders()
	generateReminders()
	var count int
	database.QueryRow("SELECT count(*) FROM notifications WHERE kind='lead-followup' AND dedupe_key LIKE ?", "lead-followup:"+lead.ID+":%").Scan(&count)
	if count != 1 {
		t.Fatalf("duplicate reminders: %d", count)
	}
	current, _ := getLeadByID(lead.ID)
	body := leadMap(current)
	body["status"] = "Closed/Lost"
	assertStatus(t, leadWorkRequest(t, h, cookies, "PUT", "/api/leads/"+lead.ID, body, leadWorkRevision(t, h, cookies, "leads", lead.ID)), 200)
	var unread int
	database.QueryRow("SELECT count(*) FROM notifications WHERE kind='lead-followup' AND path=? AND read_at IS NULL", "/leads/"+lead.ID).Scan(&unread)
	if unread != 0 {
		t.Fatal("resolved lead kept an unread follow-up reminder")
	}
	s := cloneStore()
	for i := range s.Leads {
		if s.Leads[i].ID == current.ID {
			s.Leads[i].Status = "Closed/Lost"
		}
	}
	if err := remindLeadFollowUps(database, s, func(string, int) string { return time.Now().Add(24 * time.Hour).Format("2006-01-02") }); err != nil {
		t.Fatal(err)
	}
	database.QueryRow("SELECT count(*) FROM notifications WHERE kind='lead-followup' AND dedupe_key LIKE ?", "lead-followup:"+lead.ID+":%").Scan(&count)
	if count != 1 {
		t.Fatal("closed lead received reminders")
	}
}

func TestRequirementChecklistPlansKeepAccessAndRevisionChecks(t *testing.T) {
	setupStore(t)
	h := leadIntakeMux()
	cookies := demoCookies(t, h)
	admin, _ := findUserByEmail("demo@northwind.dev")
	member := addPermissionUser(t, "member", admin.OrgID, "requirement-adviser")
	viewer := addPermissionUser(t, "viewer", admin.OrgID, "requirement-viewer")
	foreign := addPermissionUser(t, "member", "other-org", "requirement-foreign")
	c := cases[0]
	mu.Lock()
	cases[0].NextAction = "Review remaining documents"
	cases[0].NextDeadline = "2099-01-01"
	cases[0].AssignedOfficer = admin.Name
	documents = []Document{
		{RecordScope: c.RecordScope, ID: "requirement-missing", CaseID: c.ID, DocName: "Transcript", Required: true, Status: "Requested"},
		{RecordScope: c.RecordScope, ID: "requirement-complete", CaseID: c.ID, DocName: "Passport", Required: true, Status: "Verified", FilePath: "/api/documents/passport/files/1", ExpiryDate: "2099-01-01"},
	}
	mu.Unlock()
	w := featureRequest(h, "GET", "/api/cases/"+c.ID+"/blockers", nil, cookies)
	assertStatus(t, w, 200)
	var out struct {
		Checklist      []RequirementChecklist
		RequiredCount  int
		CompletedCount int
	}
	json.Unmarshal(w.Body.Bytes(), &out)
	if out.RequiredCount != 2 || out.CompletedCount != 1 || out.Checklist[0].Title != "Transcript" || out.Checklist[0].DueDate != "2099-01-01" {
		t.Fatal("checklist progress or application deadline missing")
	}
	revision := out.Checklist[0].Revision
	body := map[string]string{"responsibleId": member.ID, "dueDate": "2000-01-01"}
	assertStatus(t, leadWorkRequest(t, h, cookies, "PATCH", "/api/documents/requirement-missing/requirement-plan", body, revision), 200)
	assertStatus(t, leadWorkRequest(t, h, cookies, "PATCH", "/api/documents/requirement-missing/requirement-plan", body, revision), 409)
	w = featureRequest(h, "GET", "/api/cases/"+c.ID+"/blockers", nil, cookies)
	json.Unmarshal(w.Body.Bytes(), &out)
	if out.Checklist[0].Responsible != member.Name || !out.Checklist[0].Overdue || out.Checklist[0].DueDate != "2000-01-01" {
		t.Fatal("assigned staff and overdue deadline missing")
	}
	beforeOwner := documents[0].OwnerID
	for _, id := range []string{viewer.ID, foreign.ID} {
		body["responsibleId"] = id
		assertStatus(t, leadWorkRequest(t, h, cookies, "PATCH", "/api/documents/requirement-missing/requirement-plan", body, out.Checklist[0].Revision), 400)
	}
	if documents[0].OwnerID != beforeOwner {
		t.Fatal("requirement responsibility changed record permissions")
	}
	mu.Lock()
	for i := range documents {
		if documents[i].ID == "requirement-missing" {
			documents[i].Visibility = "private"
			documents[i].OwnerID = admin.ID
		}
	}
	mu.Unlock()
	w = dailyUserRequest(t, h, member, "GET", "/api/cases/"+c.ID+"/blockers", nil)
	assertStatus(t, w, 200)
	if strings.Contains(w.Body.String(), "Transcript") || strings.Contains(w.Body.String(), "requirement-missing") {
		t.Fatal("private checklist requirement leaked")
	}
}
