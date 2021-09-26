package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/kelseyhightower/envconfig"
	"github.com/streadway/amqp"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
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

type Todo struct {
	Id   string `bson:"_id"`
	Text string `bson:"text"`
	Done bool   `bson:"done"`
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
	err := envconfig.Process("getdao", &c)
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
		q.Name,    // queue
		"get-dao", // consumer
		false,     // auto-ack
		false,     // exclusive
		false,     // no-local
		false,     // no-wait
		nil,       // args
	)
	failOnError(err, "Failed to register a consumer")

	forever := make(chan bool)

	go func() {
		for d := range msgs {
			log.Println("Message received")
			log.Printf("%#v \n", d)

			id := string(d.Body)

			var data []byte

			if id == "0" {
				data, err = getTodos()
			} else {
				data, err = getTodo(id)
			}

			errorMsg := ""
			if err != nil {
				fmt.Println("Failed to retrieve the requested items:", err)
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

	log.Printf("Listening for getTodos...")
	<-forever // just keeps on reading the values from the channel ?
}

func getTodos() ([]byte, error) {
	var todos []Todo

	cur, err := collection.Find(ctx, bson.D{})

	if err != nil {
		return nil, err
	}

	for cur.Next(context.TODO()) {
		var todo Todo
		err := cur.Decode(&todo)

		if err != nil {
			return nil, err
		}

		todos = append(todos, todo)
	}

	cur.Close(context.TODO())

	json, err := json.Marshal(todos)

	if err != nil {
		return nil, err
	}

	return json, nil
}

func getTodo(id string) ([]byte, error) {
	var todo Todo

	objId, err := primitive.ObjectIDFromHex(id)

	if err != nil {
		return nil, err
	}

	filter := bson.M{"_id": objId}

	err = collection.FindOne(ctx, filter).Decode(&todo)

	if err == mongo.ErrNoDocuments {
		todo = Todo{}
	} else if err != nil {
		return nil, err
	}

	json, err := json.Marshal(todo)

	if err != nil {
		return nil, err
	}

	return json, nil
}
