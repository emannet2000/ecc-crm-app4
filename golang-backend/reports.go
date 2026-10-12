package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"
)

type reportSummary struct {
	Today               string                        `json:"today"`
	GeneratedAt         string                        `json:"generatedAt"`
	ScopeLabel          string                        `json:"scopeLabel"`
	PipelineByCurrency  map[string]float64            `json:"pipelineByCurrency"`
	WonByCurrency       map[string]float64            `json:"wonByCurrency"`
	BalanceByCurrency   map[string]float64            `json:"balanceByCurrency"`
	StageCurrencyValues map[string]map[string]float64 `json:"stageCurrencyValues"`
	CaseStages          map[string]int                `json:"caseStages"`

	NewLeads           int                `json:"newLeads"`
	ApprovedStudents   int                `json:"approvedStudents"`
	TotalCases         int                `json:"totalCases"`
	StageCounts        map[string]int     `json:"stageCounts"`
	StageValues        map[string]float64 `json:"stageValues"`
	CreatedCounts      map[string]int     `json:"createdCounts"`
	PipelineLabel      string             `json:"pipelineLabel"`
	WonLabel           string             `json:"wonLabel"`
	BalanceLabel       string             `json:"balanceLabel"`
	ReportingCurrency  string             `json:"reportingCurrency"`
	MissingRates       []string           `json:"missingRates"`
	Contacts           int                `json:"contacts"`
	Students           int                `json:"students"`
	Leads              int                `json:"leads"`
	ActiveCases        int                `json:"activeCases"`
	OpenTasks          int                `json:"openTasks"`
	PipelineValue      float64            `json:"pipelineValue"`
	WonValue           float64            `json:"wonValue"`
	OutstandingBalance float64            `json:"outstandingBalance"`
}

func handleReports(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "private, no-store")
	r, saved, close, ok := dailySnapshot(w, r)
	if !ok {
		return
	}
	defer close()
	contacts, students, leads, cases, tasks, deals, invoices := saved.Contacts, saved.Students, saved.Leads, saved.Cases, saved.Tasks, saved.Deals, saved.Invoices
	pipeline, won, balance := map[string]float64{}, map[string]float64{}, map[string]float64{}
	report := reportSummary{StageCounts: map[string]int{}, StageValues: map[string]float64{}, CreatedCounts: map[string]int{}, Contacts: len(contacts), Students: len(students), Leads: len(leads)}
	report.Today = organizationToday(r)
	report.GeneratedAt = utcNow()
	report.ScopeLabel = "Records available to your account"
	report.StageCurrencyValues = map[string]map[string]float64{}
	report.CaseStages = map[string]int{}
	report.PipelineByCurrency = pipeline
	report.WonByCurrency = won
	report.BalanceByCurrency = balance
	report.TotalCases = len(cases)
	for _, l := range leads {
		if l.Status == "New" {
			report.NewLeads++
		}
	}
	for _, s := range students {
		if s.VisaStatus == "Approved" {
			report.ApprovedStudents++
		}
	}
	for _, c := range cases {
		report.CaseStages[c.CurrentStage]++
		if c.CurrentStage != "Closed" && c.CurrentStage != "Approved" && c.CurrentStage != "Refused" {
			report.ActiveCases++
		}
	}
	for _, task := range tasks {
		if task.Status != "done" {
			report.OpenTasks++
		}
	}
	for _, deal := range deals {
		report.StageCounts[deal.Stage]++
		if report.StageCurrencyValues[deal.Stage] == nil {
			report.StageCurrencyValues[deal.Stage] = map[string]float64{}
		}
		report.StageCurrencyValues[deal.Stage][currencyOf(deal.Currency)] += deal.Value
		if currencyOf(deal.Currency) == "USD" {
			report.StageValues[deal.Stage] += deal.Value
		}
		if deal.Stage == "Won" {
			won[currencyOf(deal.Currency)] += deal.Value
		} else if deal.Stage != "Lost" {
			pipeline[currencyOf(deal.Currency)] += deal.Value
		}
	}
	for _, invoice := range invoices {
		if invoice.Balance > 0 {
			balance[currencyOf(invoice.Currency)] += invoice.Balance
		}
	}
	// Read creation dates from decoded records instead of serializing the workspace again.
	countCreated := func(date string) {
		if len(date) > 10 {
			date = date[:10]
		}
		if date != "" {
			report.CreatedCounts[date]++
		}
	}
	for _, item := range saved.Leads {
		countCreated(item.CreatedAt)
	}
	for _, item := range saved.Students {
		countCreated(item.CreatedAt)
	}
	for _, item := range saved.Cases {
		countCreated(item.CreatedAt)
	}
	for _, item := range saved.Deals {
		countCreated(item.CreatedAt)
	}
	for _, item := range saved.Schools {
		countCreated(item.CreatedAt)
	}
	for _, item := range saved.Agents {
		countCreated(item.CreatedAt)
	}

	target := "USD"
	storeDB(r).QueryRow("SELECT currency FROM organizations WHERE id=?", currentUser(r).OrgID).Scan(&target)
	rates := map[string]float64{target: 1}
	rows, err := storeDB(r).Query("SELECT data FROM workspace_entries WHERE org_id=? AND category='exchange_rate' ORDER BY json_extract(data,'$.asOf') DESC", currentUser(r).OrgID)
	if err != nil {
		writeError(w, 503, "Could not load reporting exchange rates")
		return
	}
	if err == nil {
		for rows.Next() {
			var payload string
			if rows.Scan(&payload) != nil {
				rows.Close()
				writeError(w, 503, "Could not load reporting exchange rates")
				return
			}
			var rate struct {
				From, To, AsOf string
				Rate           float64
			}
			json.Unmarshal([]byte(payload), &rate)
			if rate.To == target && rate.Rate > 0 {
				if _, found := rates[rate.From]; !found {
					rates[rate.From] = rate.Rate
				}
			}
		}
		if rows.Err() != nil {
			rows.Close()
			writeError(w, 503, "Could not load reporting exchange rates")
			return
		}
		rows.Close()
	}
	missing := map[string]bool{}
	convert := func(values map[string]float64) (float64, string) {
		total := 0.0
		complete := true
		parts := []string{}
		keys := []string{}
		for c := range values {
			keys = append(keys, c)
		}
		sort.Strings(keys)
		for _, c := range keys {
			v := values[c]
			parts = append(parts, fmt.Sprintf("%s %.2f", c, v))
			if rate, found := rates[c]; found {
				total += v * rate
			} else {
				missing[c] = true
				complete = false
			}
		}
		if complete {
			return money(total), fmt.Sprintf("%s %.2f", target, total)
		}
		return 0, strings.Join(parts, " · ")
	}
	report.PipelineValue, report.PipelineLabel = convert(pipeline)
	report.WonValue, report.WonLabel = convert(won)
	report.OutstandingBalance, report.BalanceLabel = convert(balance)
	report.ReportingCurrency = target
	report.MissingRates = []string{}
	for c := range missing {
		report.MissingRates = append(report.MissingRates, c)
	}
	sort.Strings(report.MissingRates)
	writeJSON(w, 200, report)
}
