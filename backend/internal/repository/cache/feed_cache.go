package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"cgoforum/internal/domain"
	"github.com/redis/go-redis/v9"
)

type FeedCache interface {
	GetLastPull(ctx context.Context, userID int64) (string, error)
	SetLastPull(ctx context.Context, userID int64, timestamp string) error
	GetAuthorArticleIDs(ctx context.Context, authorID int64, minScore, maxScore string) ([]string, error)
	AddAuthorArticle(ctx context.Context, authorID int64, articleID int64, publishedAt float64) error
	RemoveAuthorArticle(ctx context.Context, authorID int64, articleID int64) error
	GetFollowedAuthors(ctx context.Context, userID int64) ([]string, error)
	SetFollowedAuthors(ctx context.Context, userID int64, authorIDs []string) error
	InvalidateFollowCaches(ctx context.Context, userID, authorID int64) error
	InvalidateFollowedAuthors(ctx context.Context, userID int64) error
	InvalidateFollowerCount(ctx context.Context, authorID int64) error
	GetArticleSources(ctx context.Context, articleIDs []int64) (map[int64]domain.Article, []int64, error)
	SetArticleSources(ctx context.Context, articles []domain.Article) error
	InvalidateArticleSource(ctx context.Context, articleID int64) error
}

type feedCache struct {
	rdb *redis.Client
}

type articleSourceMeta struct {
	ID          int64              `json:"id"`
	UserID      int64              `json:"user_id"`
	Title       string             `json:"title"`
	Summary     string             `json:"summary"`
	CoverImg    string             `json:"cover_img"`
	Status      int16              `json:"status"`
	IsTop       bool               `json:"is_top"`
	CreatedAt   time.Time          `json:"created_at"`
	UpdatedAt   time.Time          `json:"updated_at"`
	PublishedAt *time.Time         `json:"published_at"`
	Stat        *domain.ArticleStat `json:"stat,omitempty"`
	User        *domain.User       `json:"user,omitempty"`
}

func NewFeedCache(rdb *redis.Client) FeedCache {
	return &feedCache{rdb: rdb}
}

func (c *feedCache) GetLastPull(ctx context.Context, userID int64) (string, error) {
	key := fmt.Sprintf("feed:last_pull:%d", userID)
	val, err := c.rdb.Get(ctx, key).Result()
	if err == redis.Nil {
		return "", nil
	}
	return val, err
}

func (c *feedCache) SetLastPull(ctx context.Context, userID int64, timestamp string) error {
	key := fmt.Sprintf("feed:last_pull:%d", userID)
	return c.rdb.Set(ctx, key, timestamp, 0).Err()
}

func (c *feedCache) GetAuthorArticleIDs(ctx context.Context, authorID int64, minScore, maxScore string) ([]string, error) {
	key := fmt.Sprintf("feed:author:ids:%d", authorID)
	return c.rdb.ZRevRangeByScore(ctx, key, &redis.ZRangeBy{
		Min:    minScore,
		Max:    maxScore,
		Offset: 0,
		Count:  50,
	}).Result()
}

func (c *feedCache) AddAuthorArticle(ctx context.Context, authorID int64, articleID int64, publishedAt float64) error {
	key := fmt.Sprintf("feed:author:ids:%d", authorID)
	return c.rdb.ZAdd(ctx, key, redis.Z{
		Score:  publishedAt,
		Member: articleID,
	}).Err()
}

func (c *feedCache) RemoveAuthorArticle(ctx context.Context, authorID int64, articleID int64) error {
	key := fmt.Sprintf("feed:author:ids:%d", authorID)
	return c.rdb.ZRem(ctx, key, articleID).Err()
}

func (c *feedCache) GetFollowedAuthors(ctx context.Context, userID int64) ([]string, error) {
	key := fmt.Sprintf("follow:authors:%d", userID)
	return c.rdb.SMembers(ctx, key).Result()
}

