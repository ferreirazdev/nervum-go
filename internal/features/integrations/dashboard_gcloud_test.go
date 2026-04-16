package integrations

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/nervum/nervum-go/internal/config"
	"github.com/nervum/nervum-go/internal/features/auth"
	user "github.com/nervum/nervum-go/internal/features/users"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

var testEncKey = []byte("01234567890123456789012345678901")

type stubIntegrationRepo struct {
	get    *Integration
	update func(ctx context.Context, i *Integration) error
}

func (s *stubIntegrationRepo) Create(ctx context.Context, i *Integration) error { return nil }
func (s *stubIntegrationRepo) GetByID(ctx context.Context, id uuid.UUID) (*Integration, error) {
	return nil, gorm.ErrRecordNotFound
}
func (s *stubIntegrationRepo) GetByOrganizationAndProvider(ctx context.Context, orgID uuid.UUID, provider string) (*Integration, error) {
	if s.get == nil {
		return nil, gorm.ErrRecordNotFound
	}
	return s.get, nil
}
func (s *stubIntegrationRepo) ListByOrganization(ctx context.Context, orgID uuid.UUID) ([]Integration, error) {
	return nil, nil
}
func (s *stubIntegrationRepo) Update(ctx context.Context, i *Integration) error {
	if s.update != nil {
		return s.update(ctx, i)
	}
	return nil
}
func (s *stubIntegrationRepo) Delete(ctx context.Context, id uuid.UUID) error { return nil }

func testRouterGCloud(t *testing.T, h *DashboardHandler, orgID uuid.UUID) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/organizations/:id/dashboard/gcloud/builds", func(c *gin.Context) {
		u := &user.User{ID: uuid.New(), OrganizationID: &orgID, Role: user.RoleAdmin}
		c.Set(auth.ContextUser, u)
		h.GCloudBuilds(c)
	})
	return r
}

func TestGCloudBuilds_MissingRefreshToken(t *testing.T) {
	orgID := uuid.New()
	encAccess, err := Encrypt(testEncKey, "some-access")
	if err != nil {
		t.Fatal(err)
	}
	meta, _ := json.Marshal(map[string]string{"project_id": "my-proj"})
	integ := &Integration{
		OrganizationID: orgID,
		Provider:       ProviderGCloud,
		AccessToken:    encAccess,
		RefreshToken:   "",
		Metadata:       datatypes.JSON(meta),
	}
	repo := &stubIntegrationRepo{get: integ}
	cfg := &config.IntegrationsConfig{
		EncryptionKey:      testEncKey,
		GoogleClientID:     "cid",
		GoogleClientSecret: "sec",
	}
	h := NewDashboardHandler(repo, nil, cfg)
	r := testRouterGCloud(t, h, orgID)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/organizations/"+orgID.String()+"/dashboard/gcloud/builds", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d body=%s", w.Code, w.Body.String())
	}
	var body map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body["code"] != CodeGCloudReconnectRequired {
		t.Fatalf("code: got %q want %q", body["code"], CodeGCloudReconnectRequired)
	}
	if !strings.Contains(body["error"], "refresh token") {
		t.Fatalf("error message: %q", body["error"])
	}
}

func TestGCloudBuilds_ProactiveRefreshFailsWhenAccessTokenExpired(t *testing.T) {
	orgID := uuid.New()
	encAccess, err := Encrypt(testEncKey, "stale-access")
	if err != nil {
		t.Fatal(err)
	}
	encRefresh, err := Encrypt(testEncKey, "revoked-refresh")
	if err != nil {
		t.Fatal(err)
	}
	meta, _ := json.Marshal(map[string]string{"project_id": "my-proj"})
	integ := &Integration{
		OrganizationID:       orgID,
		Provider:             ProviderGCloud,
		AccessToken:          encAccess,
		RefreshToken:         encRefresh,
		Metadata:             datatypes.JSON(meta),
		AccessTokenExpiresAt: time.Now().Add(-30 * time.Minute),
	}
	repo := &stubIntegrationRepo{get: integ}
	cfg := &config.IntegrationsConfig{
		EncryptionKey:      testEncKey,
		GoogleClientID:     "cid",
		GoogleClientSecret: "sec",
	}
	h := NewDashboardHandler(repo, nil, cfg)
	h.GCloudHTTPClient = &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			if req.URL.Host == "oauth2.googleapis.com" {
				return &http.Response{
					StatusCode: http.StatusBadRequest,
					Proto:      "HTTP/1.1",
					ProtoMajor: 1,
					ProtoMinor: 1,
					Body:       io.NopCloser(strings.NewReader(`{"error":"invalid_grant"}`)),
					Header:     make(http.Header),
				}, nil
			}
			t.Fatalf("unexpected request to %s", req.URL)
			return nil, nil
		}),
	}
	r := testRouterGCloud(t, h, orgID)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/organizations/"+orgID.String()+"/dashboard/gcloud/builds", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d body=%s", w.Code, w.Body.String())
	}
	var body map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body["code"] != CodeGCloudReconnectRequired {
		t.Fatalf("code: got %q", body["code"])
	}
	if !strings.Contains(body["error"], "could not be refreshed") {
		t.Fatalf("error: %q", body["error"])
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) { return f(req) }

