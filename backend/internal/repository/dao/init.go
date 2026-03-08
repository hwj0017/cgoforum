package dao

import (
	"cgoforum/internal/domain"

	"gorm.io/gorm"
)

func InitTables(db *gorm.DB) error {
	// Enable pgvector extension
	if err := db.Exec("CREATE EXTENSION IF NOT EXISTS vector").Error; err != nil {
		return err
	}

	// Auto migrate all tables
	if err := db.AutoMigrate(
		&domain.User{},
		&domain.Article{},
		&domain.ArticleStat{},
		&domain.Follow{},
		&domain.Like{},
		&domain.Collect{},
		&domain.EventLog{},
		&domain.ArticleEmbedding{},
	); err != nil {
		return err
	}

	// Ensure historical databases also get hot-rank columns.
	columnMigrations := []string{
		`ALTER TABLE IF EXISTS sys_article_stat ADD COLUMN IF NOT EXISTS hot_score_24h DECIMAL(10,4)`,
		`ALTER TABLE IF EXISTS sys_article_stat ADD COLUMN IF NOT EXISTS hot_score_7d DECIMAL(10,4)`,
		`ALTER TABLE IF EXISTS sys_article_stat ADD COLUMN IF NOT EXISTS last_hot_calc TIMESTAMPTZ`,
	}
	for _, sql := range columnMigrations {
		if err := db.Exec(sql).Error; err != nil {
			return err
		}
	}

	// Create additional indexes that GORM doesn't auto-create
	indexes := []string{
		// Article: partial index on status for published articles
		`CREATE INDEX IF NOT EXISTS idx_article_published ON sys_article (published_at DESC) WHERE status = 1`,
		// Article: index for top articles
		`CREATE INDEX IF NOT EXISTS idx_article_top ON sys_article (is_top) WHERE is_top = true`,
		// User: index on status/role
		`CREATE INDEX IF NOT EXISTS idx_user_status ON sys_user (status)`,
		`CREATE INDEX IF NOT EXISTS idx_user_role ON sys_user (role)`,
		// ArticleStat: partial index on hot_score for non-null published articles
		`CREATE INDEX IF NOT EXISTS idx_stat_hot_24h ON sys_article_stat (hot_score_24h DESC) WHERE hot_score_24h IS NOT NULL`,
		// Follow: composite index for follower timeline query
		`CREATE INDEX IF NOT EXISTS idx_follow_follower_time ON sys_follow (follower_id, created_at DESC)`,
		// Like: index on article_id for counting
		`CREATE INDEX IF NOT EXISTS idx_like_article ON sys_like (article_id)`,
		// Collect: index on article_id for counting
		`CREATE INDEX IF NOT EXISTS idx_collect_article ON sys_collect (article_id)`,
		// ArticleEmbedding: HNSW index for cosine similarity
		`CREATE INDEX IF NOT EXISTS idx_embedding_hnsw ON sys_article_embedding USING hnsw (embedding vector_cosine_ops)`,
	}

	for _, idx := range indexes {
		if err := db.Exec(idx).Error; err != nil {
			// Log but don't fail - index might already exist or HNSW might not be available
			continue
		}
	}

	return nil
}
