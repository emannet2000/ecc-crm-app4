package main

import (
	"sort"
	"strings"
	"time"
)

type StaffWorkload struct {
	Responsible  string `json:"responsible"`
	Leads        int    `json:"leads"`
	Applications int    `json:"applications"`
	Overdue      int    `json:"overdue"`
	MissingPlans int    `json:"missingPlans"`
	Documents    int    `json:"documents"`
}
type CollectionSummary struct {
	Currency    string  `json:"currency"`
	Received    float64 `json:"received"`
	Outstanding float64 `json:"outstanding"`
	Overdue     float64 `json:"overdue"`
	Pending     float64 `json:"pending"`
}
type StalledApplication struct {
	Record      string `json:"record"`
	Stage       string `json:"stage"`
	Days        int    `json:"days"`
	Basis       string `json:"basis"`
	Responsible string `json:"responsible"`
	Action      string `json:"action"`
	URL         string `json:"url"`
}

func operationalReport(s diskStore, today string, lastStage map[string]time.Time, names map[string]string) map[string]any {
	ownerName := func(id string) string {
		if name := names[id]; name != "" {
			return name
		}
		if strings.TrimSpace(id) == "" || strings.EqualFold(id, "Unassigned") {
			return "Unassigned"
		}
		return id
	}
	owners := map[string]*StaffWorkload{}
	owner := func(id string) *StaffWorkload {
		if owners[id] == nil {
			owners[id] = &StaffWorkload{Responsible: ownerName(id)}
		}
		return owners[id]
	}
	plan := func(row *StaffWorkload, action, deadline string) {
		if deadline != "" && deadline < today {
			row.Overdue++
		}
		if strings.TrimSpace(action) == "" || deadline == "" {
			row.MissingPlans++
		}
	}
	for _, l := range s.Leads {
		if openLead(l.Status) {
			row := owner(l.AssignedTo)
			row.Leads++
			plan(row, l.NextAction, l.FollowUpDate)
		}
	}
	stalled := []StalledApplication{}
	day, _ := time.Parse("2006-01-02", today)
	for _, c := range s.Cases {
		if !activeApplication(c.CurrentStage) {
			continue
		}
		row := owner(c.AssignedOfficer)
		row.Applications++
		plan(row, c.NextAction, c.NextDeadline)
		since := lastStage[c.ID]
		basis := "Recorded stage transition"
		if since.IsZero() {
			date := c.CreatedAt
			if date == "" {
				date = c.DateOpened
			}
			if len(date) >= 10 {
				since, _ = time.Parse("2006-01-02", date[:10])
			}
			basis = "Application creation/opening date; stage history unavailable"
		}
		if !since.IsZero() {
			date, _ := time.Parse("2006-01-02", since.Format("2006-01-02"))
			days := int(day.Sub(date).Hours() / 24)
			if days >= 14 {
				stalled = append(stalled, StalledApplication{c.CaseNumber, c.CurrentStage, days, basis, ownerName(c.AssignedOfficer), actionOrDefault(c.NextAction, "Set the next action and deadline"), "/cases/" + c.ID})
			}
		}
	}
	// These documents are already scoped with parent visibility by sqlWorkspace.
	for _, d := range s.Documents {
		if _, _, needed := documentIssue(d, today); needed {
			assigned := d.ResponsibleID
			active := false
			for _, c := range s.Cases {
				if c.ID == d.CaseID && activeApplication(c.CurrentStage) {
					active = true
					if assigned == "" {
						assigned = c.AssignedOfficer
					}
				}
			}
			for _, l := range s.Leads {
				if d.CaseID == "" && l.ID == d.LeadID && openLead(l.Status) {
					active = true
					if assigned == "" {
						assigned = l.AssignedTo
					}
				}
			}
			if active {
				if assigned == "" {
					assigned = d.OwnerID
				}
				owner(assigned).Documents++
			}
		}
	}
	totals := map[string]*CollectionSummary{}
	invoices := map[string]Invoice{}
	for _, inv := range s.Invoices {
		invoices[inv.ID] = inv
		currency := currencyOf(inv.Currency)
		if totals[currency] == nil {
			totals[currency] = &CollectionSummary{Currency: currency}
		}
		t := totals[currency]
		t.Received += inv.AmountReceived
		t.Outstanding += inv.Balance
		if inv.Balance > 0 && inv.DueDate != "" && inv.DueDate < today {
			t.Overdue += inv.Balance
		}
	}
	for _, p := range s.Payments {
		if pendingReceipt(p) {
			if inv, ok := invoices[p.InvoiceID]; ok {
				totals[currencyOf(inv.Currency)].Pending += p.Amount
			}
		}
	}
	collections := []CollectionSummary{}
	for _, t := range totals {
		t.Received = money(t.Received)
		t.Outstanding = money(t.Outstanding)
		t.Overdue = money(t.Overdue)
		t.Pending = money(t.Pending)
		collections = append(collections, *t)
	}
	workload := []StaffWorkload{}
	for _, row := range owners {
		workload = append(workload, *row)
	}
	sort.Slice(workload, func(i, j int) bool { return workload[i].Responsible < workload[j].Responsible })
	sort.Slice(collections, func(i, j int) bool { return collections[i].Currency < collections[j].Currency })
	sort.Slice(stalled, func(i, j int) bool {
		if stalled[i].Days != stalled[j].Days {
			return stalled[i].Days > stalled[j].Days
		}
		return stalled[i].URL < stalled[j].URL
	})
	return map[string]any{"workload": workload, "collections": collections, "stalled": stalled, "today": today, "method": "Current accessible records, independent of creation-date cohort filters. Long-running means 14 or more calendar days since the recorded stage transition; creation/opening date is labeled as a fallback. Pending amounts cover accessible receipts. Received amounts are current invoice totals, not collections during a date range."}
}
