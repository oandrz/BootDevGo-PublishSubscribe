package internal

import (
	"fmt"

	"github.com/bootdotdev/learn-pub-sub-starter/internal/pubsub"
	"github.com/bootdotdev/learn-pub-sub-starter/internal/routing"
	amqp "github.com/rabbitmq/amqp091-go"
)

func PublishGameLog(chnl *amqp.Channel, gamelog routing.GameLog) error {
	key := fmt.Sprintf("%s.%s", routing.GameLogSlug, gamelog.Username)

	err := pubsub.PublishGob(chnl, routing.ExchangePerilTopic, key, gamelog)
	if err != nil {
		fmt.Printf("Failed to publish game log: %v\n", err)
		return err
	}

	return nil
}
