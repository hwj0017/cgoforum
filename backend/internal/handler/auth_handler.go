package handler

import (
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"cgoforum/internal/handler/middleware"
	"cgoforum/internal/service"
	jwtpkg "cgoforum/pkg/jwt"
	"cgoforum/pkg/result"
)

type AuthHandler struct {
	authSvc    service.AuthService
	jwtHandler *jwtpkg.Handler
	logger     *zap.Logger
}

func NewAuthHandler(authSvc service.AuthService, jwtHandler *jwtpkg.Handler, logger *zap.Logger) *AuthHandler {
	return &AuthHandler{
		authSvc:    authSvc,
		jwtHandler: jwtHandler,
		logger:     logger,
	}
}

// --- Request structs ---

type RegisterReq struct {
	Username string `json:"username" binding:"required,min=3,max=50"`
	Password string `json:"password" binding:"required,min=6,max=128"`
	Nickname string `json:"nickname" binding:"required,min=1,max=50"`
}

type LoginReq struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

type RefreshReq struct {
	// Refresh token is read from cookie
}

type BanReq struct {
	UserID   int64  `json:"user_id" binding:"required"`
	Reason   string `json:"reason" binding:"required"`
	Duration int64  `json:"duration"` // Duration in seconds, 0 for permanent
}

type UnbanReq struct {
	UserID int64 `json:"user_id" binding:"required"`
}

// --- Handlers ---

func (h *AuthHandler) Register(c *gin.Context) {
	var req RegisterReq
	if err := c.ShouldBindJSON(&req); err != nil {
		result.BadRequest(c, err.Error())
		return
	}

	user, err := h.authSvc.Register(c.Request.Context(), req.Username, req.Password, req.Nickname)
	if err == service.ErrUsernameExists {
		result.BadRequest(c, "username already exists")
		return
	}
	if err != nil {
		h.logger.Error("register failed", zap.Error(err))
		result.InternalError(c, "register failed")
		return
	}

	result.Success(c, gin.H{
		"id":       strconv.FormatInt(user.ID, 10),
		"username": user.Username,
		"nickname": user.Nickname,
	})
}

func (h *AuthHandler) Login(c *gin.Context) {
	var req LoginReq
	if err := c.ShouldBindJSON(&req); err != nil {
		result.BadRequest(c, err.Error())
		return
	}

	accessToken, refreshToken, err := h.authSvc.Login(c.Request.Context(), req.Username, req.Password)
	if err == service.ErrUserNotFound || err == service.ErrInvalidPassword {
		result.Unauthorized(c, "invalid username or password")
		return
	}
	if err == service.ErrUserBanned {
		result.Forbidden(c, "account is banned")
		return
	}
	if err != nil {
		h.logger.Error("login failed", zap.Error(err))
		result.InternalError(c, "login failed")
		return
	}

	// Set refresh token in HttpOnly Secure Cookie
	c.SetCookie("refresh_token", refreshToken, 7*24*3600, "/", "", false, true)

	result.Success(c, gin.H{
		"access_token": accessToken,
	})
}

func (h *AuthHandler) Refresh(c *gin.Context) {
	refreshToken, err := c.Cookie("refresh_token")
	if err != nil || refreshToken == "" {
		result.Unauthorized(c, "missing refresh token")
		return
	}

	newAccess, newRefresh, err := h.authSvc.RefreshToken(c.Request.Context(), refreshToken)
	if err != nil {
		result.Unauthorized(c, err.Error())
		return
	}

	// Update refresh token cookie
	c.SetCookie("refresh_token", newRefresh, 7*24*3600, "/", "", false, true)

	result.Success(c, gin.H{
		"access_token": newAccess,
	})
}

func (h *AuthHandler) Logout(c *gin.Context) {
	userID := middleware.GetUserID(c)

	// Try to extract jti from refresh token cookie for blacklisting
	refreshToken, _ := c.Cookie("refresh_token")
	jti := ""
	if refreshToken != "" {
		claims, err := h.jwtHandler.ParseRefreshToken(refreshToken)
		if err == nil {
			jti = claims.JTI
		}
	}

	accessTTL := 30 * time.Minute
	if err := h.authSvc.Logout(c.Request.Context(), userID, jti, accessTTL); err != nil {
		h.logger.Error("logout failed", zap.Error(err))
		result.InternalError(c, "logout failed")
		return
	}

	// Clear cookie
	c.SetCookie("refresh_token", "", -1, "/", "", false, true)

	result.Success(c, nil)
}

func (h *AuthHandler) Ban(c *gin.Context) {
	var req BanReq
	if err := c.ShouldBindJSON(&req); err != nil {
		result.BadRequest(c, err.Error())
		return
	}

	adminID := middleware.GetUserID(c)
	duration := time.Duration(req.Duration) * time.Second
	if req.Duration == 0 {
		duration = 0 // permanent
	}

	if err := h.authSvc.BanUser(c.Request.Context(), adminID, req.UserID, req.Reason, duration); err != nil {
		h.logger.Error("ban user failed", zap.Error(err))
		result.InternalError(c, "ban user failed")
		return
	}

	result.Success(c, nil)
}

func (h *AuthHandler) Unban(c *gin.Context) {
	var req UnbanReq
	if err := c.ShouldBindJSON(&req); err != nil {
		result.BadRequest(c, err.Error())
		return
	}

	adminID := middleware.GetUserID(c)
	if err := h.authSvc.UnbanUser(c.Request.Context(), adminID, req.UserID); err != nil {
		h.logger.Error("unban user failed", zap.Error(err))
		result.InternalError(c, "unban user failed")
		return
	}

	result.Success(c, nil)
}

// RegisterRoutes registers auth routes on the given router group.
func (h *AuthHandler) RegisterRoutes(rg *gin.RouterGroup, authMiddleware gin.HandlerFunc, adminMiddleware gin.HandlerFunc) {
	auth := rg.Group("/auth")

	// Public routes
	auth.POST("/register", h.Register)
	auth.POST("/login", h.Login)

	// Authenticated routes
	auth.POST("/refresh", h.Refresh)
	auth.POST("/logout", authMiddleware, h.Logout)

	// Admin routes
	admin := auth.Group("", adminMiddleware)
	admin.POST("/ban", h.Ban)
	admin.POST("/unban", h.Unban)
}

// parseInt64 is a helper to parse path parameter as int64.
func parseInt64(c *gin.Context, key string) (int64, bool) {
	s := c.Param(key)
	v, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		result.BadRequest(c, "invalid "+key)
		return 0, false
	}
	return v, true
}
