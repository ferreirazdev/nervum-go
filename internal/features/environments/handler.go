package environment

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	entity "github.com/nervum/nervum-go/internal/features/entities"
	user "github.com/nervum/nervum-go/internal/features/users"
	"gorm.io/gorm"
)

const contextUserKey = "auth_user"

type Handler struct {
	repo       Repository
	entityRepo entity.Repository
}

func NewHandler(repo Repository, entityRepo entity.Repository) *Handler {
	return &Handler{repo: repo, entityRepo: entityRepo}
}

type environmentResponse struct {
	Environment
	ServicesCount int64 `json:"services_count"`
}

func (h *Handler) Register(r *gin.RouterGroup) {
	g := r.Group("/environments")
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
	if currentUser.OrganizationID == nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "no organization"})
		return
	}
	if !user.CanManageEnvironments(currentUser.Role) {
		c.JSON(http.StatusForbidden, gin.H{"error": "cannot manage environments"})
		return
	}
	var req Environment
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.OrganizationID != *currentUser.OrganizationID {
		c.JSON(http.StatusForbidden, gin.H{"error": "organization mismatch"})
		return
	}
	if req.Status == "" {
		req.Status = "healthy"
	}
	if err := h.repo.Create(c.Request.Context(), &req); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, environmentResponse{req, 0})
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
	if currentUser.OrganizationID == nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "no organization"})
		return
	}
	e, err := h.repo.GetByID(c.Request.Context(), id)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if e.OrganizationID != *currentUser.OrganizationID {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		return
	}
	if !user.CanViewAllEnvironments(currentUser.Role) {
		ok, err := h.repo.UserCanAccessEnvironment(c.Request.Context(), id, currentUser.ID)
		if err != nil || !ok {
			c.JSON(http.StatusForbidden, gin.H{"error": "cannot view this environment"})
			return
		}
	}
	count, _ := h.entityRepo.CountByEnvironment(c.Request.Context(), e.ID)
	c.JSON(http.StatusOK, environmentResponse{*e, count})
}

func (h *Handler) List(c *gin.Context) {
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
	u, ok := c.Get(contextUserKey)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
		return
	}
	currentUser := u.(*user.User)
	if currentUser.OrganizationID == nil || *currentUser.OrganizationID != orgID {
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
		return
	}
	var list []Environment
	if user.CanViewAllEnvironments(currentUser.Role) {
		list, err = h.repo.ListByOrganization(c.Request.Context(), orgID)
	} else {
		list, err = h.repo.ListEnvironmentsForMember(c.Request.Context(), orgID, currentUser.ID)
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	ctx := c.Request.Context()
	response := make([]environmentResponse, len(list))
	for i, env := range list {
		count, _ := h.entityRepo.CountByEnvironment(ctx, env.ID)
		response[i] = environmentResponse{env, count}
	}
	c.JSON(http.StatusOK, response)
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
	if currentUser.OrganizationID == nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "no organization"})
		return
	}
	if !user.CanManageEnvironments(currentUser.Role) {
		c.JSON(http.StatusForbidden, gin.H{"error": "cannot manage environments"})
		return
	}
	e, err := h.repo.GetByID(c.Request.Context(), id)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if e.OrganizationID != *currentUser.OrganizationID {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		return
	}
	var req struct {
		Name        *string `json:"name"`
		Description *string `json:"description"`
		Status      *string `json:"status"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.Name != nil {
		e.Name = *req.Name
	}
	if req.Description != nil {
		e.Description = *req.Description
	}
	if req.Status != nil {
		e.Status = *req.Status
	}
	if err := h.repo.Update(c.Request.Context(), e); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	count, _ := h.entityRepo.CountByEnvironment(c.Request.Context(), e.ID)
	c.JSON(http.StatusOK, environmentResponse{*e, count})
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
	if currentUser.OrganizationID == nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "no organization"})
		return
	}
	if !user.CanManageEnvironments(currentUser.Role) {
		c.JSON(http.StatusForbidden, gin.H{"error": "cannot manage environments"})
		return
	}
	e, err := h.repo.GetByID(c.Request.Context(), id)
	if err == nil && e.OrganizationID != *currentUser.OrganizationID {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		return
	}
	if err := h.repo.Delete(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.Status(http.StatusNoContent)
}
