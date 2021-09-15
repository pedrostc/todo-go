package main

import (
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"os"

	"github.com/gorilla/mux"
	"github.com/streadway/amqp"
)

const InboundQueueVar = "INBOUND_QUEUE_NAME"
const OutboundQueueVar = "OUTBOUND_QUEUE_NAME"

func main() {
	fmt.Printf("Starting the amazing API to get TODOs\n")
	setupApiRouter()
}

func setupApiRouter() {
	router := mux.NewRouter().StrictSlash(true)

	router.Path("/todo").Methods(http.MethodGet).HandlerFunc(listTodos)
	router.Path("/todo/{id}").Methods(http.MethodGet).HandlerFunc(retrieveTodo)
	router.Path("/todo/get/health").Methods(http.MethodGet).HandlerFunc(healthCheck)

	router.Use(setupLoggingMiddleware)

	log.Fatal(http.ListenAndServe(":10001", router))
}

func setupLoggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Do stuff here
		log.Println(r.RequestURI)
		log.Println(r.RemoteAddr)
		// Call the next handler, which can be another middleware in the chain, or the final handler.
		next.ServeHTTP(w, r)
	})
}

func listTodos(w http.ResponseWriter, r *http.Request) {

	if r.Method != http.MethodGet {
		http.Error(w, "Method is not supported.", http.StatusMethodNotAllowed)
		return
	}

	data, err := connectAndSend([]byte("0"))

	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(err.Error()))

		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write(data)
}

func retrieveTodo(w http.ResponseWriter, r *http.Request) {

	variables := mux.Vars(r)

	if r.Method != http.MethodGet {
		http.Error(w, "Method is not supported.", http.StatusMethodNotAllowed)
		return
	}

	data, err := connectAndSend([]byte(variables["id"]))

	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(err)

		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(data)
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

func connectAndSend(id []byte) (res []byte, err error) {

	conn, err := amqp.Dial("amqp://guest:guest@rabbitmq:5672/")
	failOnError(err, "Failed to connect to RabbitMQ")
	defer conn.Close()

	ch, err := conn.Channel()
	failOnError(err, "Failed to open a channel")
	defer ch.Close()

	msgs, err := ch.Consume(
		"amq.rabbitmq.reply-to", // queue
		"get-controller",        // consumer
		true,                    // auto-ack
		false,                   // exclusive
		false,                   // no-local
		false,                   // no-wait
		nil,                     // args
	)
	failOnError(err, "Failed to register a consumer")

	corrId := randomString(32)

	err = ch.Publish(
		"",                          // exchange
		os.Getenv(OutboundQueueVar), // routing key
		false,                       // mandatory
		false,                       // immediate
		amqp.Publishing{
			ContentType:   "text/plain",
			CorrelationId: corrId,
			ReplyTo:       "amq.rabbitmq.reply-to",
			Body:          id,
		})
	failOnError(err, "Failed to publish a message")

	for d := range msgs {
		if corrId == d.CorrelationId {
			fmt.Println("Message received from DAO")
			res = d.Body
			break
		}
	}

	return
}

func randomString(l int) string {
	bytes := make([]byte, l)
	for i := 0; i < l; i++ {
		bytes[i] = byte(randInt(65, 90))
	}
	return string(bytes)
}

func randInt(min int, max int) int {
	return min + rand.Intn(max-min)
}
