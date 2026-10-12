package main

import (
	"net/http"
	"strconv"
	"strings"
	"unicode/utf8"
)

func registerTeamChatRoutes(m *http.ServeMux) {
	m.HandleFunc("GET /api/team-chat/people", authMiddleware(handleChatPeople))
	m.HandleFunc("GET /api/team-chat/{peer}", authMiddleware(handleTeamChat))
	m.HandleFunc("POST /api/team-chat/{peer}", authMiddleware(handleTeamChat))
}
func handleChatPeople(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "private, no-store")
	u := currentUser(r)
	offset := 0
	if raw := r.URL.Query().Get("offset"); raw != "" {
		var err error
		offset, err = strconv.Atoi(raw)
		if err != nil || offset < 0 || offset > 1000000 {
			writeError(w, 400, "Invalid colleague cursor")
			return
		}
	}
	query := strings.TrimSpace(r.URL.Query().Get("q"))
	if utf8.RuneCountInString(query) > 200 {
		writeError(w, 400, "Search must be at most 200 characters")
		return
	}
	rows, err := storeDB(r).Query(`SELECT u.id,u.name,u.role,(SELECT count(*) FROM team_messages m WHERE m.org_id=? AND m.sender_id=u.id AND m.recipient_id=? AND m.read_at=''),
        COALESCE((SELECT substr(body,1,140) FROM team_messages m WHERE m.org_id=u.org_id AND ((m.sender_id=u.id AND m.recipient_id=?) OR (m.sender_id=? AND m.recipient_id=u.id)) ORDER BY seq DESC LIMIT 1),''),
        COALESCE((SELECT created_at FROM team_messages m WHERE m.org_id=u.org_id AND ((m.sender_id=u.id AND m.recipient_id=?) OR (m.sender_id=? AND m.recipient_id=u.id)) ORDER BY seq DESC LIMIT 1),'') FROM users u WHERE u.org_id=? AND u.disabled=0 AND u.id<>? AND instr(lower(u.name),lower(?))>0 ORDER BY u.name,u.id LIMIT 200 OFFSET ?`, u.OrgID, u.ID, u.ID, u.ID, u.ID, u.ID, u.OrgID, u.ID, query, offset)
	if err != nil {
		writeError(w, 500, "Unable to load colleagues")
		return
	}
	defer rows.Close()
	people := []map[string]any{}
	for rows.Next() {
		var id, name, role, lastMessage, lastMessageAt string
		var unread int
		if rows.Scan(&id, &name, &role, &unread, &lastMessage, &lastMessageAt) != nil {
			writeError(w, 500, "Unable to load colleagues")
			return
		}
		people = append(people, map[string]any{"id": id, "name": name, "role": role, "unread": unread, "lastMessage": lastMessage, "lastMessageAt": lastMessageAt})
	}
	if rows.Err() != nil {
		writeError(w, 500, "Unable to load colleagues")
		return
	}
	writeJSON(w, 200, people)
}
func handleTeamChat(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "private, no-store")
	u := currentUser(r)
	peer := r.PathValue("peer")
	var found string
	if peer == u.ID || storeDB(r).QueryRow(`SELECT id FROM users WHERE id=? AND org_id=? AND disabled=0`, peer, u.OrgID).Scan(&found) != nil {
		writeError(w, 404, "Colleague unavailable")
		return
	}
	if r.Method == "POST" {
		if !validOrigin(r) {
			writeError(w, 403, "Invalid request origin")
			return
		}
		var req struct {
			Body     string `json:"body"`
			ClientID string `json:"clientId"`
			ReadID   int64  `json:"readId"`
		}
		if !decodeRequest(w, r, &req) {
			return
		}
		if req.ReadID > 0 {
			_, err := storeDB(r).Exec(`UPDATE team_messages SET read_at=? WHERE org_id=? AND sender_id=? AND recipient_id=? AND seq<=? AND read_at=''`, utcNow(), u.OrgID, peer, u.ID, req.ReadID)
			if err != nil {
				writeError(w, 500, "Unable to update read status")
				return
			}
			writeJSON(w, 200, map[string]any{"ok": true})
			return
		}
		req.Body = strings.TrimSpace(req.Body)
		if req.Body == "" || !utf8.ValidString(req.Body) || utf8.RuneCountInString(req.Body) > 4000 || len(req.ClientID) < 8 || len(req.ClientID) > 80 {
			writeError(w, 400, "Enter a message up to 4,000 characters")
			return
		}
		_, err := storeDB(r).Exec(`INSERT INTO team_messages(org_id,sender_id,recipient_id,body,client_id,created_at,read_at) VALUES(?,?,?,?,?,?,'') ON CONFLICT(org_id,sender_id,recipient_id,client_id) DO NOTHING`, u.OrgID, u.ID, peer, req.Body, req.ClientID, utcNow())
		if err != nil {
			writeError(w, 500, "Unable to send message")
			return
		}
		var body string
		var seq int64
		if storeDB(r).QueryRow(`SELECT seq,body FROM team_messages WHERE org_id=? AND sender_id=? AND recipient_id=? AND client_id=?`, u.OrgID, u.ID, peer, req.ClientID).Scan(&seq, &body) != nil {
			writeError(w, 500, "Unable to confirm message")
			return
		}
		if body != req.Body {
			writeError(w, 409, "This message identifier was already used")
			return
		}
		writeJSON(w, 200, map[string]any{"id": seq})
		return
	}
	before := int64(9223372036854775807)
	if value := r.URL.Query().Get("before"); value != "" {
		var err error
		before, err = strconv.ParseInt(value, 10, 64)
		if err != nil || before <= 0 {
			writeError(w, 400, "Invalid history cursor")
			return
		}
	}
	rows, err := storeDB(r).Query(`SELECT seq,sender_id,body,created_at,read_at FROM team_messages WHERE org_id=? AND seq<? AND ((sender_id=? AND recipient_id=?) OR (sender_id=? AND recipient_id=?)) ORDER BY seq DESC LIMIT 100`, u.OrgID, before, u.ID, peer, peer, u.ID)
	if err != nil {
		writeError(w, 500, "Unable to load messages")
		return
	}
	defer rows.Close()
	messages := []map[string]any{}
	for rows.Next() {
		var seq int64
		var sender, body, at, readAt string
		if rows.Scan(&seq, &sender, &body, &at, &readAt) != nil {
			writeError(w, 500, "Unable to load messages")
			return
		}
		messages = append(messages, map[string]any{"id": seq, "senderId": sender, "body": body, "createdAt": at, "readAt": readAt})
	}
	if rows.Err() != nil {
		writeError(w, 500, "Unable to load messages")
		return
	}
	for i, j := 0, len(messages)-1; i < j; i, j = i+1, j-1 {
		messages[i], messages[j] = messages[j], messages[i]
	}
	writeJSON(w, 200, messages)
}
