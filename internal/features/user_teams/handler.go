package userteam

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type Handler struct {
	repo Repository
}

func NewHandler(repo Repository) *Handler {
	return &Handler{repo: repo}
}

func (h *Handler) Register(r *gin.RouterGroup) {
	g := r.Group("/user-teams")
	g.POST("", h.Create)
	g.GET("", h.List)
	g.DELETE("", h.Delete)
}

type createUserTeamRequest struct {
	UserID uuid.UUID `json:"user_id" binding:"required"`
	TeamID uuid.UUID `json:"team_id" binding:"required"`
	Role   string    `json:"role"`
}

func (h *Handler) Create(c *gin.Context) {
	var req createUserTeamRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	ut := &UserTeam{
		UserID: req.UserID,
		TeamID: req.TeamID,
		Role:   req.Role,
	}
	if err := h.repo.Create(c.Request.Context(), ut); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, ut)
}

func (h *Handler) List(c *gin.Context) {
	userIDStr := c.Query("user_id")
	teamIDStr := c.Query("team_id")
	if userIDStr != "" {
		userID, err := uuid.Parse(userIDStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user_id"})
			return
		}
		list, err := h.repo.ListByUser(c.Request.Context(), userID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, list)
		return
	}
	if teamIDStr != "" {
		teamID, err := uuid.Parse(teamIDStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid team_id"})
			return
		}
		list, err := h.repo.ListByTeam(c.Request.Context(), teamID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, list)
		return
	}
	c.JSON(http.StatusBadRequest, gin.H{"error": "user_id or team_id required"})
}

func (h *Handler) Delete(c *gin.Context) {
	var req struct {
		UserID uuid.UUID `json:"user_id" binding:"required"`
		TeamID uuid.UUID `json:"team_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	err := h.repo.Delete(c.Request.Context(), req.UserID, req.TeamID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.Status(http.StatusNoContent)
}
