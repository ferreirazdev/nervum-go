package orgservices

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/nervum/nervum-go/internal/features/auth"
	user "github.com/nervum/nervum-go/internal/features/users"
	"gorm.io/gorm"
)

// Handler serves HTTP for organization cloud services (list, add, delete).
type Handler struct {
	repo Repository
}

// NewHandler returns an orgservices Handler.
func NewHandler(repo Repository) *Handler {
	return &Handler{repo: repo}
}

// Register mounts routes under the given group. Expects group path like "/organizations".
func (h *Handler) Register(r *gin.RouterGroup) {
	g := r.Group("/:id/services")
	g.GET("", h.List)
	g.POST("", h.Create)
	g.DELETE("/:serviceId", h.Delete)
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
		c.JSON(http.StatusForbidden, gin.H{"error": "cannot access services for another organization"})
		return nil, uuid.Nil, false
	}
	return currentUser, orgID, true
}

var allowedKinds = map[string]bool{"cloud_run": true, "cloud_sql": true, "compute": true}

func (h *Handler) List(c *gin.Context) {
	_, orgID, ok := h.requireOrgMember(c)
	if !ok {
		return
	}
	kind := strings.TrimSpace(c.Query("kind"))
	var list []OrganizationService
	var err error
	if kind != "" {
		if !allowedKinds[kind] {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid kind"})
			return
		}
		list, err = h.repo.ListByOrganizationAndKind(c.Request.Context(), orgID, kind)
	} else {
		list, err = h.repo.ListByOrganization(c.Request.Context(), orgID)
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	c.JSON(http.StatusOK, list)
}

type createRequest struct {
	ServiceName   string `json:"service_name" binding:"required"`
	Location      string `json:"location"`
	Kind          string `json:"kind"`
	InstanceType  string `json:"instance_type"`
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
	serviceName := strings.TrimSpace(req.ServiceName)
	if serviceName == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "service_name is required"})
		return
	}
	location := strings.TrimSpace(req.Location)
	kind := strings.TrimSpace(req.Kind)
	if kind == "" {
		kind = "cloud_run"
	}
	if !allowedKinds[kind] {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid kind; must be cloud_run, cloud_sql, or compute"})
		return
	}
	instanceType := strings.TrimSpace(req.InstanceType)
	svc := &OrganizationService{
		OrganizationID: orgID,
		Provider:       "gcloud",
		Kind:           kind,
		ServiceName:    serviceName,
		Location:       location,
		InstanceType:   instanceType,
	}
	if err := h.repo.Create(c.Request.Context(), svc); err != nil {
		if strings.Contains(err.Error(), "duplicate") || strings.Contains(err.Error(), "unique") {
			c.JSON(http.StatusConflict, gin.H{"error": "service already added"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	c.JSON(http.StatusCreated, svc)
}

func (h *Handler) Delete(c *gin.Context) {
	_, orgID, ok := h.requireOrgMember(c)
	if !ok {
		return
	}
	serviceIDStr := c.Param("serviceId")
	if serviceIDStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "service id required"})
		return
	}
	serviceID, err := uuid.Parse(serviceIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid service id"})
		return
	}
	existing, err := h.repo.GetByID(c.Request.Context(), serviceID)
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
	if err := h.repo.Delete(c.Request.Context(), serviceID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	c.Status(http.StatusNoContent)
}
