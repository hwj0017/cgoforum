package publisher

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	amqp "github.com/rabbitmq/amqp091-go"
	"go.uber.org/zap"

	"cgoforum/internal/domain"
)

type EventPublisher interface {
	PublishArticleEvent(ctx context.Context, eventType string, article *domain.Article) error
	PublishInteractionEvent(ctx context.Context, eventType string, userID, articleID int64, action, note string) error
	PublishEmbeddingTask(ctx context.Context, articleID int64, text, modelVersion string) error
}

type eventPublisher struct {
	ch     *amqp.Channel
	logger *zap.Logger
}

func NewEventPublisher(ch *amqp.Channel, logger *zap.Logger) EventPublisher {
	return &eventPublisher{ch: ch, logger: logger}
}

// Base event structure
type EventMessage struct {
	EventID   string      `json:"event_id"`
	EventType string      `json:"event_type"`
	Timestamp string      `json:"timestamp"`
	Payload   interface{} `json:"payload"`
	Metadata  Metadata    `json:"metadata"`
}

type Metadata struct {
	RetryCount    int    `json:"retry_count"`
	SourceService string `json:"source_service"`
}

type ArticlePayload struct {
	ArticleID   int64  `json:"article_id"`
	UserID      int64  `json:"user_id"`
	Title       string `json:"title"`
	Summary     string `json:"summary"`
	ContentMD   string `json:"content_md"`
	CoverImg    string `json:"cover_img"`
	PublishedAt string `json:"published_at,omitempty"`
}

type InteractionPayload struct {
	UserID    int64  `json:"user_id"`
	ArticleID int64  `json:"article_id"`
	Action    string `json:"action"` // add or remove
	Note      string `json:"note,omitempty"`
}

type EmbeddingPayload struct {
	ArticleID    int64  `json:"article_id"`
	TextToEmbed  string `json:"text_to_embed"`
	ModelVersion string `json:"model_version"`
}

func (p *eventPublisher) publish(ctx context.Context, routingKey string, msg EventMessage) error {
	body, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("marshal message: %w", err)
	}

	err = p.ch.PublishWithContext(ctx,
		"article.events", // exchange
		routingKey,       // routing key
		false,            // mandatory
		false,            // immediate
		amqp.Publishing{
			DeliveryMode: amqp.Persistent,
			ContentType:  "application/json",
			Body:         body,
		},
	)
	if err != nil {
		return fmt.Errorf("publish message: %w", err)
	}

	p.logger.Debug("published event",
		zap.String("type", msg.EventType),
		zap.String("key", routingKey),
	)
	return nil
}

func (p *eventPublisher) PublishArticleEvent(ctx context.Context, eventType string, article *domain.Article) error {
	msg := EventMessage{
		EventID:   uuid.New().String(),
		EventType: eventType,
		Timestamp: time.Now().Format(time.RFC3339),
		Payload: ArticlePayload{
			ArticleID: article.ID,
			UserID:    article.UserID,
			Title:     article.Title,
			Summary:   article.Summary,
			ContentMD: article.ContentMD,
			CoverImg:  article.CoverImg,
			PublishedAt: func() string {
				if article.PublishedAt != nil {
					return article.PublishedAt.Format(time.RFC3339)
				}
				return ""
			}(),
		},
		Metadata: Metadata{SourceService: "article-service"},
	}

	routingKey := eventType

	return p.publish(ctx, routingKey, msg)
}

func (p *eventPublisher) PublishInteractionEvent(ctx context.Context, eventType string, userID, articleID int64, action, note string) error {
	msg := EventMessage{
		EventID:   uuid.New().String(),
		EventType: eventType,
		Timestamp: time.Now().Format(time.RFC3339),
		Payload: InteractionPayload{
			UserID:    userID,
			ArticleID: articleID,
			Action:    action,
			Note:      note,
		},
		Metadata: Metadata{SourceService: "interaction-service"},
	}

	routingKey := eventType
	if eventType == "" {
		routingKey = "interaction." + strings.TrimSpace(action)
	}

	return p.publish(ctx, routingKey, msg)
}

func (p *eventPublisher) PublishEmbeddingTask(ctx context.Context, articleID int64, text, modelVersion string) error {
	msg := EventMessage{
		EventID:   uuid.New().String(),
		EventType: "embedding.generate",
		Timestamp: time.Now().Format(time.RFC3339),
		Payload: EmbeddingPayload{
			ArticleID:    articleID,
			TextToEmbed:  text,
			ModelVersion: modelVersion,
		},
		Metadata: Metadata{SourceService: "article-service"},
	}

	return p.publish(ctx, "article.published", msg)
}
