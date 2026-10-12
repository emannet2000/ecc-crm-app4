package main

import (
	"encoding/json"
	"net/http"
)

func overdueHandoffPriorities(r *http.Request, s diskStore, today string) ([]PriorityRow, error) {
	visible := map[string]Case{}
	for _, c := range s.Cases {
		visible[c.ID] = c
	}
	rows, err := storeDB(r).Query("SELECT data FROM workspace_entries WHERE org_id=? AND category='application_handoff'", currentUser(r).OrgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []PriorityRow{}
	for rows.Next() {
		var raw string
		var h ApplicationHandoff
		if err = rows.Scan(&raw); err != nil {
			return nil, err
		}
		if err = json.Unmarshal([]byte(raw), &h); err != nil {
			return nil, err
		}
		c, ok := visible[h.CaseID]
		if !ok || h.Status != "pending" || h.DueDate == "" || h.DueDate >= today {
			continue
		}
		out = append(out, PriorityRow{h.ID, "cases", c.CaseNumber, "Review the handoff; current owner remains responsible until acceptance", h.FromName, h.DueDate, "Overdue handoff awaiting " + h.ToName, "/cases/" + c.ID + "#handoff"})
	}
	return out, rows.Err()
}
