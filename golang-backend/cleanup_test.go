package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHistoricalAssignmentMappingRequiresReviewAndPreservesOwnership(t *testing.T) {
	setupStore(t)
	h := leadIntakeMux()
	cookies := demoCookies(t, h)
	u, _ := findUserByEmail("demo@northwind.dev")
	scope := RecordScope{OrgID: u.OrgID, OwnerID: u.ID, Visibility: "organization"}
	mu.Lock()
	leads = []Lead{{RecordScope: scope, ID: "historical_lead", Name: "Legacy inquiry", Source: "Website", Status: "New", AssignedTo: " demo user "}}
	mu.Unlock()
	if err := saveStore(); err != nil {
		t.Fatal(err)
	}
	w := featureRequest(h, "GET", "/api/assignments/review", nil, cookies)
	assertStatus(t, w, 200)
	var out struct{ Items []AssignmentReview }
	json.Unmarshal(w.Body.Bytes(), &out)
	var item AssignmentReview
	for _, row := range out.Items {
		if row.ID == "historical_lead" {
			item = row
		}
	}
	if item.SuggestedID != u.ID || item.Revision == "" || !item.Editable {
		t.Fatal(w.Body.String())
	}
	path := "/api/assignments/leads/historical_lead"
	assertStatus(t, featureRequest(h, "PATCH", path, map[string]any{"staffId": u.ID, "confirmed": true}, cookies), 428)
	assertStatus(t, leadWorkRequest(t, h, cookies, "PATCH", path, map[string]any{"staffId": u.ID}, item.Revision), 400)
	assertStatus(t, leadWorkRequest(t, h, cookies, "PATCH", path, map[string]any{"staffId": "other-workspace-user", "confirmed": true}, item.Revision), 400)
	assertStatus(t, leadWorkRequest(t, h, cookies, "PATCH", path, map[string]any{"staffId": u.ID, "confirmed": true}, item.Revision), 200)
	w = featureRequest(h, "GET", "/api/leads/historical_lead", nil, cookies)
	assertStatus(t, w, 200)
	var current struct{ Lead Lead }
	json.Unmarshal(w.Body.Bytes(), &current)
	if current.Lead.AssignedTo != u.ID || current.Lead.OwnerID != scope.OwnerID || current.Lead.Visibility != scope.Visibility {
		t.Fatal(w.Body.String())
	}
	assertStatus(t, leadWorkRequest(t, h, cookies, "PATCH", path, map[string]any{"staffId": u.ID, "confirmed": true}, item.Revision), 409)
	w = featureRequest(h, "GET", "/api/assignments/review", nil, cookies)
	assertStatus(t, w, 200)
	if strings.Contains(w.Body.String(), "historical_lead") {
		t.Fatal("mapped inquiry still needs review")
	}
}

func TestRecordAuditFindsOrphansDuplicatesAndMissingActionsWithoutPrivateMatches(t *testing.T) {
	setupStore(t)
	u, _ := findUserByEmail("demo@northwind.dev")
	u.Role = "member"
	scope := RecordScope{OrgID: u.OrgID, OwnerID: u.ID, Visibility: "organization"}
	hidden := scope
	hidden.OwnerID = "other-staff"
	hidden.Visibility = "private"
	s := diskStore{Leads: []Lead{{RecordScope: scope, ID: "orphan", Name: "Orphan", Status: "Converted"}, {RecordScope: scope, ID: "next", Name: "Next", Status: "New", Email: "same@example.com"}, {RecordScope: scope, ID: "private-match", Name: "Private match", Status: "New", Email: "hidden@example.com"}}, Contacts: []Contact{{RecordScope: scope, ID: "c1", Name: "One", Email: "same@example.com", Phone: "+639171234567"}, {RecordScope: scope, ID: "c2", Name: "Two", Phone: "00639171234567"}, {RecordScope: hidden, ID: "secret", Name: "Secret", Email: "hidden@example.com"}}, Cases: []Case{{RecordScope: scope, ID: "broken", CaseNumber: "BROKEN", ClientID: "gone", StudentID: "gone"}}}
	r := httptest.NewRequest("GET", "/api/data-quality", nil)
	r = r.WithContext(context.WithValue(r.Context(), requestUserKey, u))
	found := map[string]bool{}
	extendedRecordAudit(r, s, func(entity, id, severity, title, detail string, scope RecordScope) {
		found[id+":"+title] = true
		if strings.Contains(detail, "Secret") {
			t.Fatal("private identity leaked")
		}
	})
	for _, key := range []string{"orphan:Converted lead has no linked records", "next:No next action", "next:Possible duplicate inquiry", "c1:Possible duplicate client", "c2:Possible duplicate client", "broken:Application client link unavailable", "broken:Application student link unavailable"} {
		if !found[key] {
			t.Fatalf("missing %s: %+v", key, found)
		}
	}
	if found["private-match:Possible duplicate inquiry"] {
		t.Fatal("hidden identity influenced duplicate findings")
	}
}

