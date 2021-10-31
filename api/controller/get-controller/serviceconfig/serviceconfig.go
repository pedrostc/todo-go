package serviceconfig

import "fmt"

type ServiceConfig struct {
	InboundQueueName  string `required:"true"`
	OutboundQueueName string `required:"true"`
	RabbitMqServer    string `required:"true"`
	RabbitMqPort      string `default:"5672"`
	RabbitMqUser      string `required:"true"`
	RabbitMqPswd      string `required:"true"`
}

func (c *ServiceConfig) GetRabbitConnString() string {
	return fmt.Sprintf("amqp://%s:%s@%s:%s/", c.RabbitMqUser, c.RabbitMqPswd, c.RabbitMqServer, c.RabbitMqPort)
}
