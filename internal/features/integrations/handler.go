package integrations

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/url"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/nervum/nervum-go/internal/config"
	"github.com/nervum/nervum-go/internal/features/auth"
	organization "github.com/nervum/nervum-go/internal/features/organizations"
	user "github.com/nervum/nervum-go/internal/features/users"
	"gorm.io/gorm"
)

const contextUserKey = "auth_user" // must match auth.ContextUser for protected routes

// returnToAllowlist defines allowed return_to values for OAuth redirect. Empty means default (/integrations).
func returnToAllowlist(v string) string {
	switch v {
	case "onboarding", "integrations":
		return v
	default:
		return ""
	}
}

// Handler serves HTTP for integrations: list, delete, OAuth connect and callbacks.
type Handler struct {
	repo    Repository
	orgRepo organization.Repository
	cfg     *config.IntegrationsConfig
}

// NewHandler returns an integrations Handler.
func NewHandler(repo Repository, orgRepo organization.Repository, cfg *config.IntegrationsConfig) *Handler {
	return &Handler{repo: repo, orgRepo: orgRepo, cfg: cfg}
}

// Register registers routes. Connect endpoints require auth; callbacks do not (state is verified).
func (h *Handler) Register(r *gin.RouterGroup) {
	g := r.Group("/integrations")
	g.GET("", h.List)
	g.PUT("/:id", h.Update)
	g.DELETE("/:id", h.Delete)
	g.GET("/github/connect", h.GitHubConnect)
	g.GET("/gcloud/connect", h.GCloudConnect)
	g.POST("/sentry/connect", h.SentryConnect)
}

// RegisterPublic registers callback routes without auth (state binds to org).
func (h *Handler) RegisterPublic(r *gin.RouterGroup) {
	g := r.Group("/integrations")
	g.GET("/github/callback", h.GitHubCallback)
	g.GET("/gcloud/callback", h.GCloudCallback)
}

// listResponse omits tokens.
type listResponse struct {
	ID             string          `json:"id"`
	OrganizationID string          `json:"organization_id"`
	Provider       string          `json:"provider"`
	Scopes         string          `json:"scopes,omitempty"`
	ConnectedAt    time.Time       `json:"connected_at"`
	Metadata       json.RawMessage `json:"metadata,omitempty"`
	CreatedAt      time.Time       `json:"created_at"`
	UpdatedAt      time.Time       `json:"updated_at"`
}

func (h *Handler) List(c *gin.Context) {
	u, ok := c.Get(auth.ContextUser)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
		return
	}
	currentUser := u.(*user.User)
	orgIDStr := c.Query("organization_id")
	if orgIDStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "organization_id required"})
		return
	}
	orgID, err := uuid.Parse(orgIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid organization_id"})
		return
	}
	if currentUser.OrganizationID == nil || *currentUser.OrganizationID != orgID {
		c.JSON(http.StatusForbidden, gin.H{"error": "cannot list integrations for another organization"})
		return
	}
	list, err := h.repo.ListByOrganization(c.Request.Context(), orgID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	out := make([]listResponse, 0, len(list))
	for _, i := range list {
		meta := json.RawMessage(i.Metadata)
		if meta == nil {
			meta = []byte("null")
		}
		out = append(out, listResponse{
			ID:             i.ID.String(),
			OrganizationID: i.OrganizationID.String(),
			Provider:       i.Provider,
			Scopes:         i.Scopes,
			ConnectedAt:    i.ConnectedAt,
			Metadata:       meta,
			CreatedAt:      i.CreatedAt,
			UpdatedAt:      i.UpdatedAt,
		})
	}
	c.JSON(http.StatusOK, out)
}

