package main

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func newTestStore(t *testing.T) {
	t.Helper()
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
	db.SetMaxOpenConns(1)
}

func insertTestMessages(t *testing.T, messages ...Message) {
	t.Helper()
	for _, message := range messages {
		message.ChatJID = "c1"
		message.SenderID = "sender"
		if err := store.InsertMessage(message); err != nil {
			t.Fatal(err)
		}
	}
}

func getMessagePage(t *testing.T, target string) (int, MessagePage) {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, target, nil)
	rec := httptest.NewRecorder()
	handleMessages(rec, req)
	var page MessagePage
	if rec.Code == http.StatusOK {
		if err := json.NewDecoder(rec.Body).Decode(&page); err != nil {
			t.Fatal(err)
		}
	}
	return rec.Code, page
}

func TestHandleMessagesPaginatesAtPageBoundaries(t *testing.T) {
	newTestStore(t)
	insertTestMessages(t,
		Message{ID: "m1", Timestamp: "2026-07-21T10:01:00Z"},
		Message{ID: "m2", Timestamp: "2026-07-21T10:02:00Z"},
		Message{ID: "m3", Timestamp: "2026-07-21T10:03:00Z"},
		Message{ID: "m4", Timestamp: "2026-07-21T10:04:00Z"},
		Message{ID: "m5", Timestamp: "2026-07-21T10:05:00Z"},
	)

	status, first := getMessagePage(t, "/api/chats/c1?limit=2")
	if status != http.StatusOK || !first.HasMore || first.NextCursor == nil || ids(first.Messages) != "m4,m5" {
		t.Fatalf("first page = status %d, ids %q, hasMore %v, cursor %v", status, ids(first.Messages), first.HasMore, first.NextCursor)
	}
	_, second := getMessagePage(t, "/api/chats/c1?limit=2&before="+url.QueryEscape(*first.NextCursor))
	if !second.HasMore || second.NextCursor == nil || ids(second.Messages) != "m2,m3" {
		t.Fatalf("second page = ids %q, hasMore %v, cursor %v", ids(second.Messages), second.HasMore, second.NextCursor)
	}
	_, last := getMessagePage(t, "/api/chats/c1?limit=2&before="+url.QueryEscape(*second.NextCursor))
	if last.HasMore || last.NextCursor != nil || ids(last.Messages) != "m1" {
		t.Fatalf("last page = ids %q, hasMore %v, cursor %v", ids(last.Messages), last.HasMore, last.NextCursor)
	}
}

func TestHandleMessagesOrdersEqualTimestampsByIDAndFetchesDeltas(t *testing.T) {
	newTestStore(t)
	ts := "2026-07-21T10:00:00Z"
	insertTestMessages(t,
		Message{ID: "a", Timestamp: ts},
		Message{ID: "b", Timestamp: ts},
		Message{ID: "c", Timestamp: ts},
	)

	_, first := getMessagePage(t, "/api/chats/c1?limit=2")
	if ids(first.Messages) != "b,c" || first.NextCursor == nil {
		t.Fatalf("first page ids = %q, cursor = %v", ids(first.Messages), first.NextCursor)
	}
	_, older := getMessagePage(t, "/api/chats/c1?before="+url.QueryEscape(*first.NextCursor))
	if ids(older.Messages) != "a" {
		t.Fatalf("older page ids = %q", ids(older.Messages))
	}
	if first.LatestCursor == nil {
		t.Fatal("first page has no latest cursor")
	}
	insertTestMessages(t, Message{ID: "d", Timestamp: ts}, Message{ID: "e", Timestamp: ts})
	_, delta := getMessagePage(t, "/api/chats/c1?limit=1&after="+url.QueryEscape(*first.LatestCursor))
	if ids(delta.Messages) != "d" || !delta.HasMore || delta.NextCursor == nil {
		t.Fatalf("delta = ids %q, hasMore %v, cursor %v", ids(delta.Messages), delta.HasMore, delta.NextCursor)
	}
}

