package cache

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"cgoforum/pkg/luascripts"

	"github.com/redis/go-redis/v9"
)

type InteractionCache interface {
	Like(ctx context.Context, userID, articleID int64) (changed bool, count int64, err error)
	Unlike(ctx context.Context, userID, articleID int64) (changed bool, count int64, err error)
	IsLikeCacheReady(ctx context.Context, articleID int64) (bool, error)
	WarmLikeCache(ctx context.Context, articleID int64, likedUserIDs []int64) error
	InvalidateCollectCache(ctx context.Context, articleID int64) error
	GetLikeCount(ctx context.Context, articleID int64) (int64, error)
	GetCollectCount(ctx context.Context, articleID int64) (int64, error)
	BatchGetCounts(ctx context.Context, articleIDs []int64) (map[int64]InteractionCount, []int64, error)
	BatchSetCounts(ctx context.Context, counts map[int64]InteractionCount) error
}

type InteractionCount struct {
	LikeCount    int64
	CollectCount int64
}

type interactionCache struct {
	rdb *redis.Client
}

var likeLua = redis.NewScript(luascripts.LikeScript)
var unlikeLua = redis.NewScript(luascripts.UnlikeScript)

func NewInteractionCache(rdb *redis.Client) InteractionCache {
	return &interactionCache{rdb: rdb}
}

func (c *interactionCache) Like(ctx context.Context, userID, articleID int64) (bool, int64, error) {
	usersKey := fmt.Sprintf("like:users:%d", articleID)
	countKey := fmt.Sprintf("like:count:%d", articleID)

	ret, err := likeLua.Run(ctx, c.rdb, []string{usersKey, countKey}, userID).Result()
	if err != nil {
		return false, 0, err
	}

	arr, ok := ret.([]interface{})
	if !ok || len(arr) != 2 {
		return false, 0, fmt.Errorf("invalid lua return")
	}

	changed := toInt64(arr[0]) == 1
	count := toInt64(arr[1])
	return changed, count, nil
}

func (c *interactionCache) Unlike(ctx context.Context, userID, articleID int64) (bool, int64, error) {
	usersKey := fmt.Sprintf("like:users:%d", articleID)
	countKey := fmt.Sprintf("like:count:%d", articleID)

	ret, err := unlikeLua.Run(ctx, c.rdb, []string{usersKey, countKey}, userID).Result()
	if err != nil {
		return false, 0, err
	}

	arr, ok := ret.([]interface{})
	if !ok || len(arr) != 2 {
		return false, 0, fmt.Errorf("invalid lua return")
	}

	changed := toInt64(arr[0]) == 1
	count := toInt64(arr[1])
	return changed, count, nil
}

func (c *interactionCache) IsLikeCacheReady(ctx context.Context, articleID int64) (bool, error) {
	usersKey := fmt.Sprintf("like:users:%d", articleID)
	countKey := fmt.Sprintf("like:count:%d", articleID)
	exists, err := c.rdb.Exists(ctx, usersKey, countKey).Result()
	if err != nil {
		return false, err
	}
	return exists == 2, nil
}

func (c *interactionCache) WarmLikeCache(ctx context.Context, articleID int64, likedUserIDs []int64) error {
	usersKey := fmt.Sprintf("like:users:%d", articleID)
	countKey := fmt.Sprintf("like:count:%d", articleID)

	pipe := c.rdb.TxPipeline()
	pipe.Del(ctx, usersKey, countKey)
	if len(likedUserIDs) > 0 {
		members := make([]interface{}, 0, len(likedUserIDs))
		for _, uid := range likedUserIDs {
			members = append(members, uid)
		}
		pipe.SAdd(ctx, usersKey, members...)
	}
	pipe.Set(ctx, countKey, len(likedUserIDs), 0)
	_, err := pipe.Exec(ctx)
	return err
}

func (c *interactionCache) InvalidateCollectCache(ctx context.Context, articleID int64) error {
	usersKey := fmt.Sprintf("collect:users:%d", articleID)
	countKey := fmt.Sprintf("collect:count:%d", articleID)
	return c.rdb.Del(ctx, usersKey, countKey).Err()
}

func (c *interactionCache) GetLikeCount(ctx context.Context, articleID int64) (int64, error) {
	key := fmt.Sprintf("like:count:%d", articleID)
	val, err := c.rdb.Get(ctx, key).Result()
	if err == redis.Nil {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	return strconv.ParseInt(val, 10, 64)
}

func (c *interactionCache) GetCollectCount(ctx context.Context, articleID int64) (int64, error) {
	key := fmt.Sprintf("collect:count:%d", articleID)
	val, err := c.rdb.Get(ctx, key).Result()
	if err == redis.Nil {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	return strconv.ParseInt(val, 10, 64)
}

func (c *interactionCache) BatchGetCounts(ctx context.Context, articleIDs []int64) (map[int64]InteractionCount, []int64, error) {
	res := make(map[int64]InteractionCount, len(articleIDs))
	if len(articleIDs) == 0 {
		return res, nil, nil
	}

	pipe := c.rdb.Pipeline()
	likeCmds := make(map[int64]*redis.StringCmd, len(articleIDs))
	collectCmds := make(map[int64]*redis.StringCmd, len(articleIDs))
	for _, id := range articleIDs {
		likeCmds[id] = pipe.Get(ctx, fmt.Sprintf("like:count:%d", id))
		collectCmds[id] = pipe.Get(ctx, fmt.Sprintf("collect:count:%d", id))
	}
	_, err := pipe.Exec(ctx)
	if err != nil && err != redis.Nil {
		return nil, nil, err
	}

	misses := make([]int64, 0)
	for _, id := range articleIDs {
		likeStr, likeErr := likeCmds[id].Result()
		collectStr, collectErr := collectCmds[id].Result()

		miss := false
		counts := InteractionCount{}

		if likeErr == nil {
			if n, parseErr := strconv.ParseInt(likeStr, 10, 64); parseErr == nil {
				counts.LikeCount = n
			}
		} else if likeErr == redis.Nil {
			miss = true
		}

		if collectErr == nil {
			if n, parseErr := strconv.ParseInt(collectStr, 10, 64); parseErr == nil {
				counts.CollectCount = n
			}
		} else if collectErr == redis.Nil {
			miss = true
		}

		if miss {
			misses = append(misses, id)
			continue
		}
		res[id] = counts
	}

	return res, misses, nil
}

func (c *interactionCache) BatchSetCounts(ctx context.Context, counts map[int64]InteractionCount) error {
	if len(counts) == 0 {
		return nil
	}

	pipe := c.rdb.Pipeline()
	for articleID, v := range counts {
		pipe.Set(ctx, fmt.Sprintf("like:count:%d", articleID), v.LikeCount, 10*time.Minute)
		pipe.Set(ctx, fmt.Sprintf("collect:count:%d", articleID), v.CollectCount, 10*time.Minute)
	}
	_, err := pipe.Exec(ctx)
	return err
}

func toInt64(v interface{}) int64 {
	switch val := v.(type) {
	case int64:
		return val
	case int:
		return int64(val)
	case float64:
		return int64(val)
	case string:
		n, _ := strconv.ParseInt(val, 10, 64)
		return n
	default:
		return 0
	}
}
