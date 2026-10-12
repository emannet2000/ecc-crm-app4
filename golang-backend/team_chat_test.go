package main

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestTeamChatPrivacyRetryAndReadStatus(t *testing.T) {
	setupStore(t)
	admin, _ := findUserByEmail("demo@northwind.dev")
	a := addPermissionUser(t, "viewer", admin.OrgID, "chat-a")
	b := addPermissionUser(t, "member", admin.OrgID, "chat-b")
	foreign := addPermissionUser(t, "admin", "chat-foreign", "chat-outsider")
	h := expansionTestMux()
	assertStatus(t, rawUserRequest(t, h, a, "GET", "/api/team-chat/people?offset=-1", nil), 400)
	assertStatus(t, rawUserRequest(t, h, a, "GET", "/api/team-chat/people?q=outsider", nil), 200)
	assertStatus(t, rawUserRequest(t, h, a, "GET", "/api/team-chat/"+b.ID+"?before=invalid", nil), 400)
	send := map[string]any{"body": "<script>plain text</script>", "clientId": "retry-safe-123"}
	path := "/api/team-chat/" + b.ID
	assertStatus(t, rawUserRequest(t, h, a, "POST", path, send), 200)
	assertStatus(t, rawUserRequest(t, h, a, "POST", path, send), 200)
	send["body"] = "Different retry"
	assertStatus(t, rawUserRequest(t, h, a, "POST", path, send), 409)
	w := rawUserRequest(t, h, b, "GET", "/api/team-chat/"+a.ID, nil)
	assertStatus(t, w, 200)
	var messages []struct {
		ID     int64
		Body   string
		ReadAt string
	}
	if json.Unmarshal(w.Body.Bytes(), &messages) != nil || len(messages) != 1 || messages[0].Body != "<script>plain text</script>" {
		t.Fatal(w.Body.String())
	}
	// A workspace administrator sees only their own conversations.
	w = rawUserRequest(t, h, admin, "GET", path, nil)
	assertStatus(t, w, 200)
	if strings.TrimSpace(w.Body.String()) != "[]" {
		t.Fatal("admin exposed a private conversation", w.Body.String())
	}
	assertStatus(t, rawUserRequest(t, h, foreign, "GET", path, nil), 404)
	assertStatus(t, rawUserRequest(t, h, a, "POST", "/api/team-chat/"+foreign.ID, send), 404)
	w = rawUserRequest(t, h, b, "GET", "/api/team-chat/people", nil)
	assertStatus(t, w, 200)
	var people []struct {
		ID          string
		Unread      int
		LastMessage string
	}
	if json.Unmarshal(w.Body.Bytes(), &people) != nil {
		t.Fatal(w.Body.String())
	}
	for _, p := range people {
		if p.ID == a.ID && (p.Unread != 1 || p.LastMessage != "<script>plain text</script>") {
			t.Fatal("unread count missing")
		}
		if p.ID == foreign.ID {
			t.Fatal("foreign colleague exposed")
		}
	}
	assertStatus(t, rawUserRequest(t, h, b, "POST", "/api/team-chat/"+a.ID, map[string]any{"readId": messages[0].ID}), 200)
	var unread int
	if err := database.QueryRow(`SELECT count(*) FROM team_messages WHERE recipient_id=? AND read_at=''`, b.ID).Scan(&unread); err != nil || unread != 0 {
		t.Fatalf("read status: %d %v", unread, err)
	}
	w = rawUserRequest(t, h, a, "GET", path, nil)
	assertStatus(t, w, 200)
	if err := json.Unmarshal(w.Body.Bytes(), &messages); err != nil || len(messages) != 1 || messages[0].ReadAt == "" {
		t.Fatal("sender did not receive confirmed read status", w.Body.String())
	}
	w = rawUserRequest(t, h, admin, "GET", "/api/team-chat/people", nil)
	assertStatus(t, w, 200)
	if err := json.Unmarshal(w.Body.Bytes(), &people); err != nil {
		t.Fatal(err)
	}
	for _, person := range people {
		if person.LastMessage != "" {
			t.Fatal("conversation preview exposed to administrator", w.Body.String())
		}
	}

	send["body"] = strings.Repeat("x", 4001)
	assertStatus(t, rawUserRequest(t, h, a, "POST", path, send), 400)
	if _, err := database.Exec(`UPDATE users SET disabled=1 WHERE id=?`, b.ID); err != nil {
		t.Fatal(err)
	}
	assertStatus(t, rawUserRequest(t, h, a, "GET", path, nil), 404)
}
