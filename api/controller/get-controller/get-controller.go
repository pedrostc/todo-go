package main

import (
	"log"
	"net/http"
)

func main() {
	handleRequests()
}

func handleRequests() {
	http.HandleFunc("/todo", getTodos)
	http.HandleFunc("/todo/{id}", getSingleTodo)
	log.Fatal(http.ListenAndServe(":10000", nil))
}

func getTodos(w http.ResponseWriter, r *http.Request) {

	// get from mongo
}

func getSingleTodo(w http.ResponseWriter, r *http.Request) {

	// get from mongo
}
