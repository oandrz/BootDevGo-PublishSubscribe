package main

import (
	"fmt"

	"github.com/bootdotdev/learn-pub-sub-starter/internal/gamelogic"
	"github.com/bootdotdev/learn-pub-sub-starter/internal/pubsub"
	"github.com/bootdotdev/learn-pub-sub-starter/internal/routing"
	amqp "github.com/rabbitmq/amqp091-go"
)

func main() {
	const rabbitMQConnection = "amqp://guest:guest@localhost:5672"

	conn, err := amqp.Dial(rabbitMQConnection)
	if err != nil {
		panic(err)
	}
	defer conn.Close()

	chnl, err := conn.Channel()
	if err != nil {
		panic(err)
	}
	defer chnl.Close()

	gamelogic.PrintServerHelp()

	for {
		input := gamelogic.GetInput()
		if len(input) == 0 {
			continue
		}

		switch input[0] {
		case "pause":
			fmt.Println("Pausing game...")
			err = pubsub.PublishJSON(chnl, routing.ExchangePerilDirect, routing.PauseKey, routing.PlayingState{IsPaused: true})
			if err != nil {
				fmt.Println("Error publishing pause message: ", err)
			}
			break
		case "resume":
			fmt.Println("Resuming game...")
			err = pubsub.PublishJSON(chnl, routing.ExchangePerilDirect, routing.PauseKey, routing.PlayingState{IsPaused: false})
			if err != nil {
				fmt.Println("Error publishing resume message: ", err)
			}
			break
		case "quit":
			fmt.Println("Quitting...")
			return
		default:
			fmt.Println("Unknown command")
			break
		}
	}

	println("Connected to RabbitMQ!")
}
