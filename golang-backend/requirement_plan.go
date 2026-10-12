package main

import (
	"encoding/json"
	"net/http"
	"sort"
	"strings"
	"time"
)

type RequirementPerson struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}
type RequirementChecklist struct {
	ID              string              `json:"id"`
	Title           string              `json:"title"`
	Status          string              `json:"status"`
	Attention       string              `json:"attention"`
	ExpiryDate      string              `json:"expiryDate"`
	Missing         bool                `json:"missing"`
	Expiring        bool                `json:"expiring"`
	Complete        bool                `json:"complete"`
	ResponsibleID   string              `json:"responsibleId"`
	Responsible     string              `json:"responsible"`
	DueDate         string              `json:"dueDate"`
	DocumentDueDate string              `json:"documentDueDate"`
	DeadlineSource  string              `json:"deadlineSource"`
	Overdue         bool                `json:"overdue"`
	Editable        bool                `json:"editable"`
	Revision        string              `json:"revision"`
	Route           string              `json:"route"`
	Assignees       []RequirementPerson `json:"assignees"`
}

func requirementPeople(r *http.Request) ([]User, error) {
	rows, err := storeDB(r).Query("SELECT id FROM users WHERE org_id=? AND disabled=0 ORDER BY name,id", currentUser(r).OrgID)
	if err != nil {
		return nil, err
	}
	ids := []string{}
	for rows.Next() {
		var id string
		if err = rows.Scan(&id); err != nil {
			break
		}
		ids = append(ids, id)
	}
	if err == nil {
		err = rows.Err()
	}
	rows.Close()
	if err != nil {
		return nil, err
	}
	people := []User{}
	for _, id := range ids {
		if u, ok := committedUser(r, "id", id); ok {
			people = append(people, u)
		}
	}
	return people, nil
}
func documentChecklist(r *http.Request, c Case, docs []Document, today string) ([]RequirementChecklist, int, error) {
	people, err := requirementPeople(r)
	if err != nil {
		return nil, 0, err
	}
	rows := []RequirementChecklist{}
	complete := 0
	for _, doc := range docs {
		if !doc.Required {
			continue
		}
		var record map[string]any
		json.Unmarshal([]byte(mustJSON(doc)), &record)
		row := RequirementChecklist{ID: doc.ID, Title: doc.DocName, Status: doc.Status, Complete: doc.FilePath != "" && !documentNeedsReview(doc, today), ResponsibleID: doc.ResponsibleID, Responsible: c.AssignedOfficer, DueDate: doc.DueDate, DocumentDueDate: doc.DueDate, DeadlineSource: "Document deadline", Editable: canWrite(doc.RecordScope, currentUser(r)) && canWrite(c.RecordScope, currentUser(r)), Revision: versionOf(record), Route: "/cases/" + c.ID + "#record-tools?tab=documents&document=" + doc.ID, Assignees: []RequirementPerson{}}
		row.Attention, _, _ = documentIssue(doc, today)
		row.ExpiryDate = doc.ExpiryDate
		row.Missing = doc.FilePath == ""
		day, _ := time.Parse("2006-01-02", today)
		row.Expiring = doc.ExpiryDate != "" && doc.ExpiryDate >= today && doc.ExpiryDate <= day.AddDate(0, 0, 30).Format("2006-01-02")
		if row.DueDate == "" {
			row.DueDate = c.NextDeadline
			row.DeadlineSource = "Application next-action deadline"
		}
		if row.DueDate == "" {
			row.DeadlineSource = "No deadline set"
		}
		if row.Responsible == "" {
			row.Responsible = "Unassigned"
		}
		if row.ResponsibleID != "" {
			row.Responsible = "Assigned staff unavailable"
		}
		for _, u := range people {
			if row.ResponsibleID == u.ID {
				row.Responsible = u.Name
			}
			if row.ResponsibleID == "" && c.AssignedOfficer == "" && u.ID == doc.OwnerID {
				row.Responsible = u.Name
			}
			if u.Role != "viewer" && canRead(doc.RecordScope, u) && canRead(c.RecordScope, u) {
				row.Assignees = append(row.Assignees, RequirementPerson{u.ID, u.Name})
			}
		}
		row.Overdue = !row.Complete && row.DueDate != "" && row.DueDate < today
		if row.Complete {
			complete++
		}
		rows = append(rows, row)
	}
	sort.SliceStable(rows, func(i, j int) bool {
		if rows[i].Complete != rows[j].Complete {
			return !rows[i].Complete
		}
		if rows[i].Overdue != rows[j].Overdue {
			return rows[i].Overdue
		}
		return rows[i].Title < rows[j].Title
	})
	return rows, complete, nil
}
func handleRequirementPlan(w http.ResponseWriter, r *http.Request) {
	state, release := runtimeForCall(r)
	defer release()
	var in struct {
		ResponsibleID string `json:"responsibleId"`
		DueDate       string `json:"dueDate"`
	}
	if !decodeRequest(w, r, &in) {
		return
	}
	in.ResponsibleID = strings.TrimSpace(in.ResponsibleID)
	in.DueDate = strings.TrimSpace(in.DueDate)
	if in.DueDate != "" {
		if _, err := time.Parse("2006-01-02", in.DueDate); err != nil {
			writeError(w, 400, "Choose a valid requirement deadline")
			return
		}
	}
	if r.Header.Get("If-Match") == "" {
		writeError(w, 428, "Refresh the requirements before saving")
		return
	}
	state.mu.Lock()
	defer state.mu.Unlock()
	for i, doc := range state.documents {
		if doc.ID != r.PathValue("id") {
			continue
		}
		c, ok := state.findCaseByID(doc.CaseID)
		if !ok || !doc.Required {
			writeError(w, 404, "Application requirement not found")
			return
		}
		if !canWrite(doc.RecordScope, currentUser(r)) || !canWrite(c.RecordScope, currentUser(r)) {
			writeError(w, 403, "You must be able to edit the application and requirement")
			return
		}
		if in.ResponsibleID != "" {
			u, ok := committedUser(r, "id", in.ResponsibleID)
			if !ok || u.OrgID != doc.OrgID || u.Disabled || u.Role == "viewer" || !canRead(doc.RecordScope, u) || !canRead(c.RecordScope, u) {
				writeError(w, 400, "Choose active staff who can access this requirement")
				return
			}
		}
		state.documents[i].ResponsibleID = in.ResponsibleID
		state.documents[i].DueDate = in.DueDate
		writeJSON(w, 200, map[string]any{"document": state.documents[i]})
		return
	}
	writeError(w, 404, "Requirement not found")
}
