package consumer

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"

	"cgoforum/internal/mq/publisher"
	"cgoforum/internal/repository/dao"
)

type ActivityTracker struct {
	ch       *amqp.Channel
	rdb      *redis.Client
	eventDAO dao.EventLogDAO
	logger   *zap.Logger
}

func NewActivityTracker(ch *amqp.Channel, rdb *redis.Client, eventDAO dao.EventLogDAO, logger *zap.Logger) *ActivityTracker {
	return &ActivityTracker{ch: ch, rdb: rdb, eventDAO: eventDAO, logger: logger}
}

func (a *ActivityTracker) Start(ctx context.Context) error {
	deliveries, err := a.ch.Consume("activity.tracker", "", false, false, false, false, nil)
	if err != nil {
		return err
	}

	go func() {
		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()

		batch := make([]amqp.Delivery, 0, 256)
		articleIDs := make(map[int64]struct{}, 256)

		flush := func() {
			if len(batch) == 0 {
				return
			}
			now := float64(time.Now().Unix())
			pipe := a.rdb.Pipeline()
			for aid := range articleIDs {
				for _, window := range []string{"24h", "7d"} {
					key := fmt.Sprintf("act:articles:window:%s", window)
					pipe.ZAdd(ctx, key, redis.Z{Score: now, Member: aid})
				}
			}
			if _, err := pipe.Exec(ctx); err != nil {
				a.logger.Error("failed to flush activity batch", zap.Error(err))
				for _, d := range batch {
					d.Nack(false, true)
				}
				batch = batch[:0]
				articleIDs = make(map[int64]struct{}, 256)
				return
			}
			for _, d := range batch {
				d.Ack(false)
			}
			batch = batch[:0]
			articleIDs = make(map[int64]struct{}, 256)
		}

		for {
			select {
			case <-ctx.Done():
				flush()
				return
			case <-ticker.C:
				flush()
			case msg, ok := <-deliveries:
				if !ok {
					flush()
					return
				}
				articleID, skip := a.parseMessage(ctx, msg)
				if skip {
					continue
				}
				batch = append(batch, msg)
				articleIDs[articleID] = struct{}{}
				if len(batch) >= 200 {
					flush()
				}
			}
		}
	}()

	return nil
}

func (a *ActivityTracker) parseMessage(ctx context.Context, msg amqp.Delivery) (articleID int64, skip bool) {
	var event publisher.EventMessage
	if err := json.Unmarshal(msg.Body, &event); err != nil {
		a.logger.Error("failed to unmarshal activity message", zap.Error(err))
		msg.Nack(false, false)
		return 0, true
	}
	if event.EventID == "" {
		msg.Nack(false, false)
		return 0, true
	}

	payload, ok := event.Payload.(map[string]interface{})
	if !ok {
		msg.Nack(false, false)
		return 0, true
	}

	if event.EventType != "like.added" && event.EventType != "like.removed" &&
		event.EventType != "collect.added" && event.EventType != "collect.removed" &&
		event.EventType != "interaction.added" && event.EventType != "interaction.removed" &&
		event.EventType != "article.viewed" {
		msg.Ack(false)
		return 0, true
	}

	occurredAt, err := time.Parse(time.RFC3339, event.Timestamp)
	if err != nil {
		occurredAt = time.Now()
	}
	first, err := a.eventDAO.MarkProcessed(ctx, "activity.tracker", event.EventID, occurredAt)
	if err != nil {
		a.logger.Error("failed to mark activity event", zap.Error(err), zap.String("event_id", event.EventID))
		msg.Nack(false, true)
		return 0, true
	}
	if !first {
		msg.Ack(false)
		return 0, true
	}

	articleIDFloat, _ := payload["article_id"].(float64)
	if articleIDFloat == 0 {
		msg.Ack(false)
		return 0, true
	}

	return int64(articleIDFloat), false
}
