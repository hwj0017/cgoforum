package ioc

import (
	"fmt"

	"cgoforum/config"

	amqp "github.com/rabbitmq/amqp091-go"
	"go.uber.org/zap"
)

func InitRabbitMQ(cfg *config.RabbitMQConfig, logger *zap.Logger) *amqp.Channel {
	conn, err := amqp.Dial(cfg.URL)
	if err != nil {
		logger.Fatal("failed to connect RabbitMQ", zap.Error(err))
	}

	ch, err := conn.Channel()
	if err != nil {
		logger.Fatal("failed to open channel", zap.Error(err))
	}

	// Declare DLX
	if err := ch.ExchangeDeclare(
		"dlx.article", "direct",
		true,  // durable
		false, // auto-deleted
		false, // internal
		false, // no-wait
		nil,
	); err != nil {
		logger.Fatal("failed to declare DLX exchange", zap.Error(err))
	}

	// Declare main exchange
	if err := ch.ExchangeDeclare(
		"article.events", "direct",
		true,  // durable
		false, // auto-deleted
		false, // internal
		false, // no-wait
		nil,
	); err != nil {
		logger.Fatal("failed to declare exchange", zap.Error(err))
	}

	// Declare queues with DLX config
	queues := []struct {
		name     string
		bindKeys []string
		prefetch int
	}{
		{"search.indexer", []string{"article.published", "article.updated", "article.deleted"}, 10},
		{"embedding.generator", []string{"article.published"}, 5},
		{"stat.sync", []string{"stat.update", "like.added", "like.removed"}, 50},
		{"like.persist", []string{"like.added", "like.removed"}, 100},
		{"activity.tracker", []string{"like.added", "like.removed", "collect.added", "collect.removed", "article.viewed"}, 100},
	}

	for _, q := range queues {
		// Declare queue with DLX
		_, err := ch.QueueDeclare(
			q.name,
			true,  // durable
			false, // auto-delete
			false, // exclusive
			false, // no-wait
			amqp.Table{
				"x-dead-letter-exchange": "dlx.article",
				"x-message-ttl":          int32(24 * 60 * 60 * 1000), // 24h in ms
			},
		)
		if err != nil {
			logger.Fatal(fmt.Sprintf("failed to declare queue %s", q.name), zap.Error(err))
		}

		// Bind queue to exchange
		for _, key := range q.bindKeys {
			if err := ch.QueueBind(q.name, key, "article.events", false, nil); err != nil {
				logger.Fatal(fmt.Sprintf("failed to bind queue %s to key %s", q.name, key), zap.Error(err))
			}
		}
	}

	// Declare DLX queue
	_, err = ch.QueueDeclare("dlq.article", true, false, false, false, nil)
	if err != nil {
		logger.Fatal("failed to declare DLQ", zap.Error(err))
	}
	if err := ch.QueueBind("dlq.article", "", "dlx.article", false, nil); err != nil {
		logger.Fatal("failed to bind DLQ", zap.Error(err))
	}

	logger.Info("RabbitMQ initialized successfully")
	return ch
}

// InitRabbitMQConn returns the connection for shutdown purposes.
func InitRabbitMQConn(cfg *config.RabbitMQConfig, logger *zap.Logger) *amqp.Connection {
	conn, err := amqp.Dial(cfg.URL)
	if err != nil {
		logger.Fatal("failed to connect RabbitMQ", zap.Error(err))
	}
	return conn
}
