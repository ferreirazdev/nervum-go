package auth

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	user "github.com/nervum/nervum-go/internal/features/users"
	"gorm.io/gorm"
)

const (
	CookieName  = "nervum_session"
	ContextUser = "auth_user"
)

func RequireAuth(sessionRepo SessionRepository, userRepo user.Repository) gin.HandlerFunc {
	return func(c *gin.Context) {
		tokenStr, err := c.Cookie(CookieName)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
			return
		}

		sessionID, err := uuid.Parse(tokenStr)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid session"})
			return
		}

		session, err := sessionRepo.GetByID(c.Request.Context(), sessionID)
		if err != nil {
			if err == gorm.ErrRecordNotFound {
				c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "session expired or not found"})
				return
			}
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
			return
		}

		u, err := userRepo.GetByID(c.Request.Context(), session.UserID)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "user not found"})
			return
		}

		c.Set(ContextUser, u)
		c.Next()
	}
}
