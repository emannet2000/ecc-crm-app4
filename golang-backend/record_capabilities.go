package main

import "net/http"

// Carry the authenticated user to the JSON encoder without a second decode of
// the response. Buffered handlers carry the same context explicitly.
type recordCapabilityWriter struct {
	http.ResponseWriter
	user User
}

func (w recordCapabilityWriter) recordUser() *User { return &w.user }
