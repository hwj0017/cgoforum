package consumer

import (
	"context"
	"encoding/json"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
	"go.uber.org/zap"

	"cgoforum/internal/mq/publisher"
	"cgoforum/internal/repository/dao"
)

type LikePersistConsumer struct {
	ch       *amqp.Channel
	likeDAO  dao.LikeDAO
	eventDAO dao.EventLogDAO
	logger   *zap.Logger
}

func NewLikePersistConsumer(ch *amqp.Channel, likeDAO dao.LikeDAO, eventDAO dao.EventLogDAO, logger *zap.Logger) *LikePersistConsumer {
	return &LikePersistConsumer{ch: ch, likeDAO: likeDAO, eventDAO: eventDAO, logger: logger}
}

func (c *LikePersistConsumer) Start(ctx context.Context) error {
	deliveries, err := c.ch.Consume("like.persist", "", false, false, false, false, nil)
	if err != nil {
		return err
	}

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case msg, ok := <-deliveries:
				if !ok {
					return
				}
				c.handleMessage(ctx, msg)
			}
		}
	}()

	return nil
}

func (c *LikePersistConsumer) handleMessage(ctx context.Context, msg amqp.Delivery) {
	var event publisher.EventMessage
	if err := json.Unmarshal(msg.Body, &event); err != nil {
		c.logger.Error("failed to unmarshal like persist message", zap.Error(err))
		msg.Nack(false, false)
		return
	}
	if event.EventID == "" {
		msg.Nack(false, false)
		return
	}

	if event.EventType != "like.added" && event.EventType != "like.removed" {
		msg.Ack(false)
		return
	}

	payload, ok := event.Payload.(map[string]interface{})
	if !ok {
		msg.Nack(false, false)
		return
	}

	userIDFloat, okUser := payload["user_id"].(float64)
	articleIDFloat, okArticle := payload["article_id"].(float64)
	if !okUser || !okArticle {
		msg.Nack(false, false)
		return
	}
	userID := int64(userIDFloat)
	articleID := int64(articleIDFloat)
	if userID <= 0 || articleID <= 0 {
		msg.Nack(false, false)
		return
	}

	occurredAt, err := time.Parse(time.RFC3339, event.Timestamp)
	if err != nil {
		occurredAt = time.Now()
	}
	first, err := c.eventDAO.MarkProcessed(ctx, "like.persist", event.EventID, occurredAt)
	if err != nil {
		c.logger.Error("failed to mark like persist event", zap.Error(err), zap.String("event_id", event.EventID))
		msg.Nack(false, true)
		return
	}
	if !first {
		msg.Ack(false)
		return
	}

	switch event.EventType {
	case "like.added":
		err = c.likeDAO.CreateLike(ctx, userID, articleID)
		if err != nil && err != dao.ErrDuplicate {
			c.logger.Error("failed to persist like.added", zap.Error(err), zap.Int64("user_id", userID), zap.Int64("article_id", articleID))
			msg.Nack(false, true)
			return
		}
	case "like.removed":
		if err = c.likeDAO.DeleteLike(ctx, userID, articleID); err != nil {
			c.logger.Error("failed to persist like.removed", zap.Error(err), zap.Int64("user_id", userID), zap.Int64("article_id", articleID))
			msg.Nack(false, true)
			return
		}
	}

	msg.Ack(false)
}
