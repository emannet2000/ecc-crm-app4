package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"
)

func registerProductivityRoutes(m *http.ServeMux) {
	registerReconciliationRoutes(m)
	registerFollowupTemplateRoutes(m)
	m.HandleFunc("GET /api/operations/attention", authMiddleware(handleOperationalAttention))
	m.HandleFunc("GET /api/leads/{id}/conversion-preview", authMiddleware(handleLeadConversionPreview))
	m.HandleFunc("GET /api/work-plans/{entity}/{id}", authMiddleware(handleWorkPlan))
	m.HandleFunc("PUT /api/work-plans/{entity}/{id}", authMiddleware(dailyOrigin(handleSaveWorkPlan)))
	m.HandleFunc("GET /api/contacts/{id}/history", authMiddleware(handleClientHistory))
	m.HandleFunc("GET /api/client-history/{entity}/{id}", authMiddleware(handleClientHistory))
	m.HandleFunc("GET /api/workspace-priorities", authMiddleware(handleWorkspacePriorities))
}
func workPlanEntity(entity string) bool {
	return entity == "leads" || entity == "deals" || entity == "cases" || entity == "students"
}
func recordScopeMap(record map[string]any) RecordScope {
	return RecordScope{OrgID: fmtString(record["orgId"]), OwnerID: fmtString(record["ownerId"]), TeamID: fmtString(record["teamId"]), Visibility: fmtString(record["visibility"])}
}
func handleWorkPlan(w http.ResponseWriter, r *http.Request) {
	entity := r.PathValue("entity")
	if !workPlanEntity(entity) {
		writeError(w, 404, "Work plan not found")
		return
	}
	record, err := sqlRecord(r, entity, r.PathValue("id"))
	if err != nil {
		writeError(w, 404, "Record not found")
		return
	}
	deadline := fmtString(record["nextDeadline"])
	responsible := fmtString(record["ownerId"])
	for _, key := range []string{"assignedTo", "assignedOfficer", "owner"} {
		if value := fmtString(record[key]); value != "" && !strings.EqualFold(value, "Unassigned") {
			responsible = value
			break
		}
	}
	if entity == "cases" {
		responsible = fmtString(record["assignedOfficer"])
	}
	if entity == "leads" {
		responsible = fmtString(record["assignedTo"])
		deadline = fmtString(record["followUpDate"])
	}
	writeJSON(w, 200, map[string]any{"nextAction": record["nextAction"], "deadline": deadline, "responsible": responsible, "revision": versionOf(record), "editable": canWrite(recordScopeMap(record), currentUser(r))})
}
func handleSaveWorkPlan(w http.ResponseWriter, r *http.Request) {
	var in struct {
		NextAction string `json:"nextAction"`
		Deadline   string `json:"deadline"`
	}
	if !decodeRequest(w, r, &in) {
		return
	}
	entity, id := r.PathValue("entity"), r.PathValue("id")
	if !workPlanEntity(entity) {
		writeError(w, 404, "Work plan not found")
		return
	}
	in.NextAction = strings.TrimSpace(in.NextAction)
	if in.NextAction == "" || len(in.NextAction) > 1000 {
		writeError(w, 400, "Enter a next action of up to 1,000 characters")
		return
	}
	if in.Deadline == "" {
		writeError(w, 400, "Choose a deadline for this next action")
		return
	}
	if in.Deadline != "" {
		if _, err := time.Parse("2006-01-02", in.Deadline); err != nil {
			writeError(w, 400, "Choose a valid deadline")
			return
		}
	}
	if r.Header.Get("If-Match") == "" {
		writeError(w, 428, "Refresh the plan before saving")
		return
	}
	state, release := runtimeForCall(r)
	defer release()
	state.mu.Lock()
	defer state.mu.Unlock()
	record, err := sqlRecord(r, entity, id)
	if err != nil {
		writeError(w, 404, "Record not found")
		return
	}
	if !canWrite(recordScopeMap(record), currentUser(r)) {
		writeError(w, 403, "You cannot edit this record")
		return
	}
	if strings.Trim(r.Header.Get("If-Match"), "\"") != versionOf(record) {
		writeError(w, 409, "This record changed. Your draft is kept; refresh and compare before saving.")
		return
	}
	found := false
	switch entity {
	case "leads":
		for i := range state.leads {
			if state.leads[i].ID == id {
				state.leads[i].NextAction = in.NextAction
				state.leads[i].FollowUpDate = in.Deadline
				found = true
				break
			}
		}
	case "deals":
		for i := range state.deals {
			if state.deals[i].ID == id {
				state.deals[i].NextAction = in.NextAction
				state.deals[i].NextDeadline = in.Deadline
				found = true
				break
			}
		}
	case "cases":
		for i := range state.cases {
			if state.cases[i].ID == id {
				state.cases[i].NextAction = in.NextAction
				state.cases[i].NextDeadline = in.Deadline
				found = true
				break
			}
		}
	case "students":
		for i := range state.students {
			if state.students[i].ID == id {
				state.students[i].NextAction = in.NextAction
				state.students[i].NextDeadline = in.Deadline
				found = true
				break
			}
		}
	}
	if !found {
		writeError(w, 404, "Record not found")
		return
	}
	writeJSON(w, 200, map[string]any{"saved": true})
}

