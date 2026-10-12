package main

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"
)

func TestProfessionalReportCurrencyBreakdownsAndScope(t *testing.T) {
	setupStore(t)
	u := addPermissionUser(t, "member", "professional-reports", "report-reader")
	visible := RecordScope{OrgID: u.OrgID, OwnerID: u.ID, Visibility: "organization"}
	hidden := visible
	hidden.OwnerID = "someone-else"
	hidden.Visibility = "private"
	foreign := visible
	foreign.OrgID = "another-report-org"
	for _, item := range []struct {
		id, stage, currency string
		amount              float64
		scope               RecordScope
	}{
		{"report-usd", "Proposal", "USD", 1234.56, visible}, {"report-eur", "Proposal", "EUR", 100, visible}, {"report-won", "Won", "USD", 90.25, visible}, {"report-lost", "Lost", "USD", 800, visible}, {"report-hidden", "Proposal", "USD", 999999, hidden}, {"report-foreign", "Proposal", "USD", 999999, foreign},
	} {
		insertSearchFixture(t, "deals", item.id, item.scope, map[string]any{"stage": item.stage, "currency": item.currency, "value": item.amount, "createdAt": "2026-10-01T12:00:00Z"})
	}
	insertSearchFixture(t, "cases", "report-case", visible, map[string]any{"currentStage": "Submitted", "createdAt": "2026-10-01T12:00:00Z"})
	insertSearchFixture(t, "invoices", "report-invoice", visible, map[string]any{"currency": "EUR", "balance": 25.75})
	h := http.NewServeMux()
	h.HandleFunc("GET /api/reports", authMiddleware(handleReports))
	get := func() reportSummary {
		t.Helper()
		w := rawUserRequest(t, h, u, "GET", "/api/reports", nil)
		assertStatus(t, w, 200)
		var report reportSummary
		if err := json.Unmarshal(w.Body.Bytes(), &report); err != nil {
			t.Fatal(err)
		}
		return report
	}
	report := get()
	if _, err := time.Parse(time.RFC3339, report.GeneratedAt); err != nil {
		t.Fatal("missing snapshot timestamp", err)
	}
	if report.PipelineByCurrency["USD"] != 1234.56 || report.PipelineByCurrency["EUR"] != 100 || report.WonByCurrency["USD"] != 90.25 || report.BalanceByCurrency["EUR"] != 25.75 || report.CaseStages["Submitted"] != 1 || report.StageCounts["Proposal"] != 2 || report.CreatedCounts["2026-10-01"] != 5 {
		t.Fatalf("incorrect breakdown: %+v", report)
	}
	if len(report.MissingRates) != 1 || report.MissingRates[0] != "EUR" || report.PipelineValue != 0 {
		t.Fatalf("incomplete conversion presented as a total: %+v", report)
	}
	if report.StageCurrencyValues["Proposal"]["EUR"] != 100 {
		t.Fatal("stage value lost its currency")
	}
	payload := `{"from":"EUR","to":"USD","rate":1.1,"asOf":"2026-10-01"}`
	if _, err := database.Exec(`INSERT INTO workspace_entries(id,org_id,category,user_id,record_id,data,created_at,updated_at) VALUES(?,?,'exchange_rate','','',?,?,?)`, "report-rate", u.OrgID, payload, utcNow(), utcNow()); err != nil {
		t.Fatal(err)
	}
	report = get()
	if len(report.MissingRates) != 0 || report.PipelineValue != 1344.56 || report.OutstandingBalance != 28.33 {
		t.Fatalf("conversion failed: %+v", report)
	}
}
