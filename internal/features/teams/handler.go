package teams

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	user "github.com/nervum/nervum-go/internal/features/users"
	userteam "github.com/nervum/nervum-go/internal/features/user_teams"
	"gorm.io/gorm"
)

const contextUserKey = "auth_user"

type Handler struct {
	repo         Repository
	userTeamRepo userteam.Repository
}

func NewHandler(repo Repository, userTeamRepo userteam.Repository) *Handler {
	return &Handler{repo: repo, userTeamRepo: userTeamRepo}
}

type teamResponse struct {
	Team
	EnvironmentIDs []uuid.UUID `json:"environment_ids"`
}

func teamToResponse(t *Team) teamResponse {
	envIDs := make([]uuid.UUID, 0, len(t.Environments))
	for _, te := range t.Environments {
		envIDs = append(envIDs, te.EnvironmentID)
	}
	return teamResponse{Team: *t, EnvironmentIDs: envIDs}
}

func (h *Handler) Register(r *gin.RouterGroup) {
	g := r.Group("/teams")
	g.POST("", h.Create)
	g.GET("", h.List)
	g.GET("/:id", h.GetByID)
	g.PUT("/:id", h.Update)
	g.DELETE("/:id", h.Delete)
}

type createTeamRequest struct {
	OrganizationID  uuid.UUID   `json:"organization_id" binding:"required"`
	Name            string      `json:"name" binding:"required"`
	Icon            string      `json:"icon"`
	EnvironmentIDs  []uuid.UUID `json:"environment_ids"`
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
	if !user.CanManageTeams(currentUser.Role) {
		c.JSON(http.StatusForbidden, gin.H{"error": "cannot manage teams"})
		return
	}
	var req createTeamRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.OrganizationID != *currentUser.OrganizationID {
		c.JSON(http.StatusForbidden, gin.H{"error": "organization mismatch"})
		return
	}
	t := &Team{
		OrganizationID: req.OrganizationID,
		Name:           req.Name,
		Icon:           req.Icon,
	}
	if req.EnvironmentIDs == nil {
		req.EnvironmentIDs = []uuid.UUID{}
	}
	if err := h.repo.Create(c.Request.Context(), t, req.EnvironmentIDs); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	// Reload with environments for response
	created, _ := h.repo.GetByID(c.Request.Context(), t.ID)
	c.JSON(http.StatusCreated, teamToResponse(created))
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
	t, err := h.repo.GetByID(c.Request.Context(), id)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if t.OrganizationID != *currentUser.OrganizationID {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		return
	}
	if !user.CanViewAllTeams(currentUser.Role) {
		_, err := h.userTeamRepo.GetByUserAndTeam(c.Request.Context(), currentUser.ID, id)
		if err != nil {
			c.JSON(http.StatusForbidden, gin.H{"error": "cannot view this team"})
			return
		}
	}
	c.JSON(http.StatusOK, teamToResponse(t))
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
	var list []Team
	if user.CanViewAllTeams(currentUser.Role) {
		list, err = h.repo.ListByOrganization(c.Request.Context(), orgID)
	} else {
		list, err = h.repo.ListTeamsForUserMember(c.Request.Context(), orgID, currentUser.ID)
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	resp := make([]teamResponse, len(list))
	for i := range list {
		resp[i] = teamToResponse(&list[i])
	}
	c.JSON(http.StatusOK, resp)
}

type updateTeamRequest struct {
	Name           *string     `json:"name"`
	Icon           *string     `json:"icon"`
	EnvironmentIDs []uuid.UUID `json:"environment_ids"`
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
	if !user.CanManageTeams(currentUser.Role) {
		c.JSON(http.StatusForbidden, gin.H{"error": "cannot manage teams"})
		return
	}
	t, err := h.repo.GetByID(c.Request.Context(), id)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if t.OrganizationID != *currentUser.OrganizationID {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		return
	}
	var req updateTeamRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.Name != nil {
		t.Name = *req.Name
	}
	if req.Icon != nil {
		t.Icon = *req.Icon
	}
	envIDsUUID := make([]uuid.UUID, 0, len(t.Environments))
	for _, te := range t.Environments {
		envIDsUUID = append(envIDsUUID, te.EnvironmentID)
	}
	if req.EnvironmentIDs != nil {
		envIDsUUID = req.EnvironmentIDs
	}
	if err := h.repo.Update(c.Request.Context(), t, envIDsUUID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	updated, _ := h.repo.GetByID(c.Request.Context(), t.ID)
	c.JSON(http.StatusOK, teamToResponse(updated))
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
	if !user.CanManageTeams(currentUser.Role) {
		c.JSON(http.StatusForbidden, gin.H{"error": "cannot manage teams"})
		return
	}
	t, err := h.repo.GetByID(c.Request.Context(), id)
	if err == nil && t.OrganizationID != *currentUser.OrganizationID {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		return
	}
	_ = h.userTeamRepo.DeleteByTeam(c.Request.Context(), id)
	if err := h.repo.Delete(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.Status(http.StatusNoContent)
}
