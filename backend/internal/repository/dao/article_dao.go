package dao

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"cgoforum/internal/domain"

	"github.com/pgvector/pgvector-go"
	"gorm.io/gorm"
)

type ArticleDAO interface {
	Create(ctx context.Context, article *domain.Article, stat *domain.ArticleStat) error
	FindByID(ctx context.Context, id int64) (*domain.Article, error)
	FindByIDAndIncrViewCount(ctx context.Context, id int64) (*domain.Article, error)
	Update(ctx context.Context, article *domain.Article) error
	UpdateByOwner(ctx context.Context, userID, articleID int64, title, summary, contentMD, coverImg string, status int16, now time.Time) (*domain.Article, error)
	Delete(ctx context.Context, id int64) error
	DeleteByOwnerOrAdmin(ctx context.Context, userID, articleID int64, isAdmin bool) (*domain.Article, error)
	ListPublished(ctx context.Context, cursor string, limit int) ([]domain.Article, error)
	ListByAuthor(ctx context.Context, userID int64, cursor string, limit int) ([]domain.Article, error)
	FindByIDs(ctx context.Context, ids []int64) ([]domain.Article, error)
	IncrViewCount(ctx context.Context, articleID int64) error
	UpsertEmbedding(ctx context.Context, articleID int64, chunkID string, embedding []float32, sectionTitle string, contentText string, modelVersion string) error
	VectorSearchArticleIDs(ctx context.Context, queryEmbedding []float32, limit int) ([]int64, error)
}

type articleDAO struct {
	db *gorm.DB
}

var (
	ErrArticleNotFound  = errors.New("article not found")
	ErrArticleForbidden = errors.New("no permission to modify this article")
)

func NewArticleDAO(db *gorm.DB) ArticleDAO {
	return &articleDAO{db: db}
}

func (d *articleDAO) Create(ctx context.Context, article *domain.Article, stat *domain.ArticleStat) error {
	return d.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(article).Error; err != nil {
			return err
		}
		stat.ArticleID = article.ID
		return tx.Create(stat).Error
	})
}

func (d *articleDAO) FindByID(ctx context.Context, id int64) (*domain.Article, error) {
	var article domain.Article
	if err := d.db.WithContext(ctx).
		Preload("User", func(db *gorm.DB) *gorm.DB {
			return db.Select("id", "nickname", "avatar_url")
		}).
		Preload("Stat").
		First(&article, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &article, nil
}

func (d *articleDAO) FindByIDAndIncrViewCount(ctx context.Context, id int64) (*domain.Article, error) {
	var article domain.Article
	err := d.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.WithContext(ctx).
			Preload("User", func(db *gorm.DB) *gorm.DB {
				return db.Select("id", "nickname", "avatar_url")
			}).
			Preload("Stat").
			First(&article, id).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return gorm.ErrRecordNotFound
			}
			return err
		}

		if err := tx.WithContext(ctx).Model(&domain.ArticleStat{}).
			Where("article_id = ?", id).
			Updates(map[string]interface{}{
				"view_count": gorm.Expr("view_count + 1"),
				"updated_at": gorm.Expr("NOW()"),
			}).Error; err != nil {
			return err
		}

		return nil
	})
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &article, nil
}

func (d *articleDAO) Update(ctx context.Context, article *domain.Article) error {
	return d.db.WithContext(ctx).Model(article).
		Select("title", "summary", "content_md", "cover_img", "status", "is_top", "updated_at", "published_at").
		Updates(map[string]interface{}{
			"title":        article.Title,
			"summary":      article.Summary,
			"content_md":   article.ContentMD,
			"cover_img":    article.CoverImg,
			"status":       article.Status,
			"is_top":       article.IsTop,
			"updated_at":   gorm.Expr("NOW()"),
			"published_at": article.PublishedAt,
		}).Error
}

