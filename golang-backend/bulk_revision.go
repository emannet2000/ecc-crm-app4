package main

import "net/http"

func checkBulkRevision(w http.ResponseWriter, id string, revisions map[string]string, record map[string]any) bool {
	if revisions[id] == "" {
		writeError(w, 428, "Load and review every selected record before the bulk change")
		return false
	}
	if revisions[id] != versionOf(record) {
		writeError(w, 409, "A selected record changed. No records were updated; refresh and review before retrying.")
		return false
	}
	return true
}

func handleBulkDeals(w http.ResponseWriter, r *http.Request) {
	var in struct {
		IDs       []string          `json:"ids"`
		Revisions map[string]string `json:"revisions"`
		Stage     string            `json:"stage"`
		Delete    bool              `json:"delete"`
	}
	if !decodeRequest(w, r, &in) {
		return
	}
	if len(in.IDs) < 1 || len(in.IDs) > 500 {
		writeError(w, 400, "Select 1–500 deals")
		return
	}
	if !in.Delete && !isValidDealStage(in.Stage) {
		writeError(w, 400, "Choose a valid deal stage")
		return
	}
	state, done := runtimeForCall(r)
	defer done()
	state.mu.Lock()
	defer state.mu.Unlock()
	selected := map[string]bool{}
	for _, id := range in.IDs {
		var found *Deal
		for i := range state.deals {
			if state.deals[i].ID == id {
				found = &state.deals[i]
				break
			}
		}
		if found == nil || !canWrite(found.RecordScope, currentUser(r)) {
			writeError(w, 403, "All selected deals must be editable")
			return
		}
		if !checkBulkRevision(w, id, in.Revisions, recordMap(*found)) {
			return
		}
		selected[id] = true
	}
	kept := make([]Deal, 0, len(state.deals))
	var first Deal
	for _, d := range state.deals {
		if selected[d.ID] {
			if in.Delete {
				continue
			}
			if d.Stage != in.Stage {
				d.StageHistory = append(d.StageHistory, StageChange{d.Stage, in.Stage, currentUser(r).Name, utcNow()})
			}
			d.Stage = in.Stage
			if first.ID == "" {
				first = d
			}
		}
		kept = append(kept, d)
	}
	state.deals = kept
	if in.Delete {
		activities := make([]Activity, 0, len(state.activities))
		for _, a := range state.activities {
			if !selected[a.DealID] {
				activities = append(activities, a)
			}
		}
		state.activities = activities
	}
	writeJSON(w, 200, map[string]any{"changed": len(selected), "deal": first, "deleted": "selected deals"})
}
