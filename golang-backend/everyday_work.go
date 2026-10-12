package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

func registerEverydayRoutes(m *http.ServeMux) {
	m.HandleFunc("PATCH /api/communication/{message}/handling", authMiddleware(handleMessageHandling))
	m.HandleFunc("POST /api/communication/{message}/reply", authMiddleware(handlePortalReply))
	m.HandleFunc("GET /api/client-profile/{entity}/{record}", authMiddleware(handleClientProfile))
	m.HandleFunc("PUT /api/client-profile/{entity}/{record}", authMiddleware(handleClientProfile))
	m.HandleFunc("GET /api/finance-capabilities", authMiddleware(handleFinanceCapabilities))
	m.HandleFunc("PUT /api/finance-capabilities/{staff}", authMiddleware(handleFinanceCapabilities))
	m.HandleFunc("POST /api/consultations", authMiddleware(handleConsultation))
	m.HandleFunc("PATCH /api/consultations/{meeting}", authMiddleware(handleConsultation))
	m.HandleFunc("GET /api/consultations/{meeting}/calendar.ics", authMiddleware(handleConsultationICS))
	m.HandleFunc("PUT /api/cases/{id}/client-task", authMiddleware(handleClientTask))
	m.HandleFunc("GET /api/work-outcomes", authMiddleware(handleWorkOutcomes))
}

