package main

import (
	"log"
	"net/http"
)

var (
	wa    *WAManager
	store *UserDataStore
)

func main() {
	var err error

	store, err = newUserDataStore()
	if err != nil {
		log.Fatal(err)
	}

	wa, err = newWAManager(store)
	if err != nil {
		log.Fatal(err)
	}

	http.HandleFunc("/health", withCORS(handleHealth))
	http.HandleFunc("/api/chats", withCORS(handleChats))
	http.HandleFunc("/api/chats/", withCORS(handleMessages))
	http.HandleFunc("/api/auth/status", withCORS(handleAuthStatus))
	http.HandleFunc("/api/auth/start", withCORS(handleAuthStart))
	http.HandleFunc("/api/auth/logout", withCORS(handleAuthLogout))
	http.HandleFunc("/api/auth/reset", withCORS(handleAuthReset))
	log.Println("Backend up and running on port 8090. ")
	log.Fatal(http.ListenAndServe(":8090", nil))
}
