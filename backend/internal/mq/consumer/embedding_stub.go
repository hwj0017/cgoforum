package consumer

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"

	amqp "github.com/rabbitmq/amqp091-go"
	"go.uber.org/zap"

	"cgoforum/internal/mq/publisher"
	"cgoforum/internal/repository/dao"
	"cgoforum/pkg/vectorizer"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/text"
)

// EmbeddingStub consumes embedding tasks, generates vectors and persists them.
// This lightweight consumer keeps the pipeline functional before a dedicated
// embedding worker service is introduced.
type EmbeddingStub struct {
	ch         *amqp.Channel
	articleDAO dao.ArticleDAO
	embedder   vectorizer.Embedder
	logger     *zap.Logger
}

func NewEmbeddingStub(ch *amqp.Channel, articleDAO dao.ArticleDAO, embedder vectorizer.Embedder, logger *zap.Logger) *EmbeddingStub {
	return &EmbeddingStub{ch: ch, articleDAO: articleDAO, embedder: embedder, logger: logger}
}

func (e *EmbeddingStub) Start(ctx context.Context) error {
	deliveries, err := e.ch.Consume("embedding.generator", "", false, false, false, false, nil)
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
				e.handleMessage(msg)
			}
		}
	}()

	return nil
}

func (e *EmbeddingStub) handleMessage(msg amqp.Delivery) {
	var event publisher.EventMessage
	if err := json.Unmarshal(msg.Body, &event); err != nil {
		e.logger.Error("failed to unmarshal embedding message", zap.Error(err))
		msg.Nack(false, false)
		return
	}

	payload, ok := event.Payload.(map[string]interface{})
	if !ok {
		msg.Nack(false, false)
		return
	}

	articleIDFloat, ok := payload["article_id"].(float64)
	if !ok {
		e.logger.Warn("missing article_id in payload")
		msg.Nack(false, false)
		return
	}
	articleID := int64(articleIDFloat)

	content, ok := payload["content_md"].(string)
	if !ok || strings.TrimSpace(content) == "" {
		e.logger.Warn("missing or empty content_md in payload")
		msg.Nack(false, false)
		return
	}

	articleID, chunks, modelVersion, err := parseEmbeddingPayload(event.EventType, payload)
	if err != nil {
		e.logger.Warn("invalid embedding payload", zap.Error(err), zap.String("event_type", event.EventType))
		msg.Nack(false, false)
		return
	}

	for _, chunk := range chunks {
		contentText := chunk["content_text"]
		vec, err := e.embedder.Embed(context.Background(), contentText)
		if err != nil {
			e.logger.Error("failed to generate embedding", zap.Error(err), zap.Int64("article_id", articleID))
			msg.Nack(false, true)
			return
		}

		if err := e.articleDAO.UpsertEmbedding(context.Background(), articleID, "", vec, "", contentText, modelVersion); err != nil {
			e.logger.Error("failed to upsert embedding", zap.Error(err), zap.Int64("article_id", articleID))
			msg.Nack(false, true)
			return
		}
	}

	e.logger.Info("embedding generated for article", zap.Int64("article_id", articleID))
	msg.Ack(false)
}

func parseEmbeddingPayload(eventType string, payload map[string]interface{}) (int64, []map[string]string, string, error) {
	articleIDFloat, ok := payload["article_id"].(float64)
	if !ok {
		return 0, nil, "", fmt.Errorf("missing article_id")
	}
	articleID := int64(articleIDFloat)
	modelVersion := "sentence-transformer"
	if mv, ok := payload["model_version"].(string); ok {
		mv = strings.TrimSpace(mv)
		if mv != "" {
			modelVersion = mv
		}
	}

	var content string
	if eventType == "embedding.generate" {
		content, _ = payload["text_to_embed"].(string)
	} else {
		title, _ := payload["title"].(string)
		summary, _ := payload["summary"].(string)
		body, _ := payload["content_md"].(string)
		content = strings.TrimSpace(title + "\n" + summary + "\n" + body)
	}

	if strings.TrimSpace(content) == "" {
		return 0, nil, "", fmt.Errorf("empty content")
	}

	// Parse Markdown AST
	md := goldmark.New()
	source := []byte(content)
	node := md.Parser().Parse(text.NewReader(source))

	// Extract structured chunks with truncation
	var chunks []map[string]string
	var buffer bytes.Buffer
	var totalLength int
	walker := func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}

		// Only process leaf nodes to avoid duplicates
		if n.HasChildren() {
			return ast.WalkContinue, nil
		}

		buffer.Reset()
		if segment := n.Text(source); segment != nil {
			buffer.Write(segment)
		}

		if buffer.Len() > 0 {
			chunkText := buffer.String()
			if totalLength+len(chunkText) > 8000 {
				remaining := 8000 - totalLength
				chunkText = chunkText[:remaining]
			}
			chunks = append(chunks, map[string]string{
				"content_text": chunkText,
			})
			totalLength += len(chunkText)
			if totalLength >= 8000 {
				return ast.WalkStop, nil
			}
		}
		return ast.WalkContinue, nil
	}

	if err := ast.Walk(node, walker); err != nil {
		return 0, nil, "", fmt.Errorf("failed to parse markdown: %w", err)
	}

	return articleID, chunks, modelVersion, nil
}
