package main

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"
)

//go:embed assets/countries.json
var countryData []byte

func normalizedCountry(value string) (string, bool) {
	if value == "" {
		return "", true
	}
	var data map[string][]map[string]any
	json.Unmarshal(countryData, &data)
	for _, items := range data {
		for _, item := range items {
			for _, key := range []string{"alpha_2", "alpha_3", "name", "common_name", "official_name"} {
				name, _ := item[key].(string)
				if strings.EqualFold(strings.TrimSpace(value), name) {
					code, _ := item["alpha_2"].(string)
					return code, true
				}
			}
		}
	}
	return "", false
}
func normalizeRequest(w http.ResponseWriter, r *http.Request) bool {
	if !strings.Contains(r.Header.Get("Content-Type"), "application/json") || r.Method == "GET" {
		return true
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, (2<<20)+1))
	if err != nil || len(body) > 2<<20 {
		writeError(w, 400, "Request is too large")
		return false
	}
	if len(body) == 0 {
		r.Body = io.NopCloser(bytes.NewReader(body))
		return true
	}
	var fields map[string]json.RawMessage
	if json.Unmarshal(body, &fields) != nil {
		writeError(w, 400, "Invalid JSON")
		return false
	}
	errors := map[string]string{}
	for key, value := range fields {
		var s string
		if json.Unmarshal(value, &s) != nil {
			continue
		}
		switch key {
		case "countryCode", "currentCountry", "interestedCountry", "destinationCountry", "country":
			if normalized, ok := normalizedCountry(s); ok {
				fields[key], _ = json.Marshal(normalized)
			} else {
				errors[key] = "Choose a valid country name or ISO country code"
			}
		case "phone", "contactPhone":
			if !validPhone(s) {
				errors[key] = "Use 7–15 digits with an optional + prefix"
			} else {
				fields[key], _ = json.Marshal(normalizePhone(s))
			}
		case "website":
			if s != "" {
				u, e := url.Parse(s)
				if e != nil || (u.Scheme != "https" && u.Scheme != "http") || u.Hostname() == "" || u.User != nil {
					errors[key] = "Use a full http:// or https:// website URL"
				}
			}
		}
	}
	if len(errors) > 0 {
		writeFieldErrors(w, errors)
		return false
	}
	encoded, _ := json.Marshal(fields)
	r.Body = io.NopCloser(bytes.NewReader(encoded))
	return true
}
func scopeForRelatedRequest(r *http.Request, projected diskStore) RecordScope {
	scope := newRecordScope(r)
	if r.Method != "POST" || !strings.Contains(r.Header.Get("Content-Type"), "application/json") {
		return scope
	}
	body, _ := io.ReadAll(r.Body)
	r.Body = io.NopCloser(bytes.NewReader(body))
	var fields map[string]string
	json.Unmarshal(body, &fields)
	entity := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(entity) < 2 {
		return scope
	}
	records := recordsOf(projected)
	relations := map[string]string{"leadId": "leads", "caseId": "cases", "invoiceId": "invoices", "contactId": "contacts", "clientId": "contacts"}
	for field, table := range relations {
		if payload, ok := records[table][fields[field]]; ok {
			var parent RecordScope
			json.Unmarshal([]byte(payload), &parent)
			if entity[1] == "documents" || entity[1] == "payments" || entity[1] == "activities" {
				scope.OwnerID = parent.OwnerID
			}
			if parent.Visibility == "private" || (parent.Visibility == "team" && scope.Visibility != "private") {
				scope.Visibility = parent.Visibility
				scope.TeamID = parent.TeamID
			}
		}
	}
	return scope
}
