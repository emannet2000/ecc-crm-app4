package main

import (
	"encoding/json"
	"fmt"
	"testing"
)

// Insert committed records without populating the legacy cache: search must
// observe SQL state and apply access rules without loading a workspace.
func insertSearchFixture(t *testing.T, entity, id string, scope RecordScope, fields map[string]any) {
	t.Helper()
	fields["id"], fields["orgId"], fields["ownerId"] = id, scope.OrgID, scope.OwnerID
	fields["teamId"], fields["visibility"] = scope.TeamID, scope.Visibility
	payload, err := json.Marshal(fields)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = database.Exec("INSERT INTO "+entity+"(id,data,sort_order) VALUES(?,?,0)", id, string(payload)); err != nil {
		t.Fatal(err)
	}
}

func searchFixtureResults(t *testing.T, query string) ([]SearchResult, bool) {
	t.Helper()
	w := authorizedRequest(t, workspaceMux(), "GET", "/api/search?q="+query, "")
	var response struct {
		Results   []SearchResult `json:"results"`
		Truncated bool           `json:"truncated"`
	}
	if w.Code != 200 {
		t.Fatalf("search failed: %d %s", w.Code, w.Body.String())
	}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	return response.Results, response.Truncated
}

func TestGlobalSearchSQLVisibilityAcrossEntities(t *testing.T) {
	setupStore(t)
	u, _ := findUserByEmail("demo@northwind.dev")
	u.Role = "member"
	users[u.Email] = u
	if err := saveStore(); err != nil {
		t.Fatal(err)
	}
	organization := RecordScope{OrgID: u.OrgID, OwnerID: "other", Visibility: "organization"}
	for _, parent := range []string{"cases", "leads", "invoices", "contacts", "deals"} {
		insertSearchFixture(t, parent, "search-parent-"+parent, organization, map[string]any{})
	}
	for _, entity := range searchableEntities {
		t.Run(entity.table, func(t *testing.T) {
			prefix := "search-scope-" + entity.table
			scopes := []RecordScope{
				{OrgID: u.OrgID, OwnerID: u.ID, Visibility: "private"},
				organization,
				{OrgID: u.OrgID, OwnerID: "other", Visibility: "team", TeamID: u.TeamID},
				{OrgID: u.OrgID, OwnerID: "other", Visibility: "private"},
				{OrgID: u.OrgID, OwnerID: "other", Visibility: "team", TeamID: "other-team"},
				{OrgID: "other-workspace", OwnerID: u.ID, Visibility: "organization"},
			}
			for i, scope := range scopes {
				fields := map[string]any{entity.title: fmt.Sprintf("%s-%d", prefix, i), entity.subtitle: "Search subtitle"}
				switch entity.table {
				case "documents":
					fields["caseId"] = "search-parent-cases"
				case "payments":
					fields["invoiceId"] = "search-parent-invoices"
				case "activities":
					fields["contactId"], fields["dealId"] = "search-parent-contacts", "search-parent-deals"
				}
				insertSearchFixture(t, entity.table, fmt.Sprintf("%s-%d", prefix, i), scope, fields)
			}
			results, truncated := searchFixtureResults(t, prefix)
			if len(results) != 3 || truncated {
				t.Fatalf("expected three visible results: %+v, truncated=%v", results, truncated)
			}
			for i, result := range results {
				want := fmt.Sprintf("%s-%d", prefix, i)
				if result.Entity != entity.table || result.ID != want || result.Title != want || result.Subtitle != "Search subtitle" {
					t.Fatalf("unexpected result: %+v, want %s", result, want)
				}
			}
		})
	}
}

func TestGlobalSearchSQLParentVisibilityAndLimit(t *testing.T) {
	setupStore(t)
	u, _ := findUserByEmail("demo@northwind.dev")
	u.Role = "member"
	users[u.Email] = u
	if err := saveStore(); err != nil {
		t.Fatal(err)
	}
	visible := RecordScope{OrgID: u.OrgID, OwnerID: u.ID, Visibility: "organization"}
	hidden := RecordScope{OrgID: u.OrgID, OwnerID: "other", Visibility: "private"}
	foreign := visible
	foreign.OrgID = "other-workspace"
	for _, entity := range []string{"cases", "leads", "invoices", "contacts", "deals"} {
		insertSearchFixture(t, entity, "parent-visible-"+entity, visible, map[string]any{})
		insertSearchFixture(t, entity, "parent-hidden-"+entity, hidden, map[string]any{})
		insertSearchFixture(t, entity, "parent-foreign-"+entity, foreign, map[string]any{})
	}
	// All these candidates sort before the visible results. They must be
	// excluded before LIMIT, including documents with an unreadable case and a
	// readable lead (the case takes precedence), and activity with either hidden parent.
	for i := 0; i < 40; i++ {
		parent := []string{"hidden", "foreign", "missing"}[i%3]
		insertSearchFixture(t, "documents", fmt.Sprintf("hidden-document-%02d", i), visible, map[string]any{
			"docName": "AAA-search-limit", "caseId": "parent-" + parent + "-cases", "leadId": "parent-visible-leads",
		})
	}
	for _, parent := range []string{"hidden", "foreign", "missing"} {
		insertSearchFixture(t, "documents", "hidden-lead-document-"+parent, visible, map[string]any{"docName": "AAA-search-limit", "leadId": "parent-" + parent + "-leads"})
		insertSearchFixture(t, "payments", "hidden-payment-"+parent, visible, map[string]any{"reference": "AAA-search-limit", "invoiceId": "parent-" + parent + "-invoices"})
		for _, field := range []string{"contactId", "dealId"} {
			fields := map[string]any{"title": "AAA-search-limit", "contactId": "parent-visible-contacts", "dealId": "parent-visible-deals"}
			entity := "contacts"
			if field == "dealId" {
				entity = "deals"
			}
			fields[field] = "parent-" + parent + "-" + entity
			insertSearchFixture(t, "activities", "hidden-activity-"+parent+field, visible, fields)
		}
	}
	for i := 0; i < 30; i++ {
		insertSearchFixture(t, "contacts", fmt.Sprintf("search-limit-%02d", i), visible, map[string]any{"name": fmt.Sprintf("search-limit-%02d", i)})
	}
	results, truncated := searchFixtureResults(t, "search-limit")
	if len(results) != 30 || truncated {
		t.Fatalf("exactly 30 accessible records should not truncate: %d, %v", len(results), truncated)
	}
	for i, result := range results {
		if result.ID != fmt.Sprintf("search-limit-%02d", i) {
			t.Fatalf("unexpected ordered result: %+v", result)
		}
	}
	insertSearchFixture(t, "documents", "visible-lead-document", visible, map[string]any{"docName": "ZZZ-search-limit", "leadId": "parent-visible-leads"})
	results, truncated = searchFixtureResults(t, "search-limit")
	if len(results) != 30 || !truncated || results[29].ID != "search-limit-29" {
		t.Fatalf("31 accessible records should truncate: %+v, %v", results, truncated)
	}
	// Optional activity parents may both be empty, but a payment or document
	// with no parent must never appear.
	insertSearchFixture(t, "activities", "standalone-activity", visible, map[string]any{"title": "search-standalone"})
	insertSearchFixture(t, "payments", "orphan-payment", visible, map[string]any{"reference": "search-standalone"})
	insertSearchFixture(t, "documents", "orphan-document", visible, map[string]any{"docName": "search-standalone"})
	results, truncated = searchFixtureResults(t, "search-standalone")
	if len(results) != 1 || results[0].ID != "standalone-activity" || truncated {
		t.Fatalf("unexpected parentless results: %+v", results)
	}
}
