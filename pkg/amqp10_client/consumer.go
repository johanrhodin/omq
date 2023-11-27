package amqp10_client

import (
	"context"

	"github.com/rabbitmq/omq/pkg/config"
	"github.com/rabbitmq/omq/pkg/log"
	"github.com/rabbitmq/omq/pkg/topic"
	"github.com/rabbitmq/omq/pkg/utils"

	"github.com/rabbitmq/omq/pkg/metrics"

	amqp "github.com/Azure/go-amqp"
	"github.com/prometheus/client_golang/prometheus"
)

type Amqp10Consumer struct {
	Id         int
	Connection *amqp.Conn
	Session    *amqp.Session
	Topic      string
	Config     config.Config
}

func NewConsumer(cfg config.Config, id int) *Amqp10Consumer {
	// open connection
	conn, err := amqp.Dial(context.TODO(), cfg.ConsumerUri, nil)
	if err != nil {
		log.Error("consumer failed to connect", "protocol", "amqp-1.0", "consumerId", id, "error", err.Error())
		return nil
	}

	// open seesion
	session, err := conn.NewSession(context.TODO(), nil)
	if err != nil {
		log.Error("consumer failed to create a session", "protocol", "amqp-1.0", "consumerId", id, "error", err.Error())
		return nil
	}

	topic := topic.CalculateTopic(cfg.ConsumeFrom, id)

	return &Amqp10Consumer{
		Id:         id,
		Connection: conn,
		Session:    session,
		Topic:      topic,
		Config:     cfg,
	}

}

func (c Amqp10Consumer) Start(ctx context.Context, subscribed chan bool) {
	var durability amqp.Durability
	switch c.Config.QueueDurability {
	case config.None:
		durability = amqp.DurabilityNone
	case config.Configuration:
		durability = amqp.DurabilityConfiguration
	case config.UnsettledState:
		durability = amqp.DurabilityUnsettledState
	}
	filters := amqp.NewLinkFilter("rabbitmq:stream-offset-spec", 0, c.Config.StreamOffset)
	receiver, err := c.Session.NewReceiver(context.TODO(), c.Topic, &amqp.ReceiverOptions{SourceDurability: durability, Credit: int32(c.Config.Amqp.ConsumerCredits), Filters: []amqp.LinkFilter{filters}})
	if err != nil {
		log.Error("consumer failed to create a receiver", "protocol", "amqp-1.0", "consumerId", c.Id, "error", err.Error())
		return
	}
	close(subscribed)
	log.Debug("consumer subscribed", "protocol", "amqp-1.0", "consumerId", c.Id, "terminus", c.Topic, "durability", durability)

	m := metrics.EndToEndLatency.With(prometheus.Labels{"protocol": "amqp-1.0"})

	log.Info("consumer started", "protocol", "amqp-1.0", "consumerId", c.Id, "terminus", c.Topic)

	for i := 1; i <= c.Config.ConsumeCount; i++ {
		// TODO Receive() is blocking, so cancelling the context won't really stop the consumer
		select {
		case <-ctx.Done():
			c.Stop("time limit reached")
			return
		default:
			msg, err := receiver.Receive(context.TODO(), nil)
			if err != nil {
				log.Error("failed to receive a message", "protocol", "amqp-1.0", "consumerId", c.Id, "terminus", c.Topic)
				return
			}

			payload := msg.GetData()
			m.Observe(utils.CalculateEndToEndLatency(c.Config.UseMillis, &payload))

			log.Debug("message received", "protocol", "amqp-1.0", "consumerId", c.Id, "terminus", c.Topic, "size", len(payload))

			err = receiver.AcceptMessage(context.TODO(), msg)
			if err != nil {
				log.Error("message NOT accepted", "protocol", "amqp-1.0", "consumerId", c.Id, "terminus", c.Topic)
			}
			metrics.MessagesConsumed.With(prometheus.Labels{"protocol": "amqp-1.0"}).Inc()
			log.Debug("message accepted", "protocol", "amqp-1.0", "consumerId", c.Id, "terminus", c.Topic)
		}
	}

	c.Stop("message count reached")
	log.Debug("consumer finished", "protocol", "amqp-1.0", "consumerId", c.Id)
}

func (c Amqp10Consumer) Stop(reason string) {
	log.Debug("closing connection", "protocol", "amqp-1.0", "consumerId", c.Id, "reason", reason)
	_ = c.Connection.Close()
}
