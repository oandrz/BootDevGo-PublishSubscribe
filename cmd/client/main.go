package main

import (
	"fmt"
	"time"

	"github.com/bootdotdev/learn-pub-sub-starter/internal"
	"github.com/bootdotdev/learn-pub-sub-starter/internal/gamelogic"
	"github.com/bootdotdev/learn-pub-sub-starter/internal/pubsub"
	"github.com/bootdotdev/learn-pub-sub-starter/internal/routing"
	amqp "github.com/rabbitmq/amqp091-go"
)

func main() {
	const rabbitMQConnection = "amqp://guest:guest@localhost:5672"

	conn, err := amqp.Dial(rabbitMQConnection)
	if err != nil {
		fmt.Println("Error connecting to RabbitMQ: ", err)
		panic(err)
	}
	defer conn.Close()

	chnl, err := conn.Channel()
	if err != nil {
		panic(err)
	}
	defer chnl.Close()

	username, err := gamelogic.ClientWelcome()
	if err != nil {
		panic(err)
	}

	gameState := gamelogic.NewGameState(username)

	moveQueueName := fmt.Sprintf("%s.%v", routing.ArmyMovesPrefix, username)
	moveQueueKey := fmt.Sprintf("%s.*", routing.ArmyMovesPrefix)
	err = pubsub.SubscribeJSON(
		conn,
		routing.ExchangePerilTopic,
		moveQueueName,
		moveQueueKey,
		pubsub.Transient,
		handlerMove(gameState),
	)
	if err != nil {
		fmt.Println("Error subscribing to move channel: ", err)
		panic(err)
	}

	queueName := fmt.Sprintf("%s.%v", routing.PauseKey, username)
	err = pubsub.SubscribeJSON(
		conn,
		routing.ExchangePerilDirect,
		queueName,
		routing.PauseKey,
		pubsub.Transient,
		handlerPause(gameState),
	)
	if err != nil {
		fmt.Println("Error subscribing to pause channel: ", err)
		panic(err)
	}

	warRecognitionKey := fmt.Sprintf("%s.%v", routing.WarRecognitionsPrefix, username)
	err = pubsub.SubscribeJSON(
		conn,
		routing.ExchangePerilTopic,
		routing.WarRecognitionsPrefix,
		warRecognitionKey,
		pubsub.Durable,
		handlerWar(gameState),
	)
	if err != nil {
		fmt.Println("Error subscribing to war recognition channel: ", err)
		panic(err)
	}

	for {
		input := gamelogic.GetInput()
		if len(input) == 0 {
			continue
		}

		switch input[0] {
		case "spawn":
			err = gameState.CommandSpawn(input)
			if err != nil {
				fmt.Println("Error spawning unit: ", err)
			}
			break
		case "move":
			move, err := gameState.CommandMove(input)
			if err != nil {
				fmt.Println("Error moving unit: ", err)
			}
			err = pubsub.PublishJSON(chnl, routing.ExchangePerilTopic, moveQueueName, move)
			if err != nil {
				fmt.Println("Error publishing move message: ", err)
			}
			fmt.Printf("\nMoved to %s", move.ToLocation)
			break
		case "status":
			gameState.CommandStatus()
			break
		case "help":
			gamelogic.PrintClientHelp()
			break
		case "spam":
			fmt.Println("Spamming not allowed yet!")
		case "quit":
			gamelogic.PrintQuit()
			return
		default:
			fmt.Println("Unknown command")
			break
		}
	}
}

func handlerPause(gs *gamelogic.GameState) func(routing.PlayingState, *amqp.Channel) pubsub.Acktype {
	return func(ps routing.PlayingState, ch *amqp.Channel) pubsub.Acktype {
		defer fmt.Print(">")
		gs.HandlePause(ps)

		return pubsub.Ack
	}
}

