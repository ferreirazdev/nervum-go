package organization

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	user "github.com/nervum/nervum-go/internal/features/users"
	"gorm.io/gorm"
)

const contextUserKey = "auth_user"

type Handler struct {
	repo Repository
}

func NewHandler(repo Repository) *Handler {
	return &Handler{repo: repo}
}

func (h *Handler) Register(r *gin.RouterGroup) {
	g := r.Group("/organizations")
	g.POST("", h.Create)
	g.GET("", h.List)
	g.GET("/:id", h.GetByID)
	g.PUT("/:id", h.Update)
	g.DELETE("/:id", h.Delete)
}

func (h *Handler) Create(c *gin.Context) {
	u, ok := c.Get(contextUserKey)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
		return
	}
	currentUser := u.(*user.User)
	var req Organization
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	req.OwnerID = &currentUser.ID
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
	u, ok := c.Get(contextUserKey)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
		return
	}
	currentUser := u.(*user.User)
	if currentUser.OrganizationID == nil || *currentUser.OrganizationID != id {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		return
	}
	o, err := h.repo.GetByID(c.Request.Context(), id)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	canView := user.CanViewOrganization(currentUser.Role) || (o.OwnerID != nil && *o.OwnerID == currentUser.ID)
	if !canView {
		c.JSON(http.StatusForbidden, gin.H{"error": "cannot view organization"})
		return
	}
	c.JSON(http.StatusOK, o)
}

func (h *Handler) List(c *gin.Context) {
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
	u, ok := c.Get(contextUserKey)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
		return
	}
	currentUser := u.(*user.User)
	if currentUser.OrganizationID == nil || *currentUser.OrganizationID != id {
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
		return
	}
	o, err := h.repo.GetByID(c.Request.Context(), id)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	canEdit := user.CanEditOrganization(currentUser.Role) || (o.OwnerID != nil && *o.OwnerID == currentUser.ID)
	if !canEdit {
		c.JSON(http.StatusForbidden, gin.H{"error": "cannot edit organization"})
		return
	}
	var req struct {
		Name        *string `json:"name"`
		Description *string `json:"description"`
		Website     *string `json:"website"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.Name != nil {
		o.Name = *req.Name
	}
	if req.Description != nil {
		o.Description = *req.Description
	}
	if req.Website != nil {
		o.Website = *req.Website
	}
	if err := h.repo.Update(c.Request.Context(), o); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, o)
}

func (h *Handler) Delete(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	u, ok := c.Get(contextUserKey)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
		return
	}
	currentUser := u.(*user.User)
	if currentUser.OrganizationID == nil || *currentUser.OrganizationID != id {
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
		return
	}
	o, _ := h.repo.GetByID(c.Request.Context(), id)
	if o != nil && (user.CanEditOrganization(currentUser.Role) || (o.OwnerID != nil && *o.OwnerID == currentUser.ID)) {
		// allow delete
	} else {
		c.JSON(http.StatusForbidden, gin.H{"error": "cannot delete organization"})
		return
	}
	if err := h.repo.Delete(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.Status(http.StatusNoContent)
}
