// Package health provides an HTTP handler for API health checks (liveness/readiness).
package health

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

const pingTimeout = 2 * time.Second

// Response is the JSON body for GET /health.
type Response struct {
	Status   string `json:"status"`
	Database string `json:"database,omitempty"`
}

// Handler returns a Gin handler that responds to GET /health.
// It pings the database; if the ping fails, responds with 503 and database: "unhealthy".
func Handler(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		sqlDB, err := db.DB()
		if err != nil {
			c.JSON(http.StatusServiceUnavailable, Response{
				Status:   "unhealthy",
				Database: "error: " + err.Error(),
			})
			return
		}
		ctx, cancel := context.WithTimeout(c.Request.Context(), pingTimeout)
		defer cancel()
		if err := sqlDB.PingContext(ctx); err != nil {
			c.JSON(http.StatusServiceUnavailable, Response{
				Status:   "unhealthy",
				Database: "unreachable",
			})
			return
		}
		c.JSON(http.StatusOK, Response{
			Status:   "ok",
			Database: "ok",
		})
	}
}

// LivenessHandler returns a minimal handler that always responds 200 (process is alive).
// Use for Kubernetes liveness probes when you only need to know the process is up.
func LivenessHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	}
}

// ReadinessHandler returns a handler that checks DB and responds 200 only when ready to serve traffic.
// Use for Kubernetes readiness probes; returns 503 when DB is down.
func ReadinessHandler(db *gorm.DB) gin.HandlerFunc {
	return Handler(db)
}
