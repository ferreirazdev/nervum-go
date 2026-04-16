package auth

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	user "github.com/nervum/nervum-go/internal/features/users"
)

// RequireInternalAdmin returns middleware that allows only users whose email is in allowedEmails
// (each entry should already be lowercased and trimmed). Must run after RequireAuth.
func RequireInternalAdmin(allowedEmails []string) gin.HandlerFunc {
	allowed := make(map[string]struct{}, len(allowedEmails))
	for _, e := range allowedEmails {
		e = strings.ToLower(strings.TrimSpace(e))
		if e != "" {
			allowed[e] = struct{}{}
		}
	}
	return func(c *gin.Context) {
		v, ok := c.Get(ContextUser)
		if !ok {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
			return
		}
		u, ok := v.(*user.User)
		if !ok || u == nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
			return
		}
		email := strings.ToLower(strings.TrimSpace(u.Email))
		if _, ok := allowed[email]; !ok {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "forbidden"})
			return
		}
		c.Next()
	}
}
