package main

import (
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"net/http"

	config "get-todo/serviceconfig"
	services "get-todo/services"

	"github.com/gorilla/mux"
	"github.com/kelseyhightower/envconfig"
)

type Result struct {
	Err    string
	Result string // result in json
}

type Todo struct {
	Id   string
	Text string
	Done bool
}

func (todo *Todo) isEmpty() bool {
	return todo.Id == "" && todo.Text == ""
}

func main() {
	fmt.Printf("Starting the amazing API to get TODOs\n")
	setupApiRouter()
}

func setupApiRouter() {
	router := mux.NewRouter().StrictSlash(true)
	getTodoRouter := router.PathPrefix("/todo").Methods(http.MethodGet).Subrouter()

	getTodoRouter.Path("/").HandlerFunc(listTodosHandler)
	getTodoRouter.Path("/{id}").HandlerFunc(retrieveTodoHandler)
	getTodoRouter.Path("/get/health").HandlerFunc(healthCheckHandler)

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

func connectAndSend(id []byte) (res []byte, err error) {

	// move to main init or.... main methods/ and inject them here.
	var c config.ServiceConfig
	err = envconfig.Process("get", &c)
	if err != nil {
		return nil, fmt.Errorf("there was a problem loading the service configs: %w", err)
	}

	corrId := randomString(32)

	rabbit := &services.RabbitQueue{Config: c}
	consumer, err := rabbit.PublishAndListen(id, corrId)

	if err != nil {
		return nil, fmt.Errorf("there was a problem sending a request to the DAL: %w", err)
	}

	defer rabbit.Close()

	for d := range consumer {
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
