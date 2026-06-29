package main

import (
	"encoding/json"
	"net/http"
	"strings"
)

func withCORS(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		h(w, r)
	}
}

func handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"name":   "whatsapp-tauri",
		"status": "ok",
		"mode":   "go",
	})
}

func handleChats(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(mockChats)
}

func handleMessages(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(parts) < 3 {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}
	chatID := parts[2]
	messages, ok := mockMessages[chatID]
	if !ok {
		messages = []Message{}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(messages)
}

func handleAuthStatus(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status": wa.GetStatus(),
		"qr":     wa.GetQR(),
	})
}

func handleAuthStart(w http.ResponseWriter, r *http.Request) {
	wa.StartPairing()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status": wa.GetStatus(),
		"qr":     wa.GetQR(),
	})
}

func handleAuthLogout(w http.ResponseWriter, r *http.Request) {
	wa.Disconnect()
	w.WriteHeader(http.StatusOK)
}
