package main

import (
	"log"
	"net/http"
	"time"
)

var (
	wa    *WAManager
	store *UserDataStore
)

func main() {
	log.Println("[main] Starting WhatsApp Tauri backend...")

	var err error

	log.Println("[main] Initializing user data store...")
	store, err = newUserDataStore()
	if err != nil {
		log.Fatalf("[main] Failed to open userdata.db: %v", err)
	}
	log.Println("[main] User data store ready")

	log.Println("[main] Initializing WhatsApp manager...")
	wa, err = newWAManager(store)
	if err != nil {
		log.Fatalf("[main] Failed to create WAManager: %v", err)
	}
	log.Printf("[main] WhatsApp manager ready (status: %s)", wa.GetStatus())

	http.HandleFunc("/health", withCORS(handleHealth))
	http.HandleFunc("/api/chats", withCORS(handleChats))
	http.HandleFunc("/api/chats/", withCORS(handleMessages))
	http.HandleFunc("/api/profile", withCORS(handleProfile))
	http.HandleFunc("/api/auth/status", withCORS(handleAuthStatus))
	http.HandleFunc("/api/auth/start", withCORS(handleAuthStart))
	http.HandleFunc("/api/auth/logout", withCORS(handleAuthLogout))
	http.HandleFunc("/api/auth/reset", withCORS(handleAuthReset))
	http.HandleFunc("/api/contacts", withCORS(handleContacts))
	log.Println("[main] Registered 8 HTTP handlers")

	log.Printf("[main] Listening on :8090 at %s", time.Now().Format(time.RFC3339))
	log.Fatal(http.ListenAndServe(":8090", nil))
}
