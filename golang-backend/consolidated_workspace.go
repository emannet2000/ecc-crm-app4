package main

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"sort"
	"strings"
	"time"
)

// Counts, parent permissions and pages must describe the same committed state.
func consolidatedReadSnapshot(w http.ResponseWriter, r *http.Request) (*http.Request, func(), bool) {
	tx, err := database.BeginTx(r.Context(), &sql.TxOptions{ReadOnly: true})
	if err != nil {
		writeError(w, 503, "Database unavailable; try Refresh again")
		return r, func() {}, false
	}
	return withDBTransaction(r, tx), func() { tx.Rollback() }, true
}

func registerConsolidatedRoutes(m *http.ServeMux) {
	registerEverydayRoutes(m)
	m.HandleFunc("GET /api/communication", authMiddleware(handleCommunicationWorkspace))
	m.HandleFunc("GET /api/workspace-calendar", authMiddleware(handleWorkspaceCalendar))
	m.HandleFunc("GET /api/referral-directory", authMiddleware(handleReferralDirectory))
}

func handleCommunicationWorkspace(w http.ResponseWriter, r *http.Request) {
	r, close, ok := consolidatedReadSnapshot(w, r)
	if !ok {
		return
	}
	defer close()
	u := currentUser(r)
	contactScope, contactArgs := scopeSQL(r, "client.data")
	caseScope, caseArgs := scopeSQL(r, "application.data")
	invoiceScope, invoiceArgs := scopeSQL(r, "invoice.data")
	// Unmatched messages are manager-only. Linked messages require access to
	// every linked record, including portal responses and invoice emails.
	conversation := `SELECT message.id,message.channel,message.direction,message.subject,message.body,message.status,
	message.created_at AS createdAt,message.contact_id AS contactId,message.case_id AS caseId,'' AS invoiceId,
	'conversation' AS source,'' AS lastError,'' AS recipient,'' AS queuedBy,
	COALESCE(json_extract(application.data,'$.caseNumber'),json_extract(client.data,'$.name'),'Unmatched message') AS linkedRecord,
	CASE WHEN message.case_id<>'' THEN '/cases/'||message.case_id WHEN message.contact_id<>'' THEN '/contacts/'||message.contact_id ELSE '' END AS recordRoute
	FROM conversations message LEFT JOIN contacts client ON client.id=message.contact_id LEFT JOIN cases application ON application.id=message.case_id
	WHERE message.org_id=? AND ((message.contact_id='' AND message.case_id='' AND ? IN ('admin','manager')) OR
	((message.contact_id='' OR (` + contactScope + `)) AND (message.case_id='' OR (` + caseScope + `)) AND (message.contact_id<>'' OR message.case_id<>'')))`
	args := []any{u.OrgID, u.Role}
	args = append(args, contactArgs...)
	args = append(args, caseArgs...)
	mail := `SELECT 'mail:'||message.id,'email','outbound',message.subject,message.body,message.status,
	message.created_at,message.contact_id,'',message.invoice_id,'mail',message.last_error,message.recipient,message.user_id,
	COALESCE(json_extract(invoice.data,'$.invoiceNumber'),json_extract(client.data,'$.name'),message.recipient),
	CASE WHEN message.invoice_id<>'' THEN '/invoices/'||message.invoice_id WHEN message.contact_id<>'' THEN '/contacts/'||message.contact_id ELSE '' END
	FROM mail_outbox message LEFT JOIN contacts client ON client.id=message.contact_id LEFT JOIN invoices invoice ON invoice.id=message.invoice_id
	WHERE message.org_id=? AND (message.user_id=? OR ? IN ('admin','manager')) AND message.kind NOT IN ('password_reset','invite')
	AND (message.contact_id='' OR (` + contactScope + `)) AND (message.invoice_id='' OR (` + invoiceScope + `))`
	args = append(args, u.OrgID, u.ID, u.Role)
	args = append(args, contactArgs...)
	args = append(args, invoiceArgs...)
	query := "SELECT messages.*, coalesce(json_extract(handling.data,'$.status'),CASE WHEN direction='inbound' THEN 'needs_reply' ELSE '' END) AS handlingStatus,coalesce(json_extract(handling.data,'$.assignedTo'),'') AS assignedTo,coalesce(json_extract(handling.data,'$.responseDue'),'') AS responseDue,coalesce(handling.data,'{}') AS handlingData FROM (" + conversation + " UNION ALL " + mail + ") messages LEFT JOIN workspace_entries handling ON handling.org_id=? AND handling.category='message_handling' AND handling.record_id=messages.id WHERE 1=1"
	args = append(args, u.OrgID)
	if state := r.URL.Query().Get("handling"); state != "" {
		query += " AND handlingStatus=?"
		args = append(args, state)
	}
	if r.URL.Query().Get("assigned") == "me" {
		query += " AND assignedTo=?"
		args = append(args, u.ID)
	}
	if student := r.URL.Query().Get("studentId"); student != "" {
		if _, err := sqlRecord(r, "students", student); err != nil {
			writeError(w, 404, "Student not accessible")
			return
		}
		query += " AND caseId IN (SELECT id FROM cases WHERE json_extract(data,'$.studentId')=?)"
		args = append(args, student)
	}
	for _, key := range []string{"contactId", "caseId", "invoiceId", "channel"} {
		if value := r.URL.Query().Get(key); value != "" {
			query += " AND " + key + "=?"
			args = append(args, value)
		}
	}
	if r.URL.Query().Get("unmatched") == "true" {
		query += " AND source='conversation' AND contactId='' AND caseId=''"
	}
	if q := strings.TrimSpace(r.URL.Query().Get("q")); q != "" {
		query += " AND instr(lower(subject||' '||body||' '||linkedRecord||' '||recipient),lower(?))>0"
		args = append(args, q)
	}
	var total int
	if err := storeDB(r).QueryRow("SELECT count(*) FROM ("+query+")", args...).Scan(&total); err != nil {
		writeError(w, 500, "Could not count communication")
		return
	}
	limit, offset := parseLimitOffset(r, 25, 0)
	rows, err := queryObjects(r, query+" ORDER BY createdAt DESC,id DESC LIMIT ? OFFSET ?", append(args, limit, offset)...)
	if err != nil {
		writeError(w, 500, "Could not load communication")
		return
	}
	for _, row := range rows {
		var handling map[string]any
		json.Unmarshal([]byte(fmtString(row["handlingData"])), &handling)
		row["handlingRevision"] = versionOf(handling)
		delete(row, "handlingData")
		status := fmtString(row["status"])
		row["canRetry"] = u.Role != "viewer" && row["source"] == "mail" && (row["queuedBy"] == u.ID || u.Role == "admin") && (status == "failed" || status == "waiting_configuration")
		delete(row, "queuedBy")
		row["deliveryExplanation"] = deliveryExplanation(fmtString(row["channel"]), fmtString(row["status"]))
		row["unmatched"] = row["source"] == "conversation" && row["contactId"] == "" && row["caseId"] == ""
	}
	writeJSON(w, 200, map[string]any{"messages": rows, "total": total, "limit": limit, "offset": offset})
}

