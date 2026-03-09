package invitation

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/nervum/nervum-go/internal/features/auth"
	organization "github.com/nervum/nervum-go/internal/features/organizations"
	user "github.com/nervum/nervum-go/internal/features/users"
	userenvironmentaccess "github.com/nervum/nervum-go/internal/features/user_environment_access"
	userteam "github.com/nervum/nervum-go/internal/features/user_teams"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

const (
	inviteExpiryDays = 7
	bcryptCost       = 12
	sessionDuration  = 7 * 24 * time.Hour
)

type Handler struct {
	repo          Repository
	userRepo      user.Repository
	orgRepo       organization.Repository
	userTeamRepo  userteam.Repository
	userEnvRepo   userenvironmentaccess.Repository
	sessionRepo   auth.SessionRepository
}

func NewHandler(
	repo Repository,
	userRepo user.Repository,
	orgRepo organization.Repository,
	userTeamRepo userteam.Repository,
	userEnvRepo userenvironmentaccess.Repository,
	sessionRepo auth.SessionRepository,
) *Handler {
	return &Handler{
		repo: repo, userRepo: userRepo, orgRepo: orgRepo,
		userTeamRepo: userTeamRepo, userEnvRepo: userEnvRepo, sessionRepo: sessionRepo,
	}
}

func (h *Handler) Register(r *gin.RouterGroup) {
	r.POST("/invitations", h.Create)
	r.GET("/invitations", h.List)
	r.DELETE("/invitations/:id", h.Delete)
}

func (h *Handler) RegisterPublic(r *gin.RouterGroup) {
	r.GET("/invitations/by-token/:token", h.GetByToken)
	r.POST("/invitations/accept", h.Accept)
}

func generateToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func (h *Handler) Create(c *gin.Context) {
	u, _ := c.Get(auth.ContextUser)
	currentUser := u.(*user.User)
	if currentUser.OrganizationID == nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "no organization"})
		return
	}
	if !user.CanInvite(currentUser.Role) {
		c.JSON(http.StatusForbidden, gin.H{"error": "cannot invite users"})
		return
	}
	var req struct {
		Email         string      `json:"email" binding:"required,email"`
		TeamIDs       []uuid.UUID `json:"team_ids" binding:"required"`
		EnvironmentID *uuid.UUID  `json:"environment_id"`
		Role          *string     `json:"role"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if len(req.TeamIDs) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "at least one team required"})
		return
	}
	role := user.RoleMember
	if req.Role != nil && *req.Role != "" {
		role = *req.Role
	}
	if role != user.RoleAdmin && role != user.RoleManager && role != user.RoleMember {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid role"})
		return
	}
	if !user.CanAssignInviteRole(currentUser.Role, role) {
		c.JSON(http.StatusForbidden, gin.H{"error": "cannot assign that role"})
		return
	}
	token, err := generateToken()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate token"})
		return
	}
	inv := &Invitation{
		Token:          token,
		Email:          req.Email,
		OrganizationID: *currentUser.OrganizationID,
		InvitedByID:    currentUser.ID,
		Role:           role,
		EnvironmentID:  req.EnvironmentID,
		ExpiresAt:      time.Now().Add(inviteExpiryDays * 24 * time.Hour),
		Status:         StatusPending,
	}
	if err := h.repo.Create(c.Request.Context(), inv, req.TeamIDs); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	origin := c.GetHeader("Origin")
	if origin == "" {
		origin = "http://localhost:5173"
	}
	inviteURL := origin + "/accept-invite?token=" + token
	inv.Teams = inv.Teams // reload if needed; we have team IDs in req
	c.JSON(http.StatusCreated, gin.H{
		"invitation":  inv,
		"invite_url":  inviteURL,
		"token":       token,
	})
}

func (h *Handler) List(c *gin.Context) {
	u, _ := c.Get(auth.ContextUser)
	currentUser := u.(*user.User)
	if currentUser.OrganizationID == nil {
		c.JSON(http.StatusOK, []Invitation{})
		return
	}
	if !user.CanInvite(currentUser.Role) {
		c.JSON(http.StatusForbidden, gin.H{"error": "cannot list invitations"})
		return
	}
	status := c.Query("status")
	list, err := h.repo.ListByOrganization(c.Request.Context(), *currentUser.OrganizationID, status)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, list)
}

func (h *Handler) Delete(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	inv, err := h.repo.GetByID(c.Request.Context(), id)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	u, _ := c.Get(auth.ContextUser)
	currentUser := u.(*user.User)
	if currentUser.OrganizationID == nil || inv.OrganizationID != *currentUser.OrganizationID {
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
		return
	}
	if !user.CanInvite(currentUser.Role) {
		c.JSON(http.StatusForbidden, gin.H{"error": "cannot delete invitation"})
		return
	}
	if err := h.repo.Delete(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.Status(http.StatusNoContent)
}

type inviteByTokenResponse struct {
	Email            string    `json:"email"`
	OrganizationID   string    `json:"organization_id"`
	OrganizationName string    `json:"organization_name"`
	TeamIDs          []string  `json:"team_ids"`
	EnvironmentID    *string   `json:"environment_id,omitempty"`
	Role             string    `json:"role"`
	ExpiresAt        time.Time `json:"expires_at"`
}

func (h *Handler) GetByToken(c *gin.Context) {
	token := c.Param("token")
	if token == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "token required"})
		return
	}
	inv, err := h.repo.GetByToken(c.Request.Context(), token)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "invitation not found or expired"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if inv.Status != StatusPending {
		c.JSON(http.StatusGone, gin.H{"error": "invitation already used"})
		return
	}
	if time.Now().After(inv.ExpiresAt) {
		c.JSON(http.StatusGone, gin.H{"error": "invitation expired"})
		return
	}
	org, _ := h.orgRepo.GetByID(c.Request.Context(), inv.OrganizationID)
	orgName := ""
	if org != nil {
		orgName = org.Name
	}
	teamIDs := make([]string, 0, len(inv.Teams))
	for _, t := range inv.Teams {
		teamIDs = append(teamIDs, t.TeamID.String())
	}
	resp := inviteByTokenResponse{
		Email:            inv.Email,
		OrganizationID:   inv.OrganizationID.String(),
		OrganizationName: orgName,
		TeamIDs:          teamIDs,
		Role:             inv.Role,
		ExpiresAt:        inv.ExpiresAt,
	}
	if inv.Role == "" {
		resp.Role = user.RoleMember
	}
	if inv.EnvironmentID != nil {
		s := inv.EnvironmentID.String()
		resp.EnvironmentID = &s
	}
	c.JSON(http.StatusOK, resp)
}

type acceptRequest struct {
	Token    string `json:"token" binding:"required"`
	Name     string `json:"name"`
	Password string `json:"password"`
}

func (h *Handler) Accept(c *gin.Context) {
	var req acceptRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	inv, err := h.repo.GetByToken(c.Request.Context(), req.Token)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "invitation not found or expired"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if inv.Status != StatusPending {
		c.JSON(http.StatusConflict, gin.H{"error": "invitation already used"})
		return
	}
	if time.Now().After(inv.ExpiresAt) {
		inv.Status = StatusExpired
		_ = h.repo.Update(c.Request.Context(), inv)
		c.JSON(http.StatusGone, gin.H{"error": "invitation expired"})
		return
	}

	ctx := c.Request.Context()
	existingUser, err := h.userRepo.GetByEmail(ctx, inv.Email)
	if err != nil && err != gorm.ErrRecordNotFound {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if existingUser != nil {
		// Existing user: set org, add teams, add env access
		existingUser.OrganizationID = &inv.OrganizationID
		if req.Name != "" {
			existingUser.Name = req.Name
		}
		if err := h.userRepo.Update(ctx, existingUser); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		for _, it := range inv.Teams {
			_ = h.userTeamRepo.Create(ctx, &userteam.UserTeam{UserID: existingUser.ID, TeamID: it.TeamID})
		}
		if inv.EnvironmentID != nil {
			_ = h.userEnvRepo.Create(ctx, &userenvironmentaccess.UserEnvironmentAccess{
				UserID:        existingUser.ID,
				EnvironmentID: *inv.EnvironmentID,
			})
		}
		inv.Status = StatusAccepted
		now := time.Now()
		inv.AcceptedAt = &now
		_ = h.repo.Update(ctx, inv)
		// Log in
		session := &auth.Session{UserID: existingUser.ID, ExpiresAt: time.Now().Add(sessionDuration)}
		if err := h.sessionRepo.Create(ctx, session); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create session"})
			return
		}
		setSessionCookie(c, session.ID.String())
		c.JSON(http.StatusOK, existingUser)
		return
	}

	// New user: password required
	if req.Password == "" || len(req.Password) < 8 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "password required (min 8 characters)"})
		return
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcryptCost)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	name := req.Name
	if name == "" {
		name = inv.Email
	}
	invRole := inv.Role
	if invRole == "" {
		invRole = user.RoleMember
	}
	newUser := &user.User{
		Email:          inv.Email,
		Name:           name,
		Role:           invRole,
		OrganizationID: &inv.OrganizationID,
		PasswordHash:   string(hash),
	}
	if err := h.userRepo.Create(ctx, newUser); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	for _, it := range inv.Teams {
		_ = h.userTeamRepo.Create(ctx, &userteam.UserTeam{UserID: newUser.ID, TeamID: it.TeamID})
	}
	if inv.EnvironmentID != nil {
		_ = h.userEnvRepo.Create(ctx, &userenvironmentaccess.UserEnvironmentAccess{
			UserID:        newUser.ID,
			EnvironmentID: *inv.EnvironmentID,
		})
	}
	inv.Status = StatusAccepted
	now := time.Now()
	inv.AcceptedAt = &now
	_ = h.repo.Update(ctx, inv)
	session := &auth.Session{UserID: newUser.ID, ExpiresAt: time.Now().Add(sessionDuration)}
	if err := h.sessionRepo.Create(ctx, session); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create session"})
		return
	}
	setSessionCookie(c, session.ID.String())
	c.JSON(http.StatusCreated, newUser)
}

func setSessionCookie(c *gin.Context, token string) {
	c.SetSameSite(http.SameSiteStrictMode)
	c.SetCookie(auth.CookieName, token, int(sessionDuration.Seconds()), "/", "", false, true)
}
