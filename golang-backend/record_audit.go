package main

import (
	"net/http"
	"strings"
)

func extendedRecordAudit(r *http.Request, s diskStore, add func(string, string, string, string, string, RecordScope)) {
	s.Contacts = visibleRecords(s.Contacts, currentUser(r))
	s.Leads = visibleRecords(s.Leads, currentUser(r))
	s.Students = visibleRecords(s.Students, currentUser(r))
	s.Cases = visibleRecords(s.Cases, currentUser(r))
	contacts := map[string]bool{}
	students := map[string]bool{}
	cases := map[string]bool{}
	for _, c := range s.Contacts {
		contacts[c.ID] = true
	}
	for _, st := range s.Students {
		students[st.ID] = true
	}
	for _, c := range s.Cases {
		cases[c.ID] = true
	}
	identities := map[string][]string{}
	index := func(recordID, email, phone string) {
		if key := qualityKey(email); key != "" {
			identities["email:"+key] = append(identities["email:"+key], recordID)
		}
		if key := phoneIdentity(phone); key != "" {
			identities["phone:"+key] = append(identities["phone:"+key], recordID)
		}
	}
	for _, c := range s.Contacts {
		index("contacts/"+c.ID, c.Email, c.Phone)
	}
	for _, l := range s.Leads {
		index("leads/"+l.ID, l.Email, l.Phone)
	}
	duplicate := func(l Lead) bool {
		for _, key := range []string{"email:" + qualityKey(l.Email), "phone:" + phoneIdentity(l.Phone)} {
			for _, id := range identities[key] {
				if id != "leads/"+l.ID && id != "contacts/"+l.ConvertedContactID {
					return true
				}
			}
		}
		return false
	}
	for _, l := range s.Leads {
		if openLead(l.Status) && strings.TrimSpace(l.NextAction) == "" {
			add("leads", l.ID, "warning", "No next action", l.Name+" needs a specific next staff action.", l.RecordScope)
		}
		if l.Status == "Converted" && l.ConvertedContactID == "" && l.ConvertedStudentID == "" && l.ConvertedCaseID == "" {
			add("leads", l.ID, "error", "Converted lead has no linked records", l.Name+" was marked Converted without a linked client, student or application. Review CV & conversion.", l.RecordScope)
		}
		broken := (l.ConvertedContactID != "" && !contacts[l.ConvertedContactID]) || (l.ConvertedStudentID != "" && !students[l.ConvertedStudentID]) || (l.ConvertedCaseID != "" && !cases[l.ConvertedCaseID])
		if broken {
			add("leads", l.ID, "error", "Conversion link unavailable", l.Name+" references a converted record that is missing or unavailable in this view. Ask an authorized manager to review the link.", l.RecordScope)
		}
		if duplicate(l) {
			add("leads", l.ID, "warning", "Possible duplicate inquiry", l.Name+" matches contact details on another accessible lead or client. Review matching records before deciding whether the inquiry is separate.", l.RecordScope)
		}
	}
	for _, c := range s.Cases {
		if c.ClientID != "" && !contacts[c.ClientID] {
			add("cases", c.ID, "error", "Application client link unavailable", c.CaseNumber+" references a missing or unavailable client. Review the application link.", c.RecordScope)
		}
		if c.StudentID != "" && !students[c.StudentID] {
			add("cases", c.ID, "error", "Application student link unavailable", c.CaseNumber+" references a missing or unavailable student. Review the application link.", c.RecordScope)
		}
	}
	emailGroups := map[string][]Contact{}
	phoneGroups := map[string][]Contact{}
	for _, c := range s.Contacts {
		if key := qualityKey(c.Email); key != "" {
			emailGroups[key] = append(emailGroups[key], c)
		}
		if key := phoneIdentity(c.Phone); key != "" {
			phoneGroups[key] = append(phoneGroups[key], c)
		}
	}
	reported := map[string]bool{}
	for _, groups := range []map[string][]Contact{emailGroups, phoneGroups} {
		for _, group := range groups {
			if len(group) < 2 {
				continue
			}
			for _, c := range group {
				if !reported[c.ID] {
					add("contacts", c.ID, "warning", "Possible duplicate client", c.Name+" shares an email or phone with another accessible client. Review before merging.", c.RecordScope)
					reported[c.ID] = true
				}
			}
		}
	}
}
