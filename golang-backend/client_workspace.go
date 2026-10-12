package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"
)

// Only explicit record links define a client workspace. Names, email addresses,
// and phone numbers are never used to combine people or infer permission.
type clientLinks map[string]map[string]bool

func (links clientLinks) add(entity, id string) {
	if id == "" {
		return
	}
	if links[entity] == nil {
		links[entity] = map[string]bool{}
	}
	links[entity][id] = true
}

func (links clientLinks) ids(entity string) []string {
	ids := []string{}
	for id := range links[entity] {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

func clientWorkspaceRecords(r *http.Request, entity string, fields map[string][]string) ([]map[string]any, bool, error) {
	predicate, args := repositoryVisibility(r, entity)
	conditions := []string{}
	keys := []string{}
	for field := range fields {
		keys = append(keys, field)
	}
	sort.Strings(keys)
	for _, field := range keys {
		ids := fields[field]
		if len(ids) == 0 {
			continue
		}
		column := "json_extract(record.data,'$." + field + "')"
		if field == "id" {
			column = "record.id"
		}
		conditions = append(conditions, column+" IN ("+strings.TrimSuffix(strings.Repeat("?,", len(ids)), ",")+")")
		for _, id := range ids {
			args = append(args, id)
		}
	}
	if len(conditions) == 0 {
		return []map[string]any{}, false, nil
	}
	rows, err := storeDB(r).Query("SELECT record.data FROM "+entity+" record WHERE "+predicate+" AND ("+strings.Join(conditions, " OR ")+") ORDER BY record.sort_order,record.id LIMIT 101", args...)
	if err != nil {
		return nil, false, err
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var raw string
		var record map[string]any
		if err := rows.Scan(&raw); err != nil {
			return nil, false, err
		}
		if err := json.Unmarshal([]byte(raw), &record); err != nil {
			return nil, false, err
		}
		out = append(out, record)
	}
	if err := rows.Err(); err != nil {
		return nil, false, err
	}
	more := len(out) > 100
	if more {
		out = out[:100]
	}
	return out, more, nil
}

func handleClientWorkspace(w http.ResponseWriter, r *http.Request) {
	entity, id := r.PathValue("entity"), r.PathValue("id")
	if entity != "contacts" && entity != "leads" && entity != "students" && entity != "cases" {
		writeError(w, 404, "Client workspace not found")
		return
	}
	tx, err := database.BeginTx(r.Context(), nil)
	if err != nil {
		writeError(w, 503, "Client workspace unavailable")
		return
	}
	defer tx.Rollback()
	r = r.WithContext(context.WithValue(r.Context(), requestTxKey, tx))
	anchor, err := sqlRecord(r, entity, id)
	if err != nil {
		writeError(w, 404, "Record not found")
		return
	}
	links := clientLinks{}
	links.add(entity, id)
	// Resolve each link through the same scoped read as a standalone detail page.
	addVisible := func(entity, id string) {
		if id != "" && !links[entity][id] {
			if _, err := sqlRecord(r, entity, id); err == nil {
				links.add(entity, id)
			}
		}
	}
	if entity == "leads" {
		addVisible("contacts", fmtString(anchor["convertedContactId"]))
		addVisible("students", fmtString(anchor["convertedStudentId"]))
		addVisible("cases", fmtString(anchor["convertedCaseId"]))
	}
	if entity == "cases" {
		addVisible("contacts", fmtString(anchor["clientId"]))
		addVisible("students", fmtString(anchor["studentId"]))
	}
	groups := map[string][]map[string]any{}
	more := map[string]bool{}
	load := func(entity string, filters map[string][]string) error {
		var e error
		groups[entity], more[entity], e = clientWorkspaceRecords(r, entity, filters)
		return e
	}
	loadCases := func() error {
		return load("cases", map[string][]string{"id": links.ids("cases"), "clientId": links.ids("contacts"), "studentId": links.ids("students")})
	}
	loadLeads := func() error {
		return load("leads", map[string][]string{"id": links.ids("leads"), "convertedContactId": links.ids("contacts"), "convertedStudentId": links.ids("students"), "convertedCaseId": links.ids("cases")})
	}
	collectCases := func() {
		for _, record := range groups["cases"] {
			links.add("cases", fmtString(record["id"]))
			addVisible("contacts", fmtString(record["clientId"]))
			addVisible("students", fmtString(record["studentId"]))
		}
	}
	// Discover conversion links even when an inquiry has no application yet.
	if err = loadCases(); err == nil {
		collectCases()
		err = loadLeads()
	}
	if err == nil {
		for _, record := range groups["leads"] {
			links.add("leads", fmtString(record["id"]))
			addVisible("contacts", fmtString(record["convertedContactId"]))
			addVisible("students", fmtString(record["convertedStudentId"]))
			addVisible("cases", fmtString(record["convertedCaseId"]))
		}
		err = loadCases()
		collectCases()
	}
	if err == nil {
		err = loadLeads()
	}
	for _, related := range []string{"contacts", "students"} {
		if err == nil {
			err = load(related, map[string][]string{"id": links.ids(related)})
		}
	}
	if err == nil {
		err = load("documents", map[string][]string{"caseId": links.ids("cases"), "leadId": links.ids("leads")})
	}
	if err == nil {
		err = load("invoices", map[string][]string{"clientId": links.ids("contacts"), "caseId": links.ids("cases")})
	}
	if err != nil {
		writeError(w, 503, "Could not load linked records. Retry shortly.")
		return
	}
	writeJSON(w, 200, map[string]any{"anchor": anchor, "entity": entity, "records": groups, "hasMore": more, "limit": 100, "label": recordLabel(mustJSON(anchor)), "scope": fmt.Sprintf("Accessible records linked to this %s", strings.TrimSuffix(entity, "s"))})
}
