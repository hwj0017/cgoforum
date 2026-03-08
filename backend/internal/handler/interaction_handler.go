package handler

import (
	"strconv"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"cgoforum/internal/handler/middleware"
	"cgoforum/internal/service"
	"cgoforum/pkg/result"
)

type InteractionHandler struct {
	svc    service.InteractionService
	logger *zap.Logger
}

func NewInteractionHandler(svc service.InteractionService, logger *zap.Logger) *InteractionHandler {
	return &InteractionHandler{svc: svc, logger: logger}
}

func (h *InteractionHandler) Like(c *gin.Context) {
	articleID, ok := parseInt64(c, "id")
	if !ok {
		return
	}
	userID := middleware.GetUserID(c)

	count, err := h.svc.Like(c.Request.Context(), userID, articleID)
	if err != nil {
		if err == service.ErrArticleNotFound {
			result.NotFound(c, "article not found")
			return
		}
		h.logger.Error("like failed", zap.Error(err))
		result.InternalError(c, "like failed")
		return
	}

	result.Success(c, gin.H{
		"liked": true,
		"count": count,
	})
}

func (h *InteractionHandler) Unlike(c *gin.Context) {
	articleID, ok := parseInt64(c, "id")
	if !ok {
		return
	}
	userID := middleware.GetUserID(c)

	count, err := h.svc.Unlike(c.Request.Context(), userID, articleID)
	if err != nil {
		if err == service.ErrArticleNotFound {
			result.NotFound(c, "article not found")
			return
		}
		h.logger.Error("unlike failed", zap.Error(err))
		result.InternalError(c, "unlike failed")
		return
	}

	result.Success(c, gin.H{
		"liked": false,
		"count": count,
	})
}

func (h *InteractionHandler) Collect(c *gin.Context) {
	articleID, ok := parseInt64(c, "id")
	if !ok {
		return
	}
	userID := middleware.GetUserID(c)

	count, err := h.svc.Collect(c.Request.Context(), userID, articleID)
	if err != nil {
		h.logger.Error("collect failed", zap.Error(err))
		result.InternalError(c, "collect failed")
		return
	}

	result.Success(c, gin.H{
		"collected": true,
		"count":     count,
	})
}

func (h *InteractionHandler) Uncollect(c *gin.Context) {
	articleID, ok := parseInt64(c, "id")
	if !ok {
		return
	}
	userID := middleware.GetUserID(c)

	count, err := h.svc.Uncollect(c.Request.Context(), userID, articleID)
	if err != nil {
		h.logger.Error("uncollect failed", zap.Error(err))
		result.InternalError(c, "uncollect failed")
		return
	}

	result.Success(c, gin.H{
		"collected": false,
		"count":     count,
	})
}

func (h *InteractionHandler) Follow(c *gin.Context) {
	authorID, ok := parseInt64(c, "authorId")
	if !ok {
		return
	}
	userID := middleware.GetUserID(c)

	if err := h.svc.Follow(c.Request.Context(), userID, authorID); err != nil {
		if err == service.ErrBadActor {
			result.BadRequest(c, "invalid author")
			return
		}
		h.logger.Error("follow failed", zap.Error(err))
		result.InternalError(c, "follow failed")
		return
	}

	result.Success(c, nil)
}

func (h *InteractionHandler) Unfollow(c *gin.Context) {
	authorID, ok := parseInt64(c, "authorId")
	if !ok {
		return
	}
	userID := middleware.GetUserID(c)

	if err := h.svc.Unfollow(c.Request.Context(), userID, authorID); err != nil {
		if err == service.ErrBadActor {
			result.BadRequest(c, "invalid author")
			return
		}
		h.logger.Error("unfollow failed", zap.Error(err))
		result.InternalError(c, "unfollow failed")
		return
	}

	result.Success(c, nil)
}

func (h *InteractionHandler) FeedFollowing(c *gin.Context) {
	userID := middleware.GetUserID(c)
	cursor := c.Query("cursor")
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))

	items, nextCursor, hasMore, err := h.svc.ListFollowingFeed(c.Request.Context(), userID, cursor, limit)
	if err != nil {
		h.logger.Error("list following feed failed", zap.Error(err))
		result.InternalError(c, "list following feed failed")
		return
	}

	result.SuccessPage(c, items, nextCursor, hasMore)
}

func (h *InteractionHandler) RegisterRoutes(rg *gin.RouterGroup, authMiddleware gin.HandlerFunc) {
	auth := rg.Group("", authMiddleware)
	auth.POST("/articles/:id/like", h.Like)
	auth.DELETE("/articles/:id/like", h.Unlike)
	auth.POST("/articles/:id/collect", h.Collect)
	auth.DELETE("/articles/:id/collect", h.Uncollect)
	auth.POST("/follow/:authorId", h.Follow)
	auth.DELETE("/follow/:authorId", h.Unfollow)
	auth.GET("/feed/following", h.FeedFollowing)
}
