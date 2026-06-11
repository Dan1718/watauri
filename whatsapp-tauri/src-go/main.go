package main

import (
	"log"
	"net/http"
)

func main() {
	http.HandleFunc("/health", withCORS(handleHealth))
	http.HandleFunc("/api/chats", withCORS(handleChats))
	http.HandleFunc("/api/chats/", withCORS(handleMessages))
	log.Println("Backend up and running on port 8090. ")
	log.Fatal(http.ListenAndServe(":8090", nil))
}
