package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"net/http"

	"github.com/gorilla/mux"

	"github.com/streadway/amqp"
)

func main() {
	fmt.Printf("Starting the amazing API to post TODOs\n")
	handleRequests()
}

func handleRequests() {

	router := mux.NewRouter().StrictSlash(true)

	router.HandleFunc("/todo", postTodo).Methods("POST")
	router.HandleFunc("/todo/post/health", healthCheck).Methods("GET")
	router.Use(loggingMiddleware)

	log.Fatal(http.ListenAndServe(":10000", router))
}

func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Do stuff here
		log.Println(r.RequestURI)
		// Call the next handler, which can be another middleware in the chain, or the final handler.
		next.ServeHTTP(w, r)
	})
}

func healthCheck(w http.ResponseWriter, r *http.Request) {
	fmt.Printf("in the GET\n")

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

func postTodo(w http.ResponseWriter, r *http.Request) {
	fmt.Printf("in the POST\n")

	if r.Method != http.MethodPost {
		http.Error(w, "Method is not supported.", http.StatusMethodNotAllowed)
		return
	}

	fmt.Printf("REALLY in the POST\n")

	log.Printf("request receive", r.Host, r.Method, r.RequestURI)

	reqBody, _ := ioutil.ReadAll(r.Body)

	conn, err := amqp.Dial("amqp://guest:guest@rabbitmq:5672/")
	failOnError(err, "Failed to connect to RabbitMQ")
	defer conn.Close()

	ch, err := conn.Channel()
	failOnError(err, "Failed to open a channel")
	defer ch.Close()

	q, err := ch.QueueDeclare(
		"post", // name
		false,  // durable
		false,  // delete when unused
		false,  // exclusive
		false,  // no-wait
		nil,    // arguments
	)
	failOnError(err, "Failed to declare a queue")

	err = ch.Publish(
		"",     // exchange
		q.Name, // routing key
		false,  // mandatory
		false,  // immediate
		amqp.Publishing{
			ContentType: "text/plain",
			Body:        reqBody,
		})
	failOnError(err, "Failed to publish a message")

}
