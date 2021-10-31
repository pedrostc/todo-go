package services

import (
	"fmt"
	config "get-todo/serviceconfig"

	"github.com/streadway/amqp"
)

type RabbitQueue struct {
	connection   *amqp.Connection
	channel      *amqp.Channel
	replyChannel <-chan amqp.Delivery

	Config config.ServiceConfig
}

func (queue *RabbitQueue) Init(svcConfig config.ServiceConfig) {
	queue.Config = svcConfig
}

func (queue *RabbitQueue) Connect() (err error) {
	c := queue.Config

	queue.connection, err = amqp.Dial(c.GetRabbitConnString())
	if err != nil {
		return fmt.Errorf("failed to connect to the message broker: %w", err)
	}

	queue.channel, err = queue.connection.Channel()

	if err != nil {
		return fmt.Errorf("failed to open channel for message broker connection: %w", err)
	}

	return nil
}

func (queue *RabbitQueue) Close() (err error) {
	err = queue.channel.Close()
	if err != nil {
		return fmt.Errorf("failed to close the channel to the message broker: %w", err)
	}

	err = queue.connection.Close()
	if err != nil {
		return fmt.Errorf("failed to close the connection to the message broker: %w", err)
	}

	return nil
}

func (queue *RabbitQueue) initConsumer() (err error) {
	c := queue.Config

	queue.replyChannel, err = queue.channel.Consume(
		c.InboundQueueName, // queue
		c.ConsumerName,     // consumer
		true,               // auto-ack
		false,              // exclusive
		false,              // no-local
		false,              // no-wait
		nil,                // args
	)

	if err != nil {
		return fmt.Errorf("failed create a consumer to the reply queue: %w", err)
	}

	return nil
}

func (queue *RabbitQueue) publish(body []byte, correlationId string) (err error) {
	c := queue.Config

	err = queue.channel.Publish(
		"",                  // exchange
		c.OutboundQueueName, // routing key
		false,               // mandatory
		false,               // immediate
		amqp.Publishing{
			ContentType:   "text/plain",
			CorrelationId: correlationId,
			ReplyTo:       c.InboundQueueName,
			Body:          body,
		})

	if err != nil {
		return fmt.Errorf("failed to publish the message to the queue: %w", err)
	}

	return nil
}

func (queue *RabbitQueue) PublishAndListen(body []byte, correlationId string) (consumer <-chan amqp.Delivery, err error) {
	err = queue.Connect()
	if err != nil {
		return nil, err
	}

	err = queue.initConsumer()
	if err != nil {
		return nil, err
	}

	err = queue.publish(body, correlationId)
	if err != nil {
		return nil, err
	}

	return queue.replyChannel, nil
}
