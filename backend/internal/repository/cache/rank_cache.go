package cache

import (
	"context"
	"fmt"
	"strconv"

	"github.com/redis/go-redis/v9"
)

type RankCache interface {
	GetActiveArticleIDs(ctx context.Context, window string, minScore float64) ([]int64, error)
	GetTopArticleIDs(ctx context.Context, window string, offset, limit int64) ([]int64, error)
	GetHotArticleIDs(ctx context.Context, window string, offset, limit int64) ([]int64, error)
	GetHotScore(ctx context.Context, window string, articleID int64) (float64, error)
	GetHotScores(ctx context.Context, window string, articleIDs []int64) (map[int64]float64, error)
	ReplaceHotRank(ctx context.Context, window string, scores map[int64]float64) error
}

type rankCache struct {
	rdb *redis.Client
}

func NewRankCache(rdb *redis.Client) RankCache {
	return &rankCache{rdb: rdb}
}

func (c *rankCache) GetActiveArticleIDs(ctx context.Context, window string, minScore float64) ([]int64, error) {
	key := fmt.Sprintf("act:articles:window:%s", window)
	vals, err := c.rdb.ZRangeByScore(ctx, key, &redis.ZRangeBy{
		Min: strconv.FormatInt(int64(minScore), 10),
		Max: "+inf",
	}).Result()
	if err != nil {
		return nil, err
	}
	return parseInt64List(vals), nil
}

func (c *rankCache) GetTopArticleIDs(ctx context.Context, window string, offset, limit int64) ([]int64, error) {
	key := fmt.Sprintf("rank:hot:%s", window)
	vals, err := c.rdb.ZRevRange(ctx, key, offset, offset+limit-1).Result()
	if err != nil {
		return nil, err
	}
	return parseInt64List(vals), nil
}

func (c *rankCache) GetHotArticleIDs(ctx context.Context, window string, offset, limit int64) ([]int64, error) {
	return c.GetTopArticleIDs(ctx, window, offset, limit)
}

func (c *rankCache) GetHotScore(ctx context.Context, window string, articleID int64) (float64, error) {
	key := fmt.Sprintf("rank:hot:%s", window)
	score, err := c.rdb.ZScore(ctx, key, strconv.FormatInt(articleID, 10)).Result()
	if err == redis.Nil {
		return 0, nil
	}
	return score, err
}

func (c *rankCache) GetHotScores(ctx context.Context, window string, articleIDs []int64) (map[int64]float64, error) {
	res := make(map[int64]float64, len(articleIDs))
	if len(articleIDs) == 0 {
		return res, nil
	}

	key := fmt.Sprintf("rank:hot:%s", window)
	pipe := c.rdb.Pipeline()
	cmds := make([]*redis.FloatCmd, len(articleIDs))
	for i, articleID := range articleIDs {
		cmds[i] = pipe.ZScore(ctx, key, strconv.FormatInt(articleID, 10))
	}

	_, err := pipe.Exec(ctx)
	if err != nil && err != redis.Nil {
		return nil, err
	}

	for i, cmd := range cmds {
		score, scoreErr := cmd.Result()
		if scoreErr == redis.Nil {
			continue
		}
		if scoreErr != nil {
			return nil, scoreErr
		}
		res[articleIDs[i]] = score
	}

	return res, nil
}

func (c *rankCache) ReplaceHotRank(ctx context.Context, window string, scores map[int64]float64) error {
	key := fmt.Sprintf("rank:hot:%s", window)
	tmpKey := key + ":tmp"

	pipe := c.rdb.Pipeline()
	pipe.Del(ctx, tmpKey)
	if len(scores) > 0 {
		zs := make([]redis.Z, 0, len(scores))
		for articleID, score := range scores {
			zs = append(zs, redis.Z{
				Score:  score,
				Member: strconv.FormatInt(articleID, 10),
			})
		}
		pipe.ZAdd(ctx, tmpKey, zs...)
		pipe.Rename(ctx, tmpKey, key)
	} else {
		pipe.Del(ctx, key)
	}
	_, err := pipe.Exec(ctx)
	return err
}

func parseInt64List(vals []string) []int64 {
	res := make([]int64, 0, len(vals))
	for _, v := range vals {
		id, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			continue
		}
		res = append(res, id)
	}
	return res
}
