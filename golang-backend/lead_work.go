package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"
)

func leadNextAction(value string) string {
	if strings.TrimSpace(value) == "" {
		return "Review inquiry and arrange first consultation"
	}
	return strings.TrimSpace(value)
}
func openLead(status string) bool {
	return status != "Converted" && status != "Closed/Lost" && status != "Closed"
}
func phoneIdentity(value string) string {
	value = strings.TrimPrefix(normalizePhone(strings.TrimSpace(value)), "+")
	return strings.TrimPrefix(value, "00")
}

type LeadDuplicate struct {
	ID          string `json:"id"`
	Entity      string `json:"entity"`
	EntityLabel string `json:"entityLabel"`
	Detail      string `json:"detail"`
}

func leadDuplicateCandidates(s diskStore, id, email, phone string) []LeadDuplicate {
	email, phone = qualityKey(email), phoneIdentity(phone)
	out := []LeadDuplicate{}
	convertedContact := ""
	for _, lead := range s.Leads {
		if lead.ID == id {
			convertedContact = lead.ConvertedContactID
		}
	}
	match := func(otherEmail, otherPhone string) string {
		reasons := []string{}
		if email != "" && qualityKey(otherEmail) == email {
			reasons = append(reasons, "email")
		}
		if phone != "" && phoneIdentity(otherPhone) == phone {
			reasons = append(reasons, "phone")
		}
		return strings.Join(reasons, " and ")
	}
	for _, lead := range s.Leads {
		if lead.ID == id {
			continue
		}
		if reason := match(lead.Email, lead.Phone); reason != "" {
			out = append(out, LeadDuplicate{lead.ID, "leads", "lead", "Review " + lead.Name + " — matching " + reason + " (" + lead.Status + ")."})
		}
	}
	for _, contact := range s.Contacts {
		if contact.ID == convertedContact && convertedContact != "" {
			continue
		}
		if reason := match(contact.Email, contact.Phone); reason != "" {
			out = append(out, LeadDuplicate{contact.ID, "contacts", "contact", "Review " + contact.Name + " — matching " + reason + "."})
		}
	}
	return out
}
func (state *runtimeStore) checkLeadDuplicates(w http.ResponseWriter, r *http.Request, id string, req leadRequest) bool {
	// The request projection contains only readable records. Restricted names and
	// relationships must never be exposed through duplicate hints.
	matches := leadDuplicateCandidates(diskStore{Leads: state.leads, Contacts: state.contacts}, id, req.Email, req.Phone)
	if len(matches) > 0 && !req.DuplicateConfirmed {
		writeFieldErrors(w, map[string]string{"duplicateConfirmed": "Matching lead or contact details were found. Review the record links above, or explicitly confirm a separate inquiry before saving."})
		return false
	}
	return true
}

type LeadTimelineEvent struct {
	ID         string `json:"id"`
	Kind       string `json:"kind"`
	Title      string `json:"title"`
	Body       string `json:"body"`
	Actor      string `json:"actor"`
	At         string `json:"at"`
	DocumentID string `json:"documentId,omitempty"`
	FollowUp   bool   `json:"followUp,omitempty"`
}