func (d *articleDAO) UpdateByOwner(ctx context.Context, userID, articleID int64, title, summary, contentMD, coverImg string, status int16, now time.Time) (*domain.Article, error) {
	article := &domain.Article{}
	err := d.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.WithContext(ctx).
			Select("id", "user_id", "is_top", "published_at").
			First(article, articleID).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrArticleNotFound
			}
			return err
		}

		if article.UserID != userID {
			return ErrArticleForbidden
		}

		if status == 1 && article.PublishedAt == nil {
			article.PublishedAt = &now
		}

		if err := tx.WithContext(ctx).Model(&domain.Article{}).
			Where("id = ?", articleID).
			Select("title", "summary", "content_md", "cover_img", "status", "is_top", "updated_at", "published_at").
			Updates(map[string]interface{}{
				"title":        title,
				"summary":      summary,
				"content_md":   contentMD,
				"cover_img":    coverImg,
				"status":       status,
				"is_top":       article.IsTop,
				"updated_at":   now,
				"published_at": article.PublishedAt,
			}).Error; err != nil {
			return err
		}

		article.Title = title
		article.Summary = summary
		article.ContentMD = contentMD
		article.CoverImg = coverImg
		article.Status = status
		article.UpdatedAt = now
		return nil
	})
	if err != nil {
		return nil, err
	}
	return article, nil
}

func (d *articleDAO) Delete(ctx context.Context, id int64) error {
	return d.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.WithContext(ctx).Where("article_id = ?", id).Delete(&domain.ArticleStat{}).Error; err != nil {
			return err
		}
		if err := tx.WithContext(ctx).Where("article_id = ?", id).Delete(&domain.ArticleEmbedding{}).Error; err != nil {
			return err
		}
		if err := tx.WithContext(ctx).Where("article_id = ?", id).Delete(&domain.Like{}).Error; err != nil {
			return err
		}
		if err := tx.WithContext(ctx).Where("article_id = ?", id).Delete(&domain.Collect{}).Error; err != nil {
			return err
		}

		// Hard delete; in production you might want to set status=3 instead
		return tx.WithContext(ctx).Delete(&domain.Article{}, id).Error
	})
}

func (d *articleDAO) DeleteByOwnerOrAdmin(ctx context.Context, userID, articleID int64, isAdmin bool) (*domain.Article, error) {
	article := &domain.Article{}
	err := d.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.WithContext(ctx).First(article, articleID).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrArticleNotFound
			}
			return err
		}

		if !isAdmin && article.UserID != userID {
			return ErrArticleForbidden
		}

		if err := tx.WithContext(ctx).Where("article_id = ?", articleID).Delete(&domain.ArticleStat{}).Error; err != nil {
			return err
		}
		if err := tx.WithContext(ctx).Where("article_id = ?", articleID).Delete(&domain.ArticleEmbedding{}).Error; err != nil {
			return err
		}
		if err := tx.WithContext(ctx).Where("article_id = ?", articleID).Delete(&domain.Like{}).Error; err != nil {
			return err
		}
		if err := tx.WithContext(ctx).Where("article_id = ?", articleID).Delete(&domain.Collect{}).Error; err != nil {
			return err
		}

		return tx.WithContext(ctx).Delete(&domain.Article{}, articleID).Error
	})
	if err != nil {
		return nil, err
	}
	return article, nil
}

func (d *articleDAO) ListPublished(ctx context.Context, cursor string, limit int) ([]domain.Article, error) {
	query := d.db.WithContext(ctx).
		Where("status = ?", 1).
		Preload("User", func(db *gorm.DB) *gorm.DB {
			return db.Select("id", "nickname", "avatar_url")
		}).
		Preload("Stat").
		Order("is_top DESC, published_at DESC, id DESC").
		Limit(limit + 1) // fetch one extra to determine if there's a next page

	if cursor != "" {
		// Cursor format: "published_at,id" (ISO timestamp,int64)
		var cursorTime string
		var cursorID int64
		if _, err := parseCursor(cursor, &cursorTime, &cursorID); err == nil {
			query = query.Where(
				"(published_at, id) < (?, ?)", cursorTime, cursorID,
			)
		}
	}

	var articles []domain.Article
	if err := query.Find(&articles).Error; err != nil {
		return nil, err
	}
	return articles, nil
}

