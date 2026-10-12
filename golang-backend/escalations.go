package main

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type EscalationRule struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Kind      string `json:"kind"`
	AfterDays int    `json:"afterDays"`
	ManagerID string `json:"managerId"`
	Enabled   bool   `json:"enabled"`
	Revision  string `json:"revision,omitempty"`
}

func handleEscalationRules(w http.ResponseWriter, r *http.Request) {
	if !isManager(currentUser(r)) {
		writeError(w, 403, "Manager access is required")
		return
	}
	if r.Method == "GET" {
		rows, err := queryObjects(r, "SELECT id,data FROM escalation_rules WHERE org_id=? ORDER BY id", currentUser(r).OrgID)
		if err != nil {
			writeError(w, 500, "Could not load escalations")
			return
		}
		rules := []EscalationRule{}
		for _, row := range rows {
			var rule EscalationRule
			if json.Unmarshal([]byte(fmtString(row["data"])), &rule) == nil {
				rule.ID = fmtString(row["id"])
				rule.Revision = versionOf(recordMap(rule))
				rules = append(rules, rule)
			}
		}
		people, err := requirementPeople(r)
		if err != nil {
			writeError(w, 500, "Could not load managers")
			return
		}
		managers := []RequirementPerson{}
		for _, u := range people {
			if isManager(u) && !u.Disabled {
				managers = append(managers, RequirementPerson{ID: u.ID, Name: u.Name})
			}
		}
		writeJSON(w, 200, map[string]any{"rules": rules, "managers": managers})
		return
	}
	var rule EscalationRule
	if !decodeRequest(w, r, &rule) {
		return
	}
	rule.Name = strings.TrimSpace(rule.Name)
	if rule.Name == "" || len(rule.Name) > 100 || rule.AfterDays < 1 || rule.AfterDays > 365 {
		writeError(w, 400, "Enter a name and an escalation delay of 1–365 days")
		return
	}
	switch rule.Kind {
	case "application", "lead", "task", "handoff":
	default:
		writeError(w, 400, "Choose applications, leads, tasks or unanswered handoffs")
		return
	}
	if rule.ManagerID != "" {
		u, ok := committedUser(r, "id", rule.ManagerID)
		if !ok || u.OrgID != currentUser(r).OrgID || u.Disabled || !isManager(u) {
			writeError(w, 400, "Choose an active manager in this workspace")
			return
		}
	}
	if rule.ID != "" {
		var raw string
		if storeDB(r).QueryRow("SELECT data FROM escalation_rules WHERE org_id=? AND id=?", currentUser(r).OrgID, rule.ID).Scan(&raw) != nil {
			writeError(w, 404, "Rule not found")
			return
		}
		var old EscalationRule
		json.Unmarshal([]byte(raw), &old)
		old.ID = rule.ID
		if rule.Revision != versionOf(recordMap(old)) {
			writeError(w, 409, "Rule changed. Reload before saving.")
			return
		}
	} else {
		rule.ID = newID("escalation")
	}
	rule.Revision = ""
	if _, err := storeDB(r).Exec("INSERT INTO escalation_rules VALUES(?,?,?) ON CONFLICT(id) DO UPDATE SET data=excluded.data WHERE org_id=excluded.org_id", rule.ID, currentUser(r).OrgID, mustJSON(rule)); err != nil {
		writeError(w, 500, "Could not save rule")
		return
	}
	if securityEvent(r, currentUser(r), "escalation.rule_saved:"+rule.ID) != nil {
		writeError(w, 500, "Could not record rule change")
		return
	}
	writeJSON(w, 200, map[string]any{"saved": true})
}

type escalationWork struct {
	org, entity, id, title, deadline, episode, route string
	scope                                            RecordScope
}