func TestHandleMessagesRejectsInvalidPagination(t *testing.T) {
	newTestStore(t)
	valid := encodeMessageCursor(messageCursor{Version: 1, Mode: "before", TimestampEpoch: 1, ID: "m1"})
	missingEpoch := encodeMessageCursor(messageCursor{Version: 1, Mode: "before", ID: "m1"})
		after := encodeMessageCursor(messageCursor{Version: 1, Mode: "after", Revision: 1})
	tests := []string{
		"/api/chats/c1?limit=",
		"/api/chats/c1?limit=0",
		"/api/chats/c1?limit=201",
		"/api/chats/c1?limit=nope",
		"/api/chats/c1?limit=1&limit=2",
		"/api/chats/c1?anchor=",
		"/api/chats/c1?anchor=nope",
		"/api/chats/c1?anchor=oldestUnread&anchor=oldestUnread",
		"/api/chats/c1?before=not-base64!",
		"/api/chats/c1?after=",
		"/api/chats/c1?before=" + valid + "&after=" + valid,
		"/api/chats/c1?anchor=oldestUnread&before=" + valid,
		"/api/chats/c1?anchor=oldestUnread&after=" + after,
		"/api/chats/c1?before=" + missingEpoch,
		"/api/chats/c1?after=" + valid,
		"/api/chats/c1?before=" + after,
	}
	for _, target := range tests {
		t.Run(target, func(t *testing.T) {
			status, _ := getMessagePage(t, target)
			if status != http.StatusBadRequest {
				t.Fatalf("status = %d, want %d", status, http.StatusBadRequest)
			}
		})
	}
}

func ids(messages []Message) string {
	result := ""
	for i, message := range messages {
		if i > 0 {
			result += ","
		}
		result += message.ID
	}
	return result
}