func TestGCloudExecute_RefreshesOn401AndRetries(t *testing.T) {
	orgID := uuid.New()
	encAccess, _ := Encrypt(testEncKey, "old_access")
	encRefresh, _ := Encrypt(testEncKey, "my-refresh")
	meta, _ := json.Marshal(map[string]string{"project_id": "p"})
	integ := &Integration{
		OrganizationID:       orgID,
		Provider:             ProviderGCloud,
		AccessToken:          encAccess,
		RefreshToken:         encRefresh,
		Metadata:             datatypes.JSON(meta),
		AccessTokenExpiresAt: time.Now().Add(2 * time.Hour),
	}
	var gcpCalls int32
	rt := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.URL.Host == "oauth2.googleapis.com" {
			b, _ := io.ReadAll(req.Body)
			if !strings.Contains(string(b), "grant_type=refresh_token") {
				t.Errorf("expected refresh_token grant, got %q", string(b))
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Proto:      "HTTP/1.1",
				ProtoMajor: 1,
				ProtoMinor: 1,
				Body:       io.NopCloser(strings.NewReader(`{"access_token":"new_access","expires_in":3600}`)),
				Header:     make(http.Header),
			}, nil
		}
		if strings.Contains(req.URL.Host, "googleapis.com") {
			c := atomic.AddInt32(&gcpCalls, 1)
			auth := req.Header.Get("Authorization")
			if c == 1 {
				if auth != "Bearer old_access" {
					t.Errorf("first call want Bearer old_access, got %q", auth)
				}
				return &http.Response{
					StatusCode: http.StatusUnauthorized,
					Proto:      "HTTP/1.1",
					ProtoMajor: 1,
					ProtoMinor: 1,
					Body:       io.NopCloser(strings.NewReader(`{"error":{"code":401}}`)),
					Header:     make(http.Header),
				}, nil
			}
			if auth != "Bearer new_access" {
				t.Errorf("retry want Bearer new_access, got %q", auth)
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Proto:      "HTTP/1.1",
				ProtoMajor: 1,
				ProtoMinor: 1,
				Body:       io.NopCloser(strings.NewReader(`{"builds":[]}`)),
				Header:     make(http.Header),
			}, nil
		}
		t.Fatalf("unexpected host %s", req.URL.Host)
		return nil, nil
	})
	repo := &stubIntegrationRepo{get: integ}
	cfg := &config.IntegrationsConfig{
		EncryptionKey:      testEncKey,
		GoogleClientID:     "cid",
		GoogleClientSecret: "sec",
	}
	h := NewDashboardHandler(repo, nil, cfg)
	h.GCloudHTTPClient = &http.Client{Transport: rt}

	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)

	_, token, ok := h.loadIntegration(c, orgID, ProviderGCloud)
	if !ok {
		t.Fatal("loadIntegration failed")
	}
	if token != "old_access" {
		t.Fatalf("token %q want old_access", token)
	}

	resp, _, err := h.gcloudExecute(c, integ, token, func(string) (*http.Request, error) {
		u := "https://cloudbuild.googleapis.com/v1/projects/p/builds?pageSize=20"
		return http.NewRequestWithContext(c.Request.Context(), "GET", u, nil)
	})
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status %d", resp.StatusCode)
	}
	if atomic.LoadInt32(&gcpCalls) != 2 {
		t.Fatalf("gcp calls = %d want 2", gcpCalls)
	}
}
