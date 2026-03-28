package pubsub

import (
	"bytes"
	"context"
	"encoding/gob"
	"fmt"

	amqp "github.com/rabbitmq/amqp091-go"
)

func PublishGob[T any](ch *amqp.Channel, exchange, key string, val T) error {
	var buffer bytes.Buffer
	encoder := gob.NewEncoder(&buffer)

	err := encoder.Encode(&val)
	if err != nil {
		return err
	}

	err = ch.PublishWithContext(context.Background(), exchange, key, false, false, amqp.Publishing{
		ContentType: "application/json",
		Body:        buffer.Bytes(),
	})
	if err != nil {
		return err
	}

	return nil
}

func SubscribeGob[T any](
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

	err = chn.Qos(10, 0, false)
	if err != nil {
		fmt.Printf("Error setting QoS: %v\n", err)
		return err
	}

	deliveryChannel, err := chn.Consume(queueName, "", false, false, false, false, nil)
	if err != nil {
		fmt.Printf("Error consuming messages: %v\n", err)
		return err
	}

	go func() {
		defer chn.Close()
		for msg := range deliveryChannel {
			var val T
			buffer := bytes.NewBuffer(msg.Body)
			decoder := gob.NewDecoder(buffer)

			err := decoder.Decode(&val)
			if err != nil {
				fmt.Printf("Error unmarshaling message: %v\n", err)
				msg.Nack(false, false)
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
