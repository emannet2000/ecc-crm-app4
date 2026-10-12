package main

import (
	"context"
	"net/http/httptest"
	"reflect"
	"testing"
)

func TestReportSnapshotUsesCommittedVisibilityAndParents(t *testing.T) {
	setupStore(t)
	u, _ := findUserByEmail("demo@northwind.dev")
	u.Role = "member"
	visible := RecordScope{OrgID: u.OrgID, OwnerID: "another", Visibility: "organization"}
	hidden := visible
	hidden.Visibility = "private"
	foreign := visible
	foreign.OrgID = "another-workspace"
	for _, entity := range []string{"contacts", "cases", "leads", "invoices", "deals"} {
		insertSearchFixture(t, entity, "visible-"+entity, visible, map[string]any{})
		insertSearchFixture(t, entity, "hidden-"+entity, hidden, map[string]any{})
		insertSearchFixture(t, entity, "foreign-"+entity, foreign, map[string]any{})
	}
	for _, parent := range []string{"visible", "hidden", "foreign", "missing"} {
		insertSearchFixture(t, "documents", parent+"-document", visible, map[string]any{"caseId": parent + "-cases"})
		insertSearchFixture(t, "payments", parent+"-payment", visible, map[string]any{"invoiceId": parent + "-invoices"})
		insertSearchFixture(t, "activities", parent+"-activity", visible, map[string]any{"contactId": parent + "-contacts", "dealId": parent + "-deals"})
	}
	insertSearchFixture(t, "documents", "lead-document", visible, map[string]any{"leadId": "visible-leads"})
	insertSearchFixture(t, "documents", "case-precedence-document", visible, map[string]any{"caseId": "hidden-cases", "leadId": "visible-leads"})
	insertSearchFixture(t, "activities", "unlinked-activity", visible, map[string]any{})
	r := httptest.NewRequest("GET", "/api/reports", nil)
	r = r.WithContext(context.WithValue(r.Context(), requestUserKey, u))
	s, err := sqlWorkspace(r)
	if err != nil {
		t.Fatal(err)
	}
	assertIDs := func(entity string, ids []string, required, forbidden []string) {
		t.Helper()
		set := map[string]bool{}
		for _, id := range ids {
			set[id] = true
		}
		for _, id := range required {
			if !set[id] {
				t.Errorf("%s missing %s", entity, id)
			}
		}
		for _, id := range forbidden {
			if set[id] {
				t.Errorf("%s exposed %s", entity, id)
			}
		}
	}
	var docIDs, paymentIDs, activityIDs, caseIDs []string
	for _, d := range s.Documents {
		docIDs = append(docIDs, d.ID)
	}
	for _, p := range s.Payments {
		paymentIDs = append(paymentIDs, p.ID)
	}
	for _, a := range s.Activities {
		activityIDs = append(activityIDs, a.ID)
	}
	for _, c := range s.Cases {
		caseIDs = append(caseIDs, c.ID)
	}
	daily, err := sqlDailyWorkload(r)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(daily.Documents, s.Documents) || !reflect.DeepEqual(daily.Cases, s.Cases) || !reflect.DeepEqual(daily.Invoices, s.Invoices) {
		t.Fatal("daily workload differs from visible report snapshot")
	}
	assertIDs("documents", docIDs, []string{"visible-document", "lead-document"}, []string{"hidden-document", "foreign-document", "missing-document", "case-precedence-document"})
	assertIDs("payments", paymentIDs, []string{"visible-payment"}, []string{"hidden-payment", "foreign-payment", "missing-payment"})
	assertIDs("activities", activityIDs, []string{"visible-activity", "unlinked-activity"}, []string{"hidden-activity", "foreign-activity", "missing-activity"})
	assertIDs("cases", caseIDs, []string{"visible-cases"}, []string{"hidden-cases", "foreign-cases"})
}
