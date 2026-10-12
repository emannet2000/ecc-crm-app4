package main

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"sync/atomic"
	"time"
)

var requestsTotal, requestsFailed, requestMilliseconds atomic.Int64
var startedAt = time.Now()

type observedResponse struct {
	http.ResponseWriter
	status int
}

func (w *observedResponse) WriteHeader(status int) {
	if w.status == 0 {
		w.status = status
		w.ResponseWriter.WriteHeader(status)
	}
}
func (w *observedResponse) Write(b []byte) (int, error) {
	if w.status == 0 {
		w.WriteHeader(200)
	}
	return w.ResponseWriter.Write(b)
}
func observeHTTP(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		response := &observedResponse{ResponseWriter: w}
		next.ServeHTTP(response, r)
		elapsed := time.Since(start).Milliseconds()
		requestsTotal.Add(1)
		requestMilliseconds.Add(elapsed)
		if response.status >= 500 {
			requestsFailed.Add(1)
		}
		if r.Pattern != "" {
			slog.Info("http_request", "route", r.Pattern, "method", r.Method, "status", response.status, "duration_ms", elapsed)
		}
	})
}
func handleMetrics(w http.ResponseWriter, r *http.Request) {
	if !requireAdmin(w, r) {
		return
	}
	total := requestsTotal.Load()
	average := int64(0)
	if total > 0 {
		average = requestMilliseconds.Load() / total
	}
	writeJSON(w, 200, map[string]any{"requests": total, "serverErrors": requestsFailed.Load(), "averageMilliseconds": average, "uptimeSeconds": int(time.Since(startedAt).Seconds()), "database": database.Stats(), "mode": "single-writer cache with independent SQL report and record reads"})
}

func handleOperations(w http.ResponseWriter, r *http.Request) {
	if !requireAdmin(w, r) {
		return
	}
	workerHeartbeatMu.RLock()
	workers := map[string]any{}
	alerts := []string{}
	now := time.Now().UTC()
	for _, name := range []string{"mail-delivery", "reminders", "follow-up-automation", "webhooks", "push-notifications"} {
		at, ok := workerHeartbeats[name]
		state := "not_started"
		if ok {
			state = "running"
			if now.Sub(at) > 45*time.Second {
				state = "stale"
				alerts = append(alerts, name+" worker has not run recently")
			}
		} else {
			alerts = append(alerts, name+" worker has not started")
		}
		workers[name] = map[string]any{"state": state, "lastRunAt": at}
	}
	workerHeartbeatMu.RUnlock()
	var failed, waiting int
	_ = database.QueryRow("SELECT count(*) FROM mail_outbox WHERE org_id=? AND status='failed'", currentUser(r).OrgID).Scan(&failed)
	_ = database.QueryRow("SELECT count(*) FROM mail_outbox WHERE org_id=? AND status='waiting_configuration'", currentUser(r).OrgID).Scan(&waiting)
	if failed > 0 {
		alerts = append(alerts, "Email delivery has "+strconv.Itoa(failed)+" failed message(s)")
	}
	if waiting > 0 {
		alerts = append(alerts, strconv.Itoa(waiting)+" email message(s) are waiting for SMTP configuration")
	}
	automationFailures := followupAutomationFailures.Load()
	if automationFailures > 0 {
		alerts = append(alerts, "Follow-up automation has recorded "+strconv.FormatInt(automationFailures, 10)+" failure(s) since startup")
	}
	backupPath := filepath.Join(filepath.Dir(dataPath), "backup-status.json")
	backup := map[string]any{"state": "unknown", "message": "No backup status has been recorded"}
	if b, err := os.ReadFile(backupPath); err == nil {
		_ = json.Unmarshal(b, &backup)
		if backup["state"] == "ok" {
			if checked, ok := backup["checkedAt"].(string); ok {
				if at, err := time.Parse(time.RFC3339Nano, checked); err == nil && now.Sub(at) > 36*time.Hour {
					backup["state"] = "stale"
					alerts = append(alerts, "Latest successful backup is more than 36 hours old")
				}
			}
		}
		if backup["state"] == "failed" {
			alerts = append(alerts, "Latest backup reports a failure")
		}
		verification, ok := backup["restoreVerification"].(map[string]any)
		if !ok || verification["state"] != "verified" || verification["archive"] != backup["destination"] {
			alerts = append(alerts, "Latest backup has no successful isolated restore verification")
		}
	} else {
		alerts = append(alerts, "Backup status is unknown; run the backup command to establish monitoring")
	}
	writeJSON(w, 200, map[string]any{"workers": workers, "mail": map[string]int{"failed": failed, "waitingConfiguration": waiting}, "automationFailures": automationFailures, "backup": backup, "alerts": alerts, "checkedAt": now.Format(time.RFC3339)})
}
