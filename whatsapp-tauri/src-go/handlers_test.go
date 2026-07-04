package main

import (
	"database/sql"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHandleMessagesAcceptsChatIDPath(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })

	oldStore := store
	store = &UserDataStore{db: db}
	t.Cleanup(func() { store = oldStore })
	if err := store.migrate(); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/chats/c1", nil)
	rec := httptest.NewRecorder()

	handleMessages(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}
