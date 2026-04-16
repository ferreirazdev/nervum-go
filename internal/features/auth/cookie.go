package auth

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// SetSessionCookie sets the session cookie (HttpOnly; Secure when secure is true).
// sameSite must be SameSiteNoneMode when the browser calls the API from a different origin than the API host.
func SetSessionCookie(c *gin.Context, sameSite http.SameSite, token string, maxAge time.Duration, secure bool) {
	c.SetSameSite(sameSite)
	c.SetCookie(CookieName, token, int(maxAge.Seconds()), "/", "", secure, true)
}

// ClearSessionCookie removes the session cookie.
func ClearSessionCookie(c *gin.Context, sameSite http.SameSite, secure bool) {
	c.SetSameSite(sameSite)
	c.SetCookie(CookieName, "", -1, "/", "", secure, true)
}
