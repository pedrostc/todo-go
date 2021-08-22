package main

import (
	"fmt"
	"log"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/streadway/amqp"
)

func main() {
	fmt.Printf("Starting the amazing API to get TODOs\n")
	handleRequests()
}

func handleRequests() {

	router := mux.NewRouter().StrictSlash(true)

	router.HandleFunc("/todo", getTodos).Methods("GET")
	router.HandleFunc("/todo/{id}", getSingleTodo).Methods("GET")
	router.HandleFunc("/todo/get/health", healthCheck).Methods("GET")
	router.Use(loggingMiddleware)

	log.Fatal(http.ListenAndServe(":10001", router))
}

func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Do stuff here
		log.Println(r.RequestURI)
		// Call the next handler, which can be another middleware in the chain, or the final handler.
		next.ServeHTTP(w, r)
	})
}

func getTodos(w http.ResponseWriter, r *http.Request) {

	w.WriteHeader(http.StatusOK)

	if r.Method != http.MethodGet {
		http.Error(w, "Method is not supported.", http.StatusMethodNotAllowed)
		return
	}

	connectAndSend([]byte("0"))

}

func healthCheck(w http.ResponseWriter, r *http.Request) {

	if r.Method != http.MethodGet {
		http.Error(w, "Method is not supported.", http.StatusMethodNotAllowed)
		return
	}

	fmt.Fprintf(w, "We're good to go.")
}

func failOnError(err error, msg string) {
	if err != nil {
		log.Fatalf("%s: %s", msg, err)
	}
}

func getSingleTodo(w http.ResponseWriter, r *http.Request) {

	variables := mux.Vars(r)

	w.WriteHeader(http.StatusOK)

	if r.Method != http.MethodGet {
		http.Error(w, "Method is not supported.", http.StatusMethodNotAllowed)
		return
	}

	connectAndSend([]byte(variables["id"]))

}

func connectAndSend(id []byte) {

	conn, err := amqp.Dial("amqp://guest:guest@rabbitmq:5672/")
	failOnError(err, "Failed to connect to RabbitMQ")
	defer conn.Close()

	ch, err := conn.Channel()
	failOnError(err, "Failed to open a channel")
	defer ch.Close()

	q, err := ch.QueueDeclare(
		"get", // name
		false, // durable
		false, // delete when unused
		false, // exclusive
		false, // no-wait
		nil,   // arguments
	)
	failOnError(err, "Failed to declare a queue")

	err = ch.Publish(
		"",     // exchange
		q.Name, // routing key
		false,  // mandatory
		false,  // immediate
		amqp.Publishing{
			ContentType: "text/plain",
			Body:        id,
		})
	failOnError(err, "Failed to publish a message")

}
