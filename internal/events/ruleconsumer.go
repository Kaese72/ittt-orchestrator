package events

import (
	"context"
	"encoding/json"

	log "github.com/Kaese72/huemie-lib/logging"
	"github.com/Kaese72/ittt-orchestrator/eventmodels"
	"github.com/Kaese72/ittt-orchestrator/internal/config"
	amqp "github.com/rabbitmq/amqp091-go"
)

// RuleEventHandler processes rule lifecycle events.
type RuleEventHandler interface {
	HandleRuleEvent(eventmodels.RuleEvent)
}

// StartRuleConsumer subscribes to the ruleEvents fanout exchange and forwards
// each message to the handler. It runs until ctx is cancelled.
func StartRuleConsumer(ctx context.Context, conf config.EventConfig, handler RuleEventHandler) error {
	conn, err := amqp.Dial(conf.ConnectionString)
	if err != nil {
		return err
	}
	ch, err := conn.Channel()
	if err != nil {
		conn.Close()
		return err
	}
	if err := ch.ExchangeDeclare(ruleEventsExchange, "fanout", true, false, false, false, nil); err != nil {
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
	if err := ch.QueueBind(q.Name, "", ruleEventsExchange, false, nil); err != nil {
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
				var event eventmodels.RuleEvent
				if err := json.Unmarshal(msg.Body, &event); err != nil {
					log.Error("failed to unmarshal rule event: "+err.Error(), map[string]interface{}{})
					continue
				}
				handler.HandleRuleEvent(event)
			}
		}
	}()

	return nil
}
