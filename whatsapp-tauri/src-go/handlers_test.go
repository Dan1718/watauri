package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHandleMessagesAcceptsChatIDPath(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/chats/c1", nil)
	rec := httptest.NewRecorder()

	handleMessages(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}
