package main

import (
	"context"
	"encoding/json"
	"log"
	"os"

	"github.com/google/uuid"
	"github.com/streadway/amqp"
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
	Id   string
	Text string
	Done bool
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

	conn, err := amqp.Dial("amqp://guest:guest@rabbitmq:5672/")
	failOnError(err, "Failed to connect to RabbitMQ")
	defer conn.Close()

	ch, err := conn.Channel()
	failOnError(err, "Failed to open a channel")
	defer ch.Close()

	var inboundQueueName = os.Getenv(inboudQueueNameVar)

	q, err := ch.QueueDeclare(
		inboundQueueName, // name
		false,            // durable
		false,            // delete when unused
		false,            // exclusive
		false,            // no-wait
		nil,              // arguments
	)
	failOnError(err, "Failed to declare a queue")

	msgs, err := ch.Consume(
		q.Name,     // queue
		"post-dao", // consumer
		true,       // auto-ack
		false,      // exclusive
		false,      // no-local
		false,      // no-wait
		nil,        // args
	)
	failOnError(err, "Failed to register a consumer")

	forever := make(chan bool)

	go func() {
		for d := range msgs {
			log.Printf("Received a message: %s", d.Body)

			newUuid := uuid.New().String()

			var todoJson Todo
			json.Unmarshal(d.Body, &todoJson)

			todoJson.Id = newUuid
			err = insertTodo(todoJson)
			if err != nil {
				log.Println(err)
			}
		}
	}()

	log.Printf("Listening for todos...")
	<-forever
}

func insertTodo(todo Todo) error {
	_, err := collection.InsertOne(ctx, todo)
	return err
}
