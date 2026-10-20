package main

import (
	"bufio"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"go.mau.fi/whatsmeow/proto/waE2E"
	whatsmeowTypes "go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	"google.golang.org/protobuf/proto"
)

func TestEventHubPublishesAndCleansUpSubscribers(t *testing.T) {
	hub := NewEventHub()
	updates, unsubscribe := hub.Subscribe()
	if got := hub.SubscriberCount(); got != 1 {
		t.Fatalf("subscriber count = %d, want 1", got)
	}

	want := RealtimeEvent{Type: "receipt", ChatID: "chat@g.us", MessageIDs: []string{"m1", "m2"}}
	hub.Publish(want)
	select {
	case got := <-updates:
		if got.Type != want.Type || got.ChatID != want.ChatID || strings.Join(got.MessageIDs, ",") != "m1,m2" {
			t.Fatalf("event = %#v, want %#v", got, want)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for published event")
	}

	unsubscribe()
	unsubscribe()
	if got := hub.SubscriberCount(); got != 0 {
		t.Fatalf("subscriber count after cleanup = %d, want 0", got)
	}
	hub.Publish(RealtimeEvent{Type: "chat"}) // Publishing after cleanup must be safe.
}

func TestEventHubReplacesBacklogForSlowSubscribers(t *testing.T) {
	hub := NewEventHub()
	updates, unsubscribe := hub.Subscribe()
	defer unsubscribe()

	for i := 0; i < realtimeClientBuffer+1; i++ {
		hub.Publish(RealtimeEvent{Type: "message", ChatID: fmt.Sprintf("chat-%d", i)})
	}
	if got := len(updates); got != 1 {
		t.Fatalf("queued event count after overflow = %d, want 1 resync event", got)
	}
	if got := <-updates; got.Type != "resync" {
		t.Fatalf("overflow event type = %q, want resync", got.Type)
	}
}

func TestHandleEventsStreamsEventsAndCleansUpOnDisconnect(t *testing.T) {
	hub := NewEventHub()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handleEventsWithHub(w, r, hub)
	}))
	defer server.Close()

	request, err := http.NewRequest(http.MethodGet, server.URL, nil)
	if err != nil {
		t.Fatal(err)
	}
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != http.StatusOK || !strings.HasPrefix(response.Header.Get("Content-Type"), "text/event-stream") {
		response.Body.Close()
		t.Fatalf("SSE response = %d %q", response.StatusCode, response.Header.Get("Content-Type"))
	}

	deadline := time.Now().Add(time.Second)
	for hub.SubscriberCount() != 1 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if hub.SubscriberCount() != 1 {
		response.Body.Close()
		t.Fatal("handler did not register an event subscriber")
	}
	hub.Publish(RealtimeEvent{Type: "message", ChatID: "chat-1", MessageIDs: []string{"msg-1"}})

	scanner := bufio.NewScanner(response.Body)
	got := ""
	for i := 0; i < 2; i++ {
		if !scanner.Scan() {
			response.Body.Close()
			t.Fatalf("failed reading SSE frame: %v", scanner.Err())
		}
		got += scanner.Text() + "\n"
	}
	if !strings.Contains(got, "event: message") || !strings.Contains(got, `"chatId":"chat-1"`) {
		response.Body.Close()
		t.Fatalf("SSE frame = %q", got)
	}
	if err := response.Body.Close(); err != nil {
		t.Fatalf("close SSE response: %v", err)
	}

	deadline = time.Now().Add(time.Second)
	for hub.SubscriberCount() != 0 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if hub.SubscriberCount() != 0 {
		t.Fatalf("subscriber count after disconnect = %d, want 0", hub.SubscriberCount())
	}
}

func TestHandleEventsRejectsNonGetRequests(t *testing.T) {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/events", nil)
	handleEventsWithHub(recorder, request, NewEventHub())
	if recorder.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusMethodNotAllowed)
	}
}

func TestLiveMessagePersistencePublishesChatScopedInvalidation(t *testing.T) {
	newTestStore(t)
	updates, unsubscribe := realtimeHub.Subscribe()
	defer unsubscribe()
	chatJID := parseTestJID(t, "123@s.whatsapp.net")
	senderJID := parseTestJID(t, "456@s.whatsapp.net")
	manager := &WAManager{store: store}
	event := &events.Message{
		Info: whatsmeowTypes.MessageInfo{
			MessageSource: whatsmeowTypes.MessageSource{Chat: chatJID, Sender: senderJID},
			ID:            "message-1",
			Timestamp:     time.Now(),
		},
		Message: &waE2E.Message{Conversation: proto.String("hello")},
	}
	if !manager.storeMessageEvent(event, "live") {
		t.Fatal("live message was not persisted")
	}
	select {
	case got := <-updates:
		if got.Type != "message" || got.ChatID != chatJID.String() || len(got.MessageIDs) != 1 || got.MessageIDs[0] != "message-1" {
			t.Fatalf("message invalidation = %#v", got)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for message invalidation")
	}
}