func (c *feedCache) SetFollowedAuthors(ctx context.Context, userID int64, authorIDs []string) error {
	key := fmt.Sprintf("follow:authors:%d", userID)
	pipe := c.rdb.Pipeline()
	pipe.Del(ctx, key)
	if len(authorIDs) > 0 {
		members := make([]interface{}, len(authorIDs))
		for i, id := range authorIDs {
			members[i] = id
		}
		pipe.SAdd(ctx, key, members...)
	}
	pipe.Expire(ctx, key, 10*time.Minute)
	_, err := pipe.Exec(ctx)
	return err
}

func (c *feedCache) InvalidateFollowedAuthors(ctx context.Context, userID int64) error {
	key := fmt.Sprintf("follow:authors:%d", userID)
	return c.rdb.Del(ctx, key).Err()
}

func (c *feedCache) InvalidateFollowerCount(ctx context.Context, authorID int64) error {
	key := fmt.Sprintf("follow:count:%d", authorID)
	return c.rdb.Del(ctx, key).Err()
}

func (c *feedCache) InvalidateFollowCaches(ctx context.Context, userID, authorID int64) error {
	followedAuthorsKey := fmt.Sprintf("follow:authors:%d", userID)
	followerCountKey := fmt.Sprintf("follow:count:%d", authorID)
	pipe := c.rdb.Pipeline()
	pipe.Del(ctx, followedAuthorsKey)
	pipe.Del(ctx, followerCountKey)
	_, err := pipe.Exec(ctx)
	return err
}

func (c *feedCache) GetArticleSources(ctx context.Context, articleIDs []int64) (map[int64]domain.Article, []int64, error) {
	res := make(map[int64]domain.Article, len(articleIDs))
	if len(articleIDs) == 0 {
		return res, nil, nil
	}

	keys := make([]string, 0, len(articleIDs))
	for _, id := range articleIDs {
		keys = append(keys, fmt.Sprintf("feed:article:src:%d", id))
	}

	vals, err := c.rdb.MGet(ctx, keys...).Result()
	if err != nil {
		return nil, nil, err
	}

	misses := make([]int64, 0)
	for i, v := range vals {
		id := articleIDs[i]
		if v == nil {
			misses = append(misses, id)
			continue
		}

		raw, ok := v.(string)
		if !ok || raw == "" {
			misses = append(misses, id)
			continue
		}

		var article domain.Article
		var meta articleSourceMeta
		if err := json.Unmarshal([]byte(raw), &meta); err != nil {
			misses = append(misses, id)
			continue
		}
		article = domain.Article{
			ID:          meta.ID,
			UserID:      meta.UserID,
			Title:       meta.Title,
			Summary:     meta.Summary,
			CoverImg:    meta.CoverImg,
			Status:      meta.Status,
			IsTop:       meta.IsTop,
			CreatedAt:   meta.CreatedAt,
			UpdatedAt:   meta.UpdatedAt,
			PublishedAt: meta.PublishedAt,
			Stat:        meta.Stat,
			User:        meta.User,
		}
		res[id] = article
	}

	return res, misses, nil
}

func (c *feedCache) SetArticleSources(ctx context.Context, articles []domain.Article) error {
	if len(articles) == 0 {
		return nil
	}

	pipe := c.rdb.Pipeline()
	for _, article := range articles {
		key := fmt.Sprintf("feed:article:src:%d", article.ID)
		meta := articleSourceMeta{
			ID:          article.ID,
			UserID:      article.UserID,
			Title:       article.Title,
			Summary:     article.Summary,
			CoverImg:    article.CoverImg,
			Status:      article.Status,
			IsTop:       article.IsTop,
			CreatedAt:   article.CreatedAt,
			UpdatedAt:   article.UpdatedAt,
			PublishedAt: article.PublishedAt,
			Stat:        article.Stat,
			User:        article.User,
		}
		data, err := json.Marshal(meta)
		if err != nil {
			continue
		}
		pipe.Set(ctx, key, data, 30*time.Minute)
	}
	_, err := pipe.Exec(ctx)
	return err
}

func (c *feedCache) InvalidateArticleSource(ctx context.Context, articleID int64) error {
	key := fmt.Sprintf("feed:article:src:%d", articleID)
	return c.rdb.Del(ctx, key).Err()
}
