package handler

import (
	"strconv"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"cgoforum/internal/service"
	"cgoforum/pkg/result"
)

type SearchHandler struct {
	searchSvc service.SearchService
	logger    *zap.Logger
}

func NewSearchHandler(searchSvc service.SearchService, logger *zap.Logger) *SearchHandler {
	return &SearchHandler{searchSvc: searchSvc, logger: logger}
}

func (h *SearchHandler) Search(c *gin.Context) {
	query := c.Query("q")
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))

	list, err := h.searchSvc.SemanticSearch(c.Request.Context(), query, limit)
	if err == service.ErrEmptyQuery {
		result.BadRequest(c, "query is required")
		return
	}
	if err != nil {
		h.logger.Error("semantic search failed", zap.Error(err), zap.String("query", query))
		result.InternalError(c, "search failed")
		return
	}

	result.Success(c, list)
}

func (h *SearchHandler) RegisterRoutes(rg *gin.RouterGroup) {
	rg.GET("/search", h.Search)
}
