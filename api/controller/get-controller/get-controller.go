package main

import (
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/kelseyhightower/envconfig"
	"github.com/streadway/amqp"
)

const InboundQueueVar = "INBOUND_QUEUE_NAME"
const OutboundQueueVar = "OUTBOUND_QUEUE_NAME"

type SvcConfiguration struct {
	InboundQueueName  string
	OutboundQueueName string
}

type Result struct {
	Err    string
	Result string // result in json
}

type Todo struct {
	Id   string
	Text string
	Done bool
}

func (todo Todo) isEmpty() bool {
	return todo.Id == "" && todo.Text == ""
}

func main() {
	fmt.Printf("Starting the amazing API to get TODOs\n")
	setupApiRouter()
}

func setupApiRouter() {
	router := mux.NewRouter().StrictSlash(true)

	router.Path("/todo").Methods(http.MethodGet).HandlerFunc(listTodosHandler)
	router.Path("/todo/{id}").Methods(http.MethodGet).HandlerFunc(retrieveTodoHandler)
	router.Path("/todo/get/health").Methods(http.MethodGet).HandlerFunc(healthCheckHandler)

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

func formatJsonResponse(w http.ResponseWriter, data []byte) {
	w.Header().Add("Content-Type", "application/json")
	w.Write(data)
}

func validateResponse(data []byte) bool {

	var todoJson Todo
	json.Unmarshal(data, &todoJson)

	return !todoJson.isEmpty()
}

func listTodosHandler(w http.ResponseWriter, r *http.Request) {
	data, err := connectAndSend([]byte("0"))

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	formatJsonResponse(w, data)
}

func retrieveTodoHandler(w http.ResponseWriter, r *http.Request) {
	variables := mux.Vars(r)
	data, err := connectAndSend([]byte(variables["id"]))

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

func connectAndSend(id []byte) (res []byte, err error) {

	var c SvcConfiguration
	err = envconfig.Process("get", &c)
	failOnError(err, "There was a problem loading the service configs.")

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
		c.InboundQueueName, // queue
		"get-controller",   // consumer
		true,               // auto-ack
		false,              // exclusive
		false,              // no-local
		false,              // no-wait
		nil,                // args
	)

	if err != nil {
		return nil, fmt.Errorf("failed to register the reply queue consumer: %w", err)
	}

	corrId := randomString(32)

	err = ch.Publish(
		"",                  // exchange
		c.OutboundQueueName, // routing key
		false,               // mandatory
		false,               // immediate
		amqp.Publishing{
			ContentType:   "text/plain",
			CorrelationId: corrId,
			ReplyTo:       c.InboundQueueName,
			Body:          id,
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