func TestAssignmentReviewDoesNotGuessAmbiguousNamesOrExposePrivateRecords(t *testing.T) {
	setupStore(t)
	h := leadIntakeMux()
	cookies := demoCookies(t, h)
	u, _ := findUserByEmail("demo@northwind.dev")
	u.Role = "member"
	other := u
	other.ID = "u_duplicate"
	other.Email = "duplicate@staff.test"
	scope := RecordScope{OrgID: u.OrgID, OwnerID: u.ID, Visibility: "organization"}
	private := scope
	private.OwnerID = other.ID
	private.Visibility = "private"
	mu.Lock()
	users[u.Email] = u
	users[other.Email] = other
	leads = []Lead{{RecordScope: scope, ID: "ambiguous", Name: "Ambiguous inquiry", AssignedTo: u.Name}, {RecordScope: private, ID: "private-inquiry", Name: "Secret inquiry", AssignedTo: u.Name}}
	mu.Unlock()
	if err := saveStore(); err != nil {
		t.Fatal(err)
	}
	w := featureRequest(h, "GET", "/api/assignments/review", nil, cookies)
	assertStatus(t, w, 200)
	var out struct{ Items []AssignmentReview }
	json.Unmarshal(w.Body.Bytes(), &out)
	found := false
	for _, item := range out.Items {
		if item.ID == "ambiguous" {
			found = true
			if item.SuggestedID != "" {
				t.Fatal("guessed one of two identical staff names")
			}
		}
	}
	if !found || strings.Contains(w.Body.String(), "Secret inquiry") {
		t.Fatal(w.Body.String())
	}
}

func TestEstablishedCaseResponsibilityUsesHandoffInsteadOfDirectReassignment(t *testing.T) {
	setupStore(t)
	m := http.NewServeMux()
	registerWorkspaceRoutes(m)
	m.HandleFunc("POST /api/login", handleSecureLogin)
	m.HandleFunc("PUT /api/cases/{id}", authMiddleware(updateCaseHandler))
	h := persistMutations(m)
	cookies := demoCookies(t, h)
	admin, _ := findUserByEmail("demo@northwind.dev")
	recipient := addPermissionUser(t, "member", admin.OrgID, "cleanup-recipient")
	original := snapshot().Cases[0]
	body := recordMap(original)
	body["assignedOfficer"] = recipient.ID
	path := "/api/cases/" + original.ID
	w := leadWorkRequest(t, h, cookies, "PUT", path, body, leadWorkRevision(t, h, cookies, "cases", original.ID))
	assertStatus(t, w, 400)
	if !strings.Contains(w.Body.String(), "handoffs") {
		t.Fatal(w.Body.String())
	}
	body["assignedOfficer"] = admin.ID
	w = leadWorkRequest(t, h, cookies, "PUT", path, body, leadWorkRevision(t, h, cookies, "cases", original.ID))
	assertStatus(t, w, 200)
	var out struct{ Case Case }
	json.Unmarshal(w.Body.Bytes(), &out)
	if out.Case.AssignedOfficer != admin.ID || out.Case.OwnerID != original.OwnerID {
		t.Fatal(w.Body.String())
	}
}
