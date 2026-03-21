package main

import (
	amqp "github.com/rabbitmq/amqp091-go"
)

func main() {
	const rabbitMQConnection = "amqp://guest:guest@localhost:5672"

	conn, err := amqp.Dial(rabbitMQConnection)
	if err != nil {
		panic(err)
	}
	defer conn.Close()

	println("Connected to RabbitMQ!")
}
