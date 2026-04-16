package ratelimit

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func TestIPRateLimit_RetryAfterAndHeaders(t *testing.T) {
	gin.SetMode(gin.TestMode)
	limit := IPRateLimit(2, 500*time.Millisecond)
	r := gin.New()
	r.GET("/t", limit, func(c *gin.Context) { c.Status(http.StatusOK) })

	for i := 0; i < 2; i++ {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/t", nil)
		req.RemoteAddr = "192.0.2.1:1234"
		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("request %d: status %d", i, w.Code)
		}
		if lim := w.Header().Get("X-RateLimit-Limit"); lim != "2" {
			t.Errorf("X-RateLimit-Limit = %q, want 2", lim)
		}
		if rem := w.Header().Get("X-RateLimit-Remaining"); rem != strconv.Itoa(1-i) {
			t.Errorf("request %d: X-RateLimit-Remaining = %q", i, rem)
		}
	}

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/t", nil)
	req.RemoteAddr = "192.0.2.1:1234"
	r.ServeHTTP(w, req)
	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("429 status = %d", w.Code)
	}
	ra := w.Header().Get("Retry-After")
	if ra == "" {
		t.Fatal("missing Retry-After header")
	}
	n, err := strconv.Atoi(ra)
	if err != nil || n < 1 {
		t.Fatalf("Retry-After = %q, want positive integer", ra)
	}
}
