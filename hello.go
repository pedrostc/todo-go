package main

import (
	"fmt"
	"log"
	"net/http"
)

func main() {
	var getHello = func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method is not supported.", http.StatusMethodNotAllowed)
			return
		}

		fmt.Fprintf(w, "Hello!")
	}

	http.HandleFunc("/hello", getHello)

	fmt.Printf("Starting server at port 8080\n")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatal(err)
	}
}
