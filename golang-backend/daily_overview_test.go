package main

import (
	"encoding/json"
	"testing"
)

func TestDailyOverviewIncludesAllStagesAndProtectsScope(t *testing.T) {
	setupStore(t)
	u, _ := findUserByEmail("demo@northwind.dev")
	u.Role = "member"
	scope := RecordScope{OrgID: u.OrgID, OwnerID: u.ID, Visibility: "organization"}
	mu.Lock()
	users[u.Email] = u
	deals = []Deal{{RecordScope: scope, ID: "open", Title: "Open deal", Owner: u.Name, Stage: "Proposal", Currency: "PHP", Value: 500}, {RecordScope: scope, ID: "won", Title: "Won deal", Owner: u.Name, Stage: "Won"}, {RecordScope: scope, ID: "lost", Title: "Lost deal", Owner: u.Name, Stage: "Lost"}, {RecordScope: scope, ID: "assigned-other", Title: "Other adviser", Owner: "Other adviser", Stage: "Qualified"}}
	hidden := deals[0]
	hidden.ID = "private-deal"
	hidden.OwnerID = "other-user"
	hidden.Visibility = "private"
	deals = append(deals, hidden)
	leads = []Lead{{RecordScope: scope, ID: "future", Name: "Future inquiry", AssignedTo: u.Name, Status: "New", NextAction: "Call", FollowUpDate: "2099-01-01"}, {RecordScope: scope, ID: "converted", Name: "Converted inquiry", AssignedTo: u.Name, Status: "Converted"}}
	students = []Student{{RecordScope: scope, ID: "student", Name: "Student", Program: "Nursing", Intake: "January 2027"}}
	cases = []Case{{RecordScope: scope, ID: "closed", CaseNumber: "CLOSED", AssignedOfficer: u.Name, CurrentStage: "Closed"}}
	mu.Unlock()
	if err := saveStore(); err != nil {
		t.Fatal(err)
	}
	h := expansionTestMux()
	for category, want := range map[string]int{"deals": 3, "pipeline": 1, "leads": 2, "students": 1, "cases": 1} {
		w := dailyUserRequest(t, h, u, "GET", "/api/my-work?category="+category, nil)
		assertStatus(t, w, 200)
		var out struct {
			Items    []productivityItem
			Total    int
			Counts   map[string]int
			Category string
		}
		if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
			t.Fatal(err)
		}
		if out.Total != want || len(out.Items) != want || out.Counts[category] != want || out.Category != category {
			t.Fatalf("%s: %s", category, w.Body.String())
		}
		for key, expected := range map[string]int{"deals": 3, "pipeline": 1, "leads": 2, "students": 1, "cases": 1} {
			if out.Counts[key] != expected {
				t.Fatalf("%s count changed when requesting %s: %s", key, category, w.Body.String())
			}
		}
		for _, item := range out.Items {
			if item.ID == "private-deal" || item.ID == "assigned-other" || item.Route == "" {
				t.Fatal(w.Body.String())
			}
		}
		if category == "pipeline" && (out.Items[0].ID != "open" || len(out.Items[0].Details) == 0) {
			t.Fatal(w.Body.String())
		}
	}
	w := dailyUserRequest(t, h, u, "GET", "/api/my-work?category=deals&scope=all&offset=1&limit=1", nil)
	assertStatus(t, w, 200)
	var out struct {
		Items []productivityItem
		Total int
	}
	json.Unmarshal(w.Body.Bytes(), &out)
	if out.Total != 4 || len(out.Items) != 1 {
		t.Fatal(w.Body.String())
	}
}
