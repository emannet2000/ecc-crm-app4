package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestWorkPlansPersistAndRejectStaleUpdates(t *testing.T) {
	setupStore(t)
	h := leadIntakeMux()
	cookies := demoCookies(t, h)
	for _, entity := range []string{"leads", "deals", "cases", "students"} {
		var id string
		switch entity {
		case "leads":
			id = leads[0].ID
		case "deals":
			id = deals[0].ID
		case "cases":
			id = cases[0].ID
		case "students":
			id = students[0].ID
		}
		path := "/api/work-plans/" + entity + "/" + id
		w := featureRequest(h, "GET", path, nil, cookies)
		assertStatus(t, w, 200)
		var before map[string]any
		json.Unmarshal(w.Body.Bytes(), &before)
		revision := fmtString(before["revision"])
		body := map[string]any{"nextAction": "Request the missing certificate", "deadline": "2026-10-20"}
		assertStatus(t, featureRequest(h, "PUT", path, body, cookies), 428)
		assertStatus(t, leadWorkRequest(t, h, cookies, "PUT", path, body, revision), 200)
		assertStatus(t, leadWorkRequest(t, h, cookies, "PUT", path, map[string]any{"nextAction": "Stale overwrite", "deadline": "2026-10-21"}, revision), 409)
		w = featureRequest(h, "GET", path, nil, cookies)
		assertStatus(t, w, 200)
		var after map[string]any
		json.Unmarshal(w.Body.Bytes(), &after)
		if after["nextAction"] != body["nextAction"] || after["deadline"] != body["deadline"] || before["responsible"] != after["responsible"] {
			t.Fatal(w.Body.String())
		}
		assertStatus(t, leadWorkRequest(t, h, cookies, "PUT", path, map[string]any{"nextAction": "Call", "deadline": "2026-13-40"}, fmtString(after["revision"])), 400)
	}
	w := featureRequest(h, "GET", "/api/workspace-priorities?queue=missingPlans&limit=1", nil, cookies)
	assertStatus(t, w, 200)
	var queue struct {
		Items []PriorityRow
		Total int
	}
	json.Unmarshal(w.Body.Bytes(), &queue)
	if queue.Total < 1 || len(queue.Items) != 1 {
		t.Fatal(w.Body.String())
	}
	assertStatus(t, featureRequest(h, "GET", "/api/workspace-priorities?queue=unknown", nil, cookies), 400)
}
func TestClientHistoryLinksDocumentsPaymentsAndExcludesOtherClients(t *testing.T) {
	setupStore(t)
	h := leadIntakeMux()
	cookies := demoCookies(t, h)
	u, _ := findUserByEmail("demo@northwind.dev")
	scope := RecordScope{OrgID: u.OrgID, OwnerID: u.ID, Visibility: "organization"}
	mu.Lock()
	contacts = []Contact{{RecordScope: scope, ID: "client-history", Name: "Client"}, {RecordScope: scope, ID: "other-client", Name: "Other"}}
	cases = []Case{{RecordScope: scope, ID: "case-history", ClientID: "client-history", CaseNumber: "MY-CASE"}, {RecordScope: scope, ID: "other-case", ClientID: "other-client", CaseNumber: "OTHER-CASE"}}
	leads = []Lead{{RecordScope: scope, ID: "lead-history", Name: "Inquiry", ConvertedContactID: "client-history", Status: "Converted"}}
	documents = []Document{{RecordScope: scope, ID: "history-document", LeadID: "lead-history", DocName: "Client CV"}}
	invoices = []Invoice{{RecordScope: scope, ID: "invoice-history", ClientID: "client-history", CaseID: "case-history", InvoiceNumber: "MY-INVOICE"}}
	payments = []Payment{{RecordScope: scope, ID: "payment-history", InvoiceID: "invoice-history", Reference: "MY-RECEIPT", Amount: 100, Status: "posted"}}
	mu.Unlock()
	if err := saveStore(); err != nil {
		t.Fatal(err)
	}
	_, err := database.Exec("INSERT INTO conversations VALUES('mine',?,'client-history','case-history','email','in','mine','MY-MESSAGE','Body','received','2026-10-10')", u.OrgID)
	if err != nil {
		t.Fatal(err)
	}
	_, err = database.Exec("INSERT INTO conversations VALUES('other',?,'other-client','other-case','email','in','other','OTHER-MESSAGE','Private unrelated body','received','2026-10-10')", u.OrgID)
	if err != nil {
		t.Fatal(err)
	}
	w := featureRequest(h, "GET", "/api/contacts/client-history/history", nil, cookies)
	assertStatus(t, w, 200)
	for _, wanted := range []string{"MY-CASE", "MY-INVOICE", "MY-RECEIPT", "Client CV", "MY-MESSAGE"} {
		if !strings.Contains(w.Body.String(), wanted) {
			t.Fatalf("missing %s: %s", wanted, w.Body.String())
		}
	}
	for _, hidden := range []string{"OTHER-CASE", "OTHER-MESSAGE"} {
		if strings.Contains(w.Body.String(), hidden) {
			t.Fatal("unrelated client history exposed")
		}
	}
	w = featureRequest(h, "GET", "/api/contacts/client-history/history?kind=payment&limit=1", nil, cookies)
	assertStatus(t, w, 200)
	var out struct {
		Items []ClientHistoryItem
		Total int
	}
	json.Unmarshal(w.Body.Bytes(), &out)
	if out.Total != 1 || out.Items[0].Kind != "payment" {
		t.Fatal(w.Body.String())
	}
	assertStatus(t, featureRequest(h, "GET", "/api/contacts/absent/history", nil, cookies), 404)
}
func statementRequest(h http.Handler, cookies []*http.Cookie, raw string) *httptest.ResponseRecorder {
	r := httptest.NewRequest("POST", "/api/reconciliation/import", strings.NewReader(raw))
	r.Header.Set("Content-Type", "text/csv")
	for _, c := range cookies {
		r.AddCookie(c)
	}
	w := httptest.NewRecorder()
	serveFixtureRequest(h, w, r)
	return w
}
func TestStatementMatchingIsIdempotentAndNeverPostsAnotherPayment(t *testing.T) {
	setupStore(t)
	h := leadIntakeMux()
	cookies := demoCookies(t, h)
	u, _ := findUserByEmail("demo@northwind.dev")
	scope := RecordScope{OrgID: u.OrgID, OwnerID: u.ID, Visibility: "organization"}
	mu.Lock()
	invoices = []Invoice{{RecordScope: scope, ID: "reconcile-invoice", InvoiceNumber: "REC-1", Currency: "PHP", TotalFee: 1000, LedgerInitialized: true}}
	payments = []Payment{{RecordScope: scope, ID: "reconcile-receipt", InvoiceID: "reconcile-invoice", Amount: 1000, Currency: "PHP", Status: "posted", Reference: "RECEIPT-1", PaidOn: "2026-10-10"}}
	mu.Unlock()
	if err := saveStore(); err != nil {
		t.Fatal(err)
	}
	csv := "date,reference,currency,amount\n2026-10-10,BANK-1,PHP,1000\n2026-10-10,BANK-2,PHP,1000\n"
	assertStatus(t, statementRequest(h, cookies, csv), 200)
	w := statementRequest(h, cookies, csv)
	assertStatus(t, w, 200)
	if !strings.Contains(w.Body.String(), `"imported":0`) {
		t.Fatal(w.Body.String())
	}
	assertStatus(t, statementRequest(h, cookies, "date,reference,currency,amount\n2026-10-10,VALID,PHP,1000\n2026-10-10,INVALID,PHP,NaN\n"), 400)
	w = featureRequest(h, "GET", "/api/reconciliation?status=unmatched", nil, cookies)
	assertStatus(t, w, 200)
	var out struct {
		Items []BankTransaction
		Total int
	}
	json.Unmarshal(w.Body.Bytes(), &out)
	if out.Total != 2 {
		t.Fatal(w.Body.String())
	}
	a, b := out.Items[0], out.Items[1]
	path := "/api/reconciliation/" + a.ID + "/match"
	body := map[string]any{"paymentId": "reconcile-receipt"}
	assertStatus(t, featureRequest(h, "POST", path, body, cookies), 428)
	assertStatus(t, leadWorkRequest(t, h, cookies, "POST", path, body, a.Revision), 200)
	assertStatus(t, leadWorkRequest(t, h, cookies, "POST", "/api/reconciliation/"+b.ID+"/match", body, b.Revision), 409)
	assertStatus(t, leadWorkRequest(t, h, cookies, "POST", path, body, a.Revision), 409)
	if len(payments) != 1 {
		t.Fatal("matching created a duplicate payment")
	}
	w = featureRequest(h, "GET", "/api/workspace-priorities?queue=finance", nil, cookies)
	assertStatus(t, w, 200)
	var financial struct{ Finance map[string]map[string]float64 }
	json.Unmarshal(w.Body.Bytes(), &financial)
	if financial.Finance["PHP"]["received"] != 1000 || financial.Finance["PHP"]["outstanding"] != 0 {
		t.Fatal(w.Body.String())
	}
}
func TestProductivityRemindersUseAssignedAccountsAndSafeDocumentLinks(t *testing.T) {
	setupStore(t)
	u, _ := findUserByEmail("demo@northwind.dev")
	adviser := u
	adviser.ID = "assigned-adviser"
	adviser.Name = "Adviser"
	adviser.Role = "member"
	scope := RecordScope{OrgID: u.OrgID, OwnerID: u.ID, Visibility: "organization"}
	s := diskStore{Users: map[string]storedUser{u.ID: {User: u}, adviser.ID: {User: adviser}}, Deals: []Deal{{RecordScope: scope, ID: "reminder-deal", Title: "Opportunity", Owner: adviser.ID, NextDeadline: "2026-10-01", NextAction: "Call", Stage: "Lead"}}, Leads: []Lead{{RecordScope: scope, ID: "reminder-lead", AssignedTo: adviser.ID, Status: "New"}}, Documents: []Document{{RecordScope: scope, ID: "missing-cv", LeadID: "reminder-lead", Required: true, DocName: "CV"}}}
	day := func(string, int) string { return "2026-10-11" }
	if err := remindProductivity(database, s, day); err != nil {
		t.Fatal(err)
	}
	if err := remindProductivity(database, s, day); err != nil {
		t.Fatal(err)
	}
	var count int
	if err := database.QueryRow("SELECT count(*) FROM notifications WHERE user_id=? AND kind IN ('deal-action','missing-document')", adviser.ID).Scan(&count); err != nil || count != 2 {
		t.Fatalf("assigned reminder count %d: %v", count, err)
	}
	var route string
	if err := database.QueryRow("SELECT path FROM notifications WHERE kind='missing-document' AND user_id=?", adviser.ID).Scan(&route); err != nil || route != "/leads/reminder-lead#record-tools?tab=documents" {
		t.Fatalf("wrong document route %s: %v", route, err)
	}
	adviser.Disabled = true
	s.Users = map[string]storedUser{u.ID: {User: u}, adviser.ID: {User: adviser}}
	if responsibleRecipient(s, scope, adviser.ID) != "" {
		t.Fatal("disabled account got a reminder")
	}
}

