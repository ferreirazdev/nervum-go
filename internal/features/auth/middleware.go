package auth

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	user "github.com/nervum/nervum-go/internal/features/users"
	"gorm.io/gorm"
)

// CookieName is the name of the session cookie set on login/register.
const CookieName = "nervum_session"

// ContextUser is the Gin context key under which the authenticated user is stored.
const ContextUser = "auth_user"

// RequireAuth returns a Gin middleware that validates the session cookie (or optional Bearer service token),
// loads the user, and sets the user in context. Responds with 401 when missing or invalid.
// If serviceToken and serviceUserID are non-empty, Authorization: Bearer <serviceToken> is accepted and
// the user identified by serviceUserID (UUID) is set in context (for CLI/automation).
func RequireAuth(sessionRepo SessionRepository, userRepo user.Repository, serviceToken, serviceUserID string) gin.HandlerFunc {
	return func(c *gin.Context) {
		tokenStr, err := c.Cookie(CookieName)
		if err != nil || tokenStr == "" {
			// Try Bearer token for CLI/automation
			if serviceToken != "" && serviceUserID != "" {
				authz := c.GetHeader("Authorization")
				if strings.HasPrefix(authz, "Bearer ") {
					token := strings.TrimSpace(authz[7:])
					if token == serviceToken {
						userID, err := uuid.Parse(serviceUserID)
						if err != nil {
							c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid service user config"})
							return
						}
						u, err := userRepo.GetByID(c.Request.Context(), userID)
						if err != nil {
							if err == gorm.ErrRecordNotFound {
								c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "service user not found"})
								return
							}
							c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
							return
						}
						if u.OrganizationID == nil {
							c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "service user has no organization"})
							return
						}
						c.Set(ContextUser, u)
						c.Next()
						return
					}
				}
			}
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
