package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestDealBoardIncludesEveryAccessibleRecord(t *testing.T) {
	setupStore(t)
	base := deals[0]
	deals = nil
	for i := 0; i < 275; i++ {
		d := base
		d.ID = fmt.Sprintf("board_%03d", i)
		d.Title = d.ID
		deals = append(deals, d)
	}
	hidden := base
	hidden.ID = "other_workspace"
	hidden.OrgID = "other_org"
	deals = append(deals, hidden)
	h := expansionTestMux()
	for _, tc := range []struct {
		path  string
		count int
	}{{"/api/deals?limit=25", 25}, {"/api/deals?board=all&limit=25", 275}, {"/api/deals?board=all&q=board_274", 1}} {
		w := expansionRequest(t, h, http.MethodGet, tc.path, nil, "")
		assertStatus(t, w, 200)
		var out struct {
			Deals []Deal
			Total int
		}
		json.Unmarshal(w.Body.Bytes(), &out)
		if len(out.Deals) != tc.count {
			t.Fatalf("%s returned %d deals", tc.path, len(out.Deals))
		}
		for _, d := range out.Deals {
			if d.ID == hidden.ID {
				t.Fatal("board leaked another workspace")
			}
		}
	}
}
func TestBulkContactAndDealVersionsAreAtomic(t *testing.T) {
	setupStore(t)
	h := expansionTestMux()
	for _, entity := range []string{"contacts", "deals"} {
		records := recordsOf(cloneStore())[entity]
		ids := []string{}
		versions := map[string]string{}
		for id, payload := range records {
			if len(ids) == 2 {
				break
			}
			var record map[string]any
			json.Unmarshal([]byte(payload), &record)
			ids = append(ids, id)
			versions[id] = versionOf(record)
		}
		stage := "Client"
		if entity == "deals" {
			stage = "Won"
		}
		body := map[string]any{"ids": ids, "stage": stage}
		assertStatus(t, expansionRequest(t, h, "POST", "/api/"+entity+"/bulk", body, ""), 428)
		body["revisions"] = versions
		old := versions[ids[1]]
		versions[ids[1]] = "stale"
		before := mustJSON(recordsOf(cloneStore())[entity])
		assertStatus(t, expansionRequest(t, h, "POST", "/api/"+entity+"/bulk", body, ""), 409)
		if mustJSON(recordsOf(cloneStore())[entity]) != before {
			t.Fatal("stale bulk request changed a record")
		}
		versions[ids[1]] = old
		assertStatus(t, expansionRequest(t, h, "POST", "/api/"+entity+"/bulk", body, ""), 200)
		for _, id := range ids {
			var record map[string]any
			json.Unmarshal([]byte(recordsOf(cloneStore())[entity][id]), &record)
			if record["stage"] != stage {
				t.Fatal("successful bulk update omitted record")
			}
		}
	}
}
func insertOldTrash(t *testing.T, org, id, doc string, restored bool) {
	t.Helper()
	date := time.Now().UTC().AddDate(0, 0, -40).Format(time.RFC3339)
	var marker any
	if restored {
		marker = utcNow()
	}
	data := mustJSON([]trashChange{{Entity: "documents", ID: doc, Before: mustJSON(map[string]any{"id": doc, "orgId": org}), After: ""}})
	if _, err := database.Exec("INSERT INTO trash_batches VALUES(?,?,?,?,?,?,?)", id, org, "fixture", "Old document", data, date, marker); err != nil {
		t.Fatal(err)
	}
}
func TestTrashPurgeReviewRollbackAndFileCleanup(t *testing.T) {
	setupStore(t)
	h := expansionTestMux()
	org := "org_default"
	doc := "deleted_document"
	insertOldTrash(t, org, "old", doc, false)
	insertOldTrash(t, "other_org", "other", "other_doc", false)
	dir := filepath.Join(uploadRoot(), org, doc)
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "private-file")
	os.WriteFile(path, []byte("private"), 0600)
	if _, err := database.Exec("INSERT INTO file_versions VALUES(?,?,?,?,?,?,?,?,?,?,?)", "version", org, doc, 1, "file.txt", path, "text/plain", 7, "sum", "staff", utcNow()); err != nil {
		t.Fatal(err)
	}
	preview := func() map[string]any {
		w := expansionRequest(t, h, "GET", "/api/trash/purge-preview", nil, "")
		assertStatus(t, w, 200)
		var out map[string]any
		json.Unmarshal(w.Body.Bytes(), &out)
		return out
	}
	p := preview()
	if p["count"] != float64(1) {
		t.Fatal("preview included another workspace")
	}
	assertStatus(t, expansionRequest(t, h, "POST", "/api/trash/purge", map[string]any{"confirmed": true}, ""), 428)
	database.Exec("UPDATE trash_batches SET restored_at=? WHERE id='old'", utcNow())
	assertStatus(t, expansionRequest(t, h, "POST", "/api/trash/purge", map[string]any{"confirmed": true, "revision": p["revision"]}, ""), 409)
	p = preview()
	database.Exec("CREATE TRIGGER reject_purge BEFORE INSERT ON security_events WHEN NEW.event='trash.purged' BEGIN SELECT RAISE(ABORT,'audit failure'); END")
	assertStatus(t, expansionRequest(t, h, "POST", "/api/trash/purge", map[string]any{"confirmed": true, "revision": p["revision"]}, ""), 500)
	var count int
	database.QueryRow("SELECT count(*) FROM trash_batches WHERE id='old'").Scan(&count)
	if count != 1 {
		t.Fatal("failed purge removed archive")
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatal("failed purge removed file")
	}
	database.Exec("DROP TRIGGER reject_purge")
	assertStatus(t, expansionRequest(t, h, "POST", "/api/trash/purge", map[string]any{"confirmed": true, "revision": p["revision"]}, ""), 200)
	collectPurgedFiles(context.Background())
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("purged document file remains")
	}
	database.QueryRow("SELECT count(*) FROM trash_batches WHERE id='other'").Scan(&count)
	if count != 1 {
		t.Fatal("purge touched another workspace")
	}
}
func TestTrashRetainsFilesReferencedByAnotherArchiveAndAutomaticPolicy(t *testing.T) {
	setupStore(t)
	h := expansionTestMux()
	org := "org_default"
	doc := "preserved_document"
	insertOldTrash(t, org, "old", doc, false)
	insertOldTrash(t, org, "new", doc, false)
	database.Exec("UPDATE trash_batches SET deleted_at=? WHERE id='new'", utcNow())
	path := filepath.Join(uploadRoot(), org, doc, "retained")
	os.MkdirAll(filepath.Dir(path), 0700)
	os.WriteFile(path, []byte("retained"), 0600)
	database.Exec("INSERT INTO file_versions VALUES(?,?,?,?,?,?,?,?,?,?,?)", "v", org, doc, 1, "file.txt", path, "text/plain", 8, "sum", "staff", utcNow())
	processTrashRetention(context.Background())
	var count int
	database.QueryRow("SELECT count(*) FROM trash_batches").Scan(&count)
	if count != 2 {
		t.Fatal("default policy purged trash")
	}
	assertStatus(t, expansionRequest(t, h, "PUT", "/api/trash/policy", map[string]any{"days": 30, "automatic": true}, ""), 200)
	processTrashRetention(context.Background())
	database.QueryRow("SELECT count(*) FROM trash_batches").Scan(&count)
	if count != 1 {
		t.Fatal("automatic policy did not purge old archive")
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatal("removed file needed by remaining archive")
	}
	database.Exec("UPDATE trash_batches SET deleted_at=? WHERE id='new'", time.Now().UTC().AddDate(0, 0, -40).Format(time.RFC3339))
	processTrashRetention(context.Background())
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("unreferenced file remains after final archive purge")
	}
}
func TestBulkAndTrashRejectViewers(t *testing.T) {
	setupStore(t)
	u, _ := findUserByEmail("demo@northwind.dev")
	u.Role = "viewer"
	users[u.Email] = u
	h := expansionTestMux()
	for _, path := range []string{"/api/contacts/bulk", "/api/deals/bulk", "/api/trash/purge"} {
		w := expansionRequest(t, h, "POST", path, map[string]any{}, "")
		if w.Code != 403 {
			t.Fatalf("viewer %s returned %d: %s", path, w.Code, w.Body.String())
		}
	}
}
