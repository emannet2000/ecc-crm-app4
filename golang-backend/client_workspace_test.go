package main

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

func TestClientWorkspaceFollowsExplicitLinksAndRecordPermissions(t *testing.T) {
	setupStore(t)
	h := leadIntakeMux()
	admin, _ := findUserByEmail("demo@northwind.dev")
	member := addPermissionUser(t, "member", admin.OrgID, "client-workspace-member")
	viewer := addPermissionUser(t, "viewer", admin.OrgID, "client-workspace-viewer")
	shared := RecordScope{OrgID: admin.OrgID, OwnerID: member.ID, Visibility: "organization"}
	private := RecordScope{OrgID: admin.OrgID, OwnerID: admin.ID, Visibility: "private"}
	foreign := RecordScope{OrgID: "another-workspace", OwnerID: member.ID, Visibility: "organization"}
	mu.Lock()
	contacts = []Contact{{RecordScope: shared, ID: "cw-client", Name: "Client"}, {RecordScope: shared, ID: "cw-other", Name: "Client", Email: "same@example.com"}}
	students = []Student{{RecordScope: shared, ID: "cw-student", Name: "Linked student"}}
	leads = []Lead{{RecordScope: shared, ID: "cw-lead", Name: "Linked inquiry", ConvertedContactID: "cw-client", ConvertedStudentID: "cw-student", ConvertedCaseID: "cw-case"}, {RecordScope: shared, ID: "cw-unrelated-lead", Name: "Client", Email: "same@example.com"}}
	cases = []Case{{RecordScope: shared, ID: "cw-case", ClientID: "cw-client", StudentID: "cw-student", CaseNumber: "LINKED-APPLICATION"}, {RecordScope: private, ID: "cw-hidden", ClientID: "cw-client", CaseNumber: "PRIVATE-APPLICATION"}, {RecordScope: foreign, ID: "cw-foreign", ClientID: "cw-client", CaseNumber: "FOREIGN-APPLICATION"}, {RecordScope: shared, ID: "cw-unrelated", ClientID: "cw-other", CaseNumber: "UNRELATED-APPLICATION"}}
	documents = []Document{{RecordScope: shared, ID: "cw-doc", CaseID: "cw-case", DocName: "Linked document"}, {RecordScope: shared, ID: "cw-hidden-doc", CaseID: "cw-hidden", LeadID: "cw-lead", DocName: "Hidden parent document"}}
	invoices = []Invoice{{RecordScope: shared, ID: "cw-invoice", ClientID: "cw-client", CaseID: "cw-case", InvoiceNumber: "LINKED-INVOICE"}}
	mu.Unlock()
	if err := saveStore(); err != nil {
		t.Fatal(err)
	}
	for _, anchor := range []string{"contacts/cw-client", "students/cw-student", "leads/cw-lead", "cases/cw-case"} {
		w := dailyUserRequest(t, h, member, "GET", "/api/records/"+anchor+"/workspace", nil)
		assertStatus(t, w, 200)
		for _, wanted := range []string{"Linked inquiry", "Linked student", "LINKED-APPLICATION", "Linked document", "LINKED-INVOICE", `"_editable":true`} {
			if !strings.Contains(w.Body.String(), wanted) {
				t.Fatalf("%s missing %s: %s", anchor, wanted, w.Body.String())
			}
		}
		for _, hidden := range []string{"PRIVATE-APPLICATION", "FOREIGN-APPLICATION", "UNRELATED-APPLICATION", "cw-unrelated-lead", "Hidden parent document"} {
			if strings.Contains(w.Body.String(), hidden) {
				t.Fatalf("%s exposed %s", anchor, hidden)
			}
		}
	}
	w := dailyUserRequest(t, h, viewer, "GET", "/api/records/contacts/cw-client/workspace", nil)
	assertStatus(t, w, 200)
	if strings.Contains(w.Body.String(), `"_editable":true`) {
		t.Fatal("viewer was offered an editable record")
	}
	assertStatus(t, dailyUserRequest(t, h, member, "GET", "/api/records/cases/cw-hidden/workspace", nil), 404)
	assertStatus(t, dailyUserRequest(t, h, member, "GET", "/api/records/contacts/missing/workspace", nil), 404)
}

func TestRecordCapabilitiesDoNotChangeRevisionAcrossReaders(t *testing.T) {
	setupStore(t)
	mux := http.NewServeMux()
	registerExpansionRoutes(mux)
	mux.HandleFunc("GET /api/contacts/{id}", authMiddleware(getContact))
	h := persistMutations(mux)
	admin, _ := findUserByEmail("demo@northwind.dev")
	member := addPermissionUser(t, "member", admin.OrgID, "capability-member")
	viewer := addPermissionUser(t, "viewer", admin.OrgID, "capability-viewer")
	mu.Lock()
	contacts = []Contact{{RecordScope: RecordScope{OrgID: admin.OrgID, OwnerID: member.ID, Visibility: "organization"}, ID: "capability-contact", Name: "Capability client"}}
	mu.Unlock()
	if err := saveStore(); err != nil {
		t.Fatal(err)
	}
	var revision string
	for _, user := range []User{admin, member, viewer} {
		for _, path := range []string{"/api/contacts/capability-contact", "/api/records/contacts/capability-contact"} {
			w := dailyUserRequest(t, h, user, "GET", path, nil)
			assertStatus(t, w, 200)
			var payload struct {
				Contact map[string]any `json:"contact"`
				Record  map[string]any `json:"record"`
			}
			if err := json.Unmarshal(w.Body.Bytes(), &payload); err != nil {
				t.Fatal(err)
			}
			record := payload.Contact
			if record == nil {
				record = payload.Record
			}
			if record["_editable"] != (user.Role != "viewer") {
				t.Fatal(w.Body.String())
			}
			if revision == "" {
				revision = fmtString(record["_revision"])
			}
			if revision == "" || revision != fmtString(record["_revision"]) || versionOf(record) != revision {
				t.Fatal("capability metadata changed the record version")
			}
		}
	}
}
