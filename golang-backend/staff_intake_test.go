package main

import (
	"encoding/json"
	"testing"
)

func TestStaffAssignmentAndExplicitConversion(t *testing.T) {
	setupStore(t)
	h := leadIntakeMux()
	cookies := demoCookies(t, h)
	admin, _ := findUserByEmail("demo@northwind.dev")
	mu.Lock()
	for _, kind := range []string{"disabled", "viewer", "foreign"} {
		u := admin
		u.ID = "staff_" + kind
		u.Email = kind + "@staff.test"
		u.Name = kind
		u.Role = "member"
		u.Disabled = kind == "disabled"
		if kind == "viewer" {
			u.Role = "viewer"
		}
		if kind == "foreign" {
			u.OrgID = "other-workspace"
		}
		users[u.Email] = u
	}
	mu.Unlock()
	if err := saveStore(); err != nil {
		t.Fatal(err)
	}
	w := featureRequest(h, "GET", "/api/staff-options?entity=leads", nil, cookies)
	assertStatus(t, w, 200)
	var roster struct{ Staff []RequirementPerson }
	json.Unmarshal(w.Body.Bytes(), &roster)
	if len(roster.Staff) == 0 || roster.Staff[0].ID == "" {
		t.Fatal(w.Body.String())
	}
	for _, person := range roster.Staff {
		if person.ID == "staff_disabled" || person.ID == "staff_viewer" || person.ID == "staff_foreign" {
			t.Fatal(w.Body.String())
		}
	}
	body := map[string]string{"name": "Staff inquiry", "source": "Referral", "status": "New", "assignedTo": "Misspelled Flor"}
	assertStatus(t, featureRequest(h, "POST", "/api/leads", body, cookies), 400)
	for _, id := range []string{"staff_disabled", "staff_viewer", "staff_foreign"} {
		body["assignedTo"] = id
		assertStatus(t, featureRequest(h, "POST", "/api/leads", body, cookies), 400)
	}
	body["assignedTo"] = "u_1"
	body["status"] = " Converted "
	assertStatus(t, featureRequest(h, "POST", "/api/leads", body, cookies), 400)
	body["status"] = "New"
	w = featureRequest(h, "POST", "/api/leads", body, cookies)
	assertStatus(t, w, 201)
	var created struct{ Lead Lead }
	json.Unmarshal(w.Body.Bytes(), &created)
	if created.Lead.AssignedTo != "u_1" {
		t.Fatal(w.Body.String())
	}
	update := leadMap(created.Lead)
	update["status"] = "Converted"
	assertStatus(t, leadWorkRequest(t, h, cookies, "PUT", "/api/leads/"+created.Lead.ID, update, leadWorkRevision(t, h, cookies, "leads", created.Lead.ID)), 400)
	assertStatus(t, featureRequest(h, "POST", "/api/workflows/lead-convert", map[string]string{"id": created.Lead.ID, "target": "contact"}, cookies), 200)
	w = featureRequest(h, "GET", "/api/leads/"+created.Lead.ID, nil, cookies)
	assertStatus(t, w, 200)
	json.Unmarshal(w.Body.Bytes(), &created)
	if created.Lead.Status != "Converted" || created.Lead.ConvertedContactID == "" {
		t.Fatal(w.Body.String())
	}
	update = leadMap(created.Lead)
	update["status"] = "New"
	assertStatus(t, leadWorkRequest(t, h, cookies, "PUT", "/api/leads/"+created.Lead.ID, update, leadWorkRevision(t, h, cookies, "leads", created.Lead.ID)), 400)
}

func TestDocumentReviewTracksActualVersionAndResetsOnUpload(t *testing.T) {
	setupStore(t)
	h := leadIntakeMux()
	cookies := demoCookies(t, h)
	lead := intakeLead(t, h, cookies)
	w := multipartRequest(h, "/api/leads/"+lead.ID+"/cv", "file", "client.pdf", []byte("%PDF-1.4 original"), cookies, nil)
	assertStatus(t, w, 201)
	var out struct{ Document Document }
	json.Unmarshal(w.Body.Bytes(), &out)
	doc := out.Document
	url := "/api/documents/" + doc.ID + "/review"
	rev := leadWorkRevision(t, h, cookies, "documents", doc.ID)
	assertStatus(t, featureRequest(h, "POST", url, map[string]any{"confirmed": true}, cookies), 428)
	assertStatus(t, leadWorkRequest(t, h, cookies, "POST", url, map[string]any{"confirmed": false}, rev), 400)
	w = leadWorkRequest(t, h, cookies, "POST", url, map[string]any{"confirmed": true, "reviewedBy": "Spoofed"}, rev)
	assertStatus(t, w, 200)
	json.Unmarshal(w.Body.Bytes(), &out)
	if out.Document.ReviewedByID != "u_1" || out.Document.ReviewedBy != "Demo User" || out.Document.ReviewedAt == "" || out.Document.ReviewedVersion != 1 || out.Document.Status != "Received" {
		t.Fatal(w.Body.String())
	}
	assertStatus(t, leadWorkRequest(t, h, cookies, "POST", url, map[string]any{"confirmed": true}, rev), 409)
	w = multipartRequest(h, "/api/leads/"+lead.ID+"/cv", "file", "client.pdf", []byte("%PDF-1.4 revised"), cookies, nil)
	assertStatus(t, w, 201)
	out = struct{ Document Document }{}
	json.Unmarshal(w.Body.Bytes(), &out)
	if out.Document.ReviewedByID != "" || out.Document.ReviewedBy != "" || out.Document.ReviewedAt != "" || out.Document.ReviewedVersion != 0 || out.Document.LatestVersion != 2 {
		t.Fatal(w.Body.String())
	}
	w = leadWorkRequest(t, h, cookies, "POST", url, map[string]any{"confirmed": true}, leadWorkRevision(t, h, cookies, "documents", doc.ID))
	assertStatus(t, w, 200)
	json.Unmarshal(w.Body.Bytes(), &out)
	if out.Document.ReviewedVersion != 2 {
		t.Fatal(w.Body.String())
	}
}

func TestIntakeSessionTasksAreAvailableForRealObservations(t *testing.T) {
	setupStore(t)
	h := leadIntakeMux()
	cookies := demoCookies(t, h)
	for _, task := range []string{"lead-intake", "document-review", "lead-conversion"} {
		assertStatus(t, featureRequest(h, "POST", "/api/usability/feedback", UsabilityObservation{Role: "adviser", Task: task, Comments: "Automated API fixture only; no staff session performed."}, cookies), 201)
	}
	assertStatus(t, featureRequest(h, "POST", "/api/usability/feedback", UsabilityObservation{Role: "finance", Task: "lead-conversion", Comments: "Role mismatch fixture"}, cookies), 400)
}
