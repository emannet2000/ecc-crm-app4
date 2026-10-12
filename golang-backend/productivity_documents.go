package main

import "strings"

// Describe file, review and expiry work without changing verification stage gates.
func documentIssue(d Document, today string) (string, string, bool) {
	if d.Required && d.FilePath == "" {
		return "Missing required file", "Request the missing document", true
	}
	if d.ExpiryDate != "" {
		if days, valid := dateDistance(d.ExpiryDate, today); valid && days <= 30 {
			if days < 0 {
				return "Expired document", "Request and review a replacement", true
			}
			return "Document expires within 30 days", "Arrange replacement before expiry", true
		}
	}
	if strings.EqualFold(d.Status, "Rejected") {
		return "Rejected document", "Explain the rejection and request a corrected file", true
	}
	if d.FilePath != "" && d.LatestVersion > 0 && d.ReviewedVersion != d.LatestVersion {
		return "Current version awaiting review", "Open the latest file and record its review", true
	}
	if documentNeedsReview(d, today) {
		return "Document awaiting verification", "Review the document and update its verification status", true
	}
	return "", "", false
}
func dailyDocumentItems(user User, s diskStore, today, filter string) []productivityItem {
	out := []productivityItem{}
	cases := make(map[string]*Case, len(s.Cases))
	leads := make(map[string]*Lead, len(s.Leads))
	for i := range s.Cases {
		c := &s.Cases[i]
		cases[c.ID] = c
	}
	for i := range s.Leads {
		l := &s.Leads[i]
		leads[l.ID] = l
	}
	for _, d := range s.Documents {
		if d.OrgID != user.OrgID || !canRead(d.RecordScope, user) {
			continue
		}
		reason, action, needed := documentIssue(d, today)
		if !needed {
			continue
		}
		owner, deadline, route := d.ResponsibleID, d.DueDate, ""
		if d.CaseID != "" {
			c, ok := cases[d.CaseID]
			if !ok || c.OrgID != user.OrgID || !canRead(c.RecordScope, user) || !activeApplication(c.CurrentStage) {
				continue
			}
			if owner == "" {
				owner = c.AssignedOfficer
				if owner == "" {
					owner = d.OwnerID
				}
			}
			if deadline == "" {
				deadline = c.NextDeadline
			}
			route = "/cases/" + c.ID + "#record-tools?tab=documents&document=" + d.ID
		} else {
			l, ok := leads[d.LeadID]
			if !ok || l.OrgID != user.OrgID || !canRead(l.RecordScope, user) || !openLead(l.Status) {
				continue
			}
			if owner == "" {
				owner = l.AssignedTo
				if owner == "" {
					owner = d.OwnerID
				}
			}
			if deadline == "" {
				deadline = l.FollowUpDate
			}
			route = "/leads/" + l.ID + "#record-tools?tab=documents"
		}
		if !workAssigned(owner, d.RecordScope, user, filter) {
			continue
		}
		if d.ExpiryDate != "" && (deadline == "" || d.ExpiryDate < deadline) {
			deadline = d.ExpiryDate
		}
		priority, rank := "high", 1
		if days, valid := dateDistance(deadline, today); valid && days < 0 {
			priority, rank = "urgent", 0
		}
		out = append(out, productivityItem{ID: d.ID, Entity: "document", Name: d.DocName, Reason: reason, Action: action, Owner: owner, Deadline: deadline, Priority: priority, Route: route, Rank: rank})
	}
	return out
}
