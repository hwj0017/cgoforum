package ioc

import (
	"cgoforum/config"
	"github.com/meilisearch/meilisearch-go"
	"go.uber.org/zap"
)

func InitMeilisearch(cfg *config.MeilisearchConfig, logger *zap.Logger) meilisearch.ServiceManager {
	client := meilisearch.New(cfg.Addr, meilisearch.WithAPIKey(cfg.APIKey))

	// Check connection
	_, err := client.GetStats()
	if err != nil {
		logger.Fatal("failed to connect Meilisearch", zap.Error(err))
	}

	// Create or get index
	index := client.Index(cfg.Index)
	if index != nil {
		// Configure searchable, filterable, and sortable attributes
		searchable := []string{"title", "summary", "content_text", "tags"}
		filterable := []interface{}{"user_id", "tags", "status", "published_at"}
		sortable := []string{"published_at", "like_count", "collect_count", "hot_score"}

		index.UpdateSearchableAttributes(&searchable)
		index.UpdateFilterableAttributes(&filterable)
		index.UpdateSortableAttributes(&sortable)
	}

	logger.Info("Meilisearch initialized successfully")
	return client
}
