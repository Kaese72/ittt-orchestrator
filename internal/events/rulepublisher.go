package events

import (
	"encoding/json"

	"github.com/Kaese72/ittt-orchestrator/eventmodels"
	amqp "github.com/rabbitmq/amqp091-go"
)

const ruleEventsExchange = "ruleEvents"

// RuleEventPublisher publishes rule lifecycle events to RabbitMQ.
type RuleEventPublisher struct {
	conn    *amqp.Connection
	channel *amqp.Channel
}

func NewRuleEventPublisher(connectionString string) (*RuleEventPublisher, error) {
	conn, err := amqp.Dial(connectionString)
	if err != nil {
		return nil, err
	}
	ch, err := conn.Channel()
	if err != nil {
		conn.Close()
		return nil, err
	}
	if err := ch.ExchangeDeclare(ruleEventsExchange, "fanout", true, false, false, false, nil); err != nil {
		ch.Close()
		conn.Close()
		return nil, err
	}
	return &RuleEventPublisher{conn: conn, channel: ch}, nil
}

func (p *RuleEventPublisher) Publish(event eventmodels.RuleEvent) error {
	body, err := json.Marshal(event)
	if err != nil {
		return err
	}
	return p.channel.Publish(ruleEventsExchange, "", false, false, amqp.Publishing{
		ContentType: "application/json",
		Body:        body,
	})
}

func (p *RuleEventPublisher) Close() {
	p.channel.Close()
	p.conn.Close()
}
