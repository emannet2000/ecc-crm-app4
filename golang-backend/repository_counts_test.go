package main

import (
	"context"
	"net/http/httptest"
	"testing"
)

func TestRepositoryBatchStudentCountsPreserveVisibilityAndZero(t *testing.T) {
	setupStore(t)
	u, _ := findUserByEmail("demo@northwind.dev")
	u.Role = "member"
	scope := RecordScope{OrgID: u.OrgID, OwnerID: u.ID, Visibility: "organization"}
	hidden := scope
	hidden.OwnerID = "another-user"
	hidden.Visibility = "private"
	foreign := scope
	foreign.OrgID = "another-workspace"
	students = []Student{
		{RecordScope: scope, ID: "visible-1", SchoolID: "one", AgentID: "one"},
		{RecordScope: scope, ID: "visible-2", SchoolID: "one", AgentID: "two"},
		{RecordScope: hidden, ID: "hidden", SchoolID: "one", AgentID: "one"},
		{RecordScope: foreign, ID: "foreign", SchoolID: "one", AgentID: "one"},
	}
	if err := saveStore(); err != nil {
		t.Fatal(err)
	}
	r := httptest.NewRequest("GET", "/api/schools", nil)
	r = r.WithContext(context.WithValue(r.Context(), requestUserKey, u))
	for _, entity := range []string{"schools", "agents"} {
		records := []map[string]any{{"id": "one"}, {"id": "two"}, {"id": "empty"}}
		if err := enrichRepositoryRecords(r, entity, records); err != nil {
			t.Fatal(err)
		}
		field := "studentsEnrolled"
		expected := []int{2, 0, 0}
		if entity == "agents" {
			field = "studentsReferred"
			expected = []int{1, 1, 0}
		}
		for i, record := range records {
			if record[field] != expected[i] {
				t.Fatalf("%s %v: expected %d", entity, record, expected[i])
			}
		}
		if err := enrichRepositoryRecords(r, entity, nil); err != nil {
			t.Fatal(err)
		}
	}
}