func handleWorkspaceCalendar(w http.ResponseWriter, r *http.Request) {
	r, close, ok := consolidatedReadSnapshot(w, r)
	if !ok {
		return
	}
	defer close()
	from, to := r.URL.Query().Get("from"), r.URL.Query().Get("to")
	start, startErr := time.Parse("2006-01-02", from)
	end, endErr := time.Parse("2006-01-02", to)
	if startErr != nil || endErr != nil || end.Before(start) || end.Sub(start) > 366*24*time.Hour {
		writeError(w, 400, "Choose a date range of up to one year")
		return
	}
	terms, args := []string{}, []any{}
	for _, item := range []struct{ entity, kind, title, date, status string }{
		{"tasks", "task", "title", "dueDate", "status"},
		{"cases", "application", "caseNumber", "nextDeadline", "currentStage"},
		{"leads", "followup", "name", "followUpDate", "status"},
		{"documents", "document", "docName", "dueDate", "status"},
		{"documents", "expiry", "docName", "expiryDate", "status"},
		{"invoices", "invoice", "invoiceNumber", "dueDate", "paymentMilestone"},
		{"students", "student", "name", "nextDeadline", "applicationStage"},
		{"students", "travel", "name", "travelDate", "visaStatus"},
	} {
		scope, scopeArgs := repositoryVisibility(r, item.entity)
		date := "coalesce(json_extract(record.data,'$." + item.date + "'),'')"
		predicate := scope
		switch item.entity {
		case "tasks":
			predicate += " AND lower(coalesce(json_extract(record.data,'$.status'),'')) NOT IN ('done','completed')"
		case "leads":
			predicate += " AND lower(coalesce(json_extract(record.data,'$.status'),'')) NOT IN ('converted','closed/lost','closed','lost')"
		case "cases":
			predicate += " AND lower(coalesce(json_extract(record.data,'$.currentStage'),'')) NOT IN ('closed','rejected','refused','withdrawn','approved')"
		case "invoices":
			predicate += " AND coalesce(json_extract(record.data,'$.balance'),0)>0"
		}
		route := "'/'||'" + item.entity + "'||'/'||record.id"
		if item.entity == "tasks" {
			route = "'/tasks'"
		}
		if item.entity == "documents" {
			route = "CASE WHEN coalesce(json_extract(record.data,'$.caseId'),'')<>'' THEN '/cases/'||json_extract(record.data,'$.caseId') ELSE '/leads/'||json_extract(record.data,'$.leadId') END||'#record-tools?tab=documents'"
		}
		terms = append(terms, "SELECT record.id,'"+item.kind+"' AS kind,coalesce(json_extract(record.data,'$."+item.title+"'),'') AS title,"+date+" AS date,coalesce(json_extract(record.data,'$."+item.status+"'),'') AS status,"+route+" AS route,'' AS occurredAt FROM "+item.entity+" record WHERE "+predicate+" AND "+date+">=? AND "+date+"<=?")
		args = append(args, scopeArgs...)
		args = append(args, from, to)
	}
	rows, err := queryObjects(r, strings.Join(terms, " UNION ALL "), args...)
	if err != nil {
		writeError(w, 500, "Could not load calendar")
		return
	}
	var zone string
	storeDB(r).QueryRow("SELECT timezone FROM organizations WHERE id=?", currentUser(r).OrgID).Scan(&zone)
	location, err := time.LoadLocation(zone)
	if err != nil {
		location = time.UTC
		zone = "UTC"
	}
	contactScope, contactArgs := scopeSQL(r, "client.data")
	meetings, err := queryObjects(r, "SELECT meeting.id,meeting.data FROM workspace_entries meeting JOIN contacts client ON client.id=json_extract(meeting.data,'$.contactId') WHERE meeting.org_id=? AND meeting.category='calendar_event' AND "+contactScope, append([]any{currentUser(r).OrgID}, contactArgs...)...)
	if err != nil {
		writeError(w, 500, "Could not load consultations")
		return
	}
	for _, meeting := range meetings {
		var data map[string]any
		if json.Unmarshal([]byte(fmtString(meeting["data"])), &data) != nil {
			continue
		}
		at, err := time.Parse(time.RFC3339, fmtString(data["occurredAt"]))
		if err != nil {
			continue
		}
		date := at.In(location).Format("2006-01-02")
		if date < from || date > to {
			continue
		}
		status := fmtString(data["status"])
		if status == "" {
			status = "scheduled"
		}
		rows = append(rows, map[string]any{"event": data, "revision": versionOf(data), "id": meeting["id"], "kind": "consultation", "title": data["title"], "date": date, "occurredAt": at.UTC().Format(time.RFC3339), "status": status, "route": "/contacts/" + fmtString(data["contactId"])})
	}
	if kind := r.URL.Query().Get("kind"); kind != "" {
		filtered := []map[string]any{}
		for _, row := range rows {
			if row["kind"] == kind {
				filtered = append(filtered, row)
			}
		}
		rows = filtered
	}
	sort.Slice(rows, func(i, j int) bool {
		key := func(row map[string]any) string {
			return fmtString(row["date"]) + fmtString(row["occurredAt"]) + fmtString(row["kind"]) + fmtString(row["id"])
		}
		return key(rows[i]) < key(rows[j])
	})
	limit, offset := parseLimitOffset(r, 25, 0)
	writeJSON(w, 200, map[string]any{"events": slicePage(rows, offset, limit), "total": len(rows), "timezone": zone, "today": organizationToday(r)})
}