func (h *Handler) Delete(c *gin.Context) {
	u, ok := c.Get(auth.ContextUser)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
		return
	}
	currentUser := u.(*user.User)
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	integration, err := h.repo.GetByID(c.Request.Context(), id)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	if currentUser.OrganizationID == nil || *currentUser.OrganizationID != integration.OrganizationID {
		c.JSON(http.StatusForbidden, gin.H{"error": "cannot delete integration for another organization"})
		return
	}
	org, err := h.orgRepo.GetByID(c.Request.Context(), integration.OrganizationID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	canEdit := user.CanEditOrganization(currentUser.Role) || (org.OwnerID != nil && *org.OwnerID == currentUser.ID)
	if !canEdit {
		c.JSON(http.StatusForbidden, gin.H{"error": "only organization owner or admin can disconnect integrations"})
		return
	}
	if err := h.repo.Delete(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	c.Status(http.StatusNoContent)
}

// updateMetadataRequest is the body for PUT /integrations/:id (metadata only).
type updateMetadataRequest struct {
	Metadata map[string]string `json:"metadata"`
}

func (h *Handler) Update(c *gin.Context) {
	u, ok := c.Get(auth.ContextUser)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
		return
	}
	currentUser := u.(*user.User)
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	integration, err := h.repo.GetByID(c.Request.Context(), id)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	if currentUser.OrganizationID == nil || *currentUser.OrganizationID != integration.OrganizationID {
		c.JSON(http.StatusForbidden, gin.H{"error": "cannot update integration for another organization"})
		return
	}
	org, err := h.orgRepo.GetByID(c.Request.Context(), integration.OrganizationID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	canEdit := user.CanEditOrganization(currentUser.Role) || (org.OwnerID != nil && *org.OwnerID == currentUser.ID)
	if !canEdit {
		c.JSON(http.StatusForbidden, gin.H{"error": "only organization owner or admin can update integrations"})
		return
	}
	var body updateMetadataRequest
	if err := c.ShouldBindJSON(&body); err != nil || body.Metadata == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "metadata object required"})
		return
	}
	metaJSON, err := json.Marshal(body.Metadata)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid metadata"})
		return
	}
	integration.Metadata = metaJSON
	integration.UpdatedAt = time.Now()
	if err := h.repo.Update(c.Request.Context(), integration); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	meta := json.RawMessage(integration.Metadata)
	if meta == nil {
		meta = []byte("null")
	}
	c.JSON(http.StatusOK, listResponse{
		ID:             integration.ID.String(),
		OrganizationID: integration.OrganizationID.String(),
		Provider:       integration.Provider,
		Scopes:         integration.Scopes,
		ConnectedAt:    integration.ConnectedAt,
		Metadata:       meta,
		CreatedAt:      integration.CreatedAt,
		UpdatedAt:      integration.UpdatedAt,
	})
}

func (h *Handler) GitHubConnect(c *gin.Context) {
	u, ok := c.Get(auth.ContextUser)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
		return
	}
	currentUser := u.(*user.User)
	orgIDStr := c.Query("organization_id")
	if orgIDStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "organization_id required"})
		return
	}
	orgID, err := uuid.Parse(orgIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid organization_id"})
		return
	}
	if currentUser.OrganizationID == nil || *currentUser.OrganizationID != orgID {
		c.JSON(http.StatusForbidden, gin.H{"error": "cannot connect integration for another organization"})
		return
	}
	org, err := h.orgRepo.GetByID(c.Request.Context(), orgID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	canEdit := user.CanEditOrganization(currentUser.Role) || (org.OwnerID != nil && *org.OwnerID == currentUser.ID)
	if !canEdit {
		c.JSON(http.StatusForbidden, gin.H{"error": "only organization owner or admin can connect integrations"})
		return
	}
	if h.cfg.GitHubClientID == "" || h.cfg.GitHubClientSecret == "" {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "GitHub integration is not configured"})
		return
	}
	if len(h.cfg.EncryptionKey) != 32 {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "integration encryption key is not configured or invalid (must be 32 bytes)"})
		return
	}
	returnTo := returnToAllowlist(c.Query("return_to"))
	var state string
	if returnTo != "" {
		state, err = EncodeStateWithReturn(h.cfg.EncryptionKey, orgID, returnTo)
	} else {
		state, err = EncodeState(h.cfg.EncryptionKey, orgID)
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to build state"})
		return
	}
	redirectURI := h.cfg.APIBaseURL + "/api/v1/integrations/github/callback"
	scope := "repo read:org workflow"
	params := url.Values{}
	params.Set("client_id", h.cfg.GitHubClientID)
	params.Set("redirect_uri", redirectURI)
	params.Set("scope", scope)
	params.Set("state", state)
	urlStr := "https://github.com/login/oauth/authorize?" + params.Encode()
	c.Redirect(http.StatusFound, urlStr)
}

