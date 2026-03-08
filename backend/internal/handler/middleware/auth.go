package middleware

import (
	"strings"

	"github.com/gin-gonic/gin"
	jwtpkg "cgoforum/pkg/jwt"
	"cgoforum/pkg/result"
)

const (
	ContextUserID = "userID"
	ContextRole   = "role"
	ContextJTI    = "jti"
)

// AuthRequired validates the access token (stateless hot path, no Redis).
func AuthRequired(jwtHandler *jwtpkg.Handler) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			result.Unauthorized(c, "missing authorization header")
			c.Abort()
			return
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || parts[0] != "Bearer" {
			result.Unauthorized(c, "invalid authorization format")
			c.Abort()
			return
		}

		claims, err := jwtHandler.ParseAccessToken(parts[1])
		if err != nil {
			result.Unauthorized(c, "invalid or expired token")
			c.Abort()
			return
		}

		c.Set(ContextUserID, claims.UserID)
		c.Set(ContextRole, claims.Role)
		c.Next()
	}
}

// OptionalAuth extracts user info if token is present, but doesn't fail if absent.
func OptionalAuth(jwtHandler *jwtpkg.Handler) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.Next()
			return
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || parts[0] != "Bearer" {
			c.Next()
			return
		}

		claims, err := jwtHandler.ParseAccessToken(parts[1])
		if err != nil {
			c.Next()
			return
		}

		c.Set(ContextUserID, claims.UserID)
		c.Set(ContextRole, claims.Role)
		c.Next()
	}
}

// GetUserID extracts user ID from gin context.
func GetUserID(c *gin.Context) int64 {
	id, _ := c.Get(ContextUserID)
	if id == nil {
		return 0
	}
	return id.(int64)
}

// GetRole extracts user role from gin context.
func GetRole(c *gin.Context) int16 {
	role, _ := c.Get(ContextRole)
	if role == nil {
		return 0
	}
	return role.(int16)
}