// Separate workflow entries retain existing channel delivery statuses and records.
func workflowValue(r *http.Request, category, record string) (map[string]any, error) {
	var raw string
	err := storeDB(r).QueryRow("SELECT data FROM workspace_entries WHERE org_id=? AND category=? AND record_id=? ORDER BY updated_at DESC,id DESC LIMIT 1", currentUser(r).OrgID, category, record).Scan(&raw)
	if err != nil {
		if err == sql.ErrNoRows {
			return map[string]any{}, nil
		}
		return nil, err
	}
	var out map[string]any
	err = json.Unmarshal([]byte(raw), &out)
	return out, err
}
func saveWorkflowValue(r *http.Request, category, record string, data any) error {
	// Requests are serialized within persistMutations; deletion and insertion commit together.
	if _, err := storeDB(r).Exec("DELETE FROM workspace_entries WHERE org_id=? AND category=? AND record_id=?", currentUser(r).OrgID, category, record); err != nil {
		return err
	}
	_, err := entry(r, category, record, data, false)
	return err
}
func workflowRevision(w http.ResponseWriter, r *http.Request, value map[string]any) bool {
	expected := r.Header.Get("If-Match")
	if expected == "" {
		writeError(w, 428, "Load and review the current saved version first")
		return false
	}
	if expected != versionOf(value) {
		writeError(w, 409, "This item changed; refresh before saving")
		return false
	}
	return true
}
func writableWorkflowRecord(w http.ResponseWriter, r *http.Request, entity, id string) (map[string]any, bool) {
	record, err := sqlRecord(r, entity, id)
	if err != nil {
		writeError(w, 404, "Record not accessible")
		return nil, false
	}
	scope := RecordScope{OrgID: fmtString(record["orgId"]), OwnerID: fmtString(record["ownerId"]), TeamID: fmtString(record["teamId"]), Visibility: fmtString(record["visibility"])}
	if !canWrite(scope, currentUser(r)) {
		writeError(w, 403, "You can only change records you own")
		return nil, false
	}
	return record, true
}
func accessibleMessage(r *http.Request, id string) (map[string]any, RecordScope, error) {
	var contact, application, channel, direction string
	if err := storeDB(r).QueryRow("SELECT contact_id,case_id,channel,direction FROM conversations WHERE id=? AND org_id=?", id, currentUser(r).OrgID).Scan(&contact, &application, &channel, &direction); err != nil {
		return nil, RecordScope{}, err
	}
	scope := RecordScope{OrgID: currentUser(r).OrgID}
	for _, parent := range []struct{ entity, id string }{{"contacts", contact}, {"cases", application}} {
		if parent.id == "" {
			continue
		}
		record, err := sqlRecord(r, parent.entity, parent.id)
		if err != nil {
			return nil, scope, err
		}
		if scope.OwnerID == "" {
			json.Unmarshal([]byte(mustJSON(record)), &scope)
		}
	}
	if contact == "" && application == "" && !isManager(currentUser(r)) {
		return nil, scope, fmt.Errorf("not accessible")
	}
	return map[string]any{"contactId": contact, "caseId": application, "channel": channel, "direction": direction}, scope, nil
}
func handleMessageHandling(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("message")
	message, scope, err := accessibleMessage(r, id)
	if err != nil {
		writeError(w, 404, "Message not accessible")
		return
	}
	if currentUser(r).Role == "viewer" || message["direction"] != "inbound" {
		writeError(w, 403, "Only staff can handle inbound messages")
		return
	}
	before, err := workflowValue(r, "message_handling", id)
	if err != nil {
		writeError(w, 503, "Could not load handling")
		return
	}
	if !workflowRevision(w, r, before) {
		return
	}
	var in struct{ Status, AssignedTo, ResponseDue string }
	if !decodeRequest(w, r, &in) {
		return
	}
	if in.Status != "needs_reply" && in.Status != "assigned" && in.Status != "resolved" {
		writeError(w, 400, "Choose Needs reply, Assigned or Resolved")
		return
	}
	if in.AssignedTo != "" {
		staff, err := requirementPeople(r)
		if err != nil {
			writeError(w, 503, "Could not load staff")
			return
		}
		valid := false
		for _, u := range staff {
			if u.ID == in.AssignedTo && u.Role != "viewer" && canRead(scope, u) {
				valid = true
				for _, p := range []struct{ entity, id string }{{"contacts", fmtString(message["contactId"])}, {"cases", fmtString(message["caseId"])}} {
					if p.id != "" {
						rec, e := sqlRecord(r, p.entity, p.id)
						var ps RecordScope
						json.Unmarshal([]byte(mustJSON(rec)), &ps)
						if e != nil || !canRead(ps, u) {
							valid = false
						}
					}
				}
			}
		}
		if !valid {
			writeError(w, 400, "Choose active staff with access to all linked records")
			return
		}
	}
	if in.Status == "assigned" && in.AssignedTo == "" {
		writeError(w, 400, "Assigned messages need a responsible staff member")
		return
	}
	if in.ResponseDue != "" {
		if due, err := time.Parse(time.RFC3339, in.ResponseDue); err != nil {
			writeError(w, 400, "Enter a valid response deadline with timezone")
			return
		} else {
			in.ResponseDue = due.UTC().Format(time.RFC3339)
		}
	}
	data := map[string]any{"status": in.Status, "assignedTo": in.AssignedTo, "responseDue": in.ResponseDue, "updatedBy": currentUser(r).ID, "updatedAt": utcNow()}
	if err := saveWorkflowValue(r, "message_handling", id, data); err != nil {
		writeError(w, 500, "Could not save handling")
		return
	}
	writeJSON(w, 200, map[string]any{"handling": data, "revision": versionOf(data)})
}
func handlePortalReply(w http.ResponseWriter, r *http.Request) {
	message, _, err := accessibleMessage(r, r.PathValue("message"))
	if err != nil || message["channel"] != "portal" || message["caseId"] == "" {
		writeError(w, 404, "Portal message not accessible")
		return
	}
	if _, ok := writableWorkflowRecord(w, r, "cases", fmtString(message["caseId"])); !ok {
		return
	}
	if _, ok := writableWorkflowRecord(w, r, "contacts", fmtString(message["contactId"])); !ok {
		return
	}
	var in struct{ Body string }
	if !decodeRequest(w, r, &in) {
		return
	}
	in.Body = strings.TrimSpace(in.Body)
	if in.Body == "" || len(in.Body) > 10000 {
		writeError(w, 400, "Enter a reply up to 10,000 characters")
		return
	}
	id := newID("message")
	if _, err := storeDB(r).Exec("INSERT INTO conversations VALUES(?,?,?,?,?,?,?,?,?,?,?)", id, currentUser(r).OrgID, message["contactId"], message["caseId"], "portal", "outbound", id, "Adviser reply", in.Body, "available", utcNow()); err != nil {
		writeError(w, 500, "Could not save reply")
		return
	}
	writeJSON(w, 201, map[string]any{"id": id, "message": "Reply available in the client portal. Resolve the inbound message when handling is complete."})
}

