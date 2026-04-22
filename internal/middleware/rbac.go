package middleware

import (
	"security-service/internal/rbac"
	"security-service/pkg/response"

	"github.com/gin-gonic/gin"
)

func RequirePermission(rbacService *rbac.Service, resource, action string) gin.HandlerFunc {
	return func(c *gin.Context) {
		role, exists := c.Get("role")
		if !exists {
			response.Forbidden(c, "no role found")
			c.Abort()
			return
		}

		allowed, err := rbacService.HasPermission(c.Request.Context(), role.(string), resource, action)
		if err != nil || !allowed {
			response.Forbidden(c, "insufficient permissions")
			c.Abort()
			return
		}

		c.Next()
	}
}
