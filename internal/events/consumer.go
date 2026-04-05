package events

import (
	"context"
	"encoding/json"

	log "github.com/Kaese72/huemie-lib/logging"
	"github.com/Kaese72/ittt-orchestrator/eventmodels"
	"github.com/Kaese72/ittt-orchestrator/internal/config"
	"github.com/Kaese72/ittt-orchestrator/internal/orchestrator"
	amqp "github.com/rabbitmq/amqp091-go"
)

// StartConsumer connects to RabbitMQ, subscribes to the device attribute updates
// fanout exchange, and forwards each message to the orchestrator for rule evaluation.
// It runs until ctx is cancelled.
func StartConsumer(ctx context.Context, conf config.EventConfig, orch *orchestrator.Orchestrator) error {
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
		true,  // durable
		false, // auto-deleted
		false, // internal
		false, // no-wait
		nil,
	); err != nil {
		ch.Close()
		conn.Close()
		return err
	}

	q, err := ch.QueueDeclare(
		"",    // auto-generated name
		false, // not durable
		false, // delete when unused
		true,  // exclusive
		false, // no-wait
		nil,
	)
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
				orch.HandleDeviceUpdate(update)
			}
		}
	}()

	return nil
}