// A converted lead, student and application resolve to the same contact profile.
func canonicalProfile(r *http.Request, entity, id string) (string, string, error) {
	if entity != "contacts" && entity != "leads" && entity != "students" && entity != "cases" {
		return "", "", fmt.Errorf("unsupported profile")
	}
	rec, err := sqlRecord(r, entity, id)
	if err != nil {
		return "", "", err
	}
	contact := ""
	switch entity {
	case "contacts":
		contact = id
	case "leads":
		contact = fmtString(rec["convertedContactId"])
	case "cases":
		contact = fmtString(rec["clientId"])
	case "students":
		rows, err := queryObjects(r, "SELECT id FROM cases WHERE json_extract(data,'$.orgId')=? AND json_extract(data,'$.studentId')=?", currentUser(r).OrgID, id)
		if err != nil {
			return "", "", err
		}
		for _, row := range rows {
			c, err := sqlRecord(r, "cases", fmtString(row["id"]))
			if err != nil {
				continue
			}
			next := fmtString(c["clientId"])
			if contact != "" && next != contact {
				return "", "", fmt.Errorf("Student applications have different clients; resolve the links first")
			}
			contact = next
		}
	}
	if contact != "" {
		if _, err := sqlRecord(r, "contacts", contact); err != nil {
			return "", "", err
		}
		return "contacts", contact, nil
	}
	return entity, id, nil
}
func handleClientProfile(w http.ResponseWriter, r *http.Request) {
	entity, id, err := canonicalProfile(r, r.PathValue("entity"), r.PathValue("record"))
	if err != nil {
		writeError(w, 404, "Client profile not accessible or client links need review")
		return
	}
	key := entity + ":" + id
	profile, err := workflowValue(r, "client_profile", key)
	if err != nil {
		writeError(w, 503, "Could not load client profile")
		return
	}
	// Preserve intake from the lead when conversion first establishes a contact.
	if len(profile) == 0 && entity == "contacts" {
		rows, e := queryObjects(r, "SELECT id FROM leads WHERE json_extract(data,'$.orgId')=? AND json_extract(data,'$.convertedContactId')=?", currentUser(r).OrgID, id)
		if e != nil {
			writeError(w, 503, "Could not load intake")
			return
		}
		for _, row := range rows {
			leadID := fmtString(row["id"])
			if _, e := sqlRecord(r, "leads", leadID); e != nil {
				continue
			}
			candidate, e := workflowValue(r, "client_profile", "leads:"+leadID)
			if e != nil {
				writeError(w, 503, "Could not load intake")
				return
			}
			if len(candidate) > 0 {
				profile = candidate
				break
			}
		}
	}
	if r.Method == "PUT" {
		if _, ok := writableWorkflowRecord(w, r, r.PathValue("entity"), r.PathValue("record")); !ok {
			return
		}
		if _, ok := writableWorkflowRecord(w, r, entity, id); !ok {
			return
		}
		if !workflowRevision(w, r, profile) {
			return
		}
		var in map[string]any
		if !decodeRequest(w, r, &in) {
			return
		}
		clean := map[string]any{}
		for _, field := range []string{"education", "employment", "qualifications", "preferredChannel", "contactTimes", "contactRestrictions"} {
			v, ok := in[field].(string)
			if !ok || len(v) > 10000 {
				writeError(w, 400, "Enter profile fields as text up to 10,000 characters")
				return
			}
			clean[field] = strings.TrimSpace(v)
		}
		switch clean["preferredChannel"] {
		case "", "email", "whatsapp", "phone", "portal":
		default:
			writeError(w, 400, "Choose a supported preferred channel")
			return
		}
		blocked, ok := in["doNotContact"].(bool)
		if !ok {
			writeError(w, 400, "Choose the contact restriction")
			return
		}
		clean["doNotContact"] = blocked
		clean["updatedBy"] = currentUser(r).ID
		clean["updatedAt"] = utcNow()
		if err := saveWorkflowValue(r, "client_profile", key, clean); err != nil {
			writeError(w, 500, "Could not save profile")
			return
		}
		profile = clean
	}
	record, _ := sqlRecord(r, entity, id)
	var scope RecordScope
	json.Unmarshal([]byte(mustJSON(record)), &scope)
	writeJSON(w, 200, map[string]any{"profile": profile, "revision": versionOf(profile), "entity": entity, "recordId": id, "editable": canWrite(scope, currentUser(r))})
}
func financeCapability(r *http.Request, capability string) bool {
	u := currentUser(r)
	if isManager(u) {
		return true
	}
	if u.Role != "member" {
		return false
	}
	data, err := workflowValue(r, "finance_capabilities", u.ID)
	if err != nil {
		return false
	}
	if granted, ok := data[capability].(bool); ok {
		return granted
	}
	return capability == "recordPayments" // Retain established receipt-entry access; grants add only specific powers.
}
func handleFinanceCapabilities(w http.ResponseWriter, r *http.Request) {
	u := currentUser(r)
	caps := map[string]bool{}
	for _, key := range []string{"recordPayments", "approveAdjustments", "reconcile", "refunds"} {
		caps[key] = financeCapability(r, key)
	}
	if r.Method == "GET" {
		out := map[string]any{"capabilities": caps, "canManage": isAdmin(u)}
		if isAdmin(u) {
			staff, err := queryObjects(r, "SELECT id,name,role FROM users WHERE org_id=? AND disabled=0 ORDER BY name", u.OrgID)
			if err != nil {
				writeError(w, 503, "Could not load staff")
				return
			}
			for _, person := range staff {
				grants, err := workflowValue(r, "finance_capabilities", fmtString(person["id"]))
				if err != nil {
					writeError(w, 503, "Could not load permissions")
					return
				}
				person["grants"] = grants
				person["revision"] = versionOf(grants)
			}
			out["staff"] = staff
		}
		writeJSON(w, 200, out)
		return
	}
	if !isAdmin(u) {
		writeError(w, 403, "Only administrators can change finance permissions")
		return
	}
	var role string
	if storeDB(r).QueryRow("SELECT role FROM users WHERE id=? AND org_id=? AND disabled=0", r.PathValue("staff"), u.OrgID).Scan(&role) != nil || role != "member" {
		writeError(w, 400, "Choose an active member; managers already have finance access and viewers remain read-only")
		return
	}
	before, err := workflowValue(r, "finance_capabilities", r.PathValue("staff"))
	if err != nil {
		writeError(w, 503, "Could not load permissions")
		return
	}
	if !workflowRevision(w, r, before) {
		return
	}
	var in map[string]bool
	if !decodeRequest(w, r, &in) {
		return
	}
	clean := map[string]bool{}
	for _, key := range []string{"recordPayments", "approveAdjustments", "reconcile", "refunds"} {
		clean[key] = in[key]
	}
	if err := saveWorkflowValue(r, "finance_capabilities", r.PathValue("staff"), clean); err != nil {
		writeError(w, 500, "Could not save permissions")
		return
	}
	writeJSON(w, 200, map[string]any{"capabilities": clean})
}
func contactAllowed(r *http.Request, contact string) bool {
	if contact == "" {
		return true
	}
	profile, err := workflowValue(r, "client_profile", "contacts:"+contact)
	return err == nil && profile["doNotContact"] != true
}