// githubTokenResponse matches GitHub's OAuth token response (Accept: application/json).
type githubTokenResponse struct {
	AccessToken string `json:"access_token"`
	Scope       string `json:"scope"`
	TokenType   string `json:"token_type"`
}

func (h *Handler) GitHubCallback(c *gin.Context) {
	code := c.Query("code")
	state := c.Query("state")
	if code == "" || state == "" {
		redirectFail(h.cfg.FrontendURL, c, "missing code or state")
		return
	}
	if len(h.cfg.EncryptionKey) < 32 {
		redirectFail(h.cfg.FrontendURL, c, "integration not configured")
		return
	}
	orgID, returnTo, err := DecodeStateWithReturn(h.cfg.EncryptionKey, state)
	if err != nil {
		redirectFail(h.cfg.FrontendURL, c, "invalid state")
		return
	}
	redirectURI := h.cfg.APIBaseURL + "/api/v1/integrations/github/callback"
	body := map[string]string{
		"client_id":     h.cfg.GitHubClientID,
		"client_secret": h.cfg.GitHubClientSecret,
		"code":          code,
		"redirect_uri":  redirectURI,
	}
	jsonBody, _ := json.Marshal(body)
	req, _ := http.NewRequestWithContext(c.Request.Context(), "POST", "https://github.com/login/oauth/access_token", bytes.NewReader(jsonBody))
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		redirectFail(h.cfg.FrontendURL, c, "token exchange failed")
		return
	}
	defer resp.Body.Close()
	var tok githubTokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tok); err != nil || tok.AccessToken == "" {
		redirectFail(h.cfg.FrontendURL, c, "invalid token response")
		return
	}
	encAccess, err := Encrypt(h.cfg.EncryptionKey, tok.AccessToken)
	if err != nil {
		redirectFail(h.cfg.FrontendURL, c, "failed to store token")
		return
	}
	existing, _ := h.repo.GetByOrganizationAndProvider(c.Request.Context(), orgID, ProviderGitHub)
	now := time.Now()
	if existing != nil {
		existing.AccessToken = encAccess
		existing.Scopes = tok.Scope
		existing.ConnectedAt = now
		existing.UpdatedAt = now
		_ = h.repo.Update(c.Request.Context(), existing)
	} else {
		integ := &Integration{
			OrganizationID: orgID,
			Provider:       ProviderGitHub,
			AccessToken:    encAccess,
			Scopes:         tok.Scope,
			ConnectedAt:    now,
		}
		_ = h.repo.Create(c.Request.Context(), integ)
	}
	path := "/integrations?github=connected"
	if returnTo == "onboarding" {
		path = "/onboarding?github=connected"
	}
	c.Redirect(http.StatusFound, h.cfg.FrontendURL+path)
}

