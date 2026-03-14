package entity

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	authPkg "github.com/nervum/nervum-go/internal/features/auth"
	user "github.com/nervum/nervum-go/internal/features/users"
	"gorm.io/gorm"
)

// Handler serves HTTP CRUD for entities (map nodes).
type Handler struct {
	repo Repository
}

// NewHandler returns an entity Handler using the given repository.
func NewHandler(repo Repository) *Handler {
	return &Handler{repo: repo}
}

func (h *Handler) Register(r *gin.RouterGroup) {
	g := r.Group("/entities")
	g.POST("", h.Create)
	g.GET("", h.List)
	g.GET("/:id", h.GetByID)
	g.PUT("/:id", h.Update)
	g.DELETE("/:id", h.Delete)
}

// currentUser extracts the authenticated user from Gin context (set by RequireAuth middleware).
func currentUser(c *gin.Context) *user.User {
	val, _ := c.Get(authPkg.ContextUser)
	u, _ := val.(*user.User)
	return u
}

func (h *Handler) Create(c *gin.Context) {
	cu := currentUser(c)
	if cu == nil || cu.OrganizationID == nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
		return
	}
	var req Entity
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	// Enforce organization from authenticated user — reject any client-supplied org.
	req.OrganizationID = *cu.OrganizationID
	if err := h.repo.Create(c.Request.Context(), &req); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	c.JSON(http.StatusCreated, req)
}

func (h *Handler) GetByID(c *gin.Context) {
	cu := currentUser(c)
	if cu == nil || cu.OrganizationID == nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
		return
	}
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	e, err := h.repo.GetByID(c.Request.Context(), id)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	if e.OrganizationID != *cu.OrganizationID {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		return
	}
	c.JSON(http.StatusOK, e)
}

func (h *Handler) List(c *gin.Context) {
	cu := currentUser(c)
	if cu == nil || cu.OrganizationID == nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
		return
	}
	orgIDStr := c.Query("organization_id")
	if orgIDStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "organization_id required"})
		return
	}
	orgID, err := uuid.Parse(orgIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid organization_id"})
		return
	}
	// Prevent cross-org enumeration.
	if orgID != *cu.OrganizationID {
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
		return
	}
	var envID *uuid.UUID
	if envIDStr := c.Query("environment_id"); envIDStr != "" {
		parsed, err := uuid.Parse(envIDStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid environment_id"})
			return
		}
		envID = &parsed
	}
	list, err := h.repo.ListByOrganization(c.Request.Context(), orgID, envID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	c.JSON(http.StatusOK, list)
}

func (h *Handler) Update(c *gin.Context) {
	cu := currentUser(c)
	if cu == nil || cu.OrganizationID == nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
		return
	}
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	// Verify ownership before applying the update.
	existing, err := h.repo.GetByID(c.Request.Context(), id)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	if existing.OrganizationID != *cu.OrganizationID {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		return
	}
	var req Entity
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	req.ID = id
	req.OrganizationID = *cu.OrganizationID
	if err := h.repo.Update(c.Request.Context(), &req); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	c.JSON(http.StatusOK, req)
}

func (h *Handler) Delete(c *gin.Context) {
	cu := currentUser(c)
	if cu == nil || cu.OrganizationID == nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
		return
	}
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	// Verify ownership before deleting.
	existing, err := h.repo.GetByID(c.Request.Context(), id)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	if existing.OrganizationID != *cu.OrganizationID {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		return
	}
	if err := h.repo.Delete(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	c.Status(http.StatusNoContent)
}
