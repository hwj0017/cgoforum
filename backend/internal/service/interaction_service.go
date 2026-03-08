package service

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"go.uber.org/zap"

	"cgoforum/internal/domain"
	"cgoforum/internal/mq/publisher"
	"cgoforum/internal/repository/cache"
	"cgoforum/internal/repository/dao"
)

var (
	ErrBadActor = errors.New("invalid actor")
)

const emptyFollowedAuthorsMarker = "__empty__"

type FollowFeedItem struct {
	Article domain.Article `json:"article"`
	Cursor  string         `json:"cursor"`
}

type InteractionService interface {
	Like(ctx context.Context, userID, articleID int64) (int64, error)
	Unlike(ctx context.Context, userID, articleID int64) (int64, error)
	Collect(ctx context.Context, userID, articleID int64) (int64, error)
	Uncollect(ctx context.Context, userID, articleID int64) (int64, error)
	Follow(ctx context.Context, userID, authorID int64) error
	Unfollow(ctx context.Context, userID, authorID int64) error
	IsFollowing(ctx context.Context, userID, authorID int64) (bool, error)
	ListFollowingFeed(ctx context.Context, userID int64, cursor string, limit int) ([]domain.Article, string, bool, error)
}

type interactionService struct {
	followDAO        dao.FollowDAO
	likeDAO          dao.LikeDAO
	collectDAO       dao.CollectDAO
	articleDAO       dao.ArticleDAO
	statDAO          dao.StatDAO
	interactionCache cache.InteractionCache
	feedCache        cache.FeedCache
	eventPub         publisher.EventPublisher
	logger           *zap.Logger
}

func NewInteractionService(
	followDAO dao.FollowDAO,
	likeDAO dao.LikeDAO,
	collectDAO dao.CollectDAO,
	articleDAO dao.ArticleDAO,
	statDAO dao.StatDAO,
	interactionCache cache.InteractionCache,
	feedCache cache.FeedCache,
	eventPub publisher.EventPublisher,
	logger *zap.Logger,
) InteractionService {
	return &interactionService{
		followDAO:        followDAO,
		likeDAO:          likeDAO,
		collectDAO:       collectDAO,
		articleDAO:       articleDAO,
		statDAO:          statDAO,
		interactionCache: interactionCache,
		feedCache:        feedCache,
		eventPub:         eventPub,
		logger:           logger,
	}
}

func (s *interactionService) Like(ctx context.Context, userID, articleID int64) (int64, error) {
	changed, count, err := s.interactionCache.Like(ctx, userID, articleID)
	if err != nil {
		return 0, err
	}
	if !changed {
		return count, nil
	}

	if err = s.eventPub.PublishInteractionEvent(ctx, "like.added", userID, articleID, "added", ""); err != nil {
		s.logger.Warn("publish like.added failed", zap.Error(err), zap.Int64("article_id", articleID))
	}
	return count, nil
}

func (s *interactionService) Unlike(ctx context.Context, userID, articleID int64) (int64, error) {
	changed, count, err := s.interactionCache.Unlike(ctx, userID, articleID)
	if err != nil {
		return 0, err
	}
	if !changed {
		return count, nil
	}

	if err = s.eventPub.PublishInteractionEvent(ctx, "like.removed", userID, articleID, "removed", ""); err != nil {
		s.logger.Warn("publish like.removed failed", zap.Error(err), zap.Int64("article_id", articleID))
	}
	return count, nil
}

func (s *interactionService) Collect(ctx context.Context, userID, articleID int64) (int64, error) {
	_, err := s.collectDAO.CreateCollectWithStat(ctx, userID, articleID)
	if err != nil {
		return 0, err
	}

	if cacheErr := s.interactionCache.InvalidateCollectCache(ctx, articleID); cacheErr != nil {
		s.logger.Warn("invalidate collect cache failed", zap.Error(cacheErr), zap.Int64("article_id", articleID))
	}

	return s.getCollectCount(ctx, articleID), nil
}

func (s *interactionService) Uncollect(ctx context.Context, userID, articleID int64) (int64, error) {
	_, err := s.collectDAO.DeleteCollectWithStat(ctx, userID, articleID)
	if err != nil {
		return 0, err
	}

	if cacheErr := s.interactionCache.InvalidateCollectCache(ctx, articleID); cacheErr != nil {
		s.logger.Warn("invalidate collect cache failed", zap.Error(cacheErr), zap.Int64("article_id", articleID))
	}

	return s.getCollectCount(ctx, articleID), nil
}

