package main

import (
	"encoding/json"
	"testing"
)

func TestStaffObservationsStayWithinWorkspaceAndOwner(t *testing.T) {
	setupStore(t)
	admin, _ := findUserByEmail("demo@northwind.dev")
	member := addPermissionUser(t, "member", admin.OrgID, "usability-adviser")
	other := addPermissionUser(t, "member", admin.OrgID, "usability-other")
	foreign := addPermissionUser(t, "admin", "usability-foreign", "usability-foreign-admin")
	h := expansionTestMux()
	path := "/api/usability/feedback"
	observation := UsabilityObservation{Role: "adviser", Task: "student-review", Completed: true, Seconds: 75, Comments: "Needed help finding the portal reply"}
	assertStatus(t, rawUserRequest(t, h, member, "POST", path, observation), 201)
	count := func(u User, expected int) {
		t.Helper()
		w := rawUserRequest(t, h, u, "GET", path, nil)
		assertStatus(t, w, 200)
		var out struct{ Observations []json.RawMessage }
		if json.Unmarshal(w.Body.Bytes(), &out) != nil || len(out.Observations) != expected {
			t.Fatalf("unexpected accessible observations: %s", w.Body.String())
		}
	}
	count(member, 1)
	count(admin, 1)
	count(other, 0)
	count(foreign, 0)
	observation.Role = "finance"
	assertStatus(t, rawUserRequest(t, h, member, "POST", path, observation), 400)
	observation.Role = "adviser"
	observation.Seconds = -1
	assertStatus(t, rawUserRequest(t, h, member, "POST", path, observation), 400)
}

func TestStaffRetestRequiresReferenceAndPreservesSessionContext(t *testing.T) {
	setupStore(t)
	u, _ := findUserByEmail("demo@northwind.dev")
	h := expansionTestMux()
	in := UsabilityObservation{Role: "finance", Task: "invoice-receipt", Phase: "retest", Session: "Finance A", Device: "Phone / Chrome", Comments: "Test fixture; no actual staff participation"}
	assertStatus(t, rawUserRequest(t, h, u, "POST", "/api/usability/feedback", in), 400)
	in.IssueReference = "OBS-12"
	w := rawUserRequest(t, h, u, "POST", "/api/usability/feedback", in)
	assertStatus(t, w, 201)
	var result struct{ Observation UsabilityObservation }
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.Observation.Session != in.Session || result.Observation.Device != in.Device || result.Observation.Phase != "retest" || result.Observation.IssueReference != "OBS-12" {
		t.Fatalf("lost session context: %s", w.Body.String())
	}
	in.Phase = "invented"
	assertStatus(t, rawUserRequest(t, h, u, "POST", "/api/usability/feedback", in), 400)
}

func TestNewWorkflowSessionTasksMatchParticipantRole(t *testing.T) {
	setupStore(t)
	u, _ := findUserByEmail("demo@northwind.dev")
	h := expansionTestMux()
	for _, role := range []string{"adviser", "finance"} {
		in := UsabilityObservation{Role: role, Task: "workspace-navigation", Comments: "Automated schema fixture; actual staff session pending"}
		assertStatus(t, rawUserRequest(t, h, u, "POST", "/api/usability/feedback", in), 201)
		in.Task = "document-followup"
		status := 201
		if role == "finance" {
			status = 400
		}
		assertStatus(t, rawUserRequest(t, h, u, "POST", "/api/usability/feedback", in), status)
	}
}
