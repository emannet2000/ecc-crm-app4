package main

import (
	"net/http"
	"strings"
)

func handleStaffOptions(w http.ResponseWriter, r *http.Request) {
	scope := newRecordScope(r)
	entity, id := r.URL.Query().Get("entity"), r.URL.Query().Get("id")
	if id != "" {
		record, err := sqlRecord(r, entity, id)
		if err != nil {
			writeError(w, 404, "Record not found")
			return
		}
		scope = RecordScope{OrgID: fmtString(record["orgId"]), OwnerID: fmtString(record["ownerId"]), TeamID: fmtString(record["teamId"]), Visibility: fmtString(record["visibility"])}
		if !canRead(scope, currentUser(r)) {
			writeError(w, 404, "Record not found")
			return
		}
	}
	people, err := requirementPeople(r)
	if err != nil {
		writeError(w, 500, "Could not load staff. Retry without changing your selection.")
		return
	}
	options := []RequirementPerson{}
	for _, u := range people {
		if u.Role != "viewer" && canRead(scope, u) {
			options = append(options, RequirementPerson{ID: u.ID, Name: u.Name})
		}
	}
	writeJSON(w, 200, map[string]any{"staff": options})
}

func normalizeStaffAssignment(w http.ResponseWriter, r *http.Request, value, previous, field string, scope RecordScope) (string, bool) {
	value = strings.TrimSpace(value)
	if value == "" || strings.EqualFold(value, "Unassigned") {
		return "", true
	}
	// Historical names remain editable without silently assigning a different person.
	if value == previous {
		return value, true
	}
	people, err := requirementPeople(r)
	if err != nil {
		writeError(w, 503, "Could not validate staff. Your changes have not been saved; retry.")
		return "", false
	}
	matches := []User{}
	for _, u := range people {
		if u.Role != "viewer" && canRead(scope, u) && (value == u.ID || strings.EqualFold(value, u.Name)) {
			matches = append(matches, u)
		}
	}
	if len(matches) != 1 {
		writeFieldErrors(w, map[string]string{field: "Choose an active staff account with access to this record."})
		return "", false
	}
	return matches[0].ID, true
}

func handleStaffDirectory(w http.ResponseWriter, r *http.Request) {
	state, release := runtimeForReadCall(r)
	defer release()
	state.mu.RLock()
	defer state.mu.RUnlock()
	people := []map[string]any{}
	for _, u := range state.users {
		if u.OrgID == currentUser(r).OrgID {
			people = append(people, map[string]any{"id": u.ID, "name": u.Name, "disabled": u.Disabled})
		}
	}
	writeJSON(w, 200, map[string]any{"staff": people})
}
