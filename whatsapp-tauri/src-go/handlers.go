package main

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const (
	defaultMessageLimit = 100
	maxMessageLimit     = 200
	maxMessageTextBytes = 4096
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
	if r.URL.Path != "/api/chats" {
		http.NotFound(w, r)
		return
	}
	if r.Method != http.MethodGet {
		methodNotAllowed(w, http.MethodGet)
		return
	}
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
	path := strings.TrimPrefix(r.URL.Path, "/api/chats/")
	parts := strings.Split(path, "/")
	if path == r.URL.Path || parts[0] == "" || len(parts) > 2 {
		http.NotFound(w, r)
		return
	}
	chatID := parts[0]
	if len(parts) == 2 {
		switch parts[1] {
		case "send":
			if r.Method != http.MethodPost {
				methodNotAllowed(w, http.MethodPost)
				return
			}
			handleSendMessage(w, r, chatID)
			return
		case "read":
			if r.Method != http.MethodPost {
				methodNotAllowed(w, http.MethodPost)
				return
			}
			handleReadMessage(w, r, chatID)
			return
		default:
			http.NotFound(w, r)
			return
		}
	}
	if r.Method != http.MethodGet {
		methodNotAllowed(w, http.MethodGet)
		return
	}
	log.Printf("[http] GET /api/chats/%s/messages", chatID)

	w.Header().Set("Content-Type", "application/json")
	limit, before, after, err := messagePageParams(r)
	if err != nil {
		http.Error(w, `{"error":"invalid pagination parameters"}`, http.StatusBadRequest)
		return
	}
	messages, hasMore, latestRevision, err := store.GetMessages(chatID, limit, before, after)
	if err != nil {
		log.Printf("[http] GET /api/chats/%s error: %v", chatID, err)
		http.Error(w, `{"error":"failed to fetch messages"}`, http.StatusInternalServerError)
		return
	}
	if messages == nil {
		messages = []Message{}
	}
	var nextCursor *string
	revision := latestRevision
	if after != nil && (hasMore || len(messages) == 0) {
		revision = after.Revision
		if len(messages) > 0 {
			revision = messages[len(messages)-1].Revision
		}
	}
	latest := encodeMessageCursor(messageCursor{Version: 1, Mode: "after", Revision: revision})
	latestCursor := &latest
	if hasMore && len(messages) > 0 {
		message := messages[0]
		cursor := messageCursor{Version: 1, Mode: "before", ID: message.ID}
		if after != nil {
			message = messages[len(messages)-1]
			cursor = messageCursor{Version: 1, Mode: "after", Revision: message.Revision}
		} else {
			epoch, _ := timestampEpoch(message.Timestamp)
			cursor.TimestampEpoch = epoch
		}
		encoded := encodeMessageCursor(cursor)
		nextCursor = &encoded
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
	before, err := decodeMessageCursorParam(query, "before", "before")
	if err != nil {
		return 0, nil, nil, err
	}
	after, err := decodeMessageCursorParam(query, "after", "after")
	return limit, before, after, err
}

func decodeMessageCursorParam(query map[string][]string, name, mode string) (*messageCursor, error) {
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
	if err := json.Unmarshal(decoded, &cursor); err != nil || cursor.Version != 1 || cursor.Mode != mode {
		return nil, errors.New("invalid cursor")
	}
	if mode == "before" && (cursor.ID == "" || cursor.TimestampEpoch == 0 || cursor.Revision != 0) {
		return nil, errors.New("invalid cursor")
	}
	if mode == "after" && (cursor.ID != "" || cursor.TimestampEpoch != 0 || cursor.Revision < 0) {
		return nil, errors.New("invalid cursor")
	}
	return &cursor, nil
}

func encodeMessageCursor(cursor messageCursor) string {
	payload, _ := json.Marshal(cursor)
	return base64.RawURLEncoding.EncodeToString(payload)
}

func handleSendMessage(w http.ResponseWriter, r *http.Request, chatID string) {
	var body struct {
		Text string `json:"text"`
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxMessageTextBytes+1024)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&body); err != nil {
		writeJSONError(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		writeJSONError(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if strings.TrimSpace(body.Text) == "" || len([]byte(body.Text)) > maxMessageTextBytes {
		writeJSONError(w, "text must be between 1 and 4096 bytes", http.StatusBadRequest)
		return
	}
	if wa == nil {
		writeJSONError(w, "WhatsApp is unavailable", http.StatusServiceUnavailable)
		return
	}
	message, err := wa.SendText(r.Context(), chatID, body.Text)
	if err != nil {
		log.Printf("[http] POST /api/chats/%s/send error: %v", chatID, err)
		switch {
		case errors.Is(err, errInvalidChatID):
			writeJSONError(w, "invalid chat ID", http.StatusBadRequest)
		case errors.Is(err, errWAUnavailable):
			writeJSONError(w, "WhatsApp is unavailable", http.StatusServiceUnavailable)
		case errors.Is(err, errPersistMessage):
			writeJSONError(w, "failed to store message", http.StatusInternalServerError)
		default:
			writeJSONError(w, "failed to send message", http.StatusBadGateway)
		}
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(message)
}

func handleReadMessage(w http.ResponseWriter, r *http.Request, chatID string) {
	var body struct {
		SendReceipt *bool    `json:"sendReceipt"`
		MessageIds  []string `json:"messageIds"`
	}
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&body); err != nil && !errors.Is(err, io.EOF) {
		writeJSONError(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		writeJSONError(w, "invalid request body", http.StatusBadRequest)
		return
	}
	sendReceipt := true
	if body.SendReceipt != nil {
		sendReceipt = *body.SendReceipt
	}

	if wa == nil {
		writeJSONError(w, "whatsapp is unavailable", http.StatusServiceUnavailable)
		return
	}
	unreadCount, err := wa.MarkRead(r.Context(), chatID, sendReceipt, body.MessageIds)
	if err != nil {
		log.Printf("[http] POST /api/chats/%s/read error: %v", chatID, err)
		switch {
		case errors.Is(err, errInvalidChatID):
			writeJSONError(w, "invalid chat ID", http.StatusBadRequest)
		case errors.Is(err, errWAUnavailable):
			writeJSONError(w, "WhatsApp is unavailable", http.StatusServiceUnavailable)
		case errors.Is(err, errPersistMessage):
			writeJSONError(w, "failed to update local read state", http.StatusInternalServerError)
		default:
			writeJSONError(w, "failed to mark chat read", http.StatusBadGateway)
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]int{"unreadCount": unreadCount})
}

func writeJSONError(w http.ResponseWriter, message string, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{"error": message})
}

func methodNotAllowed(w http.ResponseWriter, methods ...string) {
	w.Header().Set("Allow", strings.Join(methods, ", "))
	http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
}

func handleProfile(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, http.MethodGet)
		return
	}
	profile := Profile{}
	if wa != nil {
		profile = wa.GetProfile()
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(profile)
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
