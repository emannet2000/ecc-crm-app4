package main

import (
	"fmt"
	"os"
	"testing"
)

// Opt-in synthetic data for complete-page browser measurements; no customer data.
func TestWriteRepresentativeBrowserFixture(t *testing.T) {
	path := os.Getenv("ECC_BROWSER_FIXTURE_DATABASE")
	if path == "" {
		t.Skip("browser fixture is opt-in")
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("fixture destination must be new")
	}
	jwtSecret = []byte("representative-fixture-only")
	t.Setenv("LEGACY_DATA_FILE", "")
	if err := loadStore(path); err != nil {
		t.Fatal(err)
	}
	defer func() { database.Close(); database = nil }()
	u, _ := findUserByEmail("demo@northwind.dev")
	scope := RecordScope{OrgID: u.OrgID, OwnerID: u.ID, Visibility: "organization"}
	mu.Lock()
	for i := 0; i < 5000; i++ {
		s := scope
		if i%5 == 0 {
			s.OrgID = "browser-other-org"
		}
		id := fmt.Sprintf("browser-%05d", i)
		contacts = append(contacts, Contact{RecordScope: s, ID: id, Name: "Browser fixture client", Email: fmt.Sprintf("browser-%d@example.com", i), Stage: "Lead"})
		cases = append(cases, Case{RecordScope: s, ID: "case-" + id, ClientID: id, CaseNumber: "APP-" + id, AssignedOfficer: u.Name, CurrentStage: "Documents Pending", NextAction: "Review passport", NextDeadline: "2026-10-15"})
		documents = append(documents, Document{RecordScope: s, ID: "doc-" + id, CaseID: "case-" + id, DocName: "Passport", Required: true, Status: "Requested"})
	}
	mu.Unlock()
	if err := saveStore(); err != nil {
		t.Fatal(err)
	}
}