type ClientHistoryItem struct {
	ID     string `json:"id"`
	Kind   string `json:"kind"`
	Title  string `json:"title"`
	Detail string `json:"detail"`
	Date   string `json:"date"`
	Route  string `json:"route"`
}

func handleClientHistory(w http.ResponseWriter, r *http.Request) {

	for _, key := range []string{"from", "to"} {
		if value := r.URL.Query().Get(key); value != "" {
			if _, err := time.Parse("2006-01-02", value); err != nil {
				writeError(w, 400, "Choose valid timeline dates")
				return
			}
		}
	}
	if r.URL.Query().Get("from") != "" && r.URL.Query().Get("to") != "" && r.URL.Query().Get("from") > r.URL.Query().Get("to") {
		writeError(w, 400, "Timeline date range is reversed")
		return
	}
	r, s, close, ok := dailySnapshot(w, r)
	if !ok {
		return
	}
	defer close()
	entity := r.PathValue("entity")
	if entity == "" {
		entity = "contacts"
	}
	rootID := r.PathValue("id")
	choices := []map[string]string{}
	allowed := map[string]bool{}
	foundRoot := false
	switch entity {
	case "contacts":
		for _, c := range s.Contacts {
			if c.ID == rootID {
				allowed[c.ID] = true
				foundRoot = true
			}
		}
	case "cases":
		for _, c := range s.Cases {
			if c.ID == rootID {
				allowed[c.ClientID] = true
				foundRoot = true
			}
		}
	case "students":
		for _, student := range s.Students {
			if student.ID == rootID {
				foundRoot = true
			}
		}
		if foundRoot {
			for _, c := range s.Cases {
				if c.StudentID == rootID {
					allowed[c.ClientID] = true
				}
			}
			for _, l := range s.Leads {
				if l.ConvertedStudentID == rootID {
					allowed[l.ConvertedContactID] = true
				}
			}
		}
	default:
		writeError(w, 404, "Client timeline not found")
		return
	}
	if !foundRoot {
		writeError(w, 404, "Record not found")
		return
	}
	for _, c := range s.Contacts {
		if allowed[c.ID] {
			choices = append(choices, map[string]string{"id": c.ID, "name": c.Name})
		}
	}
	id := r.URL.Query().Get("contactId")
	if id != "" {
		found := false
		for _, c := range choices {
			found = found || c["id"] == id
		}
		if !found {
			writeError(w, 404, "Linked client not found")
			return
		}
	}
	if id == "" && len(choices) == 1 {
		id = choices[0]["id"]
	}
	if id == "" {
		writeJSON(w, 200, map[string]any{"clients": choices, "items": []ClientHistoryItem{}, "total": 0, "message": "Choose a linked client to view their timeline"})
		return
	}
	items := []ClientHistoryItem{}
	caseIDs := map[string]bool{}
	leadIDs := map[string]bool{}
	studentIDs := map[string]bool{}
	invoiceIDs := map[string]bool{}
	invoiceCurrencies := map[string]string{}
	dealIDs := map[string]bool{}
	documentIDs := map[string]Document{}
	add := func(id, kind, title, detail, date, route string) {
		items = append(items, ClientHistoryItem{id, kind, title, detail, date, route})
	}
	for _, l := range s.Leads {
		if l.ConvertedContactID == id {
			leadIDs[l.ID] = true
			add(l.ID, "inquiry", l.Name, l.Status, l.CreatedAt, "/leads/"+l.ID)
		}
	}
	for _, c := range s.Cases {
		if c.ClientID == id {
			caseIDs[c.ID] = true
			if c.StudentID != "" {
				studentIDs[c.StudentID] = true
			}
			add(c.ID, "application", c.CaseNumber, c.CurrentStage+" · "+c.NextAction, c.CreatedAt, "/cases/"+c.ID)
		}
	}
	for _, l := range s.Leads {
		if leadIDs[l.ID] && l.ConvertedStudentID != "" {
			studentIDs[l.ConvertedStudentID] = true
		}
	}
	for _, st := range s.Students {
		if studentIDs[st.ID] {
			add(st.ID, "student", st.Name, st.ApplicationStage, st.CreatedAt, "/students/"+st.ID)
		}
	}
	visibleStudents := map[string]bool{}
	for _, student := range s.Students {
		if studentIDs[student.ID] {
			visibleStudents[student.ID] = true
		}
	}
	studentIDs = visibleStudents
	for _, d := range s.Deals {
		if d.ContactID == id {
			dealIDs[d.ID] = true
			add(d.ID, "deal", d.Title, d.Stage, d.CreatedAt, "/deals/"+d.ID)
		}
	}
	for _, d := range s.Documents {
		if caseIDs[d.CaseID] || leadIDs[d.LeadID] {
			documentIDs[d.ID] = d
			route := "/cases/" + d.CaseID + "#record-tools?tab=documents"
			if d.CaseID == "" {
				route = "/leads/" + d.LeadID + "#record-tools?tab=documents"
			}
			add(d.ID, "document", d.DocName, d.Status, d.DateReceived, route)
			if d.ReviewedAt != "" {
				add(d.ID+":review", "document", d.DocName+" reviewed", d.ReviewedBy, d.ReviewedAt, route)
			}
		}
	}
	for _, inv := range s.Invoices {
		if inv.ClientID == id || caseIDs[inv.CaseID] {
			invoiceIDs[inv.ID] = true
			invoiceCurrencies[inv.ID] = currencyOf(inv.Currency)
			add(inv.ID, "invoice", inv.InvoiceNumber, currencyOf(inv.Currency)+" · Balance "+formatMoney(inv.Balance), inv.CreatedAt, "/invoices/"+inv.ID)
		}
	}
	for _, p := range s.Payments {
		if invoiceIDs[p.InvoiceID] {
			currency := p.Currency
			if currency == "" {
				currency = invoiceCurrencies[p.InvoiceID]
			}
			add(p.ID, "payment", p.Reference, currencyOf(currency)+" "+formatMoney(p.Amount)+" · "+p.Status, p.PaidOn, "/invoices/"+p.InvoiceID)
		}
	}
	for _, a := range s.Activities {
		if a.ContactID == id || dealIDs[a.DealID] {
			route := "/contacts/" + id + "#record-tools?tab=history"
			if a.DealID != "" {
				route = "/deals/" + a.DealID
			}
			add(a.ID, "activity", a.Title, a.Body, a.OccurredAt, route)
		}
	}
	for _, t := range s.Tasks {
		if t.ContactID == id {
			add(t.ID, "task", t.Title, t.Status+" · Due "+t.DueDate, t.CreatedAt, "/tasks")
		}
	}
	rows, err := queryObjects(r, "SELECT id,case_id AS caseId,contact_id AS contactId,subject,channel,body,created_at AS createdAt FROM conversations WHERE org_id=? AND (contact_id=? OR case_id IN (SELECT id FROM cases WHERE json_extract(data,'$.clientId')=?))", currentUser(r).OrgID, id, id)
	if err != nil {
		writeError(w, 503, "Could not load client conversations")
		return
	}
	for _, row := range rows {
		caseID := fmtString(row["caseId"])
		if caseID != "" && !caseIDs[caseID] {
			continue
		}
		add(fmtString(row["id"]), "conversation", fmtString(row["subject"]), fmtString(row["channel"])+" · "+fmtString(row["body"]), fmtString(row["createdAt"]), "/contacts/"+id+"#record-tools?tab=conversations")
	}

	timeline, err := queryObjects(r, "SELECT id,record_id AS recordId,data FROM workspace_entries WHERE org_id=? AND category='lead_timeline'", currentUser(r).OrgID)
	if err != nil {
		writeError(w, 503, "Could not load inquiry history")
		return
	}
	for _, row := range timeline {
		leadID := fmtString(row["recordId"])
		if !leadIDs[leadID] {
			continue
		}
		var event LeadTimelineEvent
		if json.Unmarshal([]byte(fmtString(row["data"])), &event) != nil {
			continue
		}
		add(fmtString(row["id"]), "inquiry", event.Title, event.Body, event.At, "/leads/"+leadID)
	}
	versions, err := queryObjects(r, "SELECT id,document_id AS documentId,filename,uploaded_at AS uploadedAt,version FROM file_versions WHERE org_id=?", currentUser(r).OrgID)
	if err != nil {
		writeError(w, 503, "Could not load document history")
		return
	}
	for _, v := range versions {
		d, ok := documentIDs[fmtString(v["documentId"])]
		if !ok {
			continue
		}
		route := "/cases/" + d.CaseID + "#record-tools?tab=documents"
		if d.CaseID == "" {
			route = "/leads/" + d.LeadID + "#record-tools?tab=documents"
		}
		add(fmtString(v["id"]), "document", d.DocName+" uploaded", fmtString(v["filename"]), fmtString(v["uploadedAt"]), route)
	}
	selectedClient := id
	visible := func(entity, id string) bool {
		switch entity {
		case "contacts":
			return id == selectedClient
		case "cases":
			return caseIDs[id]
		case "leads":
			return leadIDs[id]
		case "students":
			return studentIDs[id]
		case "invoices":
			return invoiceIDs[id]
		case "deals":
			return dealIDs[id]
		case "documents":
			_, ok := documentIDs[id]
			return ok
		case "payments":
			for _, p := range s.Payments {
				if p.ID == id && invoiceIDs[p.InvoiceID] {
					return true
				}
			}
		}
		return false
	}
	audit, err := queryObjects(r, "SELECT id,entity,record_id AS recordId,actor,action,label,occurred_at AS occurredAt FROM audit_events WHERE org_id=?", currentUser(r).OrgID)
	if err != nil {
		writeError(w, 503, "Could not load record history")
		return
	}
	for _, row := range audit {
		entity, recordID := fmtString(row["entity"]), fmtString(row["recordId"])
		if !visible(entity, recordID) {
			continue
		}
		route := "/" + entity + "/" + recordID
		if entity == "documents" {
			d := documentIDs[recordID]
			route = "/cases/" + d.CaseID + "#record-tools?tab=documents"
			if d.CaseID == "" {
				route = "/leads/" + d.LeadID + "#record-tools?tab=documents"
			}
		}
		if entity == "payments" {
			for _, p := range s.Payments {
				if p.ID == recordID {
					route = "/invoices/" + p.InvoiceID
				}
			}
		}
		add(fmt.Sprintf("audit:%v", row["id"]), "change", fmtString(row["label"]), fmtString(row["actor"])+" · "+fmtString(row["action"]), fmtString(row["occurredAt"]), route)
	}
	events, err := queryObjects(r, "SELECT id,case_id AS caseId,stage,entered_at AS enteredAt FROM case_stage_events WHERE org_id=?", currentUser(r).OrgID)
	if err != nil {
		writeError(w, 503, "Could not load stage history")
		return
	}
	for _, event := range events {
		caseID := fmtString(event["caseId"])
		if caseIDs[caseID] {
			add("stage:"+fmtString(event["id"]), "stage", "Application stage", fmtString(event["stage"]), fmtString(event["enteredAt"]), "/cases/"+caseID)
		}
	}
	entries, err := queryObjects(r, "SELECT id,category,record_id AS recordId,data,created_at AS createdAt,updated_at AS updatedAt FROM workspace_entries WHERE org_id=? AND category IN ('application_handoff','application_transition','application_gate_approval','refund','credit_note')", currentUser(r).OrgID)
	if err != nil {
		writeError(w, 503, "Could not load application and finance events")
		return
	}
	for _, entry := range entries {
		recordID, category := fmtString(entry["recordId"]), fmtString(entry["category"])
		var detail map[string]any
		if json.Unmarshal([]byte(fmtString(entry["data"])), &detail) != nil {
			continue
		}
		kind, title, route := "", "", ""
		switch category {
		case "application_handoff":
			if caseIDs[recordID] {
				kind = "handoff"
				title = "Application handoff"
				route = "/cases/" + recordID + "#handoff"
			}
		case "application_transition":
			if caseIDs[recordID] {
				kind = "stage"
				title = "Application transition"
				route = "/cases/" + recordID
			}
		case "application_gate_approval":
			if caseIDs[recordID] {
				kind = "approval"
				title = "Manager stage approval"
				route = "/cases/" + recordID
			}
		case "refund", "credit_note":
			if invoiceIDs[recordID] {
				kind = "payment"
				title = category
				route = "/invoices/" + recordID
			}
		}
		if kind == "" {
			continue
		}
		parts := []string{}
		for _, key := range []string{"from", "stage", "status", "actor", "fromName", "toName", "reason", "reference", "currency"} {
			if value := fmtString(detail[key]); value != "" {
				parts = append(parts, value)
			}
		}
		if detail["override"] == true {
			parts = append(parts, "Manager override")
		}
		if amount, ok := detail["amount"].(float64); ok {
			parts = append(parts, formatMoney(amount))
		}
		add("entry:"+fmtString(entry["id"]), kind, title, strings.Join(parts, " · "), fmtString(entry["updatedAt"]), route)
	}
	mail, err := queryObjects(r, "SELECT id,contact_id AS contactId,invoice_id AS invoiceId,subject,status,created_at AS createdAt,sent_at AS sentAt FROM mail_outbox WHERE org_id=? AND (contact_id=? OR invoice_id<>'')", currentUser(r).OrgID, id)
	if err != nil {
		writeError(w, 503, "Could not load email history")
		return
	}
	for _, message := range mail {
		invID := fmtString(message["invoiceId"])
		if invID != "" && !invoiceIDs[invID] {
			continue
		}
		if invID == "" && fmtString(message["contactId"]) != id {
			continue
		}
		route := "/contacts/" + id + "#record-tools?tab=conversations"
		if invID != "" {
			route = "/invoices/" + invID
		}
		add("mail:"+fmtString(message["id"]), "conversation", fmtString(message["subject"]), "Email · "+fmtString(message["status"]), fmtString(message["createdAt"]), route)
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].Date == items[j].Date {
			return items[i].ID < items[j].ID
		}
		return items[i].Date > items[j].Date
	})
	kind := r.URL.Query().Get("kind")
	filtered := []ClientHistoryItem{}
	for _, item := range items {
		if kind != "" && item.Kind != kind {
			continue
		}
		if q := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("q"))); q != "" && !strings.Contains(strings.ToLower(item.Title+" "+item.Detail), q) {
			continue
		}
		from, to := r.URL.Query().Get("from"), r.URL.Query().Get("to")
		date := item.Date
		if len(date) > 10 {
			date = date[:10]
		}
		if from != "" && (date == "" || date < from) || to != "" && (date == "" || date > to) {
			continue
		}
		filtered = append(filtered, item)
	}
	limit, offset := parseLimitOffset(r, 25, 0)
	writeJSON(w, 200, map[string]any{"items": slicePage(filtered, offset, limit), "total": len(filtered), "limit": limit, "offset": offset, "clients": choices, "contactId": id})
}
func formatMoney(value float64) string { raw, _ := json.Marshal(money(value)); return string(raw) }

