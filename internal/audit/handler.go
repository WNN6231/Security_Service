package audit

import (
	"net/http"
	"strconv"
	"time"

	"security-service/pkg/response"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) RegisterRoutes(rg *gin.RouterGroup) {
	rg.GET("/audit-logs", h.ListAuditLogs)
}

func (h *Handler) ListAuditLogs(c *gin.Context) {
	var filter ListFilter

	if uid := c.Query("user_id"); uid != "" {
		parsed, err := uuid.Parse(uid)
		if err != nil {
			response.BadRequest(c, "invalid user_id")
			return
		}
		filter.UserID = &parsed
	}

	filter.RiskLevel = c.Query("risk_level")

	if st := c.Query("start_time"); st != "" {
		t, err := time.Parse(time.RFC3339, st)
		if err != nil {
			response.BadRequest(c, "invalid start_time, use RFC3339 format")
			return
		}
		filter.StartTime = &t
	}

	if et := c.Query("end_time"); et != "" {
		t, err := time.Parse(time.RFC3339, et)
		if err != nil {
			response.BadRequest(c, "invalid end_time, use RFC3339 format")
			return
		}
		filter.EndTime = &t
	}

	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	if limit <= 0 || limit > 100 {
		limit = 20
	}

	logs, total, err := h.service.ListWithFilter(c.Request.Context(), filter, offset, limit)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "failed to query audit logs")
		return
	}

	response.OK(c, gin.H{"logs": logs, "total": total})
}
