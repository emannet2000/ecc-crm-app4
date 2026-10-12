package main

import (
	"fmt"
	"sort"
	"strings"
)

// Overview categories include records outside the due-work queue, while retaining
// the same workspace, visibility and assignment boundaries.
func dailyOverview(user User, s diskStore, filter string) map[string][]productivityItem {
	out := map[string][]productivityItem{"deals": {}, "pipeline": {}, "leads": {}, "students": {}, "cases": {}}
	allowed := func(scope RecordScope, owner string) bool {
		return scope.OrgID == user.OrgID && canRead(scope, user) && workAssigned(owner, scope, user, filter)
	}
	details := func(values ...string) []string {
		result := []string{}
		for i := 0; i+1 < len(values); i += 2 {
			if strings.TrimSpace(values[i+1]) != "" {
				result = append(result, values[i]+": "+values[i+1])
			}
		}
		return result
	}
	for _, d := range s.Deals {
		if !allowed(d.RecordScope, d.Owner) {
			continue
		}
		currency := d.Currency
		if currency == "" {
			currency = "USD"
		}
		item := productivityItem{ID: d.ID, Entity: "deal", Name: d.Title, Reason: "Stage: " + d.Stage, Action: actionOrDefault(d.NextAction, "Review opportunity and linked activity"), Owner: d.Owner, Deadline: actionOrDefault(d.NextDeadline, d.CloseDate), Priority: "medium", Route: "/deals/" + d.ID, Details: details("Created", d.CreatedAt, "Value", fmt.Sprintf("%s %.2f", currency, d.Value), "Contact", d.ContactName)}
		out["deals"] = append(out["deals"], item)
		if !strings.EqualFold(d.Stage, "Won") && !strings.EqualFold(d.Stage, "Lost") {
			out["pipeline"] = append(out["pipeline"], item)
		}
	}
	for _, l := range s.Leads {
		if !allowed(l.RecordScope, l.AssignedTo) {
			continue
		}
		out["leads"] = append(out["leads"], productivityItem{ID: l.ID, Entity: "lead", Name: l.Name, Reason: "Status: " + l.Status, Action: leadNextAction(l.NextAction), Owner: l.AssignedTo, Deadline: l.FollowUpDate, Priority: "medium", Route: "/leads/" + l.ID, Details: details("Created", l.CreatedAt, "Email", l.Email, "Phone", l.Phone, "Source", l.Source, "Referrer", l.Referrer, "Interested service", l.InterestedService)})
	}
	for _, st := range s.Students {
		if !allowed(st.RecordScope, st.OwnerID) {
			continue
		}
		owner := st.OwnerID
		if owner == user.ID {
			owner = user.Name
		}
		out["students"] = append(out["students"], productivityItem{ID: st.ID, Entity: "student", Name: st.Name, Reason: "Application stage: " + st.ApplicationStage, Action: actionOrDefault(st.NextAction, "Review student journey and linked applications"), Owner: owner, Deadline: actionOrDefault(st.NextDeadline, st.TravelDate), Priority: "medium", Route: "/students/" + st.ID, Details: details("Created", st.CreatedAt, "Student code", st.StudentCode, "School", st.SchoolName, "Program", st.Program, "Intake", st.Intake, "Acceptance", st.AcceptanceStatus, "Visa", st.VisaStatus)})
	}
	for _, c := range s.Cases {
		if !allowed(c.RecordScope, c.AssignedOfficer) {
			continue
		}
		action := c.NextAction
		if action == "" {
			action = "Review application and choose the next action"
		}
		out["cases"] = append(out["cases"], productivityItem{ID: c.ID, Entity: "case", Name: c.CaseNumber, Reason: "Stage: " + c.CurrentStage, Action: action, Owner: c.AssignedOfficer, Deadline: c.NextDeadline, Priority: "medium", Route: "/cases/" + c.ID, Details: details("Created", c.CreatedAt, "Client", c.ClientName, "Service", c.ServiceCategory, "Destination", c.DestinationCountry, "School / employer", c.SchoolOrEmployer, "Priority", c.Priority)})
	}
	for _, records := range out {
		sort.Slice(records, func(i, j int) bool {
			if records[i].Name != records[j].Name {
				return records[i].Name < records[j].Name
			}
			return records[i].ID < records[j].ID
		})
	}
	return out
}
