// Package secureheaders provides a Gin middleware that sets common security response headers.
package secureheaders

import "github.com/gin-gonic/gin"

// Middleware sets defensive HTTP headers on every response.
//
// Headers applied:
//   - X-Frame-Options: DENY — prevents clickjacking
//   - X-Content-Type-Options: nosniff — prevents MIME sniffing
//   - X-XSS-Protection: 1; mode=block — legacy XSS filter for older browsers
//   - Referrer-Policy: strict-origin-when-cross-origin — limits referrer leakage
//   - Permissions-Policy: geolocation=(), camera=(), microphone=() — disables unnecessary APIs
func Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("X-Frame-Options", "DENY")
		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("X-XSS-Protection", "1; mode=block")
		c.Header("Referrer-Policy", "strict-origin-when-cross-origin")
		c.Header("Permissions-Policy", "geolocation=(), camera=(), microphone=()")
		c.Next()
	}
}