type consultation struct {
	ContactID  string `json:"contactId"`
	Title      string `json:"title"`
	OccurredAt string `json:"occurredAt"`
	Duration   int    `json:"duration"`
	Location   string `json:"location"`
	Status     string `json:"status"`
	Attendance string `json:"attendance"`
	Invite     bool   `json:"invite"`
	Reminder   bool   `json:"reminder"`
	CreatedBy  string `json:"createdBy"`
	Sequence   int    `json:"sequence"`
}

func consultationData(r *http.Request, id string) (map[string]any, error) {
	var raw string
	err := storeDB(r).QueryRow("SELECT data FROM workspace_entries WHERE org_id=? AND category='calendar_event' AND id=?", currentUser(r).OrgID, id).Scan(&raw)
	if err != nil {
		return nil, err
	}
	var data map[string]any
	err = json.Unmarshal([]byte(raw), &data)
	return data, err
}
func handleConsultation(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("meeting")
	var before map[string]any
	if id != "" {
		var err error
		before, err = consultationData(r, id)
		if err != nil {
			writeError(w, 404, "Consultation not found")
			return
		}
		if _, ok := writableWorkflowRecord(w, r, "contacts", fmtString(before["contactId"])); !ok {
			return
		}
		if !workflowRevision(w, r, before) {
			return
		}
	}
	var in consultation
	if !decodeRequest(w, r, &in) {
		return
	}
	if before != nil && in.ContactID != fmtString(before["contactId"]) {
		writeError(w, 400, "Keep the original client when rescheduling")
		return
	}
	client, ok := writableWorkflowRecord(w, r, "contacts", in.ContactID)
	if !ok {
		return
	}
	at, err := time.Parse(time.RFC3339, in.OccurredAt)
	if err != nil || len(strings.TrimSpace(in.Title)) == 0 || len(in.Title) > 180 || len(in.Location) > 1000 || in.Duration < 5 || in.Duration > 480 {
		writeError(w, 400, "Enter a title, time with timezone and duration from 5 to 480 minutes")
		return
	}
	if in.Status == "" {
		in.Status = "scheduled"
	}
	if in.Status != "scheduled" && in.Status != "cancelled" && in.Status != "completed" {
		writeError(w, 400, "Choose Scheduled, Cancelled or Completed")
		return
	}
	if in.Attendance != "" && in.Attendance != "attended" && in.Attendance != "no_show" {
		writeError(w, 400, "Choose attendance")
		return
	}
	if in.Status == "scheduled" && in.Attendance != "" {
		writeError(w, 400, "Record attendance after completing the consultation")
		return
	}
	in.OccurredAt = at.UTC().Format(time.RFC3339)
	in.Title = strings.TrimSpace(in.Title)
	in.CreatedBy = currentUser(r).ID
	if before != nil {
		if n, ok := before["sequence"].(float64); ok {
			in.Sequence = int(n) + 1
		} else {
			in.Sequence = 1
		}
	}
	if in.Invite || in.Reminder {
		if !contactAllowed(r, in.ContactID) {
			writeError(w, 409, "Client has a do-not-contact restriction")
			return
		}
		if !validEmail(fmtString(client["email"])) {
			writeError(w, 400, "Add a valid client email before sending an invitation or reminder")
			return
		}
	}
	if in.Reminder && in.Status == "scheduled" && at.Before(time.Now().Add(24*time.Hour)) {
		writeError(w, 400, "Schedule at least 24 hours ahead to request a reminder")
		return
	}
	if id == "" {
		id, err = entry(r, "calendar_event", in.ContactID, in, false)
	} else {
		_, err = storeDB(r).Exec("UPDATE workspace_entries SET data=?,updated_at=? WHERE id=? AND org_id=? AND category='calendar_event'", mustJSON(in), utcNow(), id, currentUser(r).OrgID)
	}
	if err != nil {
		writeError(w, 500, "Could not save consultation")
		return
	}
	if _, err = storeDB(r).Exec("UPDATE mail_outbox SET status='cancelled' WHERE org_id=? AND kind LIKE ? AND status IN ('queued','waiting_configuration','failed')", currentUser(r).OrgID, "consultation:"+id+":%"); err != nil {
		writeError(w, 500, "Could not update consultation mail")
		return
	}
	if in.Invite {
		subject := in.Title
		if in.Status == "cancelled" {
			subject = "Cancelled: " + subject
		}
		if err = queueMail(r, currentUser(r), "consultation:"+id+":"+fmt.Sprint(in.Sequence), fmtString(client["email"]), subject, consultationText(in), in.ContactID, ""); err != nil {
			writeError(w, 500, "Could not queue invitation")
			return
		}
	}
	writeJSON(w, 200, map[string]any{"id": id, "event": in, "revision": versionOf(recordMap(in)), "message": "Consultation saved. Requested invitations are queued; check Communication for delivery."})
}
func consultationText(c consultation) string {
	return fmt.Sprintf("%s\nTime: %s (UTC)\nDuration: %d minutes\nLocation: %s\nStatus: %s\nPlease contact your adviser to change this appointment.", c.Title, c.OccurredAt, c.Duration, c.Location, c.Status)
}
func calendarEscape(s string) string {
	return strings.NewReplacer("\\", "\\\\", "\r", "", "\n", "\\n", ",", "\\,", ";", "\\;").Replace(s)
}
func consultationCalendar(id string, c consultation) []byte {
	at, _ := time.Parse(time.RFC3339, c.OccurredAt)
	method, status := "REQUEST", "CONFIRMED"
	if c.Status == "cancelled" {
		method, status = "CANCEL", "CANCELLED"
	}
	lines := []string{"BEGIN:VCALENDAR", "VERSION:2.0", "PRODID:-//ECC CRM//Consultations//EN", "METHOD:" + method, "BEGIN:VEVENT", "UID:" + calendarEscape(id) + "@ecc-crm", "DTSTAMP:" + time.Now().UTC().Format("20060102T150405Z"), "DTSTART:" + at.UTC().Format("20060102T150405Z"), "DTEND:" + at.Add(time.Duration(c.Duration)*time.Minute).UTC().Format("20060102T150405Z"), "SEQUENCE:" + fmt.Sprint(c.Sequence), "STATUS:" + status, "SUMMARY:" + calendarEscape(c.Title), "LOCATION:" + calendarEscape(c.Location), "DESCRIPTION:" + calendarEscape(consultationText(c)), "END:VEVENT", "END:VCALENDAR"}
	var out strings.Builder
	for _, line := range lines {
		for len(line) > 73 {
			cut := 73
			for cut > 0 && (line[cut]&0xc0) == 0x80 {
				cut--
			}
			out.WriteString(line[:cut] + "\r\n ")
			line = line[cut:]
		}
		out.WriteString(line + "\r\n")
	}
	return []byte(out.String())
}
func handleConsultationICS(w http.ResponseWriter, r *http.Request) {
	data, err := consultationData(r, r.PathValue("meeting"))
	if err != nil {
		writeError(w, 404, "Consultation not found")
		return
	}
	if _, err := sqlRecord(r, "contacts", fmtString(data["contactId"])); err != nil {
		writeError(w, 404, "Consultation not accessible")
		return
	}
	var c consultation
	json.Unmarshal([]byte(mustJSON(data)), &c)
	w.Header().Set("Content-Type", "text/calendar; charset=utf-8")
	w.Header().Set("Content-Disposition", "attachment; filename=consultation.ics")
	w.Write(consultationCalendar(r.PathValue("meeting"), c))
}
func portalAdviser(r *http.Request, assigned, owner string) string {
	if assigned == "" {
		assigned = owner
	}
	var name string
	storeDB(r).QueryRow("SELECT name FROM users WHERE org_id=? AND disabled=0 AND (id=? OR name=?) ORDER BY id LIMIT 1", portalIdentity(r).OrgID, assigned, assigned).Scan(&name)
	if name == "" {
		return "Your adviser"
	}
	return name
}
func handleWorkOutcomes(w http.ResponseWriter, r *http.Request) {
	r, close, ok := consolidatedReadSnapshot(w, r)
	if !ok {
		return
	}
	defer close()
	u := currentUser(r)
	out := map[string]any{}
	cs, ca := scopeSQL(r, "client.data")
	as, aa := scopeSQL(r, "application.data")
	base := ` FROM conversations message LEFT JOIN contacts client ON client.id=message.contact_id LEFT JOIN cases application ON application.id=message.case_id LEFT JOIN workspace_entries handling ON handling.org_id=message.org_id AND handling.category='message_handling' AND handling.record_id=message.id WHERE message.org_id=? AND message.direction='inbound' AND ((message.contact_id='' AND message.case_id='' AND ? IN ('admin','manager')) OR ((message.contact_id='' OR (` + cs + `)) AND (message.case_id='' OR (` + as + `)) AND (message.contact_id<>'' OR message.case_id<>'')))`
	args := append([]any{u.OrgID, u.Role}, ca...)
	args = append(args, aa...)
	var unanswered, overdue int
	err := storeDB(r).QueryRow("SELECT count(*),coalesce(sum(CASE WHEN coalesce(json_extract(handling.data,'$.responseDue'),'')<>'' AND json_extract(handling.data,'$.responseDue')<? THEN 1 ELSE 0 END),0)"+base+" AND coalesce(json_extract(handling.data,'$.status'),'needs_reply')<>'resolved'", append([]any{utcNow()}, args...)...).Scan(&unanswered, &overdue)
	if err != nil {
		writeError(w, 503, "Could not load message outcomes")
		return
	}
	out["unansweredMessages"] = unanswered
	out["overdueReplies"] = overdue
	for _, metric := range []struct{ key, entity, predicate string }{{"overdueTasks", "tasks", " AND json_extract(record.data,'$.dueDate')<>'' AND json_extract(record.data,'$.dueDate')<? AND lower(coalesce(json_extract(record.data,'$.status'),'')) NOT IN ('done','completed')"}, {"overdueApplications", "cases", " AND json_extract(record.data,'$.nextDeadline')<>'' AND json_extract(record.data,'$.nextDeadline')<? AND lower(coalesce(json_extract(record.data,'$.currentStage'),'')) NOT IN ('approved','closed','refused','withdrawn')"}} {
		scope, args := repositoryVisibility(r, metric.entity)
		var n int
		if err := storeDB(r).QueryRow("SELECT count(*) FROM "+metric.entity+" record WHERE "+scope+metric.predicate, append(args, organizationToday(r))...).Scan(&n); err != nil {
			writeError(w, 503, "Could not load work outcomes")
			return
		}
		out[metric.key] = n
	}
	scope, args := repositoryVisibility(r, "payments")
	rows, err := queryObjects(r, "SELECT coalesce(json_extract(invoice.data,'$.currency'),'USD') AS currency,round(sum(json_extract(record.data,'$.amount')),2) AS amount FROM payments record JOIN invoices invoice ON invoice.id=json_extract(record.data,'$.invoiceId') WHERE "+scope+" AND coalesce(json_extract(record.data,'$.status'),'posted') NOT IN ('void','pending_approval') AND json_extract(record.data,'$.paidOn')>=? AND json_extract(record.data,'$.paidOn')<=? GROUP BY currency", append(args, organizationToday(r)[:7]+"-01", organizationToday(r))...)
	if err != nil {
		writeError(w, 503, "Could not load collections")
		return
	}
	out["collectionsThisMonth"] = rows
	writeJSON(w, 200, out)
}
func portalClientTask(r *http.Request, id string) map[string]any {
	var raw string
	storeDB(r).QueryRow("SELECT data FROM workspace_entries WHERE org_id=? AND category='client_task' AND record_id=? ORDER BY updated_at DESC LIMIT 1", portalIdentity(r).OrgID, id).Scan(&raw)
	out := map[string]any{}
	json.Unmarshal([]byte(raw), &out)
	return out
}
func handleClientTask(w http.ResponseWriter, r *http.Request) {
	if _, ok := writableWorkflowRecord(w, r, "cases", r.PathValue("id")); !ok {
		return
	}
	before, err := workflowValue(r, "client_task", r.PathValue("id"))
	if err != nil {
		writeError(w, 503, "Could not load client instructions")
		return
	}
	if !workflowRevision(w, r, before) {
		return
	}
	var in struct {
		Instructions string `json:"instructions"`
		DueDate      string `json:"dueDate"`
		Done         bool   `json:"done"`
	}
	if !decodeRequest(w, r, &in) {
		return
	}
	in.Instructions = strings.TrimSpace(in.Instructions)
	if len(in.Instructions) > 5000 {
		writeError(w, 400, "Keep client instructions under 5,000 characters")
		return
	}
	if in.DueDate != "" {
		if _, err := time.Parse("2006-01-02", in.DueDate); err != nil {
			writeError(w, 400, "Enter a valid client deadline")
			return
		}
	}
	if err := saveWorkflowValue(r, "client_task", r.PathValue("id"), in); err != nil {
		writeError(w, 500, "Could not save client instructions")
		return
	}
	writeJSON(w, 200, map[string]any{"task": in, "revision": versionOf(recordMap(in))})
}
func generateConsultationReminders() {
	mutationMu.Lock()
	defer mutationMu.Unlock()
	tx, err := database.Begin()
	if err != nil {
		return
	}
	defer tx.Rollback()
	rows, err := tx.Query("SELECT id,org_id,data FROM workspace_entries WHERE category='calendar_event' AND json_extract(data,'$.reminder')=1 AND json_extract(data,'$.status')='scheduled'")
	if err != nil {
		return
	}
	type item struct{ id, org, raw string }
	all := []item{}
	for rows.Next() {
		var i item
		if rows.Scan(&i.id, &i.org, &i.raw) == nil {
			all = append(all, i)
		}
	}
	rows.Close()
	for _, i := range all {
		var c consultation
		if json.Unmarshal([]byte(i.raw), &c) != nil {
			continue
		}
		at, err := time.Parse(time.RFC3339, c.OccurredAt)
		if err != nil || at.Before(time.Now()) || at.After(time.Now().Add(24*time.Hour)) {
			continue
		}
		var raw string
		if tx.QueryRow("SELECT data FROM contacts WHERE id=? AND json_extract(data,'$.orgId')=?", c.ContactID, i.org).Scan(&raw) != nil {
			continue
		}
		var contact Contact
		json.Unmarshal([]byte(raw), &contact)
		if !validEmail(contact.Email) {
			continue
		}
		var blocked int
		if err := tx.QueryRow("SELECT count(*) FROM workspace_entries WHERE org_id=? AND category='client_profile' AND record_id=? AND json_extract(data,'$.doNotContact')=1", i.org, "contacts:"+c.ContactID).Scan(&blocked); err != nil {
			return
		}
		if blocked > 0 {
			continue
		}
		kind := "consultation:" + i.id + ":" + fmt.Sprint(c.Sequence) + ":reminder"
		var count int
		if tx.QueryRow("SELECT count(*) FROM mail_outbox WHERE org_id=? AND kind=?", i.org, kind).Scan(&count) != nil {
			return
		}
		if count > 0 {
			continue
		}
		if _, err = tx.Exec("INSERT INTO mail_outbox(id,org_id,user_id,contact_id,invoice_id,kind,recipient,subject,body,status,created_at) VALUES(?,?,?,?,?,?,?,?,?,'queued',?)", newID("mail"), i.org, c.CreatedBy, c.ContactID, "", kind, contact.Email, "Reminder: "+c.Title, consultationText(c), utcNow()); err != nil {
			return
		}
	}
	tx.Commit()
}
