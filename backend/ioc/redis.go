package ioc

import (
	"context"

	"cgoforum/config"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

func InitRedis(cfg *config.RedisConfig, logger *zap.Logger) *redis.Client {
	rdb := redis.NewClient(&redis.Options{
		Addr:     cfg.Addr,
		Password: cfg.Password,
		DB:       cfg.DB,
	})

	if err := rdb.Ping(context.Background()).Err(); err != nil {
		logger.Fatal("failed to connect redis", zap.Error(err))
	}

	logger.Info("redis initialized successfully")
	return rdb
}
