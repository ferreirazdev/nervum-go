package entity

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	authPkg "github.com/nervum/nervum-go/internal/features/auth"
	user "github.com/nervum/nervum-go/internal/features/users"
	"github.com/nervum/nervum-go/internal/pkg/types"
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
	g.GET("/with-health-check", h.ListWithHealthCheck) // must be before /:id
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

func (h *Handler) ListWithHealthCheck(c *gin.Context) {
	cu := currentUser(c)
	if cu == nil || cu.OrganizationID == nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
		return
	}
	orgID := *cu.OrganizationID
	var envID *uuid.UUID
	if envIDStr := c.Query("environment_id"); envIDStr != "" {
		parsed, err := uuid.Parse(envIDStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid environment_id"})
			return
		}
		envID = &parsed
	}
	list, err := h.repo.ListWithHealthCheck(c.Request.Context(), orgID, envID)
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
	var body map[string]interface{}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	// Apply only allowed fields (partial update)
	allowed := map[string]bool{
		"type": true, "name": true, "status": true, "owner_team_id": true, "metadata": true,
		"health_check_url": true, "health_check_method": true, "health_check_headers": true, "health_check_expected_status": true,
	}
	for k, v := range body {
		if !allowed[k] {
			continue
		}
		switch k {
		case "type":
			if s, ok := v.(string); ok {
				existing.Type = s
			}
		case "name":
			if s, ok := v.(string); ok {
				existing.Name = s
			}
		case "status":
			if s, ok := v.(string); ok {
				existing.Status = s
			}
		case "owner_team_id":
			if v == nil {
				existing.OwnerTeamID = nil
			} else if s, ok := v.(string); ok && s != "" {
				if parsed, err := uuid.Parse(s); err == nil {
					existing.OwnerTeamID = &parsed
				}
			}
		case "metadata":
			if v == nil {
				existing.Metadata = nil
			} else if m, ok := v.(map[string]interface{}); ok {
				existing.Metadata = types.JSONB(m)
			}
		case "health_check_url":
			if s, ok := v.(string); ok {
				existing.HealthCheckURL = s
			}
		case "health_check_method":
			if s, ok := v.(string); ok {
				existing.HealthCheckMethod = s
			}
		case "health_check_headers":
			if v == nil {
				existing.HealthCheckHeaders = nil
			} else if m, ok := v.(map[string]interface{}); ok {
				existing.HealthCheckHeaders = types.JSONB(m)
			}
		case "health_check_expected_status":
			switch n := v.(type) {
			case float64:
				existing.HealthCheckExpectedStatus = int(n)
			case int:
				existing.HealthCheckExpectedStatus = n
			}
		}
	}
	if err := h.repo.Update(c.Request.Context(), existing); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	c.JSON(http.StatusOK, existing)
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
