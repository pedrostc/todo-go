package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"

	"github.com/kelseyhightower/envconfig"
	"github.com/streadway/amqp"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func failOnError(err error, msg string) {
	if err != nil {
		log.Fatalf("%s: %s", msg, err)
	}
}

var collection *mongo.Collection
var ctx = context.TODO()

const inboudQueueNameVar = "INBOUND_QUEUE_NAME"

type Todo struct {
	Id string
}

type Result struct {
	Err    string
	Result string // result in json
}

type SvcConfiguration struct {
	InboundQueueName  string
	OutboundQueueName string
}

func init() {

	// var cred options.Credential

	// cred.AuthSource = "admin"
	// cred.Username = os.Getenv("MONGO_INITDB_ROOT_USERNAME")
	// cred.Password = os.Getenv("MONGO_INITDB_ROOT_PASSWORD")

	clientOptions := options.Client().ApplyURI("mongodb://root:example@mongo:27017/todoDB?authSource=admin")
	client, err := mongo.Connect(ctx, clientOptions)
	if err != nil {
		log.Fatal(err)
	}

	err = client.Ping(ctx, nil)
	if err != nil {
		log.Fatal(err)
	}

	collection = client.Database("todoDB").Collection("todos")
}

func main() {

	var c SvcConfiguration
	err := envconfig.Process("deletedao", &c)
	failOnError(err, "There was a problem loading the service configs.")

	conn, err := amqp.Dial("amqp://guest:guest@rabbitmq:5672/")
	failOnError(err, "Failed to connect to RabbitMQ")
	defer conn.Close()

	ch, err := conn.Channel()
	failOnError(err, "Failed to open a channel")
	defer ch.Close()

	q, err := ch.QueueDeclare(
		c.InboundQueueName, // name
		false,              // durable
		false,              // delete when unused
		false,              // exclusive
		false,              // no-wait
		nil,                // arguments
	)
	failOnError(err, "Failed to declare a queue")

	err = ch.Qos(
		1,     // prefetch count
		0,     // prefetch size
		false, // global
	)
	failOnError(err, "Failed to set QoS")

	msgs, err := ch.Consume(
		q.Name,       // queue
		"delete-dao", // consumer
		false,        // auto-ack
		false,        // exclusive
		false,        // no-local
		false,        // no-wait
		nil,          // args
	)
	failOnError(err, "Failed to register a consumer")

	forever := make(chan bool)

	go func() {
		for d := range msgs {
			log.Println("Message received")
			log.Printf("%#v \n", d)

			var data []byte

			var todoJson Todo
			json.Unmarshal(d.Body, &todoJson)

			data, err = deleteTodo(todoJson)

			errorMsg := ""
			if err != nil {
				fmt.Println("Failed to delete the requested items:", err)
				errorMsg = err.Error()
			}

			result := Result{errorMsg, string(data)}

			fmt.Printf("%#v", result)

			response, err := json.Marshal(result)

			if err != nil {
				fmt.Println("Failed to parse the result to JSON:", err)
			} else {
				fmt.Printf("data to send: %q \n", response)
			}

			err = ch.Publish(
				"",        // exchange
				d.ReplyTo, // routing key
				false,     // mandatory
				false,     // immediate
				amqp.Publishing{
					ContentType:   "text/plain",
					CorrelationId: d.CorrelationId,
					Body:          response,
				})

			if err != nil {
				fmt.Println("Failed to publish the response message:", err)
			}

			d.Ack(false)
		}
	}()

	log.Printf("Listening for todos...")
	<-forever
}

func deleteTodo(deletedTodo Todo) ([]byte, error) {

	filter := bson.M{"id": deletedTodo.Id}

	res, err := collection.DeleteOne(ctx, filter)

	if res.DeletedCount == 0 {
		return nil, errors.New("404")
	} else if err != nil {
		return nil, err
	}

	json, err := json.Marshal(deletedTodo)

	if err != nil {
		return nil, err
	}

	return json, nil

}
