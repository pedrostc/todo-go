package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
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
	Id   string
	Text string `json:"text,omitempty"`
	Done bool   `json:"done,omitempty"`
}

func (todo Todo) isEmpty() bool {
	return todo.Id == "" && todo.Text == "" && todo.Done == false
}

func main() {
	fmt.Printf("Starting the amazing API to patch TODOs\n")
	handleRequests()
}

func handleRequests() {

	router := mux.NewRouter().StrictSlash(true)

	router.Path("/todo/{id}").Methods(http.MethodPatch).HandlerFunc(updateTodoHandler)
	router.Path("/todo/patch/health").Methods(http.MethodGet).HandlerFunc(healthCheckHandler)

	router.Use(loggingMiddleware)

	log.Fatal(http.ListenAndServe(":10002", router))
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

func failOnError(err error, msg string) {
	if err != nil {
		log.Fatalf("%s: %s", msg, err)
	}
}

func updateTodoHandler(w http.ResponseWriter, r *http.Request) {

	if r.Method != http.MethodPatch {
		http.Error(w, "Method is not supported.", http.StatusMethodNotAllowed)
		return
	}

	reqBody, _ := ioutil.ReadAll(r.Body)

	var dadosJson Todo
	json.Unmarshal(reqBody, &dadosJson)

	if dadosJson.isEmpty() {
		http.Error(w, "Something went wrong while parsing the JSON from the request body.", http.StatusUnprocessableEntity)
		return
	}

	variables := mux.Vars(r)
	dadosJson.Id = variables["id"]

	todoBytes, err := json.Marshal(dadosJson)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	data, err := connectAndSend(todoBytes)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if validateResponse(data) {
		formatJsonResponse(w, data)
	} else {
		http.Error(w, "Not Found", http.StatusNotFound)
	}
}

func validateResponse(data []byte) bool {

	var todoJson Todo
	json.Unmarshal(data, &todoJson)

	return !todoJson.isEmpty()
}

func formatJsonResponse(w http.ResponseWriter, data []byte) {
	w.Header().Add("Content-Type", "application/json")
	w.Write(data)
}

func connectAndSend(todoBytes []byte) (res []byte, err error) {

	// INJECT BY ENV VAR
	conn, err := amqp.Dial("amqp://guest:guest@rabbitmq:5672/")

	if err != nil {
		return nil, fmt.Errorf("failed to connect to the message broker: %w", err)
	}

	defer conn.Close()

	ch, err := conn.Channel()

	if err != nil {
		return nil, fmt.Errorf("failed to open channel for message broker connection: %w", err)
	}

	defer ch.Close()

	msgs, err := ch.Consume(
		"amq.rabbitmq.reply-to", // queue
		"patch-controller",      // consumer
		true,                    // auto-ack
		false,                   // exclusive
		false,                   // no-local
		false,                   // no-wait
		nil,                     // args
	)

	if err != nil {
		return nil, fmt.Errorf("failed to register the reply queue consumer: %w", err)
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
		return nil, fmt.Errorf("failed to publish the message to the queue: %w", err)
	}

	for d := range msgs {
		if corrId == d.CorrelationId {
			fmt.Println("Message received from DAO")
			fmt.Printf("%#v \n", d)

			var data *Result
			err := json.Unmarshal(d.Body, &data)

			if err != nil {
				return nil, fmt.Errorf("failed to parse the message returned by the DAO: %w", err)
			}

			if data.Err != "" {
				return nil, fmt.Errorf("DAO error: %w", data.Err)
			}

			res = []byte(data.Result)
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