func TestProductivityViewsRespectRecordVisibilityAndFinanceRoles(t *testing.T) {
	setupStore(t)
	h := leadIntakeMux()
	admin, _ := findUserByEmail("demo@northwind.dev")
	member := addPermissionUser(t, "member", admin.OrgID, "productivity-member")
	viewer := addPermissionUser(t, "viewer", admin.OrgID, "productivity-viewer")
	shared := RecordScope{OrgID: admin.OrgID, OwnerID: admin.ID, Visibility: "organization"}
	private := shared
	private.Visibility = "private"
	mu.Lock()
	contacts = []Contact{{RecordScope: shared, ID: "shared-contact", Name: "Shared client"}}
	cases = []Case{{RecordScope: private, ID: "hidden-case", ClientID: "shared-contact", CaseNumber: "HIDDEN-CASE", NextDeadline: "2020-01-01", NextAction: "Secret action"}}
	deals = []Deal{{RecordScope: shared, ID: "shared-deal", Title: "Shared opportunity", NextDeadline: "2020-01-01", NextAction: "Follow up", Owner: admin.ID}}
	invoices = []Invoice{{RecordScope: private, ID: "hidden-invoice", InvoiceNumber: "HIDDEN-INVOICE", Currency: "PHP", TotalFee: 9999, LedgerInitialized: true}}
	mu.Unlock()
	if err := saveStore(); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec("INSERT INTO conversations VALUES('private-history-message',?,'shared-contact','hidden-case','email','in','private-history','HIDDEN-MESSAGE','Private body','received','2026-10-10')", admin.OrgID); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"/api/contacts/shared-contact/history", "/api/workspace-priorities?queue=overdue", "/api/workspace-priorities?queue=finance"} {
		w := dailyUserRequest(t, h, member, "GET", path, nil)
		assertStatus(t, w, 200)
		for _, hidden := range []string{"HIDDEN-CASE", "HIDDEN-MESSAGE", "HIDDEN-INVOICE", "9999"} {
			if strings.Contains(w.Body.String(), hidden) {
				t.Fatalf("private record leaked at %s: %s", path, w.Body.String())
			}
		}
	}
	assertStatus(t, dailyUserRequest(t, h, member, "GET", "/api/work-plans/cases/hidden-case", nil), 404)
	assertStatus(t, dailyUserRequest(t, h, member, "GET", "/api/reconciliation", nil), 403)
	assertStatus(t, dailyUserRequest(t, h, viewer, "GET", "/api/work-plans/deals/shared-deal", nil), 200)
	assertStatus(t, dailyUserRequest(t, h, viewer, "PUT", "/api/work-plans/deals/shared-deal", map[string]any{"nextAction": "Overwrite", "deadline": "2026-10-20"}), 403)
}
func TestDocumentPreviewRetainsPrivateAccessAndDownloadDefault(t *testing.T) {
	setupStore(t)
	h := leadIntakeMux()
	cookies := demoCookies(t, h)
	doc := documents[0]
	assertStatus(t, multipartRequest(h, "/api/documents/"+doc.ID+"/files", "file", "certificate.txt", []byte("Certificate details"), cookies, nil), 201)
	path := "/api/documents/" + doc.ID + "/files/1"
	download := featureRequest(h, "GET", path, nil, cookies)
	assertStatus(t, download, 200)
	if !strings.HasPrefix(download.Header().Get("Content-Disposition"), "attachment") {
		t.Fatal("default file route no longer downloads")
	}
	preview := featureRequest(h, "GET", path+"?preview=1", nil, cookies)
	assertStatus(t, preview, 200)
	if !strings.HasPrefix(preview.Header().Get("Content-Disposition"), "inline") || preview.Header().Get("X-Content-Type-Options") != "nosniff" || !strings.Contains(preview.Header().Get("Content-Security-Policy"), "sandbox") {
		t.Fatal("preview headers missing")
	}
	assertStatus(t, featureRequest(h, "GET", path+"?preview=1", nil, nil), 401)
}