func handlerMove(gs *gamelogic.GameState) func(move gamelogic.ArmyMove, chn *amqp.Channel) pubsub.Acktype {
	return func(am gamelogic.ArmyMove, ch *amqp.Channel) pubsub.Acktype {
		defer fmt.Print(">")
		outcome := gs.HandleMove(am)
		switch outcome {
		case gamelogic.MoveOutcomeSamePlayer:
			return pubsub.NackDiscard
		case gamelogic.MoveOutComeSafe:
			return pubsub.Ack
		case gamelogic.MoveOutcomeMakeWar:
			err := pubsub.PublishJSON(
				ch,
				routing.ExchangePerilTopic,
				fmt.Sprintf("%s.%s", routing.WarRecognitionsPrefix, am.Player.Username),
				gamelogic.RecognitionOfWar{
					Attacker: am.Player,
					Defender: gs.GetPlayerSnap(),
				},
			)
			if err != nil {
				fmt.Println("Error publishing war message: ", err)
				return pubsub.NackRequeue
			}
			return pubsub.Ack
		}

		fmt.Println("error: unknown move outcome")
		return pubsub.NackDiscard
	}
}

func handlerWar(gs *gamelogic.GameState) func(gamelogic.RecognitionOfWar, *amqp.Channel) pubsub.Acktype {
	return func(warRecognition gamelogic.RecognitionOfWar, ch *amqp.Channel) pubsub.Acktype {
		defer fmt.Print(">")
		warOutcome, _, _ := gs.HandleWar(warRecognition)
		switch warOutcome {
		case gamelogic.WarOutcomeNotInvolved:
			fmt.Println("No involvement	!")
			return pubsub.NackRequeue
		case gamelogic.WarOutcomeNoUnits:
			fmt.Println("No units to fight!")
			return pubsub.NackDiscard
		case gamelogic.WarOutcomeOpponentWon:
			var winner, loser string
			if gs.GetUsername() == warRecognition.Attacker.Username {
				winner = warRecognition.Defender.Username
				loser = warRecognition.Attacker.Username
			} else {
				winner = warRecognition.Attacker.Username
				loser = warRecognition.Defender.Username
			}
			msg := fmt.Sprintf("%s won a war against %s", winner, loser)

			err := internal.PublishGameLog(ch, routing.GameLog{
				CurrentTime: time.Now(),
				Message:     msg,
				Username:    warRecognition.Attacker.Username,
			})
			if err != nil {
				fmt.Println("Error publishing game log: ", err)
				return pubsub.NackRequeue
			}

			fmt.Println(msg)
			return pubsub.Ack
		case gamelogic.WarOutcomeYouWon:
			var winner, loser string
			if gs.GetUsername() == warRecognition.Attacker.Username {
				winner = warRecognition.Attacker.Username
				loser = warRecognition.Defender.Username
			} else {
				winner = warRecognition.Defender.Username
				loser = warRecognition.Attacker.Username
			}
			msg := fmt.Sprintf("%s won a war against %s", winner, loser)

			err := internal.PublishGameLog(ch, routing.GameLog{
				CurrentTime: time.Now(),
				Message:     msg,
				Username:    warRecognition.Attacker.Username,
			})
			if err != nil {
				fmt.Println("Error publishing game log: ", err)
				return pubsub.NackRequeue
			}

			fmt.Println(msg)
			return pubsub.Ack
		case gamelogic.WarOutcomeDraw:
			msg := fmt.Sprintf("A war between %s and %s resulted in a draw", warRecognition.Attacker.Username, warRecognition.Defender.Username)

			err := internal.PublishGameLog(ch, routing.GameLog{
				CurrentTime: time.Now(),
				Message:     msg,
				Username:    warRecognition.Attacker.Username,
			})
			if err != nil {
				fmt.Println("Error publishing game log: ", err)
				return pubsub.NackRequeue
			}
			fmt.Println(msg)
			return pubsub.Ack
		}

		fmt.Println("error: unknown war outcome")
		return pubsub.NackDiscard
	}
}
