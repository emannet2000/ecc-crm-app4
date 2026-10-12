package main

import (
	"sort"
	"strings"
	"time"
)

type LeadPipelineMetric struct {
	Source                    string   `json:"source"`
	Referrer                  string   `json:"referrer"`
	Leads                     int      `json:"leads"`
	Converted                 int      `json:"converted"`
	Rate                      float64  `json:"rate"`
	WithoutLoggedFollowUp     int      `json:"withoutLoggedFollowUp"`
	Overdue                   int      `json:"overdue"`
	LoggedFollowUps           int      `json:"loggedFollowUps"`
	TimedFollowUps            int      `json:"timedFollowUps"`
	AverageFirstFollowUpHours *float64 `json:"averageFirstFollowUpHours"`
}

func leadPipelineMetrics(leads []Lead, from, to, today string) []LeadPipelineMetric {
	groups := map[string]*LeadPipelineMetric{}
	hours := map[string]float64{}
	for _, lead := range leads {
		if !withinCohort(lead.CreatedAt, from, to) {
			continue
		}
		referrer := strings.TrimSpace(lead.Referrer)
		if referrer == "" {
			referrer = "Unspecified"
		}
		key := lead.Source + "\x00" + strings.ToLower(referrer)
		if groups[key] == nil {
			groups[key] = &LeadPipelineMetric{Source: lead.Source, Referrer: referrer}
		}
		row := groups[key]
		row.Leads++
		if lead.Status == "Converted" {
			row.Converted++
		}
		if lead.FirstFollowUpAt == "" {
			row.WithoutLoggedFollowUp++
		} else {
			row.LoggedFollowUps++
		}
		if openLead(lead.Status) && lead.FollowUpDate != "" && lead.FollowUpDate < today {
			if _, err := time.Parse("2006-01-02", lead.FollowUpDate); err == nil {
				row.Overdue++
			}
		}
		created, createdErr := time.Parse(time.RFC3339Nano, lead.CreatedAtTime)
		followed, followErr := time.Parse(time.RFC3339Nano, lead.FirstFollowUpAt)
		if createdErr == nil && followErr == nil && !followed.Before(created) {
			row.TimedFollowUps++
			hours[key] += followed.Sub(created).Hours()
		}
	}
	out := []LeadPipelineMetric{}
	for key, row := range groups {
		row.Rate = money(float64(row.Converted) * 100 / float64(row.Leads))
		if row.TimedFollowUps > 0 {
			value := money(hours[key] / float64(row.TimedFollowUps))
			row.AverageFirstFollowUpHours = &value
		}
		out = append(out, *row)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Source == out[j].Source {
			return out[i].Referrer < out[j].Referrer
		}
		return out[i].Source < out[j].Source
	})
	return out
}
