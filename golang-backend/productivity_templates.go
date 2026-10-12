package main

import (
	"encoding/json"
	"net/http"
	"strings"
)

type FollowupTemplate struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Service string `json:"service,omitempty"`
	Subject string `json:"subject"`
	Body    string `json:"body"`
	Builtin bool   `json:"builtin"`
}

func builtinFollowupTemplates() []FollowupTemplate {
	return []FollowupTemplate{
		{ID: "inquiry", Name: "First inquiry reply", Subject: "Your inquiry", Body: "Hello {{name}},\n\nThank you for your inquiry. Your next step is: {{nextAction}}.\nPlease let us know a suitable time to speak.\n\nThank you.", Builtin: true},
		{ID: "documents", Name: "Document request", Subject: "Documents for your application", Body: "Hello {{name}},\n\nWe are following up on your application documents.\nNext step: {{nextAction}}\nPlease respond by {{followUpDate}}, or let us know if you need help.\n\nThank you.", Builtin: true},
		{ID: "payment", Name: "Payment follow-up", Subject: "Invoice follow-up", Body: "Hello {{name}},\n\nPlease review your invoice and let us know if you have questions about the remaining balance. If you have paid, please share your receipt so we can check it.\n\nThank you.", Builtin: true},
	}
}
func registerFollowupTemplateRoutes(m *http.ServeMux) {
	m.HandleFunc("GET /api/follow-up-templates", authMiddleware(handleFollowupTemplates))
	m.HandleFunc("POST /api/follow-up-templates", authMiddleware(dailyOrigin(handleSaveFollowupTemplate)))
	m.HandleFunc("DELETE /api/follow-up-templates/{id}", authMiddleware(dailyOrigin(handleDeleteFollowupTemplate)))
}
func handleFollowupTemplates(w http.ResponseWriter, r *http.Request) {
	templates := builtinFollowupTemplates()
	rows, err := storeDB(r).Query("SELECT id,data FROM workspace_entries WHERE org_id=? AND category='followup_template' ORDER BY created_at,id", currentUser(r).OrgID)
	if err != nil {
		writeError(w, 500, "Could not load follow-up templates")
		return
	}
	defer rows.Close()
	for rows.Next() {
		var id, raw string
		var t FollowupTemplate
		if err = rows.Scan(&id, &raw); err != nil {
			break
		}
		if err = json.Unmarshal([]byte(raw), &t); err != nil {
			break
		}
		t.ID = id
		t.Builtin = false
		templates = append(templates, t)
	}
	if err == nil {
		err = rows.Err()
	}
	if err != nil {
		writeError(w, 500, "Could not read follow-up templates")
		return
	}
	writeJSON(w, 200, map[string]any{"templates": templates, "canManage": isManager(currentUser(r))})
}
func handleSaveFollowupTemplate(w http.ResponseWriter, r *http.Request) {
	if !isManager(currentUser(r)) {
		writeError(w, 403, "A manager must maintain shared templates")
		return
	}
	var t FollowupTemplate
	if !decodeRequest(w, r, &t) {
		return
	}
	t.Service = strings.TrimSpace(t.Service)
	t.Name = strings.TrimSpace(t.Name)
	t.Subject = strings.TrimSpace(t.Subject)
	t.Body = strings.TrimSpace(t.Body)
	t.ID = ""
	t.Builtin = false
	if len(t.Service) > 120 || t.Name == "" || len(t.Name) > 100 || t.Subject == "" || len(t.Subject) > 200 || t.Body == "" || len(t.Body) > 4000 {
		writeError(w, 400, "Enter a service (120 characters), name (100), subject (200) and message (4,000)")
		return
	}
	id, err := entry(r, "followup_template", "", t, false)
	if err != nil {
		writeError(w, 500, "Could not save template")
		return
	}
	writeJSON(w, 201, map[string]string{"id": id})
}
func handleDeleteFollowupTemplate(w http.ResponseWriter, r *http.Request) {
	if !isManager(currentUser(r)) {
		writeError(w, 403, "A manager must maintain shared templates")
		return
	}
	result, err := storeDB(r).Exec("DELETE FROM workspace_entries WHERE id=? AND org_id=? AND category='followup_template'", r.PathValue("id"), currentUser(r).OrgID)
	if err != nil {
		writeError(w, 500, "Could not remove template")
		return
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		writeError(w, 404, "Template not found")
		return
	}
	writeJSON(w, 200, map[string]bool{"deleted": true})
}
func handleLeadConversionPreview(w http.ResponseWriter, r *http.Request) {
	r, s, close, ok := dailySnapshot(w, r)
	if !ok {
		return
	}
	defer close()
	id := r.PathValue("id")
	for _, l := range s.Leads {
		if l.ID != id {
			continue
		}
		docs := []Document{}
		for _, d := range s.Documents {
			if d.LeadID == id || l.ConvertedCaseID != "" && d.CaseID == l.ConvertedCaseID {
				docs = append(docs, d)
			}
		}
		writeJSON(w, 200, map[string]any{"lead": l, "documents": docs, "duplicates": leadDuplicateCandidates(s, l.ID, l.Email, l.Phone), "editable": canWrite(l.RecordScope, currentUser(r)), "revision": recordVersion(l)})
		return
	}
	writeError(w, 404, "Lead not found")
}

func recordVersion(value any) string {
	var record map[string]any
	json.Unmarshal([]byte(mustJSON(value)), &record)
	return versionOf(record)
}
