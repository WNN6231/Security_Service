package user

import (
	"net/http"

	"security-service/pkg/response"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type Handler struct {
	repo Repository
}

func NewHandler(repo Repository) *Handler {
	return &Handler{repo: repo}
}

func (h *Handler) RegisterRoutes(rg *gin.RouterGroup) {
	users := rg.Group("/users")
	{
		users.GET("/:id", h.GetUser)
		users.GET("", h.ListUsers)
		users.PUT("/:id", h.UpdateUser)
		users.DELETE("/:id", h.DeleteUser)
	}
}

func (h *Handler) GetUser(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "invalid user id")
		return
	}

	u, err := h.repo.FindByID(c.Request.Context(), id)
	if err != nil {
		response.NotFound(c, "user not found")
		return
	}

	response.OK(c, u)
}

func (h *Handler) ListUsers(c *gin.Context) {
	users, total, err := h.repo.List(c.Request.Context(), 0, 20)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "internal server error")
		return
	}

	response.OK(c, gin.H{"users": users, "total": total})
}

func (h *Handler) UpdateUser(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "invalid user id")
		return
	}

	u, err := h.repo.FindByID(c.Request.Context(), id)
	if err != nil {
		response.NotFound(c, "user not found")
		return
	}

	if err := c.ShouldBindJSON(u); err != nil {
		response.BadRequest(c, "invalid request parameters")
		return
	}

	if err := h.repo.Update(c.Request.Context(), u); err != nil {
		response.Error(c, http.StatusInternalServerError, "internal server error")
		return
	}

	response.OK(c, u)
}

func (h *Handler) DeleteUser(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "invalid user id")
		return
	}

	if err := h.repo.Delete(c.Request.Context(), id); err != nil {
		response.Error(c, http.StatusInternalServerError, "internal server error")
		return
	}

	response.OK(c, gin.H{"message": "user deleted"})
}
