package main

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const (
	defaultMessageLimit = 100
	maxMessageLimit     = 200
)

func withCORS(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		log.Printf("[http] %s %s from %s", r.Method, r.URL.Path, r.RemoteAddr)
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			log.Printf("[http] OPTIONS %s -> 204 (%v)", r.URL.Path, time.Since(start))
			return
		}
		h(w, r)
		log.Printf("[http] %s %s -> done (%v)", r.Method, r.URL.Path, time.Since(start))
	}
}

func handleHealth(w http.ResponseWriter, r *http.Request) {
	mode := "go"
	if wa.GetStatus() == "connected" {
		mode = "whatsmeow"
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"name":   "whatsapp-tauri",
		"status": "ok",
		"mode":   mode,
	})
	log.Printf("[http] GET /health -> mode=%s", mode)
}

func handleChats(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	chats, err := store.GetChats()
	if err != nil {
		log.Printf("[http] GET /api/chats error: %v", err)
		http.Error(w, `{"error":"failed to fetch chats"}`, http.StatusInternalServerError)
		return
	}
	if chats == nil {
		chats = []Chat{}
	}
	json.NewEncoder(w).Encode(chats)
	log.Printf("[http] GET /api/chats -> %d chats", len(chats))
}

func handleMessages(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(parts) < 3 {
		log.Printf("[http] GET %s -> 400 bad request (path parts: %v)", r.URL.Path, parts)
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}
	chatID := parts[2]
	log.Printf("[http] GET /api/chats/%s/messages", chatID)

	w.Header().Set("Content-Type", "application/json")
	limit, before, after, err := messagePageParams(r)
	if err != nil {
		http.Error(w, `{"error":"invalid pagination parameters"}`, http.StatusBadRequest)
		return
	}
	messages, hasMore, err := store.GetMessages(chatID, limit, before, after)
	if err != nil {
		log.Printf("[http] GET /api/chats/%s error: %v", chatID, err)
		http.Error(w, `{"error":"failed to fetch messages"}`, http.StatusInternalServerError)
		return
	}
	if messages == nil {
		messages = []Message{}
	}
	var nextCursor *string
	var latestCursor *string
	if len(messages) > 0 {
		cursor := encodeMessageCursor(messageCursor{Timestamp: messages[len(messages)-1].Timestamp, ID: messages[len(messages)-1].ID})
		latestCursor = &cursor
	}
	if hasMore && len(messages) > 0 {
		message := messages[0]
		if after != nil {
			message = messages[len(messages)-1]
		}
		cursor := encodeMessageCursor(messageCursor{Timestamp: message.Timestamp, ID: message.ID})
		nextCursor = &cursor
	}
	json.NewEncoder(w).Encode(MessagePage{Messages: messages, NextCursor: nextCursor, LatestCursor: latestCursor, HasMore: hasMore})
	log.Printf("[http] GET /api/chats/%s -> %d messages", chatID, len(messages))
}

func messagePageParams(r *http.Request) (int, *messageCursor, *messageCursor, error) {
	limit := defaultMessageLimit
	query := r.URL.Query()
	if values, ok := query["limit"]; ok {
		if len(values) != 1 {
			return 0, nil, nil, errors.New("invalid limit")
		}
		parsed, err := strconv.Atoi(values[0])
		if err != nil || parsed < 1 || parsed > maxMessageLimit {
			return 0, nil, nil, errors.New("invalid limit")
		}
		limit = parsed
	}

	if _, hasBefore := query["before"]; hasBefore {
		if _, hasAfter := query["after"]; hasAfter {
			return 0, nil, nil, errors.New("before and after are mutually exclusive")
		}
	}
	before, err := decodeMessageCursorParam(query, "before")
	if err != nil {
		return 0, nil, nil, err
	}
	after, err := decodeMessageCursorParam(query, "after")
	return limit, before, after, err
}

func decodeMessageCursorParam(query map[string][]string, name string) (*messageCursor, error) {
	values, ok := query[name]
	if !ok {
		return nil, nil
	}
	if len(values) != 1 || values[0] == "" {
		return nil, errors.New("invalid cursor")
	}
	decoded, err := base64.RawURLEncoding.DecodeString(values[0])
	if err != nil {
		return nil, errors.New("invalid cursor")
	}
	var cursor messageCursor
	if err := json.Unmarshal(decoded, &cursor); err != nil || cursor.Timestamp == "" || cursor.ID == "" {
		return nil, errors.New("invalid cursor")
	}
	if _, err := time.Parse(time.RFC3339, cursor.Timestamp); err != nil {
		return nil, errors.New("invalid cursor")
	}
	return &cursor, nil
}

func encodeMessageCursor(cursor messageCursor) string {
	payload, _ := json.Marshal(cursor)
	return base64.RawURLEncoding.EncodeToString(payload)
}

func handleAuthStatus(w http.ResponseWriter, r *http.Request) {
	status := wa.GetStatus()
	hasQR := wa.GetQR() != ""
	log.Printf("[http] GET /api/auth/status -> status=%s hasQR=%v", status, hasQR)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status": status,
		"qr":     wa.GetQR(),
	})
}

func handleAuthStart(w http.ResponseWriter, r *http.Request) {
	log.Println("[http] POST /api/auth/start requested")
	wa.StartPairing()
	status := wa.GetStatus()
	hasQR := wa.GetQR() != ""
	log.Printf("[http] POST /api/auth/start -> status=%s hasQR=%v", status, hasQR)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status": status,
		"qr":     wa.GetQR(),
	})
}

func handleAuthLogout(w http.ResponseWriter, r *http.Request) {
	log.Println("[http] POST /api/auth/logout requested")
	err := wa.Logout()
	if err != nil {
		log.Printf("[http] POST /api/auth/logout error: %v", err)
		http.Error(w, `{"error":"logout failed"}`, http.StatusInternalServerError)
		return
	}
	log.Println("[http] POST /api/auth/logout -> 200")
	w.WriteHeader(http.StatusOK)
}

func handleAuthReset(w http.ResponseWriter, r *http.Request) {
	log.Println("[http] POST /api/auth/reset requested")
	wa.ResetSession()
	log.Println("[http] POST /api/auth/reset -> 200 (session cleared)")
	w.WriteHeader(http.StatusOK)
}

func handleContacts(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	contacts, err := store.GetContacts()
	if err != nil {
		log.Printf("[http] GET /api/contacts error: %v", err)
		http.Error(w, `{"error":"failed to fetch contacts"}`, http.StatusInternalServerError)
		return
	}
	if contacts == nil {
		contacts = []User{}
	}
	json.NewEncoder(w).Encode(contacts)
	log.Printf("[http] GET /api/contacts -> %d contacts", len(contacts))
}
