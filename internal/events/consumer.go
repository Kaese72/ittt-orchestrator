package events

import (
	"context"
	"encoding/json"

	log "github.com/Kaese72/huemie-lib/logging"
	"github.com/Kaese72/ittt-orchestrator/eventmodels"
	"github.com/Kaese72/ittt-orchestrator/internal/config"
	amqp "github.com/rabbitmq/amqp091-go"
)

// DeviceUpdateHandler processes device attribute update events.
type DeviceUpdateHandler interface {
	HandleDeviceUpdate(eventmodels.DeviceAttributeUpdate)
}

// StartConsumer connects to RabbitMQ, subscribes to the device attribute updates
// fanout exchange, and forwards each message to the handler for rule evaluation.
// It runs until ctx is cancelled.
func StartConsumer(ctx context.Context, conf config.EventConfig, handler DeviceUpdateHandler) error {
	conn, err := amqp.Dial(conf.ConnectionString)
	if err != nil {
		return err
	}

	ch, err := conn.Channel()
	if err != nil {
		conn.Close()
		return err
	}

	if err := ch.ExchangeDeclare(
		"deviceAttributeUpdates",
		"fanout",
		true,
		false,
		false,
		false,
		nil,
	); err != nil {
		ch.Close()
		conn.Close()
		return err
	}

	q, err := ch.QueueDeclare("", false, false, true, false, nil)
	if err != nil {
		ch.Close()
		conn.Close()
		return err
	}

	if err := ch.QueueBind(q.Name, "", "deviceAttributeUpdates", false, nil); err != nil {
		ch.Close()
		conn.Close()
		return err
	}

	msgs, err := ch.Consume(q.Name, "", true, false, false, false, nil)
	if err != nil {
		ch.Close()
		conn.Close()
		return err
	}

	go func() {
		defer ch.Close()
		defer conn.Close()
		for {
			select {
			case <-ctx.Done():
				return
			case msg, ok := <-msgs:
				if !ok {
					return
				}
				var update eventmodels.DeviceAttributeUpdate
				if err := json.Unmarshal(msg.Body, &update); err != nil {
					log.Error("failed to unmarshal device attribute update: "+err.Error(), map[string]interface{}{})
					continue
				}
				handler.HandleDeviceUpdate(update)
			}
		}
	}()

	return nil
}