func (h *Handler) GCloudConnect(c *gin.Context) {
	u, ok := c.Get(auth.ContextUser)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
		return
	}
	currentUser := u.(*user.User)
	orgIDStr := c.Query("organization_id")
	if orgIDStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "organization_id required"})
		return
	}
	orgID, err := uuid.Parse(orgIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid organization_id"})
		return
	}
	if currentUser.OrganizationID == nil || *currentUser.OrganizationID != orgID {
		c.JSON(http.StatusForbidden, gin.H{"error": "cannot connect integration for another organization"})
		return
	}
	org, err := h.orgRepo.GetByID(c.Request.Context(), orgID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	canEdit := user.CanEditOrganization(currentUser.Role) || (org.OwnerID != nil && *org.OwnerID == currentUser.ID)
	if !canEdit {
		c.JSON(http.StatusForbidden, gin.H{"error": "only organization owner or admin can connect integrations"})
		return
	}
	if h.cfg.GoogleClientID == "" || h.cfg.GoogleClientSecret == "" {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Google Cloud integration is not configured"})
		return
	}
	if len(h.cfg.EncryptionKey) != 32 {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "integration encryption key is not configured or invalid (must be 32 bytes)"})
		return
	}
	returnTo := returnToAllowlist(c.Query("return_to"))
	var state string
	if returnTo != "" {
		state, err = EncodeStateWithReturn(h.cfg.EncryptionKey, orgID, returnTo)
	} else {
		state, err = EncodeState(h.cfg.EncryptionKey, orgID)
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to build state"})
		return
	}
	redirectURI := h.cfg.APIBaseURL + "/api/v1/integrations/gcloud/callback"
	scope := "https://www.googleapis.com/auth/cloud-platform https://www.googleapis.com/auth/logging.read https://www.googleapis.com/auth/monitoring.read"
	params := url.Values{}
	params.Set("client_id", h.cfg.GoogleClientID)
	params.Set("redirect_uri", redirectURI)
	params.Set("response_type", "code")
	params.Set("scope", scope)
	params.Set("access_type", "offline")
	params.Set("prompt", "consent")
	params.Set("state", state)
	urlStr := "https://accounts.google.com/o/oauth2/v2/auth?" + params.Encode()
	c.Redirect(http.StatusFound, urlStr)
}

// gcloudTokenResponse matches Google OAuth2 token response.
type gcloudTokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
	TokenType    string `json:"token_type"`
	Scope        string `json:"scope"`
}

func (h *Handler) GCloudCallback(c *gin.Context) {
	code := c.Query("code")
	state := c.Query("state")
	if code == "" || state == "" {
		redirectFail(h.cfg.FrontendURL, c, "missing code or state")
		return
	}
	if len(h.cfg.EncryptionKey) < 32 {
		redirectFail(h.cfg.FrontendURL, c, "integration not configured")
		return
	}
	orgID, returnTo, err := DecodeStateWithReturn(h.cfg.EncryptionKey, state)
	if err != nil {
		redirectFail(h.cfg.FrontendURL, c, "invalid state")
		return
	}
	redirectURI := h.cfg.APIBaseURL + "/api/v1/integrations/gcloud/callback"
	body := "client_id=" + h.cfg.GoogleClientID +
		"&client_secret=" + h.cfg.GoogleClientSecret +
		"&code=" + code +
		"&grant_type=authorization_code&redirect_uri=" + redirectURI
	req, _ := http.NewRequestWithContext(c.Request.Context(), "POST", "https://oauth2.googleapis.com/token", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		redirectFail(h.cfg.FrontendURL, c, "token exchange failed")
		return
	}
	defer resp.Body.Close()
	var tok gcloudTokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tok); err != nil || tok.AccessToken == "" {
		redirectFail(h.cfg.FrontendURL, c, "invalid token response")
		return
	}
	encAccess, err := Encrypt(h.cfg.EncryptionKey, tok.AccessToken)
	if err != nil {
		redirectFail(h.cfg.FrontendURL, c, "failed to store token")
		return
	}
	encRefresh := ""
	if tok.RefreshToken != "" {
		encRefresh, err = Encrypt(h.cfg.EncryptionKey, tok.RefreshToken)
		if err != nil {
			encRefresh = ""
		}
	}
	existing, _ := h.repo.GetByOrganizationAndProvider(c.Request.Context(), orgID, ProviderGCloud)
	now := time.Now()
	expiresAt := now.Add(time.Duration(tok.ExpiresIn) * time.Second)
	if existing != nil {
		existing.AccessToken = encAccess
		existing.RefreshToken = encRefresh
		existing.AccessTokenExpiresAt = expiresAt
		existing.Scopes = tok.Scope
		existing.ConnectedAt = now
		existing.UpdatedAt = now
		_ = h.repo.Update(c.Request.Context(), existing)
	} else {
		integ := &Integration{
			OrganizationID:       orgID,
			Provider:             ProviderGCloud,
			AccessToken:          encAccess,
			RefreshToken:         encRefresh,
			AccessTokenExpiresAt: expiresAt,
			Scopes:               tok.Scope,
			ConnectedAt:          now,
		}
		_ = h.repo.Create(c.Request.Context(), integ)
	}
	path := "/integrations?gcloud=connected"
	if returnTo == "onboarding" {
		path = "/onboarding?gcloud=connected"
	}
	c.Redirect(http.StatusFound, h.cfg.FrontendURL+path)
}

