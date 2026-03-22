package consumer

import (
	"context"
	"encoding/json"
	"strconv"

	"github.com/meilisearch/meilisearch-go"
	amqp "github.com/rabbitmq/amqp091-go"
	"go.uber.org/zap"

	"cgoforum/internal/mq/publisher"
	"cgoforum/pkg/xss"
)

type SearchIndexer struct {
	ch     *amqp.Channel
	client meilisearch.ServiceManager
	index  string
	logger *zap.Logger
}

func NewSearchIndexer(ch *amqp.Channel, client meilisearch.ServiceManager, index string, logger *zap.Logger) *SearchIndexer {
	return &SearchIndexer{ch: ch, client: client, index: index, logger: logger}
}

type SearchDocument struct {
	ArticleID   int64  `json:"article_id"`
	UserID      int64  `json:"user_id"`
	Title       string `json:"title"`
	Summary     string `json:"summary"`
	ContentText string `json:"content_text"`
	CoverImg    string `json:"cover_img"`
	Status      int16  `json:"status"`
	PublishedAt string `json:"published_at"`
}

func (s *SearchIndexer) Start(ctx context.Context) error {
	deliveries, err := s.ch.Consume(
		"search.indexer",
		"",    // consumer tag
		false, // auto-ack (we use manual ack)
		false, // exclusive
		false, // no-local
		false, // no-wait
		nil,
	)
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
				s.handleMessage(ctx, msg)
			}
		}
	}()

	return nil
}

func (s *SearchIndexer) handleMessage(ctx context.Context, msg amqp.Delivery) {
	var event publisher.EventMessage
	if err := json.Unmarshal(msg.Body, &event); err != nil {
		s.logger.Error("failed to unmarshal message", zap.Error(err))
		msg.Nack(false, false) // don't requeue malformed messages
		return
	}

	idx := s.client.Index(s.index)

	switch event.EventType {
	case "article.published", "article.updated":
		payload, ok := event.Payload.(map[string]interface{})
		if !ok {
			s.logger.Error("invalid payload type")
			msg.Nack(false, false)
			return
		}

		contentMD, _ := payload["content_md"].(string)
		doc := SearchDocument{
			ArticleID:   int64(payload["article_id"].(float64)),
			UserID:      int64(payload["user_id"].(float64)),
			Title:       payload["title"].(string),
			Summary:     payload["summary"].(string),
			ContentText: xss.StripHTML(contentMD),
			CoverImg:    payload["cover_img"].(string),
			PublishedAt: payload["published_at"].(string),
		}

		pk := "article_id"
		if _, err := idx.AddDocumentsWithContext(ctx, []SearchDocument{doc}, &meilisearch.DocumentOptions{PrimaryKey: &pk}); err != nil {
			s.logger.Error("failed to index document", zap.Error(err))
			msg.Nack(false, true) // requeue on error
			return
		}

	case "article.deleted":
		payload, ok := event.Payload.(map[string]interface{})
		if !ok {
			msg.Nack(false, false)
			return
		}
		articleID := int64(payload["article_id"].(float64))
		if _, err := idx.DeleteDocumentWithContext(ctx, strconv.FormatInt(articleID, 10), nil); err != nil {
			s.logger.Error("failed to delete document", zap.Error(err))
			msg.Nack(false, true)
			return
		}
	}

	msg.Ack(false)
}