func (d *articleDAO) ListByAuthor(ctx context.Context, userID int64, cursor string, limit int) ([]domain.Article, error) {
	query := d.db.WithContext(ctx).
		Where("user_id = ? AND status = ?", userID, 1).
		Preload("Stat").
		Order("published_at DESC, id DESC").
		Limit(limit + 1)

	if cursor != "" {
		var cursorTime string
		var cursorID int64
		if _, err := parseCursor(cursor, &cursorTime, &cursorID); err == nil {
			query = query.Where(
				"(published_at, id) < (?, ?)", cursorTime, cursorID,
			)
		}
	}

	var articles []domain.Article
	if err := query.Find(&articles).Error; err != nil {
		return nil, err
	}
	return articles, nil
}

func (d *articleDAO) FindByIDs(ctx context.Context, ids []int64) ([]domain.Article, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	var articles []domain.Article
	if err := d.db.WithContext(ctx).
		Preload("User", func(db *gorm.DB) *gorm.DB {
			return db.Select("id", "nickname", "avatar_url")
		}).
		Preload("Stat").
		Where("id IN ?", ids).
		Find(&articles).Error; err != nil {
		return nil, err
	}
	return articles, nil
}

func (d *articleDAO) IncrViewCount(ctx context.Context, articleID int64) error {
	return d.db.WithContext(ctx).Model(&domain.ArticleStat{}).
		Where("article_id = ?", articleID).
		Updates(map[string]interface{}{
			"view_count": gorm.Expr("view_count + 1"),
			"updated_at": gorm.Expr("NOW()"),
		}).Error
}

func (d *articleDAO) UpsertEmbedding(ctx context.Context, articleID int64, chunkID string, embedding []float32, sectionTitle string, contentText string, modelVersion string) error {
	vec := pgvector.NewVector(embedding)
	return d.db.WithContext(ctx).Exec(
		`INSERT INTO sys_article_embedding (article_id, chunk_id, embedding, section_title, content_text, model_version, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, NOW())
		 ON CONFLICT (article_id, chunk_id) DO UPDATE 
		 SET embedding = EXCLUDED.embedding, 
		     section_title = EXCLUDED.section_title, 
		     content_text = EXCLUDED.content_text, 
		     model_version = EXCLUDED.model_version, 
		     updated_at = NOW()`,
		articleID,
		chunkID,
		vec,
		sectionTitle,
		contentText,
		modelVersion,
	).Error
}

func (d *articleDAO) VectorSearchArticleIDs(ctx context.Context, queryEmbedding []float32, limit int) ([]int64, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	vec := pgvector.NewVector(queryEmbedding)
	var ids []int64
	err := d.db.WithContext(ctx).Raw(
		`SELECT e.article_id
		   FROM sys_article_embedding e
		   JOIN sys_article a ON a.id = e.article_id
		  WHERE a.status = 1
		  ORDER BY e.embedding <=> ?
		  LIMIT ?`,
		vec,
		limit,
	).Scan(&ids).Error
	if err != nil {
		return nil, err
	}
	return ids, nil
}

// parseCursor parses "timestamp,id" format cursor
func parseCursor(cursor string, timeStr *string, id *int64) (int, error) {
	parts := strings.SplitN(cursor, ",", 2)
	if len(parts) != 2 {
		return 0, fmt.Errorf("invalid cursor format")
	}
	*timeStr = parts[0]
	parsedID, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid cursor id: %w", err)
	}
	*id = parsedID
	return 2, nil
}
