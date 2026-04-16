// Package ratelimit provides a simple per-IP sliding-window rate limiter for Gin.
package ratelimit

import (
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

type entry struct {
	mu      sync.Mutex
	count   int
	resetAt time.Time
}

// IPRateLimit returns a Gin middleware that limits each IP to maxPerWindow requests
// per window duration. Requests exceeding the limit receive 429 Too Many Requests.
func IPRateLimit(maxPerWindow int, window time.Duration) gin.HandlerFunc {
	var store sync.Map // map[string]*entry

	return func(c *gin.Context) {
		ip := c.ClientIP()
		now := time.Now()

		v, _ := store.LoadOrStore(ip, &entry{resetAt: now.Add(window)})
		e := v.(*entry)

		e.mu.Lock()
		if now.After(e.resetAt) {
			e.count = 0
			e.resetAt = now.Add(window)
		}
		e.count++
		over := e.count > maxPerWindow
		retryAfter := 1
		if over {
			if d := int(time.Until(e.resetAt).Seconds()); d > 0 {
				retryAfter = d
			}
		}
		remaining := maxPerWindow - e.count
		if remaining < 0 {
			remaining = 0
		}
		e.mu.Unlock()

		c.Header("X-RateLimit-Limit", strconv.Itoa(maxPerWindow))
		c.Header("X-RateLimit-Remaining", strconv.Itoa(remaining))

		if over {
			c.Header("Retry-After", strconv.Itoa(retryAfter))
			c.JSON(http.StatusTooManyRequests, gin.H{"error": "too many requests"})
			c.Abort()
			return
		}
		c.Next()
	}
}