func appendLeadEvent(r *http.Request, org, leadID string, event LeadTimelineEvent) error {
	if event.ID == "" {
		event.ID = newID("lead-event")
	}
	if event.At == "" {
		event.At = utcNow()
	}
	_, err := storeDB(r).Exec("INSERT INTO workspace_entries VALUES(?,?,'lead_timeline','',?,?,?,?)", event.ID, org, leadID, mustJSON(event), event.At, event.At)
	return err
}
func registerLeadWorkRoutes(m *http.ServeMux) {
	m.HandleFunc("GET /api/leads/{id}/timeline", authMiddleware(handleLeadTimeline))
	m.HandleFunc("POST /api/leads/{id}/timeline", authMiddleware(handleLeadTimelineLog))
	m.HandleFunc("PATCH /api/documents/{id}/requirement-plan", authMiddleware(handleRequirementPlan))
}
func handleLeadTimelineLog(w http.ResponseWriter, r *http.Request) {
	state, release := runtimeForCall(r)
	defer release()
	var in struct {
		Kind     string `json:"kind"`
		Body     string `json:"body"`
		FollowUp bool   `json:"followUp"`
	}
	if !decodeRequest(w, r, &in) {
		return
	}
	in.Body = strings.TrimSpace(in.Body)
	if (in.Kind != "call" && in.Kind != "message" && in.Kind != "note") || in.Body == "" || len(in.Body) > 4000 || (in.Kind == "note" && in.FollowUp) {
		writeError(w, 400, "Choose call, message or note and enter details within 4,000 characters. Only calls and messages can count as follow-ups.")
		return
	}
	if r.Header.Get("If-Match") == "" {
		writeError(w, 428, "Reload the lead before logging activity")
		return
	}
	state.mu.Lock()
	defer state.mu.Unlock()
	for i, lead := range state.leads {
		if lead.ID != r.PathValue("id") {
			continue
		}
		if !canWrite(lead.RecordScope, currentUser(r)) {
			writeError(w, 403, "You can only log activity for leads you own")
			return
		}
		event := LeadTimelineEvent{ID: newID("lead-event"), Kind: in.Kind, Title: "Logged " + in.Kind, Body: in.Body, Actor: currentUser(r).Name, At: utcNow(), FollowUp: in.FollowUp}
		if err := appendLeadEvent(r, lead.OrgID, lead.ID, event); err != nil {
			writeError(w, 500, "Could not save lead activity")
			return
		}
		if in.FollowUp && lead.FirstFollowUpAt == "" {
			state.leads[i].FirstFollowUpAt = event.At
		}
		writeJSON(w, 201, map[string]any{"event": event, "lead": state.leads[i]})
		return
	}
	writeError(w, 404, "Lead not found")
}
func handleLeadTimeline(w http.ResponseWriter, r *http.Request) {
	state, release := runtimeForCall(r)
	defer release()
	lead, ok := state.getLeadByID(r.PathValue("id"))
	if !ok {
		writeError(w, 404, "Lead not found")
		return
	}
	rows, err := queryObjects(r, "SELECT data FROM workspace_entries WHERE org_id=? AND category='lead_timeline' AND record_id=? ORDER BY created_at DESC,id DESC", lead.OrgID, lead.ID)
	if err != nil {
		writeError(w, 500, "Could not load lead timeline")
		return
	}
	events := []LeadTimelineEvent{}
	for _, row := range rows {
		var e LeadTimelineEvent
		if json.Unmarshal([]byte(fmtString(row["data"])), &e) != nil {
			writeError(w, 500, "Could not read lead timeline")
			return
		}
		if e.DocumentID != "" {
			if _, err := sqlRecord(r, "documents", e.DocumentID); err != nil {
				continue
			}
		}
		events = append(events, e)
	}
	for _, doc := range state.snapshot().Documents {
		if doc.LeadID != lead.ID && (lead.ConvertedCaseID == "" || doc.CaseID != lead.ConvertedCaseID) {
			continue
		}
		versions, e := queryObjects(r, "SELECT id,version,filename,uploaded_by,uploaded_at FROM file_versions WHERE org_id=? AND document_id=? ORDER BY version DESC", lead.OrgID, doc.ID)
		if e != nil {
			writeError(w, 500, "Could not load CV history")
			return
		}
		for _, v := range versions {
			actor := "Staff"
			if u, ok := committedUser(r, "id", fmtString(v["uploaded_by"])); ok && u.OrgID == lead.OrgID {
				actor = u.Name
			}
			events = append(events, LeadTimelineEvent{ID: "file:" + fmtString(v["id"]), Kind: "cv", Title: "CV / document uploaded", Body: doc.DocName + " · " + fmtString(v["filename"]) + " · version " + fmtString(v["version"]), Actor: actor, At: fmtString(v["uploaded_at"]), DocumentID: doc.ID})
		}
	}
	// Actual linked contact messages are explicitly labeled as contact history;
	// shared-contact conversations are not represented as this inquiry's messages.
	if lead.ConvertedContactID != "" {
		if _, err := sqlRecord(r, "contacts", lead.ConvertedContactID); err == nil {
			messages, e := queryObjects(r, "SELECT id,case_id,channel,direction,subject,body,status,created_at FROM conversations WHERE org_id=? AND contact_id=? ORDER BY created_at DESC", lead.OrgID, lead.ConvertedContactID)
			if e != nil {
				writeError(w, 500, "Could not load linked contact messages")
				return
			}
			for _, m := range messages {
				if caseID := fmtString(m["case_id"]); caseID != "" {
					if _, err := sqlRecord(r, "cases", caseID); err != nil {
						continue
					}
				}
				events = append(events, LeadTimelineEvent{ID: "message:" + fmtString(m["id"]), Kind: "contact-message", Title: "Linked contact · " + fmtString(m["channel"]) + " · " + fmtString(m["direction"]) + " · " + fmtString(m["status"]), Body: fmtString(m["subject"]) + "\n" + fmtString(m["body"]), At: fmtString(m["created_at"])})
			}
		}
	}
	// Existing notes are visible even for leads created before timeline tracking.
	if strings.TrimSpace(lead.Notes) != "" {
		events = append(events, LeadTimelineEvent{ID: "current-notes", Kind: "notes", Title: "Current lead notes", Body: lead.Notes})
	}
	sort.SliceStable(events, func(i, j int) bool {
		if events[i].At == events[j].At {
			return events[i].ID > events[j].ID
		}
		return events[i].At > events[j].At
	})
	owner := "Unassigned"
	if user, ok := committedUser(r, "id", lead.OwnerID); ok && user.OrgID == lead.OrgID {
		owner = user.Name
	}
	limit, offset := parseLimitOffset(r, 25, 0)
	writeJSON(w, 200, map[string]any{"events": slicePage(events, offset, limit), "total": len(events), "limit": limit, "offset": offset, "revision": versionOf(leadMap(lead)), "owner": owner})
}
func leadMap(lead Lead) map[string]any {
	var out map[string]any
	json.Unmarshal([]byte(mustJSON(lead)), &out)
	return out
}

