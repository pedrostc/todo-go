package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"os"

	"github.com/gorilla/mux"
	"github.com/streadway/amqp"
)

const OutboundQueueVar = "OUTBOUND_QUEUE_NAME"

type Result struct {
	Err    string
	Result string // result in json
}

type Todo struct {
	Id string
}

func (todo Todo) isEmpty() bool {
	return todo.Id == ""
}

func main() {
	fmt.Printf("Starting the amazing API to delete TODOs\n")
	handleRequests()
}

func handleRequests() {

	router := mux.NewRouter().StrictSlash(true)

	router.Path("/todo/{id}").Methods(http.MethodDelete).HandlerFunc(deleteTodoHandler)
	router.Path("/todo/delete/health").Methods(http.MethodGet).HandlerFunc(healthCheckHandler)

	router.Use(loggingMiddleware)

	log.Fatal(http.ListenAndServe(":10003", router))
}

func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Do stuff here
		log.Println(r.RequestURI)
		log.Println(r.RemoteAddr)
		// Call the next handler, which can be another middleware in the chain, or the final handler.
		next.ServeHTTP(w, r)
	})
}

func healthCheckHandler(w http.ResponseWriter, r *http.Request) {

	if r.Method != http.MethodGet {
		http.Error(w, "Method is not supported.", http.StatusMethodNotAllowed)
		return
	}

	fmt.Fprintf(w, "We're good to go.")
}

func deleteTodoHandler(w http.ResponseWriter, r *http.Request) {

	if r.Method != http.MethodDelete {
		http.Error(w, "Method is not supported.", http.StatusMethodNotAllowed)
		return
	}

	var dadosJson Todo

	variables := mux.Vars(r)
	dadosJson.Id = variables["id"]

	todoBytes, err := json.Marshal(dadosJson)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	err = connectAndSend(todoBytes)
	if err != nil {
		if err.Error() == "404" {
			http.Error(w, "Not Found", http.StatusNotFound)
			return
		}

		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func connectAndSend(todoBytes []byte) (err error) {

	// INJECT BY ENV VAR
	conn, err := amqp.Dial("amqp://guest:guest@rabbitmq:5672/")

	if err != nil {
		return fmt.Errorf("failed to connect to the message broker: %w", err)
	}

	defer conn.Close()

	ch, err := conn.Channel()

	if err != nil {
		return fmt.Errorf("failed to open channel for message broker connection: %w", err)
	}

	defer ch.Close()

	msgs, err := ch.Consume(
		"amq.rabbitmq.reply-to", // queue
		"delete-controller",     // consumer
		true,                    // auto-ack
		false,                   // exclusive
		false,                   // no-local
		false,                   // no-wait
		nil,                     // args
	)

	if err != nil {
		return fmt.Errorf("failed to register the reply queue consumer: %w", err)
	}

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
			Body:          todoBytes,
		})

	if err != nil {
		return fmt.Errorf("failed to publish the message to the queue: %w", err)
	}

	for d := range msgs {
		if corrId == d.CorrelationId {
			fmt.Println("Message received from DAO")
			fmt.Printf("%#v \n", d)

			var data *Result
			err := json.Unmarshal(d.Body, &data)

			if err != nil {
				return fmt.Errorf("failed to parse the message returned by the DAO: %w", err)
			}

			if data.Err == "404" {
				return errors.New("404")
			}

			if data.Err != "" {
				return fmt.Errorf("DAO error: %w", data.Err)
			}

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
