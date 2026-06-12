package main

import (
	"log"
	"net/http"
)

var wa *WAManager

func main() {
	var err error
	wa, err = newWAManager()
	if err != nil {
		log.Fatal(err)
	}

	http.HandleFunc("/health", withCORS(handleHealth))
	http.HandleFunc("/api/chats", withCORS(handleChats))
	http.HandleFunc("/api/chats/", withCORS(handleMessages))
	http.HandleFunc("/api/auth/status", withCORS(handleAuthStatus))
	http.HandleFunc("/api/auth/start", withCORS(handleAuthStart))
	http.HandleFunc("/api/auth/logout", withCORS(handleAuthLogout))
	log.Println("Backend up and running on port 8090. ")
	log.Fatal(http.ListenAndServe(":8090", nil))
}