func handleReferralDirectory(w http.ResponseWriter, r *http.Request) {
	r, close, ok := consolidatedReadSnapshot(w, r)
	if !ok {
		return
	}
	defer close()
	terms, args := []string{}, []any{}
	for _, entity := range []string{"agents", "partners"} {
		if selected := r.URL.Query().Get("entity"); selected != "" && selected != entity {
			continue
		}
		scope, scopeArgs := repositoryVisibility(r, entity)
		name := "name"
		if entity == "partners" {
			name = "legalCompanyName"
		}
		terms = append(terms, "SELECT record.id,'"+entity+"' AS entity,json_extract(record.data,'$."+name+"') AS name,record.data FROM "+entity+" record WHERE "+scope)
		args = append(args, scopeArgs...)
	}
	if len(terms) == 0 {
		writeError(w, 400, "Choose agents or partners")
		return
	}
	query := "SELECT * FROM (" + strings.Join(terms, " UNION ALL ") + ")"
	if q := strings.TrimSpace(r.URL.Query().Get("q")); q != "" {
		query += " WHERE instr(lower(coalesce(name,'')||' '||coalesce(json_extract(data,'$.contactPerson'),'')||' '||coalesce(json_extract(data,'$.contactEmail'),'')||' '||coalesce(json_extract(data,'$.agentCode'),'')),lower(?))>0"
		args = append(args, q)
	}
	var total int
	if err := storeDB(r).QueryRow("SELECT count(*) FROM ("+query+")", args...).Scan(&total); err != nil {
		writeError(w, 500, "Could not count directory")
		return
	}
	limit, offset := parseLimitOffset(r, 25, 0)
	rows, err := queryObjects(r, query+" ORDER BY name COLLATE NOCASE,entity,id LIMIT ? OFFSET ?", append(args, limit, offset)...)
	if err != nil {
		writeError(w, 500, "Could not load directory")
		return
	}
	for _, row := range rows {
		var data map[string]any
		json.Unmarshal([]byte(fmtString(row["data"])), &data)
		row["type"] = "Agent"
		if row["entity"] == "partners" {
			row["type"] = "Partner"
			if kind := fmtString(data["type"]); kind != "" {
				row["type"] = "Partner · " + kind
			}
		}
		row["status"], row["commissionRate"], row["agreementStart"], row["agreementExpiry"] = data["contractStatus"], data["commissionRatePercent"], data["agreementStart"], data["agreementExpiry"]
		row["route"] = "/" + fmtString(row["entity"]) + "/" + fmtString(row["id"])
		delete(row, "data")
	}
	writeJSON(w, 200, map[string]any{"items": rows, "total": total})
}
