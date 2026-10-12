package main

import "net/http"

func handleDocumentReview(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Confirmed bool `json:"confirmed"`
	}
	if !decodeRequest(w, r, &in) {
		return
	}
	if !in.Confirmed {
		writeError(w, 400, "Open the document and confirm you reviewed this version.")
		return
	}
	if r.Header.Get("If-Match") == "" {
		writeError(w, 428, "Refresh the document before recording its review.")
		return
	}
	state, release := runtimeForCall(r)
	defer release()
	state.mu.Lock()
	defer state.mu.Unlock()
	for i, doc := range state.documents {
		if doc.ID != r.PathValue("id") {
			continue
		}
		if !canWrite(doc.RecordScope, currentUser(r)) {
			writeError(w, 403, "You must be able to edit this document to record its review.")
			return
		}
		version := 0
		err := storeDB(r).QueryRow("SELECT coalesce(max(version),0) FROM file_versions WHERE org_id=? AND document_id=?", doc.OrgID, doc.ID).Scan(&version)
		if err != nil || version == 0 {
			writeError(w, 409, "Upload a file version before recording its review.")
			return
		}
		state.documents[i].ReviewedByID = currentUser(r).ID
		state.documents[i].ReviewedBy = currentUser(r).Name
		state.documents[i].ReviewedAt = utcNow()
		state.documents[i].ReviewedVersion = version
		writeJSON(w, 200, map[string]any{"document": state.documents[i]})
		return
	}
	writeError(w, 404, "Document not found")
}
