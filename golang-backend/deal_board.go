package main

import (
	"net/http"
	"strings"
	"time"
)

type BoardCurrency struct {
	Count    int     `json:"count"`
	Total    float64 `json:"total"`
	Open     float64 `json:"open"`
	Weighted float64 `json:"weighted"`
	Won      float64 `json:"won"`
}
type BoardStage struct {
	Count  int                `json:"count"`
	Values map[string]float64 `json:"values"`
}
type BoardSummary struct {
	Count      int                      `json:"count"`
	OpenCount  int                      `json:"openCount"`
	WonCount   int                      `json:"wonCount"`
	Currencies map[string]BoardCurrency `json:"currencies"`
	Stages     map[string]BoardStage    `json:"stages"`
}

func dealBoardPredicate(r *http.Request, where string, args []any) (string, []any) {
	owner := r.URL.Query().Get("owner")
	if owner == "__mine__" {
		u := currentUser(r)
		where += ` AND (json_extract(record.data,'$.owner') IN (?,?) OR ((trim(COALESCE(json_extract(record.data,'$.owner'),''))='' OR lower(trim(json_extract(record.data,'$.owner')))='unassigned') AND json_extract(record.data,'$.ownerId')=?))`
		args = append(args, u.ID, u.Name, u.ID)
	} else if owner != "" {
		where += " AND json_extract(record.data,'$.owner')=?"
		args = append(args, owner)
	}
	for _, filter := range []struct{ key, operator string }{{"from", ">="}, {"to", "<="}} {
		if date := r.URL.Query().Get(filter.key); date != "" {
			where += " AND json_extract(record.data,'$.closeDate')" + filter.operator + "?"
			args = append(args, date)
		}
	}
	return where, args
}
func validBoardDates(w http.ResponseWriter, r *http.Request) bool {
	for _, key := range []string{"from", "to"} {
		if value := r.URL.Query().Get(key); value != "" {
			if _, err := time.Parse("2006-01-02", value); err != nil {
				writeError(w, 400, "Choose valid closing dates")
				return false
			}
		}
	}
	if r.URL.Query().Get("from") != "" && r.URL.Query().Get("to") != "" && r.URL.Query().Get("from") > r.URL.Query().Get("to") {
		writeError(w, 400, "Closing date range is reversed")
		return false
	}
	return true
}
func boardTotals(r *http.Request, where string, args []any) (BoardSummary, []string, error) {
	summary := BoardSummary{Currencies: map[string]BoardCurrency{}, Stages: map[string]BoardStage{}}
	value := `COALESCE(json_extract(record.data,'$.value'),0)`
	stage := `COALESCE(json_extract(record.data,'$.stage'),'')`
	probability := `COALESCE(json_extract(record.data,'$.probability'),CASE ` + stage + ` WHEN 'Lead' THEN 0.1 WHEN 'Qualified' THEN 0.25 WHEN 'Proposal' THEN 0.5 WHEN 'Negotiation' THEN 0.75 WHEN 'Won' THEN 1 ELSE 0 END)`
	rows, err := storeDB(r).Query(`SELECT `+stage+`,COALESCE(NULLIF(json_extract(record.data,'$.currency'),''),'USD'),count(*),sum(`+value+`),sum(`+value+`*`+probability+`) FROM deals record WHERE `+where+` GROUP BY 1,2`, args...)
	if err != nil {
		return summary, nil, err
	}
	for rows.Next() {
		var name, currency string
		var count int
		var amount, weighted float64
		if err = rows.Scan(&name, &currency, &count, &amount, &weighted); err != nil {
			break
		}
		total := summary.Currencies[currency]
		total.Count += count
		total.Total += amount
		summary.Count += count
		if name == "Won" {
			total.Won += amount
			summary.WonCount += count
		} else if name != "Lost" {
			total.Open += amount
			total.Weighted += weighted
			summary.OpenCount += count
		}
		summary.Currencies[currency] = total
		column := summary.Stages[name]
		if column.Values == nil {
			column.Values = map[string]float64{}
		}
		column.Count += count
		column.Values[currency] = amount
		summary.Stages[name] = column
	}
	if err == nil {
		err = rows.Err()
	}
	rows.Close()
	if err != nil {
		return summary, nil, err
	}
	clean := r.Clone(r.Context())
	url := *r.URL
	params := url.Query()
	for _, key := range []string{"owner", "from", "to", "snapshot"} {
		params.Del(key)
	}
	url.RawQuery = params.Encode()
	clean.URL = &url
	ownerWhere, ownerArgs := repositoryPredicate(clean, "deals")
	ownerRows, err := storeDB(r).Query("SELECT DISTINCT json_extract(record.data,'$.owner') FROM deals record WHERE "+ownerWhere+" AND trim(COALESCE(json_extract(record.data,'$.owner'),''))<>'' ORDER BY 1", ownerArgs...)
	if err != nil {
		return summary, nil, err
	}
	owners := []string{}
	for ownerRows.Next() {
		var owner string
		if err = ownerRows.Scan(&owner); err != nil {
			break
		}
		owners = append(owners, owner)
	}
	if err == nil {
		err = ownerRows.Err()
	}
	ownerRows.Close()
	return summary, owners, err
}
func boardSnapshot(r *http.Request) string {
	var revision string
	storeDB(r).QueryRow("SELECT value FROM app_meta WHERE key=?", "deal-board:"+currentUser(r).OrgID).Scan(&revision)
	return versionOf(map[string]any{"revision": revision, "scope": recordMap(currentUser(r)), "query": strings.TrimSpace(r.URL.Query().Get("q")), "owner": r.URL.Query().Get("owner"), "from": r.URL.Query().Get("from"), "to": r.URL.Query().Get("to")})
}
