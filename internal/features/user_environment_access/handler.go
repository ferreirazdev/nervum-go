package userenvironmentaccess

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	authPkg "github.com/nervum/nervum-go/internal/features/auth"
	environment "github.com/nervum/nervum-go/internal/features/environments"
	user "github.com/nervum/nervum-go/internal/features/users"
	"gorm.io/gorm"
)

// Handler serves HTTP CRUD for user-environment access records.
type Handler struct {
	repo    Repository
	envRepo environment.Repository
}

// NewHandler returns a user_environment_access Handler using the given repositories.
func NewHandler(repo Repository, envRepo environment.Repository) *Handler {
	return &Handler{repo: repo, envRepo: envRepo}
}

func (h *Handler) Register(r *gin.RouterGroup) {
	g := r.Group("/user-environment-access")
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

// envBelongsToOrg checks that the given environment belongs to the given organization.
func (h *Handler) envBelongsToOrg(c *gin.Context, envID, orgID uuid.UUID) bool {
	env, err := h.envRepo.GetByID(c.Request.Context(), envID)
	if err != nil {
		return false
	}
	return env.OrganizationID == orgID
}

func (h *Handler) Create(c *gin.Context) {
	cu := currentUser(c)
	if cu == nil || cu.OrganizationID == nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
		return
	}
	var req UserEnvironmentAccess
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	// Verify the target environment belongs to the current user's organization.
	if !h.envBelongsToOrg(c, req.EnvironmentID, *cu.OrganizationID) {
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
		return
	}
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
	u, err := h.repo.GetByID(c.Request.Context(), id)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	// Verify the access record's environment belongs to the current user's org.
	if !h.envBelongsToOrg(c, u.EnvironmentID, *cu.OrganizationID) {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		return
	}
	c.JSON(http.StatusOK, u)
}

func (h *Handler) List(c *gin.Context) {
	cu := currentUser(c)
	if cu == nil || cu.OrganizationID == nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
		return
	}
	userIDStr := c.Query("user_id")
	envIDStr := c.Query("environment_id")
	if userIDStr != "" {
		userID, err := uuid.Parse(userIDStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user_id"})
			return
		}
		// Users may only list their own access records.
		if userID != cu.ID {
			c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
			return
		}
		list, err := h.repo.ListByUser(c.Request.Context(), userID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
			return
		}
		c.JSON(http.StatusOK, list)
		return
	}
	if envIDStr != "" {
		envID, err := uuid.Parse(envIDStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid environment_id"})
			return
		}
		// Verify the environment belongs to the current user's org.
		if !h.envBelongsToOrg(c, envID, *cu.OrganizationID) {
			c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
			return
		}
		list, err := h.repo.ListByEnvironment(c.Request.Context(), envID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
			return
		}
		c.JSON(http.StatusOK, list)
		return
	}
	c.JSON(http.StatusBadRequest, gin.H{"error": "user_id or environment_id required"})
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
	// Verify ownership via existing record's environment.
	existing, err := h.repo.GetByID(c.Request.Context(), id)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	if !h.envBelongsToOrg(c, existing.EnvironmentID, *cu.OrganizationID) {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		return
	}
	var req UserEnvironmentAccess
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	req.ID = id
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
	// Verify ownership via existing record's environment.
	existing, err := h.repo.GetByID(c.Request.Context(), id)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	if !h.envBelongsToOrg(c, existing.EnvironmentID, *cu.OrganizationID) {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		return
	}
	if err := h.repo.Delete(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	c.Status(http.StatusNoContent)
}
