package job

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

type ActivityWindowCleanupJob struct {
	rdb    *redis.Client
	logger *zap.Logger
}

func NewActivityWindowCleanupJob(rdb *redis.Client, logger *zap.Logger) *ActivityWindowCleanupJob {
	return &ActivityWindowCleanupJob{rdb: rdb, logger: logger}
}

func (j *ActivityWindowCleanupJob) Start(ctx context.Context) {
	j.run(ctx)
	ticker := time.NewTicker(10 * time.Minute)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				j.run(ctx)
			}
		}
	}()
}

func (j *ActivityWindowCleanupJob) run(ctx context.Context) {
	now := time.Now().Unix()
	windows := []struct {
		suffix    string
		retention int64
	}{
		{suffix: "24h", retention: 24 * 3600},
		{suffix: "7d", retention: 7 * 24 * 3600},
	}

	for _, w := range windows {
		key := fmt.Sprintf("act:articles:window:%s", w.suffix)
		cutoff := now - w.retention
		if err := j.rdb.ZRemRangeByScore(ctx, key, "-inf", fmt.Sprintf("%d", cutoff)).Err(); err != nil {
			j.logger.Warn("cleanup activity window failed", zap.String("key", key), zap.Error(err))
		}
	}
}