// This hook runs in the same mutation transaction as records and audit history.
func trackLeadTimeline(r *http.Request, before, after diskStore, actor string) error {
	old := map[string]Lead{}
	for _, lead := range before.Leads {
		old[lead.ID] = lead
	}
	for _, lead := range after.Leads {
		previous, exists := old[lead.ID]
		if !exists {
			if err := appendLeadEvent(r, lead.OrgID, lead.ID, LeadTimelineEvent{Kind: "created", Title: "Lead created", Body: "Source: " + lead.Source + " · Referrer: " + lead.Referrer, Actor: actor}); err != nil {
				return err
			}
			continue
		}
		if (previous.Status != lead.Status && !openLead(lead.Status)) || previous.FollowUpDate != lead.FollowUpDate {
			if _, err := storeDB(r).Exec("UPDATE notifications SET read_at=coalesce(read_at,?) WHERE org_id=? AND kind='lead-followup' AND path=?", utcNow(), lead.OrgID, "/leads/"+lead.ID); err != nil {
				return err
			}
		}
		changes := []string{}
		if previous.Status != lead.Status {
			changes = append(changes, "Status: "+previous.Status+" → "+lead.Status)
		}
		if previous.NextAction != lead.NextAction {
			changes = append(changes, "Next action: "+lead.NextAction)
		}
		if previous.FollowUpDate != lead.FollowUpDate {
			changes = append(changes, "Follow-up: "+lead.FollowUpDate)
		}
		if previous.AssignedTo != lead.AssignedTo {
			assigned := lead.AssignedTo
			if user, ok := committedUser(r, "id", assigned); ok && user.OrgID == lead.OrgID {
				assigned = user.Name + " (" + user.ID + ")"
			}
			changes = append(changes, "Assigned consultant: "+assigned)
		}
		if previous.Notes != lead.Notes {
			changes = append(changes, "Notes updated:\n"+lead.Notes)
		}
		if len(changes) > 0 {
			if err := appendLeadEvent(r, lead.OrgID, lead.ID, LeadTimelineEvent{Kind: "change", Title: "Lead updated", Body: strings.Join(changes, "\n"), Actor: actor}); err != nil {
				return err
			}
		}
	}
	previousDocs := map[string]Document{}
	for _, doc := range before.Documents {
		previousDocs[doc.ID] = doc
	}
	for _, doc := range after.Documents {
		previous := previousDocs[doc.ID]
		leadID := doc.LeadID
		if leadID == "" {
			leadID = previous.LeadID
		}
		if leadID == "" {
			continue
		}
		if doc.ReviewedAt != "" && (previous.ReviewedAt != doc.ReviewedAt || previous.ReviewedVersion != doc.ReviewedVersion) {
			if err := appendLeadEvent(r, doc.OrgID, leadID, LeadTimelineEvent{Kind: "document-review", Title: "Document version reviewed", Body: fmt.Sprintf("%s · Version %d", doc.DocName, doc.ReviewedVersion), Actor: actor, DocumentID: doc.ID}); err != nil {
				return err
			}
		}

		if previous.LeadID != "" && doc.CaseID != "" && previous.CaseID == "" {
			if err := appendLeadEvent(r, doc.OrgID, leadID, LeadTimelineEvent{Kind: "conversion", Title: "Document moved to application", Body: doc.DocName, Actor: actor, DocumentID: doc.ID}); err != nil {
				return err
			}
		}
	}
	return nil
}

func remindLeadFollowUps(db DBRunner, s diskStore, day func(string, int) string) error {
	people := map[string]User{}
	for _, stored := range s.Users {
		people[stored.User.ID] = stored.User
	}
	for _, lead := range s.Leads {
		today := day(lead.OrgID, 0)
		if !openLead(lead.Status) || lead.FollowUpDate == "" || lead.FollowUpDate > today {
			continue
		}
		if _, err := time.Parse("2006-01-02", lead.FollowUpDate); err != nil {
			continue
		}
		recipient := lead.OwnerID
		if strings.TrimSpace(lead.AssignedTo) != "" {
			recipient = ""
			for _, u := range people {
				if u.OrgID == lead.OrgID && (u.ID == lead.AssignedTo || u.Name == lead.AssignedTo) {
					if recipient != "" {
						recipient = ""
						break
					}
					recipient = u.ID
				}
			}
		}
		u, ok := people[recipient]
		if !ok || u.Disabled || !canRead(lead.RecordScope, u) {
			continue
		}
		title := "Lead follow-up due today"
		if lead.FollowUpDate < today {
			title = "Overdue lead follow-up"
		}
		if _, err := db.Exec("INSERT OR IGNORE INTO notifications VALUES(?,?,?,?,?,?,?,?,?,NULL)", newID("notice"), lead.OrgID, u.ID, "lead-followup", title, lead.Name+": "+leadNextAction(lead.NextAction)+" · Due "+lead.FollowUpDate, "/leads/"+lead.ID, "lead-followup:"+lead.ID+":"+today, utcNow()); err != nil {
			return err
		}
	}
	return nil
}
