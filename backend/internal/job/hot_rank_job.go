package job

import (
	"context"
	"time"

	"go.uber.org/zap"

	"cgoforum/internal/service"
)

type HotRankJob struct {
	rankSvc service.RankService
	logger  *zap.Logger
}

func NewHotRankJob(rankSvc service.RankService, logger *zap.Logger) *HotRankJob {
	return &HotRankJob{rankSvc: rankSvc, logger: logger}
}

func (j *HotRankJob) Start(ctx context.Context) {
	j.run24h(ctx)
	j.run7d(ctx)

	ticker24h := time.NewTicker(10 * time.Minute)
	ticker7d := time.NewTicker(1 * time.Hour)

	go func() {
		defer ticker24h.Stop()
		defer ticker7d.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker24h.C:
				j.run24h(ctx)
			case <-ticker7d.C:
				j.run7d(ctx)
			}
		}
	}()
}

func (j *HotRankJob) run24h(ctx context.Context) {
	if err := j.rankSvc.Rebuild24h(ctx); err != nil {
		j.logger.Warn("rebuild 24h hot rank failed", zap.Error(err))
	}
}

func (j *HotRankJob) run7d(ctx context.Context) {
	if err := j.rankSvc.Rebuild7d(ctx); err != nil {
		j.logger.Warn("rebuild 7d hot rank failed", zap.Error(err))
	}
}
