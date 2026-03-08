package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"go.uber.org/zap"

	"cgoforum/internal/domain"
	"cgoforum/internal/mq/publisher"
	"cgoforum/internal/repository/cache"
	"cgoforum/internal/repository/dao"
	"cgoforum/pkg/snowflake"
	"cgoforum/pkg/xss"
)

var (
	ErrArticleNotFound   = errors.New("article not found")
	ErrArticleForbidden  = errors.New("no permission to modify this article")
	ErrArticleBadRequest = errors.New("invalid article data")
)

type ArticleService interface {
	Create(ctx context.Context, userID int64, title, summary, contentMD, coverImg string, status int16) (*domain.Article, error)
	GetByID(ctx context.Context, id int64) (*domain.Article, error)
	Update(ctx context.Context, userID int64, articleID int64, title, summary, contentMD, coverImg string, status int16) error
	Delete(ctx context.Context, userID int64, articleID int64, role int16) error
	List(ctx context.Context, cursor string, limit int) ([]domain.Article, string, bool, error)
	ListByAuthor(ctx context.Context, userID int64, cursor string, limit int) ([]domain.Article, string, bool, error)
}

type articleService struct {
	articleDAO dao.ArticleDAO
	feedCache  cache.FeedCache
	eventPub   publisher.EventPublisher
	logger     *zap.Logger
}

func NewArticleService(
	articleDAO dao.ArticleDAO,
	feedCache cache.FeedCache,
	eventPub publisher.EventPublisher,
	logger *zap.Logger,
) ArticleService {
	return &articleService{
		articleDAO: articleDAO,
		feedCache:  feedCache,
		eventPub:   eventPub,
		logger:     logger,
	}
}

func (s *articleService) Create(ctx context.Context, userID int64, title, summary, contentMD, coverImg string, status int16) (*domain.Article, error) {
	// Sanitize content
	contentMD = xss.SanitizeMarkdown(contentMD)

	now := time.Now()
	article := &domain.Article{
		ID:        snowflake.GenerateID(),
		UserID:    userID,
		Title:     title,
		Summary:   summary,
		ContentMD: contentMD,
		CoverImg:  coverImg,
		Status:    status,
		CreatedAt: now,
		UpdatedAt: now,
	}

	if status == 1 {
		article.PublishedAt = &now
	}

	stat := &domain.ArticleStat{
		UpdatedAt: now,
	}

	if err := s.articleDAO.Create(ctx, article, stat); err != nil {
		return nil, fmt.Errorf("create article: %w", err)
	}

	// If published, add to feed cache and publish event
	if status == 1 {
		_ = s.feedCache.AddAuthorArticle(ctx, userID, article.ID, float64(now.Unix()))
		_ = s.feedCache.SetArticleSources(ctx, []domain.Article{*article})

		if err := s.eventPub.PublishArticleEvent(ctx, "article.published", article); err != nil {
			s.logger.Error("failed to publish article event", zap.Error(err))
		}

		// Publish embedding task (stub will just ACK it)
		text := article.Title + " " + article.Summary + " " + xss.StripHTML(article.ContentMD)
		if len(text) > 2000 {
			text = text[:2000]
		}
		if err := s.eventPub.PublishEmbeddingTask(ctx, article.ID, text, "bge-m3-v1"); err != nil {
			s.logger.Error("failed to publish embedding task", zap.Error(err))
		}
	}

	return article, nil
}

func (s *articleService) GetByID(ctx context.Context, id int64) (*domain.Article, error) {
	article, err := s.articleDAO.FindByIDAndIncrViewCount(ctx, id)
	if err != nil {
		return nil, err
	}
	if article == nil {
		return nil, ErrArticleNotFound
	}

	if err := s.eventPub.PublishInteractionEvent(ctx, "article.viewed", 0, id, "view", ""); err != nil {
		s.logger.Warn("failed to publish article.viewed event", zap.Error(err), zap.Int64("article_id", id))
	}

	return article, nil
}

func (s *articleService) Update(ctx context.Context, userID int64, articleID int64, title, summary, contentMD, coverImg string, status int16) error {
	// Sanitize content
	contentMD = xss.SanitizeMarkdown(contentMD)

	now := time.Now()
	article, err := s.articleDAO.UpdateByOwner(ctx, userID, articleID, title, summary, contentMD, coverImg, status, now)
	if err != nil {
		if errors.Is(err, dao.ErrArticleNotFound) {
			return ErrArticleNotFound
		}
		if errors.Is(err, dao.ErrArticleForbidden) {
			return ErrArticleForbidden
		}
		return fmt.Errorf("update article: %w", err)
	}

	// Re-publish to update search index
	if status == 1 {
		_ = s.feedCache.AddAuthorArticle(ctx, userID, article.ID, float64(now.Unix()))
		_ = s.feedCache.SetArticleSources(ctx, []domain.Article{*article})
		if err := s.eventPub.PublishArticleEvent(ctx, "article.updated", article); err != nil {
			s.logger.Error("failed to publish article update event", zap.Error(err))
		}
	} else {
		_ = s.feedCache.InvalidateArticleSource(ctx, article.ID)
	}

	return nil
}

func (s *articleService) Delete(ctx context.Context, userID int64, articleID int64, role int16) error {
	article, err := s.articleDAO.DeleteByOwnerOrAdmin(ctx, userID, articleID, role >= 1)
	if err != nil {
		if errors.Is(err, dao.ErrArticleNotFound) {
			return ErrArticleNotFound
		}
		if errors.Is(err, dao.ErrArticleForbidden) {
			return ErrArticleForbidden
		}
		return fmt.Errorf("delete article: %w", err)
	}

	// Remove from feed cache
	_ = s.feedCache.RemoveAuthorArticle(ctx, article.UserID, articleID)
	_ = s.feedCache.InvalidateArticleSource(ctx, articleID)

	// Publish delete event
	if err := s.eventPub.PublishArticleEvent(ctx, "article.deleted", article); err != nil {
		s.logger.Error("failed to publish article delete event", zap.Error(err))
	}

	return nil
}

func (s *articleService) List(ctx context.Context, cursor string, limit int) ([]domain.Article, string, bool, error) {
	if limit <= 0 || limit > 50 {
		limit = 20
	}

	articles, err := s.articleDAO.ListPublished(ctx, cursor, limit)
	if err != nil {
		return nil, "", false, err
	}

	hasMore := len(articles) > limit
	if hasMore {
		articles = articles[:limit]
	}

	nextCursor := ""
	if hasMore && len(articles) > 0 {
		last := articles[len(articles)-1]
		if last.PublishedAt != nil {
			nextCursor = fmt.Sprintf("%s,%d", last.PublishedAt.Format(time.RFC3339Nano), last.ID)
		}
	}

	return articles, nextCursor, hasMore, nil
}

func (s *articleService) ListByAuthor(ctx context.Context, userID int64, cursor string, limit int) ([]domain.Article, string, bool, error) {
	if limit <= 0 || limit > 50 {
		limit = 20
	}

	articles, err := s.articleDAO.ListByAuthor(ctx, userID, cursor, limit)
	if err != nil {
		return nil, "", false, err
	}

	hasMore := len(articles) > limit
	if hasMore {
		articles = articles[:limit]
	}

	nextCursor := ""
	if hasMore && len(articles) > 0 {
		last := articles[len(articles)-1]
		if last.PublishedAt != nil {
			nextCursor = fmt.Sprintf("%s,%d", last.PublishedAt.Format(time.RFC3339Nano), last.ID)
		}
	}

	return articles, nextCursor, hasMore, nil
}