type PriorityRow struct {
	ID          string `json:"id"`
	Entity      string `json:"entity"`
	Title       string `json:"title"`
	Action      string `json:"action"`
	Responsible string `json:"responsible"`
	Deadline    string `json:"deadline"`
	Reason      string `json:"reason"`
	Route       string `json:"route"`
}

func handleWorkspacePriorities(w http.ResponseWriter, r *http.Request) {
	r, s, close, ok := dailySnapshot(w, r)
	if !ok {
		return
	}
	defer close()
	today := organizationToday(r)
	manager := isManager(currentUser(r))
	queues := map[string][]PriorityRow{"overdue": {}, "documents": {}, "missingPlans": {}, "finance": {}}
	if manager {
		queues["unassigned"] = []PriorityRow{}
		queues["stalled"] = []PriorityRow{}
		queues["overdueHandoffs"] = []PriorityRow{}
	}
	plan := func(entity, id, title, action, owner, deadline string, active bool) {
		if !active {
			return
		}
		row := PriorityRow{id, entity, title, action, owner, deadline, "", "/" + entity + "/" + id}
		if manager && (strings.TrimSpace(owner) == "" || strings.EqualFold(strings.TrimSpace(owner), "Unassigned")) {
			unassigned := row
			unassigned.Reason = "No responsible staff assigned"
			unassigned.Action = "Assign an owner and confirm the next action"
			queues["unassigned"] = append(queues["unassigned"], unassigned)
		}
		if deadline != "" && deadline < today {
			row.Reason = "Overdue action"
			queues["overdue"] = append(queues["overdue"], row)
		}
		if strings.TrimSpace(action) == "" || deadline == "" {
			row.Reason = "Set the next action and deadline"
			queues["missingPlans"] = append(queues["missingPlans"], row)
		}
	}
	for _, l := range s.Leads {
		plan("leads", l.ID, l.Name, l.NextAction, l.AssignedTo, l.FollowUpDate, openLead(l.Status))
	}
	for _, d := range s.Deals {
		plan("deals", d.ID, d.Title, d.NextAction, d.Owner, d.NextDeadline, d.Stage != "Won" && d.Stage != "Lost")
	}
	for _, c := range s.Cases {
		plan("cases", c.ID, c.CaseNumber, c.NextAction, c.AssignedOfficer, c.NextDeadline, activeApplication(c.CurrentStage))
	}
	for _, st := range s.Students {
		plan("students", st.ID, st.Name, st.NextAction, st.OwnerID, st.NextDeadline, st.ApplicationStage != "Completed" && st.ApplicationStage != "Closed" && st.ApplicationStage != "Withdrawn")
	}
	for _, item := range dailyDocumentItems(currentUser(r), s, today, "all") {
		queues["documents"] = append(queues["documents"], PriorityRow{item.ID, "documents", item.Name, item.Action, item.Owner, item.Deadline, item.Reason, item.Route})
	}
	if manager {
		lastStage := map[string]time.Time{}
		rows, err := storeDB(r).Query("SELECT case_id,MAX(entered_at) FROM case_stage_events WHERE org_id=? GROUP BY case_id", currentUser(r).OrgID)
		if err != nil {
			writeError(w, 503, "Could not load application stage history")
			return
		}
		for rows.Next() {
			var id, at string
			if err = rows.Scan(&id, &at); err != nil {
				break
			}
			lastStage[id], _ = time.Parse(time.RFC3339Nano, at)
		}
		if err == nil {
			err = rows.Err()
		}
		rows.Close()
		if err != nil {
			writeError(w, 503, "Could not read application stage history")
			return
		}
		report := operationalReport(s, today, lastStage, map[string]string{})
		for _, app := range report["stalled"].([]StalledApplication) {
			queues["stalled"] = append(queues["stalled"], PriorityRow{strings.TrimPrefix(app.URL, "/cases/"), "cases", app.Record, app.Action, app.Responsible, "", fmt.Sprintf("%d days · %s", app.Days, app.Basis), app.URL})
		}
		handoffs, err := overdueHandoffPriorities(r, s, today)
		if err != nil {
			writeError(w, 503, "Could not load overdue handoffs")
			return
		}
		queues["overdueHandoffs"] = handoffs
	}
	totals := map[string]map[string]float64{}
	invoiceIDs := map[string]Invoice{}
	for _, inv := range s.Invoices {
		invoiceIDs[inv.ID] = inv
		currency := currencyOf(inv.Currency)
		if totals[currency] == nil {
			totals[currency] = map[string]float64{"received": 0, "outstanding": 0, "overdue": 0, "pending": 0}
		}
		totals[currency]["received"] += inv.AmountReceived
		totals[currency]["outstanding"] += inv.Balance
		if inv.Balance > 0 {
			reason := "Outstanding balance"
			if inv.DueDate != "" && inv.DueDate < today {
				reason = "Overdue invoice"
				totals[currency]["overdue"] += inv.Balance
			}
			queues["finance"] = append(queues["finance"], PriorityRow{inv.ID, "invoices", inv.InvoiceNumber, "Review balance " + currency + " " + formatMoney(inv.Balance), inv.OwnerID, inv.DueDate, reason, "/invoices/" + inv.ID})
		}
	}
	for _, p := range s.Payments {
		if !pendingReceipt(p) {
			continue
		}
		inv := invoiceIDs[p.InvoiceID]
		currency := currencyOf(inv.Currency)
		if totals[currency] != nil {
			totals[currency]["pending"] += p.Amount
		}
		queues["finance"] = append(queues["finance"], PriorityRow{p.ID, "payments", p.Reference, "Review receipt " + currency + " " + formatMoney(p.Amount), p.OwnerID, p.PaidOn, "Pending approval", "/invoices/" + p.InvoiceID + "#record-tools?tab=workflows"})
	}
	for _, values := range totals {
		for key, value := range values {
			values[key] = money(value)
		}
	}
	counts := map[string]int{}
	for key, rows := range queues {
		counts[key] = len(rows)
		if key == "stalled" {
			continue
		}
		sort.Slice(rows, func(i, j int) bool {
			if rows[i].Deadline == rows[j].Deadline {
				return rows[i].ID < rows[j].ID
			}
			if rows[i].Deadline == "" {
				return false
			}
			if rows[j].Deadline == "" {
				return true
			}
			return rows[i].Deadline < rows[j].Deadline
		})
	}
	queue := r.URL.Query().Get("queue")
	if queue == "" {
		queue = "overdue"
	}
	items, valid := queues[queue]
	if !valid {
		writeError(w, 400, "Choose a supported queue")
		return
	}
	limit, offset := parseLimitOffset(r, 25, 0)
	writeJSON(w, 200, map[string]any{"queue": queue, "items": slicePage(items, offset, limit), "total": len(items), "counts": counts, "finance": totals, "canManage": manager, "today": today, "limit": limit, "offset": offset})
}
func activeApplication(stage string) bool {
	return stage != "Approved" && stage != "Closed" && stage != "Refused" && stage != "Withdrawn"
}

func actionOrDefault(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}
