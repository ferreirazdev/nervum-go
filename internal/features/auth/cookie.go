package auth

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// SetSessionCookie sets the session cookie (Secure + HttpOnly). sameSite must be
// SameSiteNoneMode when the browser calls the API from a different origin than the API host.
func SetSessionCookie(c *gin.Context, sameSite http.SameSite, token string, maxAge time.Duration) {
	c.SetSameSite(sameSite)
	c.SetCookie(CookieName, token, int(maxAge.Seconds()), "/", "", true, true)
}

// ClearSessionCookie removes the session cookie.
func ClearSessionCookie(c *gin.Context, sameSite http.SameSite) {
	c.SetSameSite(sameSite)
	c.SetCookie(CookieName, "", -1, "/", "", true, true)
}