func redirectFail(frontendURL string, c *gin.Context, reason string) {
	c.Redirect(http.StatusFound, frontendURL+"/integrations?error="+url.QueryEscape(reason))
}

// sentryConnectRequest is the body for POST /integrations/sentry/connect.
type sentryConnectRequest struct {
	Token       string `json:"token"`
	OrgSlug     string `json:"org_slug"`
	ProjectSlug string `json:"project_slug"`
}

// SentryConnect saves a Sentry Auth Token for the organization after validating it against the Sentry API.
func (h *Handler) SentryConnect(c *gin.Context) {
	u, ok := c.Get(auth.ContextUser)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
		return
	}
	currentUser := u.(*user.User)

	var body sentryConnectRequest
	if err := c.ShouldBindJSON(&body); err != nil || body.Token == "" || body.OrgSlug == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "token and org_slug are required"})
		return
	}

	if currentUser.OrganizationID == nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "user has no organization"})
		return
	}
	orgID := *currentUser.OrganizationID

	org, err := h.orgRepo.GetByID(c.Request.Context(), orgID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	canEdit := user.CanEditOrganization(currentUser.Role) || (org.OwnerID != nil && *org.OwnerID == currentUser.ID)
	if !canEdit {
		c.JSON(http.StatusForbidden, gin.H{"error": "only organization owner or admin can connect integrations"})
		return
	}
	if len(h.cfg.EncryptionKey) != 32 {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "integration encryption key is not configured"})
		return
	}

	// Validate the token by calling the Sentry organizations endpoint.
	validateURL := "https://sentry.io/api/0/organizations/" + url.PathEscape(body.OrgSlug) + "/"
	req, err := http.NewRequestWithContext(c.Request.Context(), "GET", validateURL, nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to build validation request"})
		return
	}
	req.Header.Set("Authorization", "Bearer "+body.Token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "failed to reach Sentry API"})
		return
	}

	defer resp.Body.Close()
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "invalid Sentry token or organization slug"})
		return
	}
	if resp.StatusCode != http.StatusOK {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "Sentry API returned an error — check your token and organization slug"})
		return
	}

	encToken, err := Encrypt(h.cfg.EncryptionKey, body.Token)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to store token"})
		return
	}

	meta := map[string]string{"org_slug": body.OrgSlug}
	if body.ProjectSlug != "" {
		meta["project_slug"] = body.ProjectSlug
	}
	metaJSON, _ := json.Marshal(meta)

	now := time.Now()
	existing, _ := h.repo.GetByOrganizationAndProvider(c.Request.Context(), orgID, ProviderSentry)
	if existing != nil {
		existing.AccessToken = encToken
		existing.Metadata = metaJSON
		existing.ConnectedAt = now
		existing.UpdatedAt = now
		if err := h.repo.Update(c.Request.Context(), existing); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
			return
		}
		c.JSON(http.StatusOK, listResponse{
			ID:             existing.ID.String(),
			OrganizationID: existing.OrganizationID.String(),
			Provider:       existing.Provider,
			ConnectedAt:    existing.ConnectedAt,
			Metadata:       metaJSON,
			CreatedAt:      existing.CreatedAt,
			UpdatedAt:      existing.UpdatedAt,
		})
		return
	}

	integ := &Integration{
		OrganizationID: orgID,
		Provider:       ProviderSentry,
		AccessToken:    encToken,
		Metadata:       metaJSON,
		ConnectedAt:    now,
	}
	if err := h.repo.Create(c.Request.Context(), integ); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	c.JSON(http.StatusCreated, listResponse{
		ID:             integ.ID.String(),
		OrganizationID: integ.OrganizationID.String(),
		Provider:       integ.Provider,
		ConnectedAt:    integ.ConnectedAt,
		Metadata:       metaJSON,
		CreatedAt:      integ.CreatedAt,
		UpdatedAt:      integ.UpdatedAt,
	})
}