func (s *interactionService) getCollectCount(ctx context.Context, articleID int64) int64 {
	stat, err := s.statDAO.GetStat(ctx, articleID)
	if err != nil {
		s.logger.Warn("query collect count failed", zap.Error(err), zap.Int64("article_id", articleID))
		return 0
	}
	if stat == nil {
		return 0
	}
	if stat.CollectCount < 0 {
		return 0
	}
	return stat.CollectCount
}

func (s *interactionService) Follow(ctx context.Context, userID, authorID int64) error {
	if userID == authorID || userID == 0 || authorID == 0 {
		return ErrBadActor
	}

	err := s.followDAO.CreateFollow(ctx, userID, authorID)
	if err != nil && !errors.Is(err, dao.ErrDuplicate) {
		return err
	}
	_ = s.feedCache.InvalidateFollowCaches(ctx, userID, authorID)
	return nil
}

func (s *interactionService) Unfollow(ctx context.Context, userID, authorID int64) error {
	if userID == authorID || userID == 0 || authorID == 0 {
		return ErrBadActor
	}
	if err := s.followDAO.DeleteFollow(ctx, userID, authorID); err != nil {
		return err
	}
	_ = s.feedCache.InvalidateFollowCaches(ctx, userID, authorID)
	return nil
}

func (s *interactionService) IsFollowing(ctx context.Context, userID, authorID int64) (bool, error) {
	return s.followDAO.IsFollowing(ctx, userID, authorID)
}

func (s *interactionService) ListFollowingFeed(ctx context.Context, userID int64, cursor string, limit int) ([]domain.Article, string, bool, error) {
	if limit <= 0 || limit > 50 {
		limit = 20
	}

	var cursorTS int64
	var cursorID int64
	if cursor == "" {
		last, err := s.feedCache.GetLastPull(ctx, userID)
		if err == nil && last != "" {
			if v, parseErr := strconv.ParseInt(last, 10, 64); parseErr == nil {
				cursorTS = v
			}
		}
		if cursorTS == 0 {
			cursorTS = time.Now().Unix()
		}
	} else {
		t, id, err := parseFeedCursor(cursor)
		if err != nil {
			return nil, "", false, err
		}
		cursorTS = t
		cursorID = id
	}

	authorIDs, err := s.getFollowedAuthorIDs(ctx, userID)
	if err != nil {
		s.logger.Warn("get followed authors failed, fallback empty feed", zap.Error(err), zap.Int64("user_id", userID))
		return []domain.Article{}, "", false, nil
	}
	if len(authorIDs) == 0 {
		return []domain.Article{}, "", false, nil
	}

	candidates := make([]int64, 0, 256)
	seen := make(map[int64]struct{}, 256)
	for _, aid := range authorIDs {
		ids, cacheErr := s.feedCache.GetAuthorArticleIDs(ctx, aid, "-inf", strconv.FormatInt(cursorTS, 10))
		if cacheErr != nil {
			continue
		}
		for _, sid := range ids {
			id, parseErr := strconv.ParseInt(sid, 10, 64)
			if parseErr != nil {
				continue
			}
			if _, ok := seen[id]; ok {
				continue
			}
			seen[id] = struct{}{}
			candidates = append(candidates, id)
		}
	}

	if len(candidates) == 0 {
		return []domain.Article{}, "", false, nil
	}

	articleMap, misses, err := s.feedCache.GetArticleSources(ctx, candidates)
	if err != nil {
		s.logger.Warn("get article source cache failed", zap.Error(err), zap.Int64("user_id", userID))
		articleMap = make(map[int64]domain.Article, len(candidates))
		misses = candidates
	}

	if len(misses) > 0 {
		dbArticles, dbErr := s.articleDAO.FindByIDs(ctx, misses)
		if dbErr != nil {
			s.logger.Warn("load feed article metas from db failed, fallback cache only", zap.Error(dbErr), zap.Int64("user_id", userID))
			dbArticles = []domain.Article{}
		}
		metaArticles := make([]domain.Article, 0, len(dbArticles))
		for _, a := range dbArticles {
			meta := trimToFeedMeta(a)
			articleMap[meta.ID] = meta
			metaArticles = append(metaArticles, meta)
		}
		_ = s.feedCache.SetArticleSources(ctx, metaArticles)
	}

	articles := make([]domain.Article, 0, len(candidates))
	for _, id := range candidates {
		a, ok := articleMap[id]
		if ok {
			articles = append(articles, a)
		}
	}

	sortFeedArticles(articles)

	filtered := make([]domain.Article, 0, limit+1)
	for _, a := range articles {
		if a.Status != 1 || a.PublishedAt == nil {
			continue
		}
		ts := a.PublishedAt.Unix()
		if cursor != "" {
			if ts > cursorTS {
				continue
			}
			if ts == cursorTS && a.ID >= cursorID {
				continue
			}
		}
		filtered = append(filtered, a)
		if len(filtered) >= limit+1 {
			break
		}
	}

	hasMore := len(filtered) > limit
	if hasMore {
		filtered = filtered[:limit]
	}

	nextCursor := ""
	if len(filtered) > 0 {
		last := filtered[len(filtered)-1]
		nextCursor = formatFeedCursor(last.PublishedAt.Unix(), last.ID)
		_ = s.feedCache.SetLastPull(ctx, userID, strconv.FormatInt(last.PublishedAt.Unix(), 10))
	}

	return filtered, nextCursor, hasMore, nil
}

