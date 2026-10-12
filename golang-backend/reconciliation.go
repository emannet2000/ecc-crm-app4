package main

import (
	"crypto/sha256"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"
)

type BankTransaction struct {
	ID        string  `json:"id"`
	Date      string  `json:"date"`
	Reference string  `json:"reference"`
	Currency  string  `json:"currency"`
	Amount    float64 `json:"amount"`
	PaymentID string  `json:"paymentId"`
	InvoiceID string  `json:"invoiceId"`
	MatchedBy string  `json:"matchedBy"`
	MatchedAt string  `json:"matchedAt"`
	Revision  string  `json:"revision,omitempty"`
}

func registerReconciliationRoutes(m *http.ServeMux) {
	m.HandleFunc("GET /api/reconciliation", authMiddleware(handleReconciliation))
	m.HandleFunc("POST /api/reconciliation/import", authMiddleware(dailyOrigin(handleReconciliationImport)))
	m.HandleFunc("POST /api/reconciliation/{id}/match", authMiddleware(dailyOrigin(handleReconciliationMatch)))
}
func financeReviewer(w http.ResponseWriter, r *http.Request) bool {
	if !financeCapability(r, "reconcile") {
		writeError(w, 403, "Finance reconciliation permission is required")
		return false
	}
	return true
}
func handleReconciliation(w http.ResponseWriter, r *http.Request) {
	if !financeReviewer(w, r) {
		return
	}
	r, s, close, ok := dailySnapshot(w, r)
	if !ok {
		return
	}
	defer close()
	rows, err := queryObjects(r, "SELECT id,data FROM workspace_entries WHERE org_id=? AND category='bank_transaction'", currentUser(r).OrgID)
	if err != nil {
		writeError(w, 503, "Could not load statement transactions")
		return
	}
	invoices := map[string]Invoice{}
	for _, i := range s.Invoices {
		invoices[i.ID] = i
	}
	used := map[string]bool{}
	all := []BankTransaction{}
	unmatched := 0
	for _, row := range rows {
		var b BankTransaction
		if json.Unmarshal([]byte(fmtString(row["data"])), &b) != nil {
			writeError(w, 503, "Could not read a statement transaction")
			return
		}
		b.ID = fmtString(row["id"])
		b.Revision = versionOf(recordMap(b))
		if b.PaymentID != "" {
			used[b.PaymentID] = true
		} else {
			unmatched++
		}
		all = append(all, b)
	}
	candidates := []Payment{}
	for _, p := range s.Payments {
		if receiptActive(p) && !pendingReceipt(p) && !used[p.ID] {
			if inv, ok := invoices[p.InvoiceID]; ok {
				p.Currency = currencyOf(inv.Currency)
				candidates = append(candidates, p)
			}
		}
	}
	sort.Slice(all, func(i, j int) bool {
		if all[i].Date == all[j].Date {
			return all[i].ID < all[j].ID
		}
		return all[i].Date > all[j].Date
	})
	filtered := []BankTransaction{}
	filter := r.URL.Query().Get("status")
	for _, b := range all {
		if filter == "" || filter == "unmatched" && b.PaymentID == "" || filter == "matched" && b.PaymentID != "" {
			filtered = append(filtered, b)
		}
	}
	limit, offset := parseLimitOffset(r, 25, 0)
	writeJSON(w, 200, map[string]any{"items": slicePage(filtered, offset, limit), "total": len(filtered), "unmatched": unmatched, "receipts": candidates, "limit": limit, "offset": offset})
}
func parseStatement(raw string) ([]BankTransaction, error) {
	reader := csv.NewReader(strings.NewReader(raw))
	records, err := reader.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("Read a valid CSV statement: %w", err)
	}
	if len(records) < 2 || len(records) > 1001 {
		return nil, fmt.Errorf("Use a statement containing 1 to 1,000 transactions")
	}
	columns := map[string]int{}
	for index, name := range records[0] {
		name = strings.ToLower(strings.TrimSpace(strings.TrimPrefix(name, "\ufeff")))
		if _, exists := columns[name]; exists {
			return nil, fmt.Errorf("Duplicate column %s", name)
		}
		columns[name] = index
	}
	for _, name := range []string{"date", "reference", "currency", "amount"} {
		if _, ok := columns[name]; !ok {
			return nil, fmt.Errorf("CSV needs date, reference, currency and amount columns")
		}
	}
	out := []BankTransaction{}
	for index, row := range records[1:] {
		get := func(name string) string { return strings.TrimSpace(row[columns[name]]) }
		amount, err := strconv.ParseFloat(get("amount"), 64)
		if err != nil || math.IsNaN(amount) || math.IsInf(amount, 0) || amount <= 0 || amount > 1e12 {
			return nil, fmt.Errorf("Row %d: enter a positive received amount", index+2)
		}
		if money(amount) != amount {
			return nil, fmt.Errorf("Row %d: amount must have no more than two decimal places", index+2)
		}
		date := get("date")
		if _, err = time.Parse("2006-01-02", date); err != nil {
			return nil, fmt.Errorf("Row %d: date must be YYYY-MM-DD", index+2)
		}
		currency := strings.ToUpper(get("currency"))
		reference := get("reference")
		if !validCurrency(currency) || reference == "" || len(reference) > 200 {
			return nil, fmt.Errorf("Row %d: use a supported currency and reference of up to 200 characters", index+2)
		}
		out = append(out, BankTransaction{Date: date, Reference: reference, Currency: currency, Amount: amount})
	}
	return out, nil
}
func handleReconciliationImport(w http.ResponseWriter, r *http.Request) {
	if !financeReviewer(w, r) {
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 2<<20)
	raw, err := io.ReadAll(r.Body)
	if err != nil {
		writeError(w, 400, "Statement must be smaller than 2 MB")
		return
	}
	items, err := parseStatement(string(raw))
	if err != nil {
		writeError(w, 400, err.Error())
		return
	}
	inserted := 0
	org := currentUser(r).OrgID
	for _, b := range items {
		hash := sha256.Sum256([]byte(org + "\x00" + b.Date + "\x00" + b.Reference + "\x00" + b.Currency + "\x00" + formatMoney(b.Amount)))
		b.ID = "bank_" + hex.EncodeToString(hash[:])
		result, err := storeDB(r).Exec("INSERT OR IGNORE INTO workspace_entries VALUES(?,?,'bank_transaction',?,'',?,?,?)", b.ID, org, currentUser(r).ID, mustJSON(b), utcNow(), utcNow())
		if err != nil {
			writeError(w, 503, "Statement import failed; retry the same file safely")
			return
		}
		count, _ := result.RowsAffected()
		inserted += int(count)
	}
	writeJSON(w, 200, map[string]any{"imported": inserted, "duplicates": len(items) - inserted})
}
func handleReconciliationMatch(w http.ResponseWriter, r *http.Request) {
	if !financeReviewer(w, r) {
		return
	}
	var in struct {
		PaymentID string `json:"paymentId"`
	}
	if !decodeRequest(w, r, &in) {
		return
	}
	var raw string
	org := currentUser(r).OrgID
	id := r.PathValue("id")
	if storeDB(r).QueryRow("SELECT data FROM workspace_entries WHERE id=? AND org_id=? AND category='bank_transaction'", id, org).Scan(&raw) != nil {
		writeError(w, 404, "Transaction not found")
		return
	}
	var b BankTransaction
	if json.Unmarshal([]byte(raw), &b) != nil {
		writeError(w, 503, "Could not read transaction")
		return
	}
	if r.Header.Get("If-Match") == "" {
		writeError(w, 428, "Refresh reconciliation before matching")
		return
	}
	b.Revision = ""
	if strings.Trim(r.Header.Get("If-Match"), "\"") != versionOf(recordMap(b)) {
		writeError(w, 409, "Transaction changed; refresh before matching")
		return
	}
	if b.PaymentID != "" {
		writeError(w, 409, "This transaction is already matched")
		return
	}
	record, err := sqlRecord(r, "payments", in.PaymentID)
	if err != nil {
		writeError(w, 404, "Receipt not found")
		return
	}
	var p Payment
	json.Unmarshal([]byte(mustJSON(record)), &p)
	invoice, err := sqlRecord(r, "invoices", p.InvoiceID)
	if err != nil {
		writeError(w, 404, "Linked invoice not found")
		return
	}
	if !receiptActive(p) || pendingReceipt(p) || money(p.Amount) != b.Amount || currencyOf(fmtString(invoice["currency"])) != b.Currency {
		writeError(w, 400, "Choose a posted receipt with the same amount and currency")
		return
	}
	var used int
	if err = storeDB(r).QueryRow("SELECT count(*) FROM workspace_entries WHERE org_id=? AND category='bank_transaction' AND json_extract(data,'$.paymentId')=?", org, p.ID).Scan(&used); err != nil {
		writeError(w, 503, "Could not check receipt matching")
		return
	}
	if used > 0 {
		writeError(w, 409, "This receipt is already matched to another transaction")
		return
	}
	b.PaymentID = p.ID
	b.InvoiceID = p.InvoiceID
	b.MatchedBy = currentUser(r).ID
	b.MatchedAt = utcNow()
	if _, err = storeDB(r).Exec("UPDATE workspace_entries SET data=?,updated_at=? WHERE id=? AND org_id=? AND data=?", mustJSON(b), utcNow(), id, org, raw); err != nil {
		writeError(w, 503, "Could not save match; your selection is kept")
		return
	}
	writeJSON(w, 200, map[string]any{"matched": true})
}