func TestHandleMessagesAcceptsChatIDPath(t *testing.T) {
	newTestStore(t)

	req := httptest.NewRequest(http.MethodGet, "/api/chats/c1", nil)
	rec := httptest.NewRecorder()

	handleMessages(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestHandleMessagesReturnsReceiptChanges(t *testing.T) {
	newTestStore(t)
	insertTestMessages(t, Message{ID: "m1", Timestamp: "2026-07-21T10:00:00Z", Status: "sent"})
	_, initial := getMessagePage(t, "/api/chats/c1")
	if err := store.UpdateMessageStatus([]string{"m1"}, "read"); err != nil {
		t.Fatal(err)
	}
	_, delta := getMessagePage(t, "/api/chats/c1?after="+url.QueryEscape(*initial.LatestCursor))
	if len(delta.Messages) != 1 || delta.Messages[0].ID != "m1" || delta.Messages[0].Status != "read" {
		t.Fatalf("receipt delta = %#v", delta.Messages)
	}
}

func TestHandleMessagesReturnsBackfilledChanges(t *testing.T) {
	newTestStore(t)
	insertTestMessages(t, Message{ID: "new", Timestamp: "2026-07-21T10:00:00Z"})
	_, initial := getMessagePage(t, "/api/chats/c1")
	insertTestMessages(t, Message{ID: "old", Timestamp: "2020-01-01T00:00:00Z"})
	_, delta := getMessagePage(t, "/api/chats/c1?after="+url.QueryEscape(*initial.LatestCursor))
	if ids(delta.Messages) != "old" {
		t.Fatalf("backfill delta ids = %q", ids(delta.Messages))
	}
}

func TestHandleMessagesOrdersEqualInstantsWithMixedOffsets(t *testing.T) {
	newTestStore(t)
	insertTestMessages(t,
		Message{ID: "a", Timestamp: "2026-07-21T12:00:00+02:00"},
		Message{ID: "b", Timestamp: "2026-07-21T10:00:00Z"},
		Message{ID: "c", Timestamp: "2026-07-21T10:01:00Z"},
	)
	_, first := getMessagePage(t, "/api/chats/c1?limit=2")
	if ids(first.Messages) != "b,c" || first.NextCursor == nil {
		t.Fatalf("first page ids = %q", ids(first.Messages))
	}
	_, older := getMessagePage(t, "/api/chats/c1?before="+url.QueryEscape(*first.NextCursor))
	if ids(older.Messages) != "a" {
		t.Fatalf("older page ids = %q", ids(older.Messages))
	}
}

func TestChatRoutesAreExactAndMethodSpecific(t *testing.T) {
	newTestStore(t)
	tests := []struct {
		method string
		path   string
		want   int
	}{
		{http.MethodGet, "/api/chats/c1", http.StatusOK},
		{http.MethodPost, "/api/chats/c1", http.StatusMethodNotAllowed},
		{http.MethodGet, "/api/chats/c1/send", http.StatusMethodNotAllowed},
		{http.MethodGet, "/api/chats/c1/messages", http.StatusNotFound},
		{http.MethodPost, "/api/chats/c1/send/extra", http.StatusNotFound},
	}
	for _, test := range tests {
		t.Run(test.method+" "+test.path, func(t *testing.T) {
			rec := httptest.NewRecorder()
			handleMessages(rec, httptest.NewRequest(test.method, test.path, nil))
			if rec.Code != test.want {
				t.Fatalf("status = %d, want %d", rec.Code, test.want)
			}
		})
	}
}

func TestSendMessageValidatesBeforeNetworkCall(t *testing.T) {
	oldWA := wa
	wa = nil
	t.Cleanup(func() { wa = oldWA })
	tests := []string{
		`{}`,
		`{"text":"   "}`,
		`{"text":`,
		`{"text":"ok","extra":true}`,
		`{"text":"` + strings.Repeat("x", maxMessageTextBytes+1) + `"}`,
	}
	for _, body := range tests {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/api/chats/c1/send", strings.NewReader(body))
		handleMessages(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("body length %d: status = %d, want %d", len(body), rec.Code, http.StatusBadRequest)
		}
	}
}

func TestSendMessageReturnsInformativeJSONErrors(t *testing.T) {
	oldWA := wa
	t.Cleanup(func() { wa = oldWA })
	tests := []struct {
		name   string
		chatID string
		setup  func()
		status int
		error  string
	}{
		{"unavailable", "123@s.whatsapp.net", func() { wa = nil }, http.StatusServiceUnavailable, "WhatsApp is unavailable"},
		{"invalid chat ID", "invalid chat id", func() { wa = &WAManager{} }, http.StatusBadRequest, "invalid chat ID"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			test.setup()
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPost, "/api/chats/c1/send", strings.NewReader(`{"text":"hello"}`))
			handleSendMessage(rec, req, test.chatID)
			if rec.Code != test.status || rec.Header().Get("Content-Type") != "application/json" {
				t.Fatalf("status = %d, content-type = %q", rec.Code, rec.Header().Get("Content-Type"))
			}
			var body map[string]string
			if err := json.NewDecoder(rec.Body).Decode(&body); err != nil || body["error"] != test.error {
				t.Fatalf("body = %#v, err = %v", body, err)
			}
		})
	}
}

func TestMigrateBackfillsExistingMessageMetadata(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	for _, query := range []string{
		`CREATE TABLE chats (jid TEXT PRIMARY KEY, last_message_timestamp TEXT)`,
		`CREATE TABLE messages (id TEXT PRIMARY KEY, chat_jid TEXT NOT NULL, sender_jid TEXT NOT NULL, text TEXT, timestamp TEXT, status TEXT, media_type TEXT, is_from_me INTEGER)`,
		`INSERT INTO chats (jid, last_message_timestamp) VALUES ('c1', '2026-07-21T12:00:00+02:00')`,
		`INSERT INTO messages (id, chat_jid, sender_jid, text, timestamp, status, is_from_me) VALUES ('m1', 'c1', 'sender', 'hello', '2026-07-21T12:00:00+02:00', 'sent', 0)`,
	} {
		if _, err := db.Exec(query); err != nil {
			t.Fatal(err)
		}
	}
	s := &UserDataStore{db: db}
	if err := s.migrate(); err != nil {
		t.Fatal(err)
	}
	messages, _, revision, err := s.GetMessages("c1", 10, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if ids(messages) != "m1" || revision == 0 {
		t.Fatalf("messages = %q, revision = %d", ids(messages), revision)
	}
	var epoch int64
	if err := db.QueryRow(`SELECT timestamp_epoch FROM messages WHERE id = 'm1'`).Scan(&epoch); err != nil || epoch == 0 {
		t.Fatalf("epoch = %d, err = %v", epoch, err)
	}
}
