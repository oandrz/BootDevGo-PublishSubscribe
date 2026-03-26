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

	queueKey := fmt.Sprintf("%s.*", routing.GameLogSlug)
	err = pubsub.SubscribeGob(
		conn,
		routing.ExchangePerilTopic,
		routing.GameLogSlug,
		queueKey,
		pubsub.Durable,
		handleLog(),
	)

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
}

func handleLog() func(log routing.GameLog, chn *amqp.Channel) pubsub.Acktype {
	return func(log routing.GameLog, ch *amqp.Channel) pubsub.Acktype {
		defer fmt.Print(">")
		err := gamelogic.WriteLog(log)
		if err != nil {
			fmt.Println("Error writing log: ", err)
			return pubsub.NackRequeue
		}
		return pubsub.Ack
	}
}
