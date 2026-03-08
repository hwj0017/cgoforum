package handler

import (
	"strconv"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"cgoforum/internal/service"
	"cgoforum/pkg/result"
)

type RankHandler struct {
	rankSvc service.RankService
	logger  *zap.Logger
}

func NewRankHandler(rankSvc service.RankService, logger *zap.Logger) *RankHandler {
	return &RankHandler{rankSvc: rankSvc, logger: logger}
}

func (h *RankHandler) HotList(c *gin.Context) {
	window := c.DefaultQuery("window", "24h")
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))

	articles, err := h.rankSvc.ListHot(c.Request.Context(), window, limit)
	if err != nil {
		h.logger.Error("list hot rank failed", zap.Error(err))
		result.InternalError(c, "list hot rank failed")
		return
	}
	result.Success(c, gin.H{"list": articles})
}

func (h *RankHandler) RegisterRoutes(rg *gin.RouterGroup) {
	rank := rg.Group("/rank")
	rank.GET("/hot", h.HotList)
}
