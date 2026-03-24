package pubsub

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/bootdotdev/learn-pub-sub-starter/internal/routing"
	amqp "github.com/rabbitmq/amqp091-go"
)

func PublishJSON[T any](ch *amqp.Channel, exchange, key string, val T) error {
	valBytes, err := json.Marshal(val)
	if err != nil {
		return err
	}
	err = ch.PublishWithContext(context.Background(), exchange, key, false, false, amqp.Publishing{
		ContentType: "application/json",
		Body:        valBytes,
	})
	if err != nil {
		return err
	}

	return nil
}

func DeclareAndBind(
	conn *amqp.Connection,
	exchange,
	queueName,
	key string,
	queueType SimpleQueueType,
) (*amqp.Channel, amqp.Queue, error) {
	chnl, err := conn.Channel()
	if err != nil {
		return nil, amqp.Queue{}, err
	}

	exchangeType := "direct"
	if strings.Contains(exchange, "topic") {
		exchangeType = "topic"
	}
	err = chnl.ExchangeDeclare(
		exchange,
		exchangeType,
		true,
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		return nil, amqp.Queue{}, err
	}

	args := amqp.Table{
		"x-dead-letter-exchange": routing.ExchangePerilDeadLetter,
	}
	queue, err := chnl.QueueDeclare(
		queueName,
		queueType == Durable,
		queueType == Transient,
		queueType == Transient,
		false,
		args,
	)
	if err != nil {
		return nil, amqp.Queue{}, err
	}

	err = chnl.QueueBind(queue.Name, key, exchange, false, nil)
	if err != nil {
		return nil, amqp.Queue{}, err
	}

	return chnl, queue, nil
}

func SubscribeJSON[T any](
	conn *amqp.Connection,
	exchange,
	queueName,
	key string,
	queueType SimpleQueueType,
	handler func(T, *amqp.Channel) Acktype,
) error {
	chn, _, err := DeclareAndBind(conn, exchange, queueName, key, queueType)
	if err != nil {
		return err
	}

	deliveryChannel, err := chn.Consume(queueName, "", false, false, false, false, nil)
	if err != nil {
		return err
	}

	go func() {
		defer chn.Close()
		for msg := range deliveryChannel {
			var val T
			err = json.Unmarshal(msg.Body, &val)
			if err != nil {
				fmt.Printf("Error unmarshaling message: %v\n", err)
				continue
			}

			switch handler(val, chn) {
			case Ack:
				err = msg.Ack(false)
				if err != nil {
					fmt.Printf("Error acknowledging message: %v\n", err)
				}
				fmt.Println("Acknowledged message")
			case NackDiscard:
				err = msg.Nack(false, false)
				if err != nil {
					fmt.Printf("Error nacking message: %v\n", err)
				}
				fmt.Println("Nacked Discard message")
			case NackRequeue:
				err = msg.Nack(false, true)
				if err != nil {
					fmt.Printf("Error nacking message: %v\n", err)
				}
				fmt.Println("Nacked Requeue message")
			}
		}
	}()

	return nil
}

type SimpleQueueType string
type Acktype int

const (
	Durable   SimpleQueueType = "durable"
	Transient SimpleQueueType = "transient"
)

const (
	Ack Acktype = iota
	NackDiscard
	NackRequeue
)
