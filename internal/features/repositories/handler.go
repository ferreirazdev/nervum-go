package repositories

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/nervum/nervum-go/internal/features/auth"
	user "github.com/nervum/nervum-go/internal/features/users"
	"gorm.io/gorm"
)

// Handler serves HTTP for organization repositories (list, add, delete).
type Handler struct {
	repo Repository
}

// NewHandler returns a repositories Handler.
func NewHandler(repo Repository) *Handler {
	return &Handler{repo: repo}
}

// Register mounts routes under the given group. Expects group path like "/organizations".
func (h *Handler) Register(r *gin.RouterGroup) {
	g := r.Group("/:id/repositories")
	g.GET("", h.List)
	g.POST("", h.Create)
	g.DELETE("/:repoId", h.Delete)
}

func (h *Handler) requireOrgMember(c *gin.Context) (*user.User, uuid.UUID, bool) {
	u, ok := c.Get(auth.ContextUser)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
		return nil, uuid.Nil, false
	}
	currentUser := u.(*user.User)
	idStr := c.Param("id")
	if idStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "organization id required"})
		return nil, uuid.Nil, false
	}
	orgID, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid organization id"})
		return nil, uuid.Nil, false
	}
	if currentUser.OrganizationID == nil || *currentUser.OrganizationID != orgID {
		c.JSON(http.StatusForbidden, gin.H{"error": "cannot access repositories for another organization"})
		return nil, uuid.Nil, false
	}
	return currentUser, orgID, true
}

func (h *Handler) List(c *gin.Context) {
	_, orgID, ok := h.requireOrgMember(c)
	if !ok {
		return
	}
	list, err := h.repo.ListByOrganization(c.Request.Context(), orgID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	c.JSON(http.StatusOK, list)
}

type createRequest struct {
	FullName string `json:"full_name" binding:"required"`
}

func (h *Handler) Create(c *gin.Context) {
	_, orgID, ok := h.requireOrgMember(c)
	if !ok {
		return
	}
	var req createRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	fullName := strings.TrimSpace(req.FullName)
	parts := strings.SplitN(fullName, "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "full_name must be owner/repo"})
		return
	}
	repo := &OrganizationRepository{
		OrganizationID: orgID,
		Provider:       "github",
		FullName:       fullName,
	}
	if err := h.repo.Create(c.Request.Context(), repo); err != nil {
		if strings.Contains(err.Error(), "duplicate") || strings.Contains(err.Error(), "unique") {
			c.JSON(http.StatusConflict, gin.H{"error": "repository already added"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	c.JSON(http.StatusCreated, repo)
}

func (h *Handler) Delete(c *gin.Context) {
	_, orgID, ok := h.requireOrgMember(c)
	if !ok {
		return
	}
	repoIDStr := c.Param("repoId")
	if repoIDStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "repository id required"})
		return
	}
	repoID, err := uuid.Parse(repoIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid repository id"})
		return
	}
	existing, err := h.repo.GetByID(c.Request.Context(), repoID)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	if existing.OrganizationID != orgID {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		return
	}
	if err := h.repo.Delete(c.Request.Context(), repoID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	c.Status(http.StatusNoContent)
}
