package middleware

import (
	"github.com/gin-gonic/gin"
	"cgoforum/pkg/result"
)

// RequireRole checks if the authenticated user has one of the required roles.
func RequireRole(roles ...int16) gin.HandlerFunc {
	roleSet := make(map[int16]bool, len(roles))
	for _, r := range roles {
		roleSet[r] = true
	}
	return func(c *gin.Context) {
		userRole := GetRole(c)
		if !roleSet[userRole] {
			result.Forbidden(c, "insufficient permissions")
			c.Abort()
			return
		}
		c.Next()
	}
}

// IsAdmin checks if the user is admin (role 1) or super admin (role 2).
func IsAdmin() gin.HandlerFunc {
	return RequireRole(1, 2)
}