func (s *interactionService) getFollowedAuthorIDs(ctx context.Context, userID int64) ([]int64, error) {
	cached, err := s.feedCache.GetFollowedAuthors(ctx, userID)
	if err == nil && len(cached) > 0 {
		if len(cached) == 1 && cached[0] == emptyFollowedAuthorsMarker {
			return []int64{}, nil
		}
		res := make([]int64, 0, len(cached))
		for _, raw := range cached {
			if raw == emptyFollowedAuthorsMarker {
				continue
			}
			id, parseErr := strconv.ParseInt(raw, 10, 64)
			if parseErr == nil {
				res = append(res, id)
			}
		}
		if len(res) > 0 {
			return res, nil
		}
	}

	ids, err := s.followDAO.ListFollowingIDs(ctx, userID)
	if err != nil {
		return nil, err
	}
	if len(ids) == 0 {
		if setErr := s.feedCache.SetFollowedAuthors(ctx, userID, []string{emptyFollowedAuthorsMarker}); setErr != nil {
			s.logger.Warn("set empty followed authors cache failed", zap.Error(setErr), zap.Int64("user_id", userID))
		}
		return ids, nil
	}
	cacheVals := make([]string, 0, len(ids))
	for _, id := range ids {
		cacheVals = append(cacheVals, strconv.FormatInt(id, 10))
	}
	if setErr := s.feedCache.SetFollowedAuthors(ctx, userID, cacheVals); setErr != nil {
		s.logger.Warn("set followed authors cache failed", zap.Error(setErr), zap.Int64("user_id", userID))
	}
	return ids, nil
}

func parseFeedCursor(cursor string) (int64, int64, error) {
	parts := splitByComma(cursor)
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("invalid cursor")
	}
	ts, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return 0, 0, fmt.Errorf("invalid cursor timestamp")
	}
	id, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		return 0, 0, fmt.Errorf("invalid cursor id")
	}
	return ts, id, nil
}

func formatFeedCursor(ts int64, id int64) string {
	return fmt.Sprintf("%d,%d", ts, id)
}

func splitByComma(s string) []string {
	for i := 0; i < len(s); i++ {
		if s[i] == ',' {
			return []string{s[:i], s[i+1:]}
		}
	}
	return []string{s}
}

func sortFeedArticles(items []domain.Article) {
	for i := 0; i < len(items); i++ {
		for j := i + 1; j < len(items); j++ {
			if items[i].PublishedAt == nil || items[j].PublishedAt == nil {
				continue
			}
			leftTS := items[i].PublishedAt.Unix()
			rightTS := items[j].PublishedAt.Unix()
			if rightTS > leftTS || (rightTS == leftTS && items[j].ID > items[i].ID) {
				items[i], items[j] = items[j], items[i]
			}
		}
	}
}

func trimToFeedMeta(a domain.Article) domain.Article {
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
