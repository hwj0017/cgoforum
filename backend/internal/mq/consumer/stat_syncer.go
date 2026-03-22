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

type StatSyncer struct {
	ch       *amqp.Channel
	statDAO  dao.StatDAO
	eventDAO dao.EventLogDAO
	logger   *zap.Logger
}

func NewStatSyncer(ch *amqp.Channel, statDAO dao.StatDAO, eventDAO dao.EventLogDAO, logger *zap.Logger) *StatSyncer {
	return &StatSyncer{ch: ch, statDAO: statDAO, eventDAO: eventDAO, logger: logger}
}

func (s *StatSyncer) Start(ctx context.Context) error {
	deliveries, err := s.ch.Consume("stat.sync", "", false, false, false, false, nil)
	if err != nil {
		return err
	}

	go func() {
		ticker := time.NewTicker(3 * time.Second)
		defer ticker.Stop()

		batch := make([]amqp.Delivery, 0, 128)
		agg := map[string]map[int64]int{
			"like_count": {},
			"view_count": {},
		}

		flush := func() {
			if len(batch) == 0 {
				return
			}
			for field, fieldMap := range agg {
				for articleID, delta := range fieldMap {
					if delta == 0 {
						continue
					}
					if err := s.statDAO.UpsertStat(ctx, articleID, field, delta); err != nil {
						s.logger.Error("failed to flush stat batch", zap.Error(err), zap.Int64("article_id", articleID), zap.String("field", field), zap.Int("delta", delta))
						for _, d := range batch {
							d.Nack(false, true)
						}
						batch = batch[:0]
						agg["like_count"] = map[int64]int{}
						agg["view_count"] = map[int64]int{}
						return
					}
				}
			}

			for _, d := range batch {
				d.Ack(false)
			}
			batch = batch[:0]
			agg["like_count"] = map[int64]int{}
			agg["view_count"] = map[int64]int{}
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

				field, articleID, delta, discard := s.parseMessage(ctx, msg)
				if discard {
					continue
				}
				if field != "" {
					agg[field][articleID] += delta
				}
				batch = append(batch, msg)
				if len(batch) >= 100 {
					flush()
				}
			}
		}
	}()

	return nil
}

func (s *StatSyncer) parseMessage(ctx context.Context, msg amqp.Delivery) (field string, articleID int64, delta int, discard bool) {
	var event publisher.EventMessage
	if err := json.Unmarshal(msg.Body, &event); err != nil {
		s.logger.Error("failed to unmarshal stat message", zap.Error(err))
		msg.Nack(false, false)
		return "", 0, 0, true
	}
	if event.EventID == "" {
		msg.Nack(false, false)
		return "", 0, 0, true
	}

	payload, ok := event.Payload.(map[string]interface{})
	if !ok {
		msg.Nack(false, false)
		return "", 0, 0, true
	}

	if event.EventType != "like.added" && event.EventType != "like.removed" && event.EventType != "stat.update" {
		msg.Ack(false)
		return "", 0, 0, true
	}

	occurredAt, err := time.Parse(time.RFC3339, event.Timestamp)
	if err != nil {
		occurredAt = time.Now()
	}
	first, err := s.eventDAO.MarkProcessed(ctx, "stat.sync", event.EventID, occurredAt)
	if err != nil {
		s.logger.Error("failed to mark stat event", zap.Error(err), zap.String("event_id", event.EventID))
		msg.Nack(false, true)
		return "", 0, 0, true
	}
	if !first {
		msg.Ack(false)
		return "", 0, 0, true
	}

	articleIDFloat, _ := payload["article_id"].(float64)
	action, _ := payload["action"].(string)

	// Determine field and delta based on event type
	field = ""
	delta = 1
	switch event.EventType {
	case "like.added":
		field = "like_count"
	case "like.removed":
		field = "like_count"
		delta = -1
	case "stat.update":
		// Generic stat update
		if action == "add" {
			field = "view_count"
		}
	}

	if field == "" {
		msg.Ack(false)
		return "", 0, 0, true
	}

	return field, int64(articleIDFloat), delta, false
}
