package auth

import (
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	organization "github.com/nervum/nervum-go/internal/features/organizations"
	user "github.com/nervum/nervum-go/internal/features/users"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

const (
	sessionDuration = 7 * 24 * time.Hour
	bcryptCost      = 12
)

// Handler handles auth HTTP routes: register, login, logout, and me (current user).
type Handler struct {
	sessionRepo SessionRepository
	userRepo    user.Repository
	orgRepo     organization.Repository
}

// NewHandler returns an auth Handler with the given session and user repositories.
func NewHandler(sessionRepo SessionRepository, userRepo user.Repository, orgRepo organization.Repository) *Handler {
	return &Handler{sessionRepo: sessionRepo, userRepo: userRepo, orgRepo: orgRepo}
}

func (h *Handler) Register(r *gin.RouterGroup) {
	g := r.Group("/auth")
	g.POST("/register", h.RegisterUser)
	g.POST("/login", h.Login)
	g.POST("/logout", h.Logout)
	g.GET("/me", RequireAuth(h.sessionRepo, h.userRepo), h.Me)
}

type registerRequest struct {
	Name     string `json:"name" binding:"required"`
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required,min=8"`
}

func (h *Handler) RegisterUser(c *gin.Context) {
	var req registerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	existing, err := h.userRepo.GetByEmail(c.Request.Context(), req.Email)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	if existing != nil {
		c.JSON(http.StatusConflict, gin.H{"error": "email already registered"})
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcryptCost)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	u := &user.User{
		ID:           uuid.New(),
		Email:        req.Email,
		Name:         req.Name,
		Role:         user.RoleAdmin,
		PasswordHash: string(hash),
	}
	if err := h.userRepo.Create(c.Request.Context(), u); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	session := &Session{
		UserID:    u.ID,
		ExpiresAt: time.Now().Add(sessionDuration),
	}
	if err := h.sessionRepo.Create(c.Request.Context(), session); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	setSessionCookie(c, session.ID.String())
	c.JSON(http.StatusCreated, u)
}

type loginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

func (h *Handler) Login(c *gin.Context) {
	var req loginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	u, err := h.userRepo.GetByEmail(c.Request.Context(), req.Email)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials"})
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(req.Password)); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials"})
		return
	}

	session := &Session{
		UserID:    u.ID,
		ExpiresAt: time.Now().Add(sessionDuration),
	}
	if err := h.sessionRepo.Create(c.Request.Context(), session); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	setSessionCookie(c, session.ID.String())
	c.JSON(http.StatusOK, u)
}

func (h *Handler) Logout(c *gin.Context) {
	tokenStr, err := c.Cookie(CookieName)
	if err == nil {
		if sessionID, err := uuid.Parse(tokenStr); err == nil {
			_ = h.sessionRepo.Delete(c.Request.Context(), sessionID)
		}
	}
	clearSessionCookie(c)
	c.Status(http.StatusNoContent)
}

func (h *Handler) Me(c *gin.Context) {
	u, _ := c.Get(ContextUser)
	c.JSON(http.StatusOK, u)
}

func setSessionCookie(c *gin.Context, token string) {
	c.SetSameSite(http.SameSiteStrictMode)
	c.SetCookie(CookieName, token, int(sessionDuration.Seconds()), "/", "", false, true)
}

func clearSessionCookie(c *gin.Context) {
	c.SetSameSite(http.SameSiteStrictMode)
	c.SetCookie(CookieName, "", -1, "/", "", false, true)
}
