package main

import (
	"encoding/csv"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLegacyMigrationPreservesAccountsAndRecords(t *testing.T) {
	setupStore(t)
	mu.Lock()
	contacts[0].Name = "Migrated name"
	mu.Unlock()
	payload, err := json.Marshal(snapshot())
	if err != nil {
		t.Fatal(err)
	}
	directory := t.TempDir()
	legacy := filepath.Join(directory, "crm.json")
	if err = os.WriteFile(legacy, payload, 0600); err != nil {
		t.Fatal(err)
	}
	user, _ := findUserByEmail("demo@northwind.dev")
	if err = loadStore(filepath.Join(directory, "crm.sqlite3")); err != nil {
		t.Fatal(err)
	}
	restored, _ := findUserByEmail(user.Email)
	if restored.Password != user.Password || contacts[0].Name != "Migrated name" {
		t.Fatal("migration lost data")
	}
	old, _ := os.ReadFile(legacy)
	if string(old) != string(payload) {
		t.Fatal("migration altered source")
	}
	mu.Lock()
	contacts[0].Name = "Database wins"
	mu.Unlock()
	if err = saveStore(); err != nil {
		t.Fatal(err)
	}
	if err = loadStore(dataPath); err != nil {
		t.Fatal(err)
	}
	if contacts[0].Name != "Database wins" {
		t.Fatal("legacy file was reimported")
	}
	var total int
	if err = database.QueryRow("SELECT count(*) FROM contacts").Scan(&total); err != nil || total != len(contacts) {
		t.Fatal("records are not stored individually")
	}
}

func TestMalformedLegacyFilePreventsSeeding(t *testing.T) {
	directory := t.TempDir()
	legacy := filepath.Join(directory, "crm.json")
	os.WriteFile(legacy, []byte("broken legacy file"), 0600)
	if err := loadStore(filepath.Join(directory, "crm.sqlite3")); err == nil {
		t.Fatal("must stop on migration error")
	}
	contents, _ := os.ReadFile(legacy)
	if string(contents) != "broken legacy file" {
		t.Fatal("legacy data was overwritten")
	}
}

func TestEmptyDatasetStaysEmpty(t *testing.T) {
	setupStore(t)
	mu.Lock()
	contacts = []Contact{}
	mu.Unlock()
	if err := saveStore(); err != nil {
		t.Fatal(err)
	}
	if err := loadStore(dataPath); err != nil {
		t.Fatal(err)
	}
	if len(contacts) != 0 {
		t.Fatal("empty dataset was reseeded")
	}
}

func authorizedRequest(t *testing.T, handler http.Handler, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	user, _ := findUserByEmail("demo@northwind.dev")
	token, err := issueToken(user)
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(method, path, strings.NewReader(body))
	request.Header.Set("Authorization", "Bearer "+token)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}

func workspaceMux() http.Handler {
	mux := http.NewServeMux()
	registerConsolidatedRoutes(mux)
	mux.HandleFunc("GET /api/search", authMiddleware(handleSearch))
	mux.HandleFunc("GET /api/audit", authMiddleware(handleAudit))
	mux.HandleFunc("GET /api/exports/{entity}", authMiddleware(handleExport))
	mux.HandleFunc("POST /api/contacts", authMiddleware(createContact))
	mux.HandleFunc("PUT /api/contacts/{id}", authMiddleware(updateContact))
	mux.HandleFunc("DELETE /api/contacts/{id}", authMiddleware(deleteContact))
	return persistMutations(mux)
}

func TestGlobalSearchAndLiteralWildcards(t *testing.T) {
	setupStore(t)
	handler := workspaceMux()
	response := authorizedRequest(t, handler, "GET", "/api/search?q=Ada", "")
	var data struct {
		Results   []SearchResult
		Truncated bool
	}
	json.Unmarshal(response.Body.Bytes(), &data)
	if response.Code != 200 || len(data.Results) < 2 {
		t.Fatalf("search failed: %s", response.Body.String())
	}
	found := false
	for _, result := range data.Results {
		if result.Entity == "contacts" && result.ID == "c_1" {
			found = true
		}
	}
	if !found {
		t.Fatal("contact missing from search")
	}
	for _, query := range []string{"%25%25", "__", "%27%20OR%201%3D1%20--"} {
		response = authorizedRequest(t, handler, "GET", "/api/search?q="+query, "")
		data.Results = nil
		json.Unmarshal(response.Body.Bytes(), &data)
		if response.Code != 200 || len(data.Results) != 0 {
			t.Fatalf("query should be literal: %s", response.Body.String())
		}
	}
}

func TestAuditCreateUpdateDeleteAndRestart(t *testing.T) {
	setupStore(t)
	handler := workspaceMux()
	response := authorizedRequest(t, handler, "POST", "/api/contacts", `{"name":"Tracked contact","email":"tracked@example.com","stage":"Lead"}`)
	if response.Code != 201 {
		t.Fatal(response.Body.String())
	}
	var created struct{ Contact Contact }
	json.Unmarshal(response.Body.Bytes(), &created)
	response = authorizedRequest(t, handler, "PUT", "/api/contacts/"+created.Contact.ID, `{"name":"Renamed contact","email":"tracked@example.com","stage":"Qualified"}`)
	if response.Code != 200 {
		t.Fatal(response.Body.String())
	}
	response = authorizedRequest(t, handler, "DELETE", "/api/contacts/"+created.Contact.ID, "")
	if response.Code < 200 || response.Code >= 300 {
		t.Fatal(response.Body.String())
	}
	if err := loadStore(dataPath); err != nil {
		t.Fatal(err)
	}
	response = authorizedRequest(t, handler, "GET", "/api/audit?limit=2", "")
	var history struct {
		Events []AuditEvent
		Total  int
	}
	json.Unmarshal(response.Body.Bytes(), &history)
	if response.Code != 200 || history.Total != 3 || len(history.Events) != 2 || history.Events[0].Action != "deleted" || history.Events[0].Actor != "demo@northwind.dev" {
		t.Fatalf("unexpected history: %s", response.Body.String())
	}
	response = authorizedRequest(t, handler, "GET", "/api/audit?limit=2&offset=2", "")
	json.Unmarshal(response.Body.Bytes(), &history)
	if len(history.Events) != 1 || history.Events[0].Action != "created" {
		t.Fatal("history pagination failed")
	}
}

func TestCSVExportsAllRowsAndNeutralizesFormulas(t *testing.T) {
	setupStore(t)
	mu.Lock()
	contacts[0].Name = "=HYPERLINK(\"evil\")"
	contacts[0].Notes = "Line one\nLine two, quoted \"value\""
	mu.Unlock()
	if err := saveStore(); err != nil {
		t.Fatal(err)
	}
	handler := workspaceMux()
	response := authorizedRequest(t, handler, "GET", "/api/exports/contacts", "")
	rows, err := csv.NewReader(strings.NewReader(response.Body.String())).ReadAll()
	if err != nil || response.Code != 200 || len(rows) != len(contacts)+1 {
		t.Fatalf("bad export: %s", response.Body.String())
	}
	columns := map[string]int{}
	for i, key := range rows[0] {
		columns[key] = i
	}
	if rows[1][columns["name"]] != "'=HYPERLINK(\"evil\")" || rows[1][columns["notes"]] != contacts[0].Notes {
		t.Fatal("CSV escaping failed")
	}
	response = authorizedRequest(t, handler, "GET", "/api/exports/users", "")
	if response.Code != 404 {
		t.Fatal("users must not be exported")
	}
}

func TestDatabaseTransactionFailureRollsBackHistory(t *testing.T) {
	setupStore(t)
	if _, err := database.Exec(`CREATE TRIGGER fail_audit BEFORE INSERT ON audit_events BEGIN SELECT RAISE(ABORT, 'test failure'); END`); err != nil {
		t.Fatal(err)
	}
	count := len(contacts)
	response := authorizedRequest(t, workspaceMux(), "POST", "/api/contacts", `{"name":"Rollback","email":"rollback@example.com","stage":"Lead"}`)
	if response.Code != 500 || len(contacts) != count {
		t.Fatal("memory was not rolled back")
	}
	var stored, events int
	database.QueryRow("SELECT count(*) FROM contacts").Scan(&stored)
	database.QueryRow("SELECT count(*) FROM audit_events").Scan(&events)
	if stored != count || events != 0 {
		t.Fatal("transaction was only partially committed")
	}
}

func TestWorkspaceEndpointsRequireAuthentication(t *testing.T) {
	setupStore(t)
	handler := workspaceMux()
	for _, path := range []string{"/api/search?q=Ada", "/api/audit", "/api/exports/contacts", "/api/communication", "/api/workspace-calendar?from=2026-11-01&to=2026-11-30", "/api/referral-directory"} {
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequest("GET", path, nil))
		if response.Code != 401 {
			t.Fatalf("unauthenticated access: %s", path)
		}
	}
}
