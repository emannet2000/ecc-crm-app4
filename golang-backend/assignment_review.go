package main

import (
	"encoding/json"
	"net/http"
	"sort"
	"strings"
)

type AssignmentReview struct {
	Entity      string              `json:"entity"`
	ID          string              `json:"id"`
	Label       string              `json:"label"`
	Previous    string              `json:"previous"`
	SuggestedID string              `json:"suggestedId"`
	Editable    bool                `json:"editable"`
	Revision    string              `json:"revision"`
	Route       string              `json:"route"`
	Staff       []RequirementPerson `json:"staff"`
}

func handleAssignmentReview(w http.ResponseWriter, r *http.Request) {
	state, release := runtimeForReadCall(r)
	defer release()
	s := state.snapshot()
	people, err := requirementPeople(r)
	if err != nil {
		writeError(w, 503, "Could not load staff. Retry the assignment review.")
		return
	}
	items := []AssignmentReview{}
	add := func(entity, id, label, value string, scope RecordScope, record any) {
		value = strings.TrimSpace(value)
		if value == "" || strings.EqualFold(value, "Unassigned") || scope.OrgID != currentUser(r).OrgID || !canRead(scope, currentUser(r)) {
			return
		}
		options := []RequirementPerson{}
		matches := []string{}
		for _, u := range people {
			if u.Role == "viewer" || !canRead(scope, u) {
				continue
			}
			if u.ID == value {
				return
			}
			options = append(options, RequirementPerson{u.ID, u.Name})
			if strings.EqualFold(strings.TrimSpace(u.Name), value) {
				matches = append(matches, u.ID)
			}
		}
		suggested := ""
		if len(matches) == 1 {
			suggested = matches[0]
		}
		route := "/" + entity + "/" + id
		if entity == "tasks" {
			route = "/tasks"
		}
		items = append(items, AssignmentReview{entity, id, label, value, suggested, canWrite(scope, currentUser(r)), versionOf(recordMap(record)), route, options})
	}
	for _, l := range s.Leads {
		add("leads", l.ID, l.Name, l.AssignedTo, l.RecordScope, l)
	}
	for _, c := range s.Cases {
		add("cases", c.ID, c.CaseNumber, c.AssignedOfficer, c.RecordScope, c)
	}
	for _, d := range s.Deals {
		add("deals", d.ID, d.Title, d.Owner, d.RecordScope, d)
	}
	for _, t := range s.Tasks {
		add("tasks", t.ID, t.Title, t.Owner, t.RecordScope, t)
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].Entity != items[j].Entity {
			return items[i].Entity < items[j].Entity
		}
		return items[i].ID < items[j].ID
	})
	limit, offset := parseLimitOffset(r, 25, 0)
	writeJSON(w, 200, map[string]any{"items": slicePage(items, offset, limit), "total": len(items), "limit": limit, "offset": offset})
}
func handleAssignmentMapping(w http.ResponseWriter, r *http.Request) {
	var in struct {
		StaffID   string `json:"staffId"`
		Confirmed bool   `json:"confirmed"`
	}
	if !decodeRequest(w, r, &in) {
		return
	}
	if !in.Confirmed {
		writeError(w, 400, "Confirm that this account belongs to the historical assignee.")
		return
	}
	if r.Header.Get("If-Match") == "" {
		writeError(w, 428, "Refresh assignment review before saving.")
		return
	}
	state, release := runtimeForCall(r)
	defer release()
	state.mu.Lock()
	defer state.mu.Unlock()
	entity, id := r.PathValue("entity"), r.PathValue("id")
	record, err := sqlRecord(r, entity, id)
	if err != nil {
		writeError(w, 404, "Assignment record not found")
		return
	}
	if strings.Trim(r.Header.Get("If-Match"), "\"") != versionOf(record) {
		writeError(w, 409, "This assignment changed. Refresh review before mapping.")
		return
	}
	apply := func(scope RecordScope, value *string) bool {
		if !canWrite(scope, currentUser(r)) {
			writeError(w, 403, "You must be able to edit this record.")
			return false
		}
		u, ok := committedUser(r, "id", in.StaffID)
		if !ok || u.Disabled || u.Role == "viewer" || u.OrgID != scope.OrgID || !canRead(scope, u) {
			writeError(w, 400, "Choose an active staff account with access to this record.")
			return false
		}
		if current, exists := committedUser(r, "id", *value); exists && !current.Disabled && current.Role != "viewer" && canRead(scope, current) {
			writeError(w, 409, "This record already has an active account assignment. Use its editor or application handoff to change responsibility.")
			return false
		}
		*value = u.ID
		writeJSON(w, 200, map[string]any{"mapped": id, "staffId": u.ID, "staffName": u.Name})
		return true
	}
	switch entity {
	case "leads":
		for i := range state.leads {
			if state.leads[i].ID == id {
				apply(state.leads[i].RecordScope, &state.leads[i].AssignedTo)
				return
			}
		}
	case "cases":
		for i := range state.cases {
			if state.cases[i].ID == id {
				apply(state.cases[i].RecordScope, &state.cases[i].AssignedOfficer)
				return
			}
		}
	case "deals":
		for i := range state.deals {
			if state.deals[i].ID == id {
				apply(state.deals[i].RecordScope, &state.deals[i].Owner)
				return
			}
		}
	case "tasks":
		for i := range state.tasks {
			if state.tasks[i].ID == id {
				apply(state.tasks[i].RecordScope, &state.tasks[i].Owner)
				return
			}
		}
	}
	writeError(w, 404, "Assignment record not found")
}

func recordMap(record any) map[string]any {
	var out map[string]any
	json.Unmarshal([]byte(mustJSON(record)), &out)
	return out
}