func runEscalations(db DBRunner, s diskStore, day func(string, int) string) error {
	rows, err := db.Query("SELECT data FROM escalation_rules")
	if err != nil {
		return err
	}
	rules := []EscalationRule{}
	for rows.Next() {
		var raw string
		var rule EscalationRule
		if err = rows.Scan(&raw); err != nil {
			break
		}
		if err = json.Unmarshal([]byte(raw), &rule); err != nil {
			break
		}
		rules = append(rules, rule)
	}
	if err == nil {
		err = rows.Err()
	}
	rows.Close()
	if err != nil {
		return err
	}
	handoffs := []ApplicationHandoff{}
	hrows, err := db.Query("SELECT data FROM workspace_entries WHERE category='application_handoff' AND json_extract(data,'$.status')='pending'")
	if err != nil {
		return err
	}
	for hrows.Next() {
		var raw string
		var h ApplicationHandoff
		if err = hrows.Scan(&raw); err != nil {
			break
		}
		if err = json.Unmarshal([]byte(raw), &h); err != nil {
			break
		}
		handoffs = append(handoffs, h)
	}
	if err == nil {
		err = hrows.Err()
	}
	hrows.Close()
	if err != nil {
		return err
	}
	cases := map[string]Case{}
	for _, c := range s.Cases {
		cases[c.ID] = c
	}
	active := map[string]bool{}
	for _, rule := range rules {
		if !rule.Enabled {
			continue
		}
		var org string
		if err = db.QueryRow("SELECT org_id FROM escalation_rules WHERE id=?", rule.ID).Scan(&org); err != nil {
			return err
		}
		work := []escalationWork{}
		add := func(scope RecordScope, entity, id, title, deadline, route string) {
			if scope.OrgID != org || deadline == "" || deadline > day(org, -rule.AfterDays) {
				return
			}
			if _, e := time.Parse("2006-01-02", deadline); e != nil {
				return
			}
			work = append(work, escalationWork{org, entity, id, title, deadline, deadline, route, scope})
		}
		switch rule.Kind {
		case "application":
			for _, c := range s.Cases {
				if activeApplication(c.CurrentStage) {
					add(c.RecordScope, "cases", c.ID, c.CaseNumber, c.NextDeadline, "/cases/"+c.ID)
				}
			}
		case "lead":
			for _, l := range s.Leads {
				if openLead(l.Status) {
					add(l.RecordScope, "leads", l.ID, l.Name, l.FollowUpDate, "/leads/"+l.ID)
				}
			}
		case "task":
			for _, t := range s.Tasks {
				if t.Status != "done" {
					add(t.RecordScope, "tasks", t.ID, t.Title, t.DueDate, "/tasks")
				}
			}
		case "handoff":
			for _, h := range handoffs {
				c, ok := cases[h.CaseID]
				created, e := time.Parse(time.RFC3339, h.CreatedAt)
				if !ok || c.OrgID != org || e != nil || time.Since(created) < time.Duration(rule.AfterDays)*24*time.Hour {
					continue
				}
				work = append(work, escalationWork{org, "cases", c.ID, c.CaseNumber, h.DueDate, h.ID, "/cases/" + c.ID + "#handoff", c.RecordScope})
			}
		}
		for _, item := range work {
			for _, stored := range s.Users {
				u := stored.User
				if u.OrgID != org || u.Disabled || !isManager(u) || !canRead(item.scope, u) || rule.ManagerID != "" && u.ID != rule.ManagerID {
					continue
				}
				key := "escalation:" + rule.ID + ":" + item.entity + ":" + item.id + ":" + item.episode + ":" + u.ID
				active[key] = true
				result, e := db.Exec("INSERT OR IGNORE INTO escalation_deliveries VALUES(?,?,?,?,?,?)", rule.ID, item.entity, item.id, item.episode, u.ID, utcNow())
				if e != nil {
					return e
				}
				inserted, e := result.RowsAffected()
				if e != nil {
					return e
				}
				if inserted == 0 {
					continue
				}
				body := rule.Name + ": " + item.title + " needs manager attention after " + fmtInt(rule.AfterDays) + " day(s)."
				if item.deadline != "" {
					body += " Due " + item.deadline + "."
				}
				if _, e = db.Exec("INSERT OR IGNORE INTO notifications VALUES(?,?,?,?,?,?,?,?,?,NULL)", newID("notice"), org, u.ID, "escalation", "Manager escalation", body, item.route, key, utcNow()); e != nil {
					return e
				}
			}
		}
	}
	notices, err := db.Query("SELECT id,dedupe_key FROM notifications WHERE kind='escalation' AND read_at IS NULL")
	if err != nil {
		return err
	}
	retire := []string{}
	for notices.Next() {
		var id, key string
		if err = notices.Scan(&id, &key); err != nil {
			break
		}
		if !active[key] {
			retire = append(retire, id)
		}
	}
	if err == nil {
		err = notices.Err()
	}
	notices.Close()
	if err != nil {
		return err
	}
	for _, id := range retire {
		if _, err = db.Exec("UPDATE notifications SET read_at=? WHERE id=? AND read_at IS NULL", utcNow(), id); err != nil {
			return err
		}
	}
	return nil
}
func fmtInt(n int) string { return strconv.Itoa(n) }
