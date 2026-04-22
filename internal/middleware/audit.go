package middleware

import (
	"strings"

	"security-service/internal/audit"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func AuditLog(auditService *audit.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Process request first
		c.Next()

		// Record after response is written
		go func() {
			action := c.Request.Method + " " + c.Request.URL.Path
			status := c.Writer.Status()

			var userID *uuid.UUID
			if uidStr, exists := c.Get("user_id"); exists {
				if parsed, err := uuid.Parse(uidStr.(string)); err == nil {
					userID = &parsed
				}
			}

			riskLevel := "LOW"
			isLoginPath := strings.HasSuffix(c.Request.URL.Path, "/auth/login")
			if status == 403 || (status == 401 && c.Request.Method == "POST" && isLoginPath) {
				riskLevel = "HIGH"
			}

			entry := &audit.Log{
				UserID:    userID,
				Action:    action,
				IP:        clientIP(c),
				UserAgent: c.Request.UserAgent(),
				Status:    status,
				RiskLevel: riskLevel,
			}
			// Best-effort; password fields are never logged because
			// we only record method, path, IP, status — no request body.
			_ = auditService.Record(c.Request.Context(), entry)
		}()
	}
}

// clientIP extracts the real client IP, preferring X-Forwarded-For.
func clientIP(c *gin.Context) string {
	if xff := c.GetHeader("X-Forwarded-For"); xff != "" {
		if parts := strings.SplitN(xff, ",", 2); len(parts) > 0 {
			return strings.TrimSpace(parts[0])
		}
	}
	return c.ClientIP()
}
