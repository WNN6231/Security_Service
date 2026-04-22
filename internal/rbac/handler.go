package rbac

import (
	"net/http"
	"strings"

	"security-service/internal/validator"
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
	roles := rg.Group("/roles")
	{
		roles.POST("", h.CreateRole)
		roles.GET("", h.ListRoles)
	}
	rg.POST("/users/:id/roles", h.AssignRole)
}

// ---------- DTOs ----------

type CreateRoleRequest struct {
	Name        string   `json:"name" binding:"required"`
	Description string   `json:"description"`
	Permissions []string `json:"permissions"`
}

type AssignRoleRequest struct {
	RoleID string `json:"role_id" binding:"required,uuid"`
}

// ---------- Handlers ----------

func (h *Handler) CreateRole(c *gin.Context) {
	var req CreateRoleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request parameters")
		return
	}

	req.Name = validator.SanitizeString(req.Name)
	req.Description = validator.SanitizeString(req.Description)
	for i := range req.Permissions {
		req.Permissions[i] = strings.TrimSpace(req.Permissions[i])
	}

	if validator.IsBlank(req.Name) || len(req.Name) < 2 || len(req.Name) > 50 {
		response.BadRequest(c, "role name must be 2-50 characters")
		return
	}

	role, err := h.service.CreateRole(c.Request.Context(), req.Name, req.Description, req.Permissions)
	if err != nil {
		response.Error(c, http.StatusConflict, "role already exists or invalid permissions")
		return
	}

	response.Created(c, role)
}

func (h *Handler) ListRoles(c *gin.Context) {
	roles, err := h.service.ListRoles(c.Request.Context())
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "internal server error")
		return
	}

	response.OK(c, roles)
}

func (h *Handler) AssignRole(c *gin.Context) {
	userID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "invalid user id")
		return
	}

	var req AssignRoleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request parameters")
		return
	}

	roleID, _ := uuid.Parse(req.RoleID)

	if err := h.service.AssignRoleToUser(c.Request.Context(), userID, roleID); err != nil {
		response.BadRequest(c, "role not found or already assigned")
		return
	}

	response.OK(c, gin.H{"message": "role assigned"})
}
