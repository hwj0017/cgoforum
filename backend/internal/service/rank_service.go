package service

import (
	"context"
	"math"
	"sort"
	"time"

	"go.uber.org/zap"

	"cgoforum/internal/domain"
	"cgoforum/internal/repository/cache"
	"cgoforum/internal/repository/dao"
)

type RankService interface {
	Rebuild24h(ctx context.Context) error
	Rebuild7d(ctx context.Context) error
	ListHot(ctx context.Context, window string, limit int) ([]domain.Article, error)
}

type rankService struct {
	rankCache cache.RankCache
	feedCache cache.FeedCache
	articleDAO dao.ArticleDAO
	statDAO   dao.StatDAO
	logger    *zap.Logger
}

func NewRankService(rankCache cache.RankCache, feedCache cache.FeedCache, articleDAO dao.ArticleDAO, statDAO dao.StatDAO, logger *zap.Logger) RankService {
	return &rankService{
		rankCache: rankCache,
		feedCache: feedCache,
		articleDAO: articleDAO,
		statDAO:   statDAO,
		logger:    logger,
	}
}

func (s *rankService) Rebuild24h(ctx context.Context) error {
	return s.rebuild(ctx, "24h", 24*time.Hour)
}

func (s *rankService) Rebuild7d(ctx context.Context) error {
	return s.rebuild(ctx, "7d", 7*24*time.Hour)
}

func (s *rankService) ListHot(ctx context.Context, window string, limit int) ([]domain.Article, error) {
	if window != "24h" && window != "7d" {
		window = "24h"
	}
	if limit <= 0 || limit > 100 {
		limit = 20
	}

	ids, err := s.rankCache.GetHotArticleIDs(ctx, window, 0, int64(limit))
	if err != nil {
		s.logger.Warn("get hot rank ids failed, fallback empty list", zap.Error(err), zap.String("window", window))
		return []domain.Article{}, nil
	}
	if len(ids) == 0 {
		return []domain.Article{}, nil
	}

	articleMap, misses, err := s.feedCache.GetArticleSources(ctx, ids)
	if err != nil {
		s.logger.Warn("rank list get source cache failed", zap.Error(err))
		articleMap = make(map[int64]domain.Article, len(ids))
		misses = ids
	}

	if len(misses) > 0 {
		dbArticles, dbErr := s.articleDAO.FindByIDs(ctx, misses)
		if dbErr != nil {
			s.logger.Warn("rank list load metas from db failed, fallback cache only", zap.Error(dbErr), zap.String("window", window))
			dbArticles = []domain.Article{}
		}
		metaArticles := make([]domain.Article, 0, len(dbArticles))
		for _, a := range dbArticles {
			meta := trimToRankMeta(a)
			articleMap[meta.ID] = meta
			metaArticles = append(metaArticles, meta)
		}
		_ = s.feedCache.SetArticleSources(ctx, metaArticles)
	}

	articles := make([]domain.Article, 0, len(ids))
	for _, id := range ids {
		a, ok := articleMap[id]
		if ok {
			articles = append(articles, a)
		}
	}

	order := make(map[int64]int, len(ids))
	for i, id := range ids {
		order[id] = i
	}
	sort.SliceStable(articles, func(i, j int) bool {
		return order[articles[i].ID] < order[articles[j].ID]
	})

	return articles, nil
}

func (s *rankService) rebuild(ctx context.Context, window string, duration time.Duration) error {
	now := time.Now()
	cutoff := float64(now.Add(-duration).Unix())

	activeIDs, err := s.rankCache.GetActiveArticleIDs(ctx, window, cutoff)
	if err != nil {
		return err
	}
	topIDs, err := s.rankCache.GetTopArticleIDs(ctx, window, 0, 200)
	if err != nil {
		return err
	}

	poolSet := make(map[int64]struct{}, len(activeIDs)+len(topIDs))
	for _, id := range activeIDs {
		poolSet[id] = struct{}{}
	}
	for _, id := range topIDs {
		poolSet[id] = struct{}{}
	}

	if len(poolSet) == 0 {
		return s.rankCache.ReplaceHotRank(ctx, window, map[int64]float64{})
	}

	poolIDs := make([]int64, 0, len(poolSet))
	for id := range poolSet {
		poolIDs = append(poolIDs, id)
	}

	articleMap, misses, err := s.feedCache.GetArticleSources(ctx, poolIDs)
	if err != nil {
		s.logger.Warn("rank rebuild get source cache failed", zap.Error(err))
		articleMap = make(map[int64]domain.Article, len(poolIDs))
		misses = poolIDs
	}

	if len(misses) > 0 {
		dbArticles, dbErr := s.articleDAO.FindByIDs(ctx, misses)
		if dbErr != nil {
			return dbErr
		}
		metaArticles := make([]domain.Article, 0, len(dbArticles))
		for _, a := range dbArticles {
			meta := trimToRankMeta(a)
			articleMap[meta.ID] = meta
			metaArticles = append(metaArticles, meta)
		}
		_ = s.feedCache.SetArticleSources(ctx, metaArticles)
	}

	articles := make([]domain.Article, 0, len(poolIDs))
	for _, id := range poolIDs {
		if a, ok := articleMap[id]; ok {
			articles = append(articles, a)
		}
	}

	oldScores, oldScoresErr := s.rankCache.GetHotScores(ctx, window, poolIDs)
	if oldScoresErr != nil {
		s.logger.Warn("batch get hot scores failed", zap.Error(oldScoresErr), zap.String("window", window))
		oldScores = map[int64]float64{}
	}

	scores := make(map[int64]float64, len(articles))
	for _, article := range articles {
		if article.Status != 1 || article.PublishedAt == nil {
			continue
		}
		var likeCount int64
		var collectCount int64
		var commentCount int64
		if article.Stat != nil {
			likeCount = article.Stat.LikeCount
			collectCount = article.Stat.CollectCount
			commentCount = article.Stat.CommentCount
		}

		hours := now.Sub(*article.PublishedAt).Hours()
		if hours < 0 {
			hours = 0
		}
		raw := float64(likeCount)*1 + float64(collectCount)*10 + float64(commentCount)*5
		score := raw / math.Pow(hours+2, 1.2)

		oldScore, ok := oldScores[article.ID]
		if ok && oldScore > 0 {
			diff := math.Abs(score-oldScore) / oldScore
			if diff < 0.005 {
				score = oldScore
			}
		}
		scores[article.ID] = score
	}

	if err := s.rankCache.ReplaceHotRank(ctx, window, scores); err != nil {
		return err
	}

	if err := s.statDAO.BatchUpdateHotScores(ctx, window, scores); err != nil {
		s.logger.Warn("batch update hot scores failed", zap.Error(err), zap.String("window", window))
	}

	return nil
}

func trimToRankMeta(a domain.Article) domain.Article {
	return domain.Article{
		ID:          a.ID,
		UserID:      a.UserID,
		Title:       a.Title,
		Summary:     a.Summary,
		CoverImg:    a.CoverImg,
		Status:      a.Status,
		IsTop:       a.IsTop,
		CreatedAt:   a.CreatedAt,
		UpdatedAt:   a.UpdatedAt,
		PublishedAt: a.PublishedAt,
		User:        a.User,
		Stat:        a.Stat,
	}
}
