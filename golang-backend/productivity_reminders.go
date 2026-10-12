package main

import "strings"

func responsibleRecipient(s diskStore, scope RecordScope, assigned string) string {
	assigned = strings.TrimSpace(assigned)
	if assigned == "" || strings.EqualFold(assigned, "Unassigned") {
		assigned = scope.OwnerID
	}
	matches := []User{}
	for _, stored := range s.Users {
		u := stored.User
		if u.OrgID == scope.OrgID && !u.Disabled && u.Role != "viewer" && canRead(scope, u) && (u.ID == assigned || strings.EqualFold(strings.TrimSpace(u.Name), assigned)) {
			matches = append(matches, u)
		}
	}
	if len(matches) == 1 {
		return matches[0].ID
	}
	return ""
}
func documentResponsible(s diskStore, d Document) (string, string, bool) {
	assigned := d.ResponsibleID
	var parentScope RecordScope
	route := ""
	active := false
	for _, c := range s.Cases {
		if c.ID == d.CaseID && c.OrgID == d.OrgID {
			parentScope = c.RecordScope
			route = "/cases/" + c.ID + "#record-tools?tab=documents"
			active = activeApplication(c.CurrentStage)
			if assigned == "" {
				assigned = c.AssignedOfficer
				if assigned == "" {
					assigned = c.OwnerID
				}
			}
		}
	}
	if d.CaseID == "" {
		for _, l := range s.Leads {
			if l.ID == d.LeadID && l.OrgID == d.OrgID {
				parentScope = l.RecordScope
				route = "/leads/" + l.ID + "#record-tools?tab=documents"
				active = openLead(l.Status)
				if assigned == "" {
					assigned = l.AssignedTo
					if assigned == "" {
						assigned = l.OwnerID
					}
				}
			}
		}
	}
	recipient := responsibleRecipient(s, d.RecordScope, assigned)
	for _, stored := range s.Users {
		if stored.User.ID == recipient && !canRead(parentScope, stored.User) {
			return "", route, false
		}
	}
	return recipient, route, active
}
func remindProductivity(db DBRunner, s diskStore, day func(string, int) string) error {
	activeReminders := map[string]bool{}
	orgs := map[string]bool{}
	for _, stored := range s.Users {
		orgs[stored.User.OrgID] = true
	}
	add := func(scope RecordScope, assigned, kind, id, title, body, path string) error {
		recipient := responsibleRecipient(s, scope, assigned)
		if recipient == "" {
			return nil
		}
		key := kind + ":" + id + ":" + day(scope.OrgID, 0)
		activeReminders[scope.OrgID+"\x00"+recipient+"\x00"+key] = true
		_, err := db.Exec("INSERT INTO notifications VALUES(?,?,?,?,?,?,?,?,?,NULL) ON CONFLICT(user_id,dedupe_key) DO UPDATE SET title=excluded.title,body=excluded.body,path=excluded.path,read_at=CASE WHEN notifications.body<>excluded.body OR notifications.path<>excluded.path THEN NULL ELSE notifications.read_at END", newID("notice"), scope.OrgID, recipient, kind, title, body, path, key, utcNow())
		return err
	}
	for _, d := range s.Deals {
		if d.Stage != "Won" && d.Stage != "Lost" && d.NextDeadline != "" && d.NextDeadline <= day(d.OrgID, 0) {
			if err := add(d.RecordScope, d.Owner, "deal-action", d.ID, "Deal action due", d.Title+": "+d.NextAction+" · "+d.NextDeadline, "/deals/"+d.ID); err != nil {
				return err
			}
		}
	}
	for _, st := range s.Students {
		if st.ApplicationStage != "Closed" && st.ApplicationStage != "Completed" && st.ApplicationStage != "Withdrawn" && st.NextDeadline != "" && st.NextDeadline <= day(st.OrgID, 0) {
			if err := add(st.RecordScope, st.OwnerID, "student-action", st.ID, "Student action due", st.Name+": "+st.NextAction+" · "+st.NextDeadline, "/students/"+st.ID); err != nil {
				return err
			}
		}
	}

	for _, d := range s.Documents {
		reason, action, needed := documentIssue(d, day(d.OrgID, 0))
		if !needed {
			continue
		}
		if reason != "Expired document" && reason != "Document expires within 30 days" && d.DueDate != "" && d.DueDate > day(d.OrgID, 0) {
			continue
		}
		recipient, route, active := documentResponsible(s, d)
		if !active || recipient == "" {
			continue
		}
		kind, title := "document-action", reason
		if d.Required && d.FilePath == "" {
			kind, title = "missing-document", "Required document missing"
		}
		if err := add(d.RecordScope, recipient, kind, d.ID, title, d.DocName+" · "+action, route); err != nil {
			return err
		}
	}
	// Retire reminders after completion, reassignment or rescheduling, and older
	// daily reminders. Do not reopen a read reminder unless its content changed.
	rows, err := db.Query("SELECT id,org_id,user_id,dedupe_key FROM notifications WHERE read_at IS NULL AND kind IN ('deal-action','student-action','missing-document','document-action')")
	if err != nil {
		return err
	}
	stale := []string{}
	for rows.Next() {
		var id, org, user, key string
		if err = rows.Scan(&id, &org, &user, &key); err != nil {
			break
		}
		if orgs[org] && !activeReminders[org+"\x00"+user+"\x00"+key] {
			stale = append(stale, id)
		}
	}
	if err == nil {
		err = rows.Err()
	}
	rows.Close()
	if err != nil {
		return err
	}
	for _, id := range stale {
		if _, err = db.Exec("UPDATE notifications SET read_at=COALESCE(read_at,?) WHERE id=?", utcNow(), id); err != nil {
			return err
		}
	}
	return nil
}
