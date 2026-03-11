package user

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

const contextUserKey = "auth_user"

// Handler serves HTTP CRUD for users (create, list, get, update, delete).
type Handler struct {
	repo Repository
}

// NewHandler returns a user Handler using the given repository.
func NewHandler(repo Repository) *Handler {
	return &Handler{repo: repo}
}

func (h *Handler) Register(r *gin.RouterGroup) {
	g := r.Group("/users")
	g.POST("", h.Create)
	g.GET("", h.List)
	g.GET("/:id", h.GetByID)
	g.PUT("/:id", h.Update)
	g.DELETE("/:id", h.Delete)
}

func (h *Handler) Create(c *gin.Context) {
	var req User
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := h.repo.Create(c.Request.Context(), &req); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, req)
}

func (h *Handler) GetByID(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	u, err := h.repo.GetByID(c.Request.Context(), id)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, u)
}

func (h *Handler) List(c *gin.Context) {
	orgIDStr := c.Query("organization_id")
	if orgIDStr != "" {
		orgID, err := uuid.Parse(orgIDStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid organization_id"})
			return
		}
		cu, ok := c.Get(contextUserKey)
		if !ok {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
			return
		}
		currentUser := cu.(*User)
		if currentUser.OrganizationID == nil || *currentUser.OrganizationID != orgID {
			c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
			return
		}
		if !CanListOrgMembers(currentUser.Role) {
			c.JSON(http.StatusForbidden, gin.H{"error": "cannot list organization members"})
			return
		}
		list, err := h.repo.ListByOrganization(c.Request.Context(), orgID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, list)
		return
	}
	list, err := h.repo.List(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, list)
}

func (h *Handler) Update(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	cu, ok := c.Get(contextUserKey)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
		return
	}
	currentUser := cu.(*User)
	u, err := h.repo.GetByID(c.Request.Context(), id)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	var req struct {
		Name                  *string    `json:"name"`
		OrganizationID        *uuid.UUID `json:"organization_id"`
		Role                  *string    `json:"role"`
		OnboardingCompleted   *bool      `json:"onboarding"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.Name != nil {
		u.Name = *req.Name
	}
	if req.OrganizationID != nil {
		u.OrganizationID = req.OrganizationID
	}
	if req.Role != nil {
		if currentUser.Role != RoleAdmin {
			c.JSON(http.StatusForbidden, gin.H{"error": "only admin can change role"})
			return
		}
		if *req.Role != RoleAdmin && *req.Role != RoleManager && *req.Role != RoleMember {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid role"})
			return
		}
		u.Role = *req.Role
	}
	if req.OnboardingCompleted != nil && *req.OnboardingCompleted {
		if id != currentUser.ID {
			c.JSON(http.StatusForbidden, gin.H{"error": "can only set own onboarding"})
			return
		}
		u.OnboardingCompleted = true
	}
	if err := h.repo.Update(c.Request.Context(), u); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, u)
}

func (h *Handler) Delete(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	if err := h.repo.Delete(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.Status(http.StatusNoContent)
}
