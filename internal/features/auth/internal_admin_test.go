package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	user "github.com/nervum/nervum-go/internal/features/users"
)

func TestRequireInternalAdmin_AllowedAndDenied(t *testing.T) {
	gin.SetMode(gin.TestMode)
	allow := []string{"ferreirazdev@gmail.com", "other@example.com"}

	t.Run("allowed exact case", func(t *testing.T) {
		r := gin.New()
		r.Use(func(c *gin.Context) {
			c.Set(ContextUser, &user.User{ID: uuid.New(), Email: "Ferreirazdev@gmail.com"})
			c.Next()
		})
		r.Use(RequireInternalAdmin(allow))
		r.GET("/x", func(c *gin.Context) { c.Status(http.StatusOK) })

		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/x", nil))
		if w.Code != http.StatusOK {
			t.Fatalf("code %d", w.Code)
		}
	})

	t.Run("denied", func(t *testing.T) {
		r := gin.New()
		r.Use(func(c *gin.Context) {
			c.Set(ContextUser, &user.User{ID: uuid.New(), Email: "intruder@evil.com"})
			c.Next()
		})
		r.Use(RequireInternalAdmin(allow))
		r.GET("/x", func(c *gin.Context) { c.Status(http.StatusOK) })

		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/x", nil))
		if w.Code != http.StatusForbidden {
			t.Fatalf("code %d want 403", w.Code)
		}
	})

	t.Run("no user", func(t *testing.T) {
		r := gin.New()
		r.Use(RequireInternalAdmin(allow))
		r.GET("/x", func(c *gin.Context) { c.Status(http.StatusOK) })

		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/x", nil))
		if w.Code != http.StatusUnauthorized {
			t.Fatalf("code %d want 401", w.Code)
		}
	})
}
