package middleware

import (
	"security-service/internal/rbac"
	"security-service/pkg/response"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func RequirePermission(rbacService *rbac.Service, permCode string) gin.HandlerFunc {
	return func(c *gin.Context) {
		userIDStr, exists := c.Get("user_id")
		if !exists {
			response.Forbidden(c, "access denied")
			c.Abort()
			return
		}

		userID, err := uuid.Parse(userIDStr.(string))
		if err != nil {
			response.Forbidden(c, "access denied")
			c.Abort()
			return
		}

		allowed, err := rbacService.CheckPermission(c.Request.Context(), userID, permCode)
		if err != nil || !allowed {
			response.Forbidden(c, "access denied")
			c.Abort()
			return
		}

		c.Next()
	}
}
