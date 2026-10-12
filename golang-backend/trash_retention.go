package main

import (
	"context"
	"database/sql"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type trashPolicy struct {
	Days      int  `json:"days"`
	Automatic bool `json:"automatic"`
}

func loadTrashPolicy(r *http.Request) (trashPolicy, error) {
	p := trashPolicy{Days: 30}
	var automatic int
	err := storeDB(r).QueryRow("SELECT days,automatic FROM trash_policy WHERE org_id=?", currentUser(r).OrgID).Scan(&p.Days, &automatic)
	if err == sql.ErrNoRows {
		return p, nil
	}
	p.Automatic = automatic != 0
	return p, err
}
func handleTrashPolicy(w http.ResponseWriter, r *http.Request) {
	if currentUser(r).Role != "admin" {
		writeError(w, 403, "Administrator access is required")
		return
	}
	var p trashPolicy
	if !decodeRequest(w, r, &p) {
		return
	}
	if p.Days < 1 || p.Days > 3650 {
		writeError(w, 400, "Retention must be 1–3650 days")
		return
	}
	_, err := storeDB(r).Exec("INSERT INTO trash_policy(org_id,days,automatic) VALUES(?,?,?) ON CONFLICT(org_id) DO UPDATE SET days=excluded.days,automatic=excluded.automatic", currentUser(r).OrgID, p.Days, p.Automatic)
	if err != nil {
		writeError(w, 500, "Could not save retention policy")
		return
	}
	if err = securityEvent(r, currentUser(r), "trash.policy_updated"); err != nil {
		writeError(w, 500, "Could not record policy change")
		return
	}
	writeJSON(w, 200, p)
}
func trashPreview(r *http.Request) (map[string]any, error) {
	p, err := loadTrashPolicy(r)
	if err != nil {
		return nil, err
	}
	cutoff := time.Now().UTC().AddDate(0, 0, -p.Days).Format(time.RFC3339)
	rows, err := queryObjects(r, "SELECT id,label,data,deleted_at AS deletedAt,restored_at AS restoredAt FROM trash_batches WHERE org_id=? AND deleted_at<=? ORDER BY id", currentUser(r).OrgID, cutoff)
	if err != nil {
		return nil, err
	}
	// Fingerprint the exact archived contents, not only the IDs: restore and policy changes invalidate review.
	fileRows, err := queryObjects(r, `SELECT id,size,storage_path AS path FROM file_versions WHERE org_id=? AND NOT EXISTS(SELECT 1 FROM documents WHERE documents.id=file_versions.document_id) AND NOT EXISTS(SELECT 1 FROM trash_batches,json_each(trash_batches.data) change WHERE NOT(trash_batches.org_id=? AND trash_batches.deleted_at<=?) AND json_extract(change.value,'$.entity')='documents' AND json_extract(change.value,'$.id')=file_versions.document_id) ORDER BY id`, currentUser(r).OrgID, currentUser(r).OrgID, cutoff)
	if err != nil {
		return nil, err
	}
	var bytes int64
	for _, file := range fileRows {
		if size, ok := file["size"].(int64); ok {
			bytes += size
		}
	}
	photoRows, err := queryObjects(r, `SELECT entity,record_id AS recordId FROM record_photos WHERE org_id=? AND NOT EXISTS(SELECT 1 FROM contacts WHERE record_photos.entity='contacts' AND contacts.id=record_photos.record_id) AND NOT EXISTS(SELECT 1 FROM students WHERE record_photos.entity='students' AND students.id=record_photos.record_id) AND NOT EXISTS(SELECT 1 FROM trash_batches,json_each(trash_batches.data) change WHERE NOT(trash_batches.org_id=? AND trash_batches.deleted_at<=?) AND json_extract(change.value,'$.entity')=record_photos.entity AND json_extract(change.value,'$.id')=record_photos.record_id) ORDER BY entity,record_id`, currentUser(r).OrgID, currentUser(r).OrgID, cutoff)
	if err != nil {
		return nil, err
	}
	revision := versionOf(map[string]any{"policy": p, "batches": rows, "files": fileRows, "photos": photoRows})
	labels := []map[string]any{}
	for _, row := range rows {
		labels = append(labels, map[string]any{"id": row["id"], "label": row["label"], "deletedAt": row["deletedAt"], "restoredAt": row["restoredAt"]})
	}
	return map[string]any{"policy": p, "batches": labels, "count": len(rows), "files": len(fileRows), "bytes": bytes, "photos": len(photoRows), "revision": revision}, nil
}
func handleTrashPreview(w http.ResponseWriter, r *http.Request) {
	if !isManager(currentUser(r)) {
		writeError(w, 403, "Manager access is required")
		return
	}
	out, err := trashPreview(r)
	if err != nil {
		writeError(w, 500, "Could not preview permanent deletion")
		return
	}
	writeJSON(w, 200, out)
}
func purgeTrash(r *http.Request, preview map[string]any) error {
	for _, b := range preview["batches"].([]map[string]any) {
		if _, err := storeDB(r).Exec("DELETE FROM trash_batches WHERE org_id=? AND id=?", currentUser(r).OrgID, b["id"]); err != nil {
			return err
		}
	}
	// Preserve files for every live document and every remaining archive, including other deletions of related records.
	orphan := `org_id=? AND NOT EXISTS(SELECT 1 FROM documents WHERE documents.id=file_versions.document_id) AND NOT EXISTS(SELECT 1 FROM trash_batches, json_each(trash_batches.data) change WHERE json_extract(change.value,'$.entity')='documents' AND json_extract(change.value,'$.id')=file_versions.document_id)`
	if _, err := storeDB(r).Exec("INSERT OR IGNORE INTO file_gc(path) SELECT storage_path FROM file_versions WHERE "+orphan, currentUser(r).OrgID); err != nil {
		return err
	}
	if _, err := storeDB(r).Exec("DELETE FROM file_versions WHERE "+orphan, currentUser(r).OrgID); err != nil {
		return err
	}
	_, err := storeDB(r).Exec(`DELETE FROM record_photos WHERE org_id=? AND NOT EXISTS(SELECT 1 FROM contacts WHERE record_photos.entity='contacts' AND contacts.id=record_photos.record_id) AND NOT EXISTS(SELECT 1 FROM students WHERE record_photos.entity='students' AND students.id=record_photos.record_id) AND NOT EXISTS(SELECT 1 FROM trash_batches,json_each(trash_batches.data) change WHERE json_extract(change.value,'$.entity')=record_photos.entity AND json_extract(change.value,'$.id')=record_photos.record_id)`, currentUser(r).OrgID)
	if err != nil {
		return err
	}
	if len(preview["batches"].([]map[string]any)) > 0 || preview["files"].(int) > 0 || preview["photos"].(int) > 0 {
		return securityEvent(r, currentUser(r), "trash.purged")
	}
	return nil
}
func handleTrashPurge(w http.ResponseWriter, r *http.Request) {
	if currentUser(r).Role != "admin" {
		writeError(w, 403, "Administrator access is required")
		return
	}
	var in struct {
		Revision  string `json:"revision"`
		Confirmed bool   `json:"confirmed"`
	}
	if !decodeRequest(w, r, &in) {
		return
	}
	if !in.Confirmed || in.Revision == "" {
		writeError(w, 428, "Preview and confirm permanent deletion first")
		return
	}
	preview, err := trashPreview(r)
	if err != nil {
		writeError(w, 500, "Could not preview permanent deletion")
		return
	}
	if in.Revision != preview["revision"] {
		writeError(w, 409, "Trash or its policy changed. Preview again before deleting.")
		return
	}
	if err = purgeTrash(r, preview); err != nil {
		writeError(w, 500, "Could not purge trash")
		return
	}
	writeJSON(w, 200, map[string]any{"purged": preview["count"]})
}

// Only committed GC rows are visible. Failed removals remain queued for a later worker pass.
func processTrashRetention(ctx context.Context) {
	rows, err := database.QueryContext(ctx, "SELECT org_id FROM trash_policy WHERE automatic=1")
	if err != nil {
		return
	}
	orgs := []string{}
	for rows.Next() {
		var org string
		if rows.Scan(&org) == nil {
			orgs = append(orgs, org)
		}
	}
	rows.Close()
	for _, org := range orgs {
		tx, err := database.BeginTx(ctx, nil)
		if err != nil {
			continue
		}
		r := httpRequestForWorker(ctx, tx)
		r = r.WithContext(context.WithValue(r.Context(), requestUserKey, User{OrgID: org, Role: "admin"}))
		err = lockSQLWriter(r)
		if err == nil {
			var p map[string]any
			p, err = trashPreview(r)
			if err == nil && p["policy"].(trashPolicy).Automatic {
				err = purgeTrash(r, p)
			}
		}
		if err == nil {
			err = tx.Commit()
		}
		if err != nil {
			tx.Rollback()
		}
	}
	collectPurgedFiles(ctx)
}
func collectPurgedFiles(ctx context.Context) {
	rows, err := database.QueryContext(ctx, "SELECT path FROM file_gc LIMIT 200")
	if err != nil {
		return
	}
	paths := []string{}
	for rows.Next() {
		var path string
		if rows.Scan(&path) == nil {
			paths = append(paths, path)
		}
	}
	rows.Close()
	root, err := filepath.EvalSymlinks(uploadRoot())
	if err != nil {
		return
	}
	for _, path := range paths {
		if ctx.Err() != nil {
			return
		}
		absolute, err := filepath.Abs(path)
		if err != nil {
			continue
		}
		parent, err := filepath.EvalSymlinks(filepath.Dir(absolute))
		if os.IsNotExist(err) {
			database.ExecContext(ctx, "DELETE FROM file_gc WHERE path=?", path)
			continue
		}
		if err != nil {
			continue
		}
		rel, err := filepath.Rel(root, parent)
		// Resolve the parent before removal so an archive path cannot escape through a symlink.
		if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			continue
		}
		var count int
		if database.QueryRowContext(ctx, "SELECT (SELECT count(*) FROM file_versions WHERE storage_path=?)+(SELECT count(*) FROM documents WHERE json_extract(data,'$.filePath')=?)", path, path).Scan(&count) != nil || count != 0 {
			continue
		}
		err = os.Remove(absolute)
		if err == nil || os.IsNotExist(err) {
			database.ExecContext(ctx, "DELETE FROM file_gc WHERE path=?", path)
		}
	}
}
