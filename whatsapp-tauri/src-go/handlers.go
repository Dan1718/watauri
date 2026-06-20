package main

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"time"
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
	if len(parts) < 4 {
		log.Printf("[http] GET %s -> 400 bad request (path parts: %v)", r.URL.Path, parts)
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}
	chatID := parts[2]
	log.Printf("[http] GET /api/chats/%s/messages", chatID)

	w.Header().Set("Content-Type", "application/json")
	messages, err := store.GetMessages(chatID)
	if err != nil {
		log.Printf("[http] GET /api/chats/%s error: %v", chatID, err)
		http.Error(w, `{"error":"failed to fetch messages"}`, http.StatusInternalServerError)
		return
	}
	if messages == nil {
		messages = []Message{}
	}
	json.NewEncoder(w).Encode(messages)
	log.Printf("[http] GET /api/chats/%s -> %d messages", chatID, len(messages))
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
