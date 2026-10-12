package main

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

// Managers see workspace delivery counts and backup health, without deployment
// paths, credentials, message contents or another workspace's delivery failures.
func handleOperationalAttention(w http.ResponseWriter, r *http.Request) {
	if !isManager(currentUser(r)) {
		writeError(w, 403, "Manager access is required")
		return
	}
	org := currentUser(r).OrgID
	counts := map[string]int{}
	checks := map[string]string{
		"emailFailed":       "SELECT count(*) FROM mail_outbox WHERE org_id=? AND status='failed'",
		"emailWaiting":      "SELECT count(*) FROM mail_outbox WHERE org_id=? AND status='waiting_configuration'",
		"emailUncertain":    "SELECT count(*) FROM mail_outbox WHERE org_id=? AND status IN ('delivery_unknown','uncertain')",
		"whatsappFailed":    "SELECT count(*) FROM conversations WHERE org_id=? AND channel='whatsapp' AND status='failed'",
		"whatsappUncertain": "SELECT count(*) FROM conversations WHERE org_id=? AND channel='whatsapp' AND status IN ('uncertain','delivery_unknown')",
		"webhookFailed":     "SELECT count(*) FROM webhook_deliveries WHERE org_id=? AND status='failed'",
	}
	for key, query := range checks {
		var value int
		if err := storeDB(r).QueryRow(query, org).Scan(&value); err != nil {
			writeError(w, 503, "Could not load operational attention")
			return
		}
		counts[key] = value
	}
	deliveries, err := queryObjects(r, "SELECT id,subject,status,contact_id AS contactId,invoice_id AS invoiceId,'email' AS channel,created_at AS createdAt FROM mail_outbox WHERE org_id=? AND status IN ('failed','delivery_unknown','uncertain','waiting_configuration') AND kind NOT IN ('password_reset','invite') UNION ALL SELECT id,subject,status,contact_id AS contactId,'' AS invoiceId,channel,created_at AS createdAt FROM conversations WHERE org_id=? AND status IN ('failed','delivery_unknown','uncertain') ORDER BY createdAt DESC,id DESC LIMIT 25", org, org)
	if err != nil {
		writeError(w, 503, "Could not load delivery attention")
		return
	}
	safeDeliveries := []map[string]any{}
	for _, item := range deliveries {
		entity, id, tab := "contacts", fmtString(item["contactId"]), "conversations"
		if invoice := fmtString(item["invoiceId"]); invoice != "" {
			entity, id, tab = "invoices", invoice, "email"
		}
		if id == "" {
			continue
		}
		if _, err := sqlRecord(r, entity, id); err != nil {
			continue
		}
		item["route"] = "/" + entity + "/" + id + "#record-tools?tab=" + tab
		item["deliveryExplanation"] = deliveryExplanation(fmtString(item["channel"]), fmtString(item["status"]))
		delete(item, "contactId")
		delete(item, "invoiceId")
		safeDeliveries = append(safeDeliveries, item)
	}
	backup := map[string]any{"state": "unknown", "restoreState": "not_verified"}
	path := filepath.Join(filepath.Dir(dataPath), "backup-status.json")
	if raw, err := os.ReadFile(path); err == nil {
		var saved map[string]any
		if json.Unmarshal(raw, &saved) == nil {
			backup["state"] = saved["state"]
			backup["checkedAt"] = saved["checkedAt"]
			if at, err := time.Parse(time.RFC3339Nano, fmtString(saved["checkedAt"])); err == nil && saved["state"] == "ok" && time.Since(at) > 36*time.Hour {
				backup["state"] = "stale"
			}
			if v, ok := saved["restoreVerification"].(map[string]any); ok && v["state"] == "verified" && v["archive"] == saved["destination"] {
				backup["restoreState"] = "verified"
			}
		}
	}
	writeJSON(w, 200, map[string]any{"counts": counts, "deliveries": safeDeliveries, "backup": backup, "checkedAt": utcNow()})
}
