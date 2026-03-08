package cache

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

type AuthCache interface {
	SetTokenBlacklist(ctx context.Context, jti string, ttl time.Duration) error
	IsTokenBlacklisted(ctx context.Context, jti string) (bool, error)
	SetUserBan(ctx context.Context, userID int64, reason string, ttl time.Duration) error
	GetUserBan(ctx context.Context, userID int64) (string, error)
	RemoveUserBan(ctx context.Context, userID int64) error
}

type authCache struct {
	rdb *redis.Client
}

func NewAuthCache(rdb *redis.Client) AuthCache {
	return &authCache{rdb: rdb}
}

func (c *authCache) SetTokenBlacklist(ctx context.Context, jti string, ttl time.Duration) error {
	key := fmt.Sprintf("auth:token:blacklist:%s", jti)
	return c.rdb.Set(ctx, key, 1, ttl).Err()
}

func (c *authCache) IsTokenBlacklisted(ctx context.Context, jti string) (bool, error) {
	key := fmt.Sprintf("auth:token:blacklist:%s", jti)
	val, err := c.rdb.Exists(ctx, key).Result()
	if err != nil {
		return false, err
	}
	return val > 0, nil
}

func (c *authCache) SetUserBan(ctx context.Context, userID int64, reason string, ttl time.Duration) error {
	key := fmt.Sprintf("auth:user:ban:%d", userID)
	// Format: reason|expire_timestamp
	expireTS := time.Now().Add(ttl).Unix()
	val := fmt.Sprintf("%s|%d", reason, expireTS)
	if ttl == 0 {
		// Permanent ban - use a very long TTL (10 years)
		val = fmt.Sprintf("%s|0", reason)
		ttl = 10 * 365 * 24 * time.Hour
	}
	return c.rdb.Set(ctx, key, val, ttl).Err()
}

func (c *authCache) GetUserBan(ctx context.Context, userID int64) (string, error) {
	key := fmt.Sprintf("auth:user:ban:%d", userID)
	val, err := c.rdb.Get(ctx, key).Result()
	if err == redis.Nil {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	// Check if ban has expired (for permanent bans stored with expire_ts=0)
	// For TTL-based bans, Redis handles expiration
	parts := splitFirst(val, '|')
	if len(parts) == 2 && parts[1] != "0" {
		expireTS, _ := strconv.ParseInt(parts[1], 10, 64)
		if time.Now().Unix() > expireTS {
			_ = c.RemoveUserBan(ctx, userID)
			return "", nil
		}
	}
	return val, nil
}

func (c *authCache) RemoveUserBan(ctx context.Context, userID int64) error {
	key := fmt.Sprintf("auth:user:ban:%d", userID)
	return c.rdb.Del(ctx, key).Err()
}

func splitFirst(s string, sep rune) []string {
	for i, c := range s {
		if c == sep {
			return []string{s[:i], s[i+1:]}
		}
	}
	return []string{s}
}
