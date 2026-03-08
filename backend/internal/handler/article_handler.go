package handler

import (
	"strconv"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"cgoforum/internal/handler/middleware"
	"cgoforum/internal/service"
	"cgoforum/pkg/result"
)

type ArticleHandler struct {
	articleSvc service.ArticleService
	logger     *zap.Logger
}

func NewArticleHandler(articleSvc service.ArticleService, logger *zap.Logger) *ArticleHandler {
	return &ArticleHandler{
		articleSvc: articleSvc,
		logger:     logger,
	}
}

// --- Request structs ---

type CreateArticleReq struct {
	Title     string `json:"title" binding:"required,min=1,max=200"`
	Summary   string `json:"summary" binding:"max=500"`
	ContentMD string `json:"content_md" binding:"required"`
	CoverImg  string `json:"cover_img" binding:"max=500"`
	Status    int16  `json:"status"` // 0:draft, 1:published
}

type UpdateArticleReq struct {
	Title     string `json:"title" binding:"min=1,max=200"`
	Summary   string `json:"summary" binding:"max=500"`
	ContentMD string `json:"content_md" binding:"required"`
	CoverImg  string `json:"cover_img" binding:"max=500"`
	Status    int16  `json:"status"`
}

// --- Handlers ---

func (h *ArticleHandler) Create(c *gin.Context) {
	var req CreateArticleReq
	if err := c.ShouldBindJSON(&req); err != nil {
		result.BadRequest(c, err.Error())
		return
	}

	userID := middleware.GetUserID(c)
	if req.Status != 0 && req.Status != 1 {
		req.Status = 1 // default to published
	}

	article, err := h.articleSvc.Create(
		c.Request.Context(),
		userID,
		req.Title,
		req.Summary,
		req.ContentMD,
		req.CoverImg,
		req.Status,
	)
	if err != nil {
		h.logger.Error("create article failed", zap.Error(err))
		result.InternalError(c, "create article failed")
		return
	}

	result.Success(c, gin.H{
		"id":     strconv.FormatInt(article.ID, 10),
		"status": article.Status,
	})
}

func (h *ArticleHandler) GetByID(c *gin.Context) {
	id, ok := parseInt64(c, "id")
	if !ok {
		return
	}

	article, err := h.articleSvc.GetByID(c.Request.Context(), id)
	if err == service.ErrArticleNotFound {
		result.NotFound(c, "article not found")
		return
	}
	if err != nil {
		h.logger.Error("get article failed", zap.Error(err))
		result.InternalError(c, "get article failed")
		return
	}

	result.Success(c, article)
}

func (h *ArticleHandler) Update(c *gin.Context) {
	id, ok := parseInt64(c, "id")
	if !ok {
		return
	}

	var req UpdateArticleReq
	if err := c.ShouldBindJSON(&req); err != nil {
		result.BadRequest(c, err.Error())
		return
	}

	userID := middleware.GetUserID(c)
	if err := h.articleSvc.Update(
		c.Request.Context(),
		userID,
		id,
		req.Title,
		req.Summary,
		req.ContentMD,
		req.CoverImg,
		req.Status,
	); err == service.ErrArticleNotFound {
		result.NotFound(c, "article not found")
		return
	} else if err == service.ErrArticleForbidden {
		result.Forbidden(c, "no permission")
		return
	} else if err != nil {
		h.logger.Error("update article failed", zap.Error(err))
		result.InternalError(c, "update article failed")
		return
	}

	result.Success(c, nil)
}

func (h *ArticleHandler) Delete(c *gin.Context) {
	id, ok := parseInt64(c, "id")
	if !ok {
		return
	}

	userID := middleware.GetUserID(c)
	role := middleware.GetRole(c)

	if err := h.articleSvc.Delete(c.Request.Context(), userID, id, role); err == service.ErrArticleNotFound {
		result.NotFound(c, "article not found")
		return
	} else if err == service.ErrArticleForbidden {
		result.Forbidden(c, "no permission")
		return
	} else if err != nil {
		h.logger.Error("delete article failed", zap.Error(err))
		result.InternalError(c, "delete article failed")
		return
	}

	result.Success(c, nil)
}

func (h *ArticleHandler) List(c *gin.Context) {
	cursor := c.Query("cursor")
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))

	articles, nextCursor, hasMore, err := h.articleSvc.List(c.Request.Context(), cursor, limit)
	if err != nil {
		h.logger.Error("list articles failed", zap.Error(err))
		result.InternalError(c, "list articles failed")
		return
	}

	result.SuccessPage(c, articles, nextCursor, hasMore)
}

func (h *ArticleHandler) ListByAuthor(c *gin.Context) {
	uid, ok := parseInt64(c, "uid")
	if !ok {
		return
	}

	cursor := c.Query("cursor")
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))

	articles, nextCursor, hasMore, err := h.articleSvc.ListByAuthor(c.Request.Context(), uid, cursor, limit)
	if err != nil {
		h.logger.Error("list author articles failed", zap.Error(err))
		result.InternalError(c, "list author articles failed")
		return
	}

	result.SuccessPage(c, articles, nextCursor, hasMore)
}

// RegisterRoutes registers article routes.
func (h *ArticleHandler) RegisterRoutes(rg *gin.RouterGroup, authMiddleware gin.HandlerFunc) {
	articles := rg.Group("/articles")

	// Public routes
	articles.GET("", h.List)
	articles.GET("/:id", h.GetByID)

	// Authenticated routes
	authArticles := articles.Group("", authMiddleware)
	authArticles.POST("", h.Create)
	authArticles.PUT("/:id", h.Update)
	authArticles.DELETE("/:id", h.Delete)

	// Author's articles (under users group)
	rg.GET("/users/:uid/articles", h.ListByAuthor)
}
