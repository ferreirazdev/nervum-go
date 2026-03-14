package integrations

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/nervum/nervum-go/internal/config"
	"github.com/nervum/nervum-go/internal/features/auth"
	organization "github.com/nervum/nervum-go/internal/features/organizations"
	user "github.com/nervum/nervum-go/internal/features/users"
	"gorm.io/gorm"
)

// Dashboard response types matching nervum-ui mockDashboard.ts for drop-in replacement.
type (
	dashboardGitHubCommit struct {
		ID        string `json:"id"`
		Hash      string `json:"hash"`
		Message   string `json:"message"`
		Author    string `json:"author"`
		Repo      string `json:"repo"`
		CreatedAt string `json:"created_at"`
	}
	dashboardGitHubPR struct {
		ID        string `json:"id"`
		Number    int    `json:"number"`
		Title     string `json:"title"`
		State     string `json:"state"`
		Author    string `json:"author"`
		CreatedAt string `json:"created_at"`
	}
	dashboardGitHubMerge struct {
		ID           string `json:"id"`
		Title        string `json:"title"`
		SourceBranch string `json:"sourceBranch"`
		TargetBranch string `json:"targetBranch"`
		Author       string `json:"author"`
		CreatedAt    string `json:"created_at"`
	}
	dashboardGCloudBuild struct {
		ID              string `json:"id"`
		BuildID         string `json:"buildId"`
		Status          string `json:"status"`
		DurationSeconds *int   `json:"durationSeconds,omitempty"`
		Trigger         string `json:"trigger"`
		CreatedAt       string `json:"created_at"`
	}
	dashboardGCloudDeploy struct {
		ID          string `json:"id"`
		ServiceName string `json:"serviceName"`
		Revision    string `json:"revision"`
		Status      string `json:"status"`
		Region      string `json:"region"`
		CreatedAt   string `json:"created_at"`
	}
	dashboardGCloudLogEntry struct {
		ID        string `json:"id"`
		Message   string `json:"message"`
		Severity  string `json:"severity"`
		Service   string `json:"service"`
		CreatedAt string `json:"created_at"`
	}
	dashboardGCloudServiceHealth struct {
		ID     string  `json:"id"`
		Name   string  `json:"name"`
		Status string  `json:"status"`
		Detail *string `json:"detail,omitempty"`
	}
	dashboardSentryIssue struct {
		ID       string `json:"id"`
		Title    string `json:"title"`
		Level    string `json:"level"`
		Project  string `json:"project"`
		Count    string `json:"count"`
		LastSeen string `json:"last_seen"`
	}
	dashboardSentryStats struct {
		TotalIssues      int `json:"total_issues"`
		UnresolvedIssues int `json:"unresolved_issues"`
		ProjectCount     int `json:"project_count"`
	}
	dashboardSentryRelease struct {
		ID            string   `json:"id"`
		Version       string   `json:"version"`
		Project       string   `json:"project"`
		CrashFreeRate *float64 `json:"crash_free_rate,omitempty"`
		NewIssues     int      `json:"new_issues"`
		CreatedAt     string   `json:"created_at"`
	}
)

// DashboardHandler serves dashboard proxy endpoints for GitHub and GCloud data.
type DashboardHandler struct {
	repo    Repository
	orgRepo organization.Repository
	cfg     *config.IntegrationsConfig
}

// NewDashboardHandler returns a DashboardHandler.
func NewDashboardHandler(repo Repository, orgRepo organization.Repository, cfg *config.IntegrationsConfig) *DashboardHandler {
	return &DashboardHandler{repo: repo, orgRepo: orgRepo, cfg: cfg}
}

// Register mounts dashboard proxy routes under the given group. Expects group path like "/organizations".
func (h *DashboardHandler) Register(r *gin.RouterGroup) {
	g := r.Group("/:id/dashboard")
	g.GET("/github/repos", h.GitHubRepos)
	g.GET("/github/commits", h.GitHubCommits)
	g.GET("/github/pulls", h.GitHubPulls)
	g.GET("/github/merges", h.GitHubMerges)
	g.GET("/gcloud/builds", h.GCloudBuilds)
	g.GET("/gcloud/deploys", h.GCloudDeploys)
	g.GET("/gcloud/logs", h.GCloudLogs)
	g.GET("/gcloud/services-health", h.GCloudServicesHealth)
	// Cloud Run v2 API proxy (list services, get service, list revisions)
	v2 := g.Group("/gcloud/v2")
	v2.GET("/services", h.GCloudV2ServicesList)
	v2.GET("/services/:serviceName/revisions", h.GCloudV2ServiceRevisions)
	v2.GET("/services/:serviceName", h.GCloudV2ServiceGet)
	// Cloud SQL Admin API proxy
	sqlGroup := g.Group("/gcloud/sql")
	sqlGroup.GET("/instances/:instanceName/databases", h.GCloudSQLDatabases)
	sqlGroup.GET("/instances/:instanceName/backupRuns", h.GCloudSQLBackupRuns)
	sqlGroup.GET("/instances/:instanceName", h.GCloudSQLInstanceGet)
	sqlGroup.GET("/instances", h.GCloudSQLInstancesList)
	// Compute Engine API proxy
	computeGroup := g.Group("/gcloud/compute")
	computeGroup.POST("/instances/:zone/:instanceName/start", h.GCloudComputeInstanceStart)
	computeGroup.POST("/instances/:zone/:instanceName/stop", h.GCloudComputeInstanceStop)
	computeGroup.GET("/instances/:zone/:instanceName", h.GCloudComputeInstanceGet)
	computeGroup.GET("/instances", h.GCloudComputeInstancesList)
	// Sentry proxy
	g.GET("/sentry/issues", h.SentryIssues)
	g.GET("/sentry/stats", h.SentryStats)
	g.GET("/sentry/releases", h.SentryReleases)
}

func (h *DashboardHandler) requireOrgMember(c *gin.Context) (*user.User, uuid.UUID, bool) {
	u, ok := c.Get(auth.ContextUser)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
		return nil, uuid.Nil, false
	}
	currentUser := u.(*user.User)
	idStr := c.Param("id")
	if idStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "organization id required"})
		return nil, uuid.Nil, false
	}
	orgID, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid organization id"})
		return nil, uuid.Nil, false
	}
	if currentUser.OrganizationID == nil || *currentUser.OrganizationID != orgID {
		c.JSON(http.StatusForbidden, gin.H{"error": "cannot access dashboard for another organization"})
		return nil, uuid.Nil, false
	}
	return currentUser, orgID, true
}

// gcloudRefreshResponse matches Google OAuth2 token response for refresh_token grant.
type gcloudRefreshResponse struct {
	AccessToken string `json:"access_token"`
	ExpiresIn   int    `json:"expires_in"`
}

// refreshGCloudToken uses the integration's refresh_token to obtain a new access_token, updates the integration in DB, and returns the new token.
// Caller must have loaded integ with RefreshToken and use GCloud client credentials from cfg.
func (h *DashboardHandler) refreshGCloudToken(c *gin.Context, integ *Integration) (newAccessToken string, ok bool) {
	if integ.RefreshToken == "" || h.cfg.GoogleClientID == "" || h.cfg.GoogleClientSecret == "" {
		return "", false
	}
	refreshToken, err := Decrypt(h.cfg.EncryptionKey, integ.RefreshToken)
	if err != nil {
		return "", false
	}
	body := "client_id=" + url.QueryEscape(h.cfg.GoogleClientID) +
		"&client_secret=" + url.QueryEscape(h.cfg.GoogleClientSecret) +
		"&refresh_token=" + url.QueryEscape(refreshToken) +
		"&grant_type=refresh_token"
	req, err := http.NewRequestWithContext(c.Request.Context(), "POST", "https://oauth2.googleapis.com/token", bytes.NewReader([]byte(body)))
	if err != nil {
		return "", false
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", false
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", false
	}
	var tok gcloudRefreshResponse
	if err := json.NewDecoder(resp.Body).Decode(&tok); err != nil || tok.AccessToken == "" {
		return "", false
	}
	encAccess, err := Encrypt(h.cfg.EncryptionKey, tok.AccessToken)
	if err != nil {
		return "", false
	}
	integ.AccessToken = encAccess
	if tok.ExpiresIn > 0 {
		integ.AccessTokenExpiresAt = time.Now().Add(time.Duration(tok.ExpiresIn) * time.Second)
	}
	integ.UpdatedAt = time.Now()
	if err := h.repo.Update(c.Request.Context(), integ); err != nil {
		return "", false
	}
	return tok.AccessToken, true
}

func (h *DashboardHandler) loadIntegration(c *gin.Context, orgID uuid.UUID, provider string) (*Integration, string, bool) {
	integ, err := h.repo.GetByOrganizationAndProvider(c.Request.Context(), orgID, provider)
	if err != nil || integ == nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "no " + provider + " integration connected for this organization"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		}
		return nil, "", false
	}
	if len(h.cfg.EncryptionKey) != 32 {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "integration not configured"})
		return nil, "", false
	}
	token, err := Decrypt(h.cfg.EncryptionKey, integ.AccessToken)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load integration"})
		return nil, "", false
	}
	// For GCloud, refresh access token if expired or expiring within 5 minutes
	if provider == ProviderGCloud && integ.RefreshToken != "" {
		refreshThreshold := integ.AccessTokenExpiresAt.Add(-5 * time.Minute)
		if integ.AccessTokenExpiresAt.IsZero() || time.Now().After(refreshThreshold) {
			if newToken, refreshed := h.refreshGCloudToken(c, integ); refreshed {
				token = newToken
			}
		}
	}
	return integ, token, true
}

// GitHub API list repos response (minimal)
type ghRepo struct {
	ID       int    `json:"id"`
	FullName string `json:"full_name"`
	Name     string `json:"name"`
	Private  bool   `json:"private"`
	HTMLURL  string `json:"html_url"`
}

func (h *DashboardHandler) GitHubRepos(c *gin.Context) {
	_, orgID, ok := h.requireOrgMember(c)
	if !ok {
		return
	}
	_, token, ok := h.loadIntegration(c, orgID, ProviderGitHub)
	if !ok {
		return
	}
	url := "https://api.github.com/user/repos?per_page=100&sort=updated"
	req, _ := http.NewRequestWithContext(c.Request.Context(), "GET", url, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github.v3+json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "failed to call GitHub"})
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		c.JSON(resp.StatusCode, gin.H{"error": "GitHub API error"})
		return
	}
	var gh []ghRepo
	if err := json.NewDecoder(resp.Body).Decode(&gh); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to parse GitHub response"})
		return
	}
	c.JSON(http.StatusOK, gh)
}

// GitHub API response shapes (minimal)
type ghCommit struct {
	SHA    string `json:"sha"`
	Commit struct {
		Message string `json:"message"`
		Author  struct {
			Name string `json:"name"`
			Date string `json:"date"`
		} `json:"author"`
	} `json:"commit"`
	Author *struct {
		Login string `json:"login"`
	} `json:"author"`
}

type ghPull struct {
	ID        int     `json:"id"`
	Number    int     `json:"number"`
	Title     string  `json:"title"`
	State     string  `json:"state"`
	CreatedAt string  `json:"created_at"`
	MergedAt  *string `json:"merged_at"`
	User      struct {
		Login string `json:"login"`
	} `json:"user"`
	Head struct {
		Ref string `json:"ref"`
	} `json:"head"`
	Base struct {
		Ref string `json:"ref"`
	} `json:"base"`
}

func (h *DashboardHandler) GitHubCommits(c *gin.Context) {
	_, orgID, ok := h.requireOrgMember(c)
	if !ok {
		return
	}
	_, token, ok := h.loadIntegration(c, orgID, ProviderGitHub)
	if !ok {
		return
	}
	repo := c.Query("repo")
	if repo == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "query param repo=owner/repo is required"})
		return
	}
	parts := strings.SplitN(repo, "/", 2)
	if len(parts) != 2 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "repo must be owner/repo"})
		return
	}
	owner, repoName := parts[0], parts[1]
	url := "https://api.github.com/repos/" + owner + "/" + repoName + "/commits?per_page=20"
	req, _ := http.NewRequestWithContext(c.Request.Context(), "GET", url, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github.v3+json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "failed to call GitHub"})
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		c.JSON(resp.StatusCode, gin.H{"error": "GitHub API error"})
		return
	}
	var gh []ghCommit
	if err := json.NewDecoder(resp.Body).Decode(&gh); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to parse GitHub response"})
		return
	}
	out := make([]dashboardGitHubCommit, 0, len(gh))
	for i, c := range gh {
		author := c.Commit.Author.Name
		if c.Author != nil && c.Author.Login != "" {
			author = c.Author.Login
		}
		hash := c.SHA
		if len(hash) > 7 {
			hash = hash[:7]
		}
		out = append(out, dashboardGitHubCommit{
			ID:        "gh-c-" + strconv.Itoa(i+1),
			Hash:      hash,
			Message:   c.Commit.Message,
			Author:    author,
			Repo:      repoName,
			CreatedAt: c.Commit.Author.Date,
		})
	}
	c.JSON(http.StatusOK, out)
}

func (h *DashboardHandler) GitHubPulls(c *gin.Context) {
	_, orgID, ok := h.requireOrgMember(c)
	if !ok {
		return
	}
	_, token, ok := h.loadIntegration(c, orgID, ProviderGitHub)
	if !ok {
		return
	}
	repo := c.Query("repo")
	if repo == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "query param repo=owner/repo is required"})
		return
	}
	parts := strings.SplitN(repo, "/", 2)
	if len(parts) != 2 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "repo must be owner/repo"})
		return
	}
	owner, repoName := parts[0], parts[1]
	url := "https://api.github.com/repos/" + owner + "/" + repoName + "/pulls?state=all&per_page=20"
	req, _ := http.NewRequestWithContext(c.Request.Context(), "GET", url, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github.v3+json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "failed to call GitHub"})
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		c.JSON(resp.StatusCode, gin.H{"error": "GitHub API error"})
		return
	}
	var gh []ghPull
	if err := json.NewDecoder(resp.Body).Decode(&gh); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to parse GitHub response"})
		return
	}
	out := make([]dashboardGitHubPR, 0, len(gh))
	for _, p := range gh {
		state := p.State
		if p.MergedAt != nil && *p.MergedAt != "" {
			state = "merged"
		} else if state == "closed" {
			state = "closed"
		}
		out = append(out, dashboardGitHubPR{
			ID:        "gh-p-" + strconv.Itoa(p.Number),
			Number:    p.Number,
			Title:     p.Title,
			State:     state,
			Author:    p.User.Login,
			CreatedAt: p.CreatedAt,
		})
	}
	c.JSON(http.StatusOK, out)
}

func (h *DashboardHandler) GitHubMerges(c *gin.Context) {
	_, orgID, ok := h.requireOrgMember(c)
	if !ok {
		return
	}
	_, token, ok := h.loadIntegration(c, orgID, ProviderGitHub)
	if !ok {
		return
	}
	repo := c.Query("repo")
	if repo == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "query param repo=owner/repo is required"})
		return
	}
	parts := strings.SplitN(repo, "/", 2)
	if len(parts) != 2 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "repo must be owner/repo"})
		return
	}
	owner, repoName := parts[0], parts[1]
	url := "https://api.github.com/repos/" + owner + "/" + repoName + "/pulls?state=closed&per_page=20"
	req, _ := http.NewRequestWithContext(c.Request.Context(), "GET", url, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github.v3+json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "failed to call GitHub"})
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		c.JSON(resp.StatusCode, gin.H{"error": "GitHub API error"})
		return
	}
	var gh []ghPull
	if err := json.NewDecoder(resp.Body).Decode(&gh); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to parse GitHub response"})
		return
	}
	out := make([]dashboardGitHubMerge, 0)
	for i, p := range gh {
		if p.MergedAt == nil || *p.MergedAt == "" {
			continue
		}
		out = append(out, dashboardGitHubMerge{
			ID:           "gh-m-" + strconv.Itoa(i+1),
			Title:        p.Title,
			SourceBranch: p.Head.Ref,
			TargetBranch: p.Base.Ref,
			Author:       p.User.Login,
			CreatedAt:    *p.MergedAt,
		})
	}
	c.JSON(http.StatusOK, out)
}

// GCP API response shapes (minimal)
type gcpBuild struct {
	ID         string `json:"id"`
	Status     string `json:"status"`
	CreateTime string `json:"createTime"`
	FinishTime string `json:"finishTime"`
	Source     *struct {
		RepoSource *struct {
			BranchName string `json:"branchName"`
			TagName    string `json:"tagName"`
		} `json:"repoSource"`
	} `json:"source"`
	BuildTriggerID string `json:"buildTriggerId"`
}

type gcpBuildList struct {
	Builds []gcpBuild `json:"builds"`
}

func (h *DashboardHandler) gcpProjectID(c *gin.Context, integ *Integration) (string, bool) {
	if len(integ.Metadata) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "GCloud integration has no project_id in metadata; configure it first"})
		return "", false
	}
	var meta struct {
		ProjectID string `json:"project_id"`
	}
	if err := json.Unmarshal(integ.Metadata, &meta); err != nil || meta.ProjectID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "GCloud integration metadata must include project_id"})
		return "", false
	}
	return meta.ProjectID, true
}

// gcpRegion returns the Cloud Run region from integration metadata, or "us-central1" if unset.
func (h *DashboardHandler) gcpRegion(integ *Integration) string {
	if len(integ.Metadata) == 0 {
		return "us-central1"
	}
	var meta struct {
		Region string `json:"region"`
	}
	if err := json.Unmarshal(integ.Metadata, &meta); err != nil {
		return "us-central1"
	}
	if meta.Region == "" {
		return "us-central1"
	}
	return meta.Region
}

// gcpErrorMessage builds a user-facing error string for GCP API non-OK responses.
func gcpErrorMessage(apiName string, statusCode int, body []byte) string {
	base := apiName + " error"
	switch statusCode {
	case http.StatusUnauthorized:
		base = "Authentication failed; token may be expired."
	case http.StatusForbidden:
		base = apiName + " not enabled or permission denied. Enable the API in GCP Console and ensure the account has the required viewer role."
	case http.StatusNotFound:
		base = "Project or resource not found."
	}
	detail := strings.TrimSpace(string(body))
	if len(detail) > 200 {
		detail = detail[:200] + "..."
	}
	if detail != "" {
		return base + " (" + strconv.Itoa(statusCode) + ": " + detail + ")"
	}
	return base + " (" + strconv.Itoa(statusCode) + ")"
}

func (h *DashboardHandler) GCloudBuilds(c *gin.Context) {
	_, orgID, ok := h.requireOrgMember(c)
	if !ok {
		return
	}
	integ, token, ok := h.loadIntegration(c, orgID, ProviderGCloud)
	if !ok {
		return
	}
	projectID, ok := h.gcpProjectID(c, integ)
	if !ok {
		return
	}
	url := "https://cloudbuild.googleapis.com/v1/projects/" + projectID + "/builds?pageSize=20"
	req, _ := http.NewRequestWithContext(c.Request.Context(), "GET", url, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "failed to call Cloud Build API"})
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 500))
		msg := gcpErrorMessage("Cloud Build API", resp.StatusCode, body)
		c.JSON(resp.StatusCode, gin.H{"error": msg})
		return
	}
	var list gcpBuildList
	if err := json.NewDecoder(resp.Body).Decode(&list); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to parse response"})
		return
	}
	out := make([]dashboardGCloudBuild, 0, len(list.Builds))
	for i, b := range list.Builds {
		status := strings.ToLower(b.Status)
		if status == "success" || status == "failure" || status == "working" {
			// ok
		} else if status == "status_unknown" || status == "cancelled" {
			status = "failure"
		} else {
			status = "working"
		}
		trigger := "Manual"
		if b.Source != nil && b.Source.RepoSource != nil {
			if b.Source.RepoSource.BranchName != "" {
				trigger = "Branch " + b.Source.RepoSource.BranchName
			} else if b.Source.RepoSource.TagName != "" {
				trigger = "Tag " + b.Source.RepoSource.TagName
			}
		}
		var dur *int
		if b.CreateTime != "" && b.FinishTime != "" {
			t1, _ := time.Parse(time.RFC3339, b.CreateTime)
			t2, _ := time.Parse(time.RFC3339, b.FinishTime)
			if t2.After(t1) {
				sec := int(t2.Sub(t1).Seconds())
				dur = &sec
			}
		}
		out = append(out, dashboardGCloudBuild{
			ID:              "gcb-" + strconv.Itoa(i+1),
			BuildID:         b.ID,
			Status:          status,
			DurationSeconds: dur,
			Trigger:         trigger,
			CreatedAt:       b.CreateTime,
		})
	}
	c.JSON(http.StatusOK, out)
}

// Cloud Run: list services and revisions (simplified)
type gcpRunService struct {
	Name     string `json:"name"`
	Metadata struct {
		Annotations *struct {
			RunRegion string `json:"run.googleapis.com/location"`
		} `json:"annotations"`
	} `json:"metadata"`
	Status *struct {
		Conditions []struct {
			Type   string `json:"type"`
			Status string `json:"status"`
		} `json:"conditions"`
	} `json:"status"`
}

type gcpRunServiceList struct {
	Items []gcpRunService `json:"items"`
}

func (h *DashboardHandler) GCloudDeploys(c *gin.Context) {
	_, orgID, ok := h.requireOrgMember(c)
	if !ok {
		return
	}
	integ, token, ok := h.loadIntegration(c, orgID, ProviderGCloud)
	if !ok {
		return
	}
	projectID, ok := h.gcpProjectID(c, integ)
	if !ok {
		return
	}
	location := h.gcpRegion(integ)
	url := "https://run.googleapis.com/v1/projects/" + projectID + "/locations/" + location + "/services"
	req, _ := http.NewRequestWithContext(c.Request.Context(), "GET", url, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "failed to call Cloud Run API"})
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 500))
		msg := gcpErrorMessage("Cloud Run API", resp.StatusCode, body)
		c.JSON(resp.StatusCode, gin.H{"error": msg})
		return
	}
	var list gcpRunServiceList
	if err := json.NewDecoder(resp.Body).Decode(&list); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to parse response"})
		return
	}
	out := make([]dashboardGCloudDeploy, 0)
	for i, svc := range list.Items {
		name := svc.Name
		if idx := strings.LastIndex(name, "/"); idx >= 0 {
			name = name[idx+1:]
		}
		region := location
		if svc.Metadata.Annotations != nil {
			region = svc.Metadata.Annotations.RunRegion
		}
		status := "active"
		if svc.Status != nil && len(svc.Status.Conditions) > 0 {
			for _, c := range svc.Status.Conditions {
				if c.Type == "Ready" && c.Status != "True" {
					status = "deploying"
					break
				}
			}
		}
		out = append(out, dashboardGCloudDeploy{
			ID:          "gcd-" + strconv.Itoa(i+1),
			ServiceName: name,
			Revision:    name + "-00001",
			Status:      status,
			Region:      region,
			CreatedAt:   time.Now().UTC().Format(time.RFC3339),
		})
	}
	c.JSON(http.StatusOK, out)
}

// Logging API: entries.list (POST)
type gcpLogEntry struct {
	Timestamp   string                 `json:"timestamp"`
	Severity    string                 `json:"severity"`
	TextPayload string                 `json:"textPayload"`
	JSONPayload map[string]interface{} `json:"jsonPayload"`
	Resource    *struct {
		Labels *struct {
			ServiceName string `json:"service_name"`
		} `json:"labels"`
	} `json:"resource"`
}

type gcpLogListResponse struct {
	Entries []gcpLogEntry `json:"entries"`
}

func (h *DashboardHandler) GCloudLogs(c *gin.Context) {
	_, orgID, ok := h.requireOrgMember(c)
	if !ok {
		return
	}
	integ, token, ok := h.loadIntegration(c, orgID, ProviderGCloud)
	if !ok {
		return
	}
	projectID, ok := h.gcpProjectID(c, integ)
	if !ok {
		return
	}
	// Use Logging API v2 entries:list
	body := map[string]interface{}{
		"resourceNames": []string{"projects/" + projectID},
		"pageSize":      20,
		"orderBy":       "timestamp desc",
	}
	jsonBody, _ := json.Marshal(body)
	req, _ := http.NewRequestWithContext(c.Request.Context(), "POST", "https://logging.googleapis.com/v2/entries:list", bytes.NewReader(jsonBody))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "failed to call Logging API"})
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 500))
		msg := gcpErrorMessage("Logging API", resp.StatusCode, body)
		c.JSON(resp.StatusCode, gin.H{"error": msg})
		return
	}
	var list gcpLogListResponse
	if err := json.NewDecoder(resp.Body).Decode(&list); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to parse response"})
		return
	}
	out := make([]dashboardGCloudLogEntry, 0, len(list.Entries))
	for i, e := range list.Entries {
		msg := e.TextPayload
		if msg == "" && e.JSONPayload != nil {
			if m, ok := e.JSONPayload["message"].(string); ok {
				msg = m
			} else {
				msg = "[json payload]"
			}
		}
		if msg == "" {
			msg = "[no message]"
		}
		sev := strings.ToLower(e.Severity)
		if sev != "info" && sev != "warning" && sev != "error" {
			sev = "info"
		}
		svc := "unknown"
		if e.Resource != nil && e.Resource.Labels != nil {
			svc = e.Resource.Labels.ServiceName
		}
		out = append(out, dashboardGCloudLogEntry{
			ID:        "gcl-" + strconv.Itoa(i+1),
			Message:   msg,
			Severity:  sev,
			Service:   svc,
			CreatedAt: e.Timestamp,
		})
	}
	c.JSON(http.StatusOK, out)
}

func (h *DashboardHandler) GCloudServicesHealth(c *gin.Context) {
	_, orgID, ok := h.requireOrgMember(c)
	if !ok {
		return
	}
	integ, token, ok := h.loadIntegration(c, orgID, ProviderGCloud)
	if !ok {
		return
	}
	projectID, ok := h.gcpProjectID(c, integ)
	if !ok {
		return
	}
	location := h.gcpRegion(integ)
	url := "https://run.googleapis.com/v1/projects/" + projectID + "/locations/" + location + "/services"
	req, _ := http.NewRequestWithContext(c.Request.Context(), "GET", url, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "failed to call Cloud Run API"})
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 500))
		msg := gcpErrorMessage("Cloud Run API", resp.StatusCode, body)
		c.JSON(resp.StatusCode, gin.H{"error": msg})
		return
	}
	var list gcpRunServiceList
	if err := json.NewDecoder(resp.Body).Decode(&list); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to parse response"})
		return
	}
	out := make([]dashboardGCloudServiceHealth, 0, len(list.Items))
	for i, svc := range list.Items {
		name := svc.Name
		if idx := strings.LastIndex(name, "/"); idx >= 0 {
			name = name[idx+1:]
		}
		status := "healthy"
		if svc.Status != nil && len(svc.Status.Conditions) > 0 {
			for _, c := range svc.Status.Conditions {
				if c.Type == "Ready" && c.Status != "True" {
					status = "degraded"
					break
				}
			}
		}
		detail := "All instances serving"
		out = append(out, dashboardGCloudServiceHealth{
			ID:     "gch-" + strconv.Itoa(i+1),
			Name:   name,
			Status: status,
			Detail: &detail,
		})
	}
	c.JSON(http.StatusOK, out)
}

// cloudRunV2Regions is a static list of Cloud Run (fully managed) regions for listing services across all regions.
// See https://cloud.google.com/run/docs/locations
var cloudRunV2Regions = []string{
	"africa-south1", "asia-east1", "asia-east2", "asia-northeast1", "asia-northeast2", "asia-northeast3",
	"asia-south1", "asia-south2", "asia-southeast1", "asia-southeast2", "asia-southeast3",
	"australia-southeast1", "australia-southeast2", "europe-central2", "europe-north1", "europe-north2",
	"europe-southwest1", "europe-west1", "europe-west2", "europe-west3", "europe-west4", "europe-west6",
	"europe-west8", "europe-west9", "europe-west10", "europe-west12", "me-central1", "me-central2", "me-west1",
	"northamerica-northeast1", "northamerica-northeast2", "northamerica-south1", "southamerica-east1", "southamerica-west1",
	"us-central1", "us-east1", "us-east4", "us-east5", "us-south1", "us-west1", "us-west2", "us-west3", "us-west4",
}

// gcpRunV2ListResponse matches Cloud Run v2 list services response for aggregation.
type gcpRunV2ListResponse struct {
	Services      []json.RawMessage `json:"services"`
	NextPageToken string            `json:"nextPageToken"`
	Unreachable   []string          `json:"unreachable"`
}

// GCloudV2ServicesList lists Cloud Run v2 services. If query "region" is set, lists only that region; otherwise lists all regions (first page per region).
func (h *DashboardHandler) GCloudV2ServicesList(c *gin.Context) {
	_, orgID, ok := h.requireOrgMember(c)
	if !ok {
		return
	}
	integ, token, ok := h.loadIntegration(c, orgID, ProviderGCloud)
	if !ok {
		return
	}
	projectID, ok := h.gcpProjectID(c, integ)
	if !ok {
		return
	}
	regionFilter := strings.TrimSpace(c.Query("region"))
	if regionFilter != "" {
		// Single region: one API call
		reqURL := "https://run.googleapis.com/v2/projects/" + projectID + "/locations/" + url.PathEscape(regionFilter) + "/services?pageSize=100"
		if pageToken := c.Query("pageToken"); pageToken != "" {
			reqURL += "&pageToken=" + url.QueryEscape(pageToken)
		}
		req, _ := http.NewRequestWithContext(c.Request.Context(), "GET", reqURL, nil)
		req.Header.Set("Authorization", "Bearer "+token)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			c.JSON(http.StatusBadGateway, gin.H{"error": "failed to call Cloud Run API"})
			return
		}
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		if resp.StatusCode != http.StatusOK {
			msg := gcpErrorMessage("Cloud Run API", resp.StatusCode, body)
			c.JSON(resp.StatusCode, gin.H{"error": msg})
			return
		}
		c.Data(resp.StatusCode, "application/json", body)
		return
	}
	// All regions: aggregate first page per region
	var allServices []json.RawMessage
	var unreachable []string
	for _, location := range cloudRunV2Regions {
		reqURL := "https://run.googleapis.com/v2/projects/" + projectID + "/locations/" + location + "/services?pageSize=100"
		req, _ := http.NewRequestWithContext(c.Request.Context(), "GET", reqURL, nil)
		req.Header.Set("Authorization", "Bearer "+token)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			continue
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			if resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusNotFound {
				unreachable = append(unreachable, location)
			}
			continue
		}
		var list gcpRunV2ListResponse
		if err := json.Unmarshal(body, &list); err != nil {
			continue
		}
		allServices = append(allServices, list.Services...)
		if len(list.Unreachable) > 0 {
			unreachable = append(unreachable, list.Unreachable...)
		}
	}
	out := gcpRunV2ListResponse{Services: allServices, Unreachable: unreachable}
	c.Header("Content-Type", "application/json")
	json.NewEncoder(c.Writer).Encode(out)
}

// parseCloudRunV2ServiceLocation extracts location and short service name from a full resource name
// (projects/{project}/locations/{location}/services/{id}) or returns ("", name) if not in that form.
func parseCloudRunV2ServiceName(name string) (location, shortName string) {
	parts := strings.SplitN(name, "/", 10)
	if len(parts) >= 6 && parts[0] == "projects" && parts[2] == "locations" && parts[4] == "services" {
		return parts[3], parts[5]
	}
	return "", name
}

// GCloudV2ServiceGet proxies GET run.googleapis.com/v2/.../services/{serviceName}. Location from query "location" or from serviceName if full resource name.
func (h *DashboardHandler) GCloudV2ServiceGet(c *gin.Context) {
	_, orgID, ok := h.requireOrgMember(c)
	if !ok {
		return
	}
	integ, token, ok := h.loadIntegration(c, orgID, ProviderGCloud)
	if !ok {
		return
	}
	projectID, ok := h.gcpProjectID(c, integ)
	if !ok {
		return
	}
	serviceName := c.Param("serviceName")
	if serviceName == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "service name required"})
		return
	}
	location := c.Query("location")
	shortName := serviceName
	if location == "" {
		location, shortName = parseCloudRunV2ServiceName(serviceName)
	}
	if location == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "location required (query param or full resource name)"})
		return
	}
	reqURL := "https://run.googleapis.com/v2/projects/" + projectID + "/locations/" + url.PathEscape(location) + "/services/" + url.PathEscape(shortName)
	req, _ := http.NewRequestWithContext(c.Request.Context(), "GET", reqURL, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "failed to call Cloud Run API"})
		return
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		msg := gcpErrorMessage("Cloud Run API", resp.StatusCode, body)
		c.JSON(resp.StatusCode, gin.H{"error": msg})
		return
	}
	c.Data(resp.StatusCode, "application/json", body)
}

// GCloudV2ServiceRevisions proxies GET run.googleapis.com/v2/.../services/{serviceName}/revisions. Location from query "location" or from serviceName if full resource name.
func (h *DashboardHandler) GCloudV2ServiceRevisions(c *gin.Context) {
	_, orgID, ok := h.requireOrgMember(c)
	if !ok {
		return
	}
	integ, token, ok := h.loadIntegration(c, orgID, ProviderGCloud)
	if !ok {
		return
	}
	projectID, ok := h.gcpProjectID(c, integ)
	if !ok {
		return
	}
	serviceName := c.Param("serviceName")
	if serviceName == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "service name required"})
		return
	}
	location := c.Query("location")
	shortName := serviceName
	if location == "" {
		location, shortName = parseCloudRunV2ServiceName(serviceName)
	}
	if location == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "location required (query param or full resource name)"})
		return
	}
	reqURL := "https://run.googleapis.com/v2/projects/" + projectID + "/locations/" + url.PathEscape(location) + "/services/" + url.PathEscape(shortName) + "/revisions"
	if pageToken := c.Query("pageToken"); pageToken != "" {
		reqURL += "?pageToken=" + url.QueryEscape(pageToken)
	}
	req, _ := http.NewRequestWithContext(c.Request.Context(), "GET", reqURL, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "failed to call Cloud Run API"})
		return
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		msg := gcpErrorMessage("Cloud Run API", resp.StatusCode, body)
		c.JSON(resp.StatusCode, gin.H{"error": msg})
		return
	}
	c.Data(resp.StatusCode, "application/json", body)
}

// --- Cloud SQL Admin API proxy (sqladmin.googleapis.com/v1) ---

func (h *DashboardHandler) GCloudSQLInstancesList(c *gin.Context) {
	_, orgID, ok := h.requireOrgMember(c)
	if !ok {
		return
	}
	integ, token, ok := h.loadIntegration(c, orgID, ProviderGCloud)
	if !ok {
		return
	}
	projectID, ok := h.gcpProjectID(c, integ)
	if !ok {
		return
	}

	fmt.Println("--------", projectID)
	reqURL := "https://sqladmin.googleapis.com/v1/projects/" + projectID + "/instances"
	if pageToken := c.Query("pageToken"); pageToken != "" {
		reqURL += "?pageToken=" + url.QueryEscape(pageToken)
	}
	req, _ := http.NewRequestWithContext(c.Request.Context(), "GET", reqURL, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "failed to call Cloud SQL API"})
		return
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		msg := gcpErrorMessage("Cloud SQL API", resp.StatusCode, body)
		c.JSON(resp.StatusCode, gin.H{"error": msg})
		return
	}
	c.Data(resp.StatusCode, "application/json", body)
}

func (h *DashboardHandler) GCloudSQLInstanceGet(c *gin.Context) {
	_, orgID, ok := h.requireOrgMember(c)
	if !ok {
		return
	}
	integ, token, ok := h.loadIntegration(c, orgID, ProviderGCloud)
	if !ok {
		return
	}
	projectID, ok := h.gcpProjectID(c, integ)
	if !ok {
		return
	}
	instanceName := c.Param("instanceName")
	if instanceName == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "instance name required"})
		return
	}
	reqURL := "https://sqladmin.googleapis.com/v1/projects/" + projectID + "/instances/" + url.PathEscape(instanceName)
	req, _ := http.NewRequestWithContext(c.Request.Context(), "GET", reqURL, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "failed to call Cloud SQL API"})
		return
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		msg := gcpErrorMessage("Cloud SQL API", resp.StatusCode, body)
		c.JSON(resp.StatusCode, gin.H{"error": msg})
		return
	}
	c.Data(resp.StatusCode, "application/json", body)
}

func (h *DashboardHandler) GCloudSQLDatabases(c *gin.Context) {
	_, orgID, ok := h.requireOrgMember(c)
	if !ok {
		return
	}
	integ, token, ok := h.loadIntegration(c, orgID, ProviderGCloud)
	if !ok {
		return
	}
	projectID, ok := h.gcpProjectID(c, integ)
	if !ok {
		return
	}
	instanceName := c.Param("instanceName")
	if instanceName == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "instance name required"})
		return
	}
	reqURL := "https://sqladmin.googleapis.com/v1/projects/" + projectID + "/instances/" + url.PathEscape(instanceName) + "/databases"
	req, _ := http.NewRequestWithContext(c.Request.Context(), "GET", reqURL, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "failed to call Cloud SQL API"})
		return
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		msg := gcpErrorMessage("Cloud SQL API", resp.StatusCode, body)
		c.JSON(resp.StatusCode, gin.H{"error": msg})
		return
	}
	c.Data(resp.StatusCode, "application/json", body)
}

func (h *DashboardHandler) GCloudSQLBackupRuns(c *gin.Context) {
	_, orgID, ok := h.requireOrgMember(c)
	if !ok {
		return
	}
	integ, token, ok := h.loadIntegration(c, orgID, ProviderGCloud)
	if !ok {
		return
	}
	projectID, ok := h.gcpProjectID(c, integ)
	if !ok {
		return
	}
	instanceName := c.Param("instanceName")
	if instanceName == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "instance name required"})
		return
	}
	reqURL := "https://sqladmin.googleapis.com/v1/projects/" + projectID + "/instances/" + url.PathEscape(instanceName) + "/backupRuns"
	var backupQ []string
	if v := c.Query("maxResults"); v != "" {
		backupQ = append(backupQ, "maxResults="+url.QueryEscape(v))
	}
	if v := c.Query("pageToken"); v != "" {
		backupQ = append(backupQ, "pageToken="+url.QueryEscape(v))
	}
	if len(backupQ) > 0 {
		reqURL += "?" + strings.Join(backupQ, "&")
	}
	req, _ := http.NewRequestWithContext(c.Request.Context(), "GET", reqURL, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "failed to call Cloud SQL API"})
		return
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		msg := gcpErrorMessage("Cloud SQL API", resp.StatusCode, body)
		c.JSON(resp.StatusCode, gin.H{"error": msg})
		return
	}
	c.Data(resp.StatusCode, "application/json", body)
}

// --- Compute Engine API proxy (compute.googleapis.com/compute/v1) ---

func (h *DashboardHandler) GCloudComputeInstancesList(c *gin.Context) {
	_, orgID, ok := h.requireOrgMember(c)
	if !ok {
		return
	}
	integ, token, ok := h.loadIntegration(c, orgID, ProviderGCloud)
	if !ok {
		return
	}
	projectID, ok := h.gcpProjectID(c, integ)
	if !ok {
		return
	}
	reqURL := "https://compute.googleapis.com/compute/v1/projects/" + projectID + "/aggregated/instances"
	var q []string
	if v := c.Query("filter"); v != "" {
		q = append(q, "filter="+url.QueryEscape(v))
	}
	if v := c.Query("maxResults"); v != "" {
		q = append(q, "maxResults="+url.QueryEscape(v))
	}
	if v := c.Query("pageToken"); v != "" {
		q = append(q, "pageToken="+url.QueryEscape(v))
	}
	if len(q) > 0 {
		reqURL += "?" + strings.Join(q, "&")
	}
	req, _ := http.NewRequestWithContext(c.Request.Context(), "GET", reqURL, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "failed to call Compute Engine API"})
		return
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		msg := gcpErrorMessage("Compute Engine API", resp.StatusCode, body)
		c.JSON(resp.StatusCode, gin.H{"error": msg})
		return
	}
	c.Data(resp.StatusCode, "application/json", body)
}

func (h *DashboardHandler) GCloudComputeInstanceGet(c *gin.Context) {
	_, orgID, ok := h.requireOrgMember(c)
	if !ok {
		return
	}
	integ, token, ok := h.loadIntegration(c, orgID, ProviderGCloud)
	if !ok {
		return
	}
	projectID, ok := h.gcpProjectID(c, integ)
	if !ok {
		return
	}
	zone := c.Param("zone")
	instanceName := c.Param("instanceName")
	if zone == "" || instanceName == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "zone and instance name required"})
		return
	}
	reqURL := "https://compute.googleapis.com/compute/v1/projects/" + projectID + "/zones/" + url.PathEscape(zone) + "/instances/" + url.PathEscape(instanceName)
	req, _ := http.NewRequestWithContext(c.Request.Context(), "GET", reqURL, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "failed to call Compute Engine API"})
		return
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		msg := gcpErrorMessage("Compute Engine API", resp.StatusCode, body)
		c.JSON(resp.StatusCode, gin.H{"error": msg})
		return
	}
	c.Data(resp.StatusCode, "application/json", body)
}

func (h *DashboardHandler) GCloudComputeInstanceStart(c *gin.Context) {
	_, orgID, ok := h.requireOrgMember(c)
	if !ok {
		return
	}
	integ, token, ok := h.loadIntegration(c, orgID, ProviderGCloud)
	if !ok {
		return
	}
	projectID, ok := h.gcpProjectID(c, integ)
	if !ok {
		return
	}
	zone := c.Param("zone")
	instanceName := c.Param("instanceName")
	if zone == "" || instanceName == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "zone and instance name required"})
		return
	}
	reqURL := "https://compute.googleapis.com/compute/v1/projects/" + projectID + "/zones/" + url.PathEscape(zone) + "/instances/" + url.PathEscape(instanceName) + "/start"
	req, _ := http.NewRequestWithContext(c.Request.Context(), "POST", reqURL, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "failed to call Compute Engine API"})
		return
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		msg := gcpErrorMessage("Compute Engine API", resp.StatusCode, body)
		c.JSON(resp.StatusCode, gin.H{"error": msg})
		return
	}
	c.Data(resp.StatusCode, "application/json", body)
}

func (h *DashboardHandler) GCloudComputeInstanceStop(c *gin.Context) {
	_, orgID, ok := h.requireOrgMember(c)
	if !ok {
		return
	}
	integ, token, ok := h.loadIntegration(c, orgID, ProviderGCloud)
	if !ok {
		return
	}
	projectID, ok := h.gcpProjectID(c, integ)
	if !ok {
		return
	}
	zone := c.Param("zone")
	instanceName := c.Param("instanceName")
	if zone == "" || instanceName == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "zone and instance name required"})
		return
	}
	reqURL := "https://compute.googleapis.com/compute/v1/projects/" + projectID + "/zones/" + url.PathEscape(zone) + "/instances/" + url.PathEscape(instanceName) + "/stop"
	req, _ := http.NewRequestWithContext(c.Request.Context(), "POST", reqURL, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "failed to call Compute Engine API"})
		return
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		msg := gcpErrorMessage("Compute Engine API", resp.StatusCode, body)
		c.JSON(resp.StatusCode, gin.H{"error": msg})
		return
	}
	c.Data(resp.StatusCode, "application/json", body)
}

// ─── Sentry proxy ─────────────────────────────────────────────────────────────

// sentryIssueAPI is the minimal shape returned by Sentry's issues endpoint.
type sentryIssueAPI struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	Level    string `json:"level"`
	Count    string `json:"count"`
	LastSeen string `json:"lastSeen"`
	Status   string `json:"status"`
	Project  struct {
		Slug string `json:"slug"`
		Name string `json:"name"`
	} `json:"project"`
}

// sentryReleaseAPI is the minimal shape returned by Sentry's releases endpoint.
type sentryReleaseAPI struct {
	Version     string  `json:"version"`
	DateCreated string  `json:"dateCreated"`
	NewGroups   int     `json:"newGroups"`
	Projects    []struct {
		Slug string `json:"slug"`
		Name string `json:"name"`
	} `json:"projects"`
	CrashFreeSessions *float64 `json:"crashFreeSessions"`
}

func sentryOrgSlug(meta []byte) string {
	var m map[string]string
	if err := json.Unmarshal(meta, &m); err != nil {
		return ""
	}
	return m["org_slug"]
}

func sentryProjectSlug(meta []byte) string {
	var m map[string]string
	if err := json.Unmarshal(meta, &m); err != nil {
		return ""
	}
	return m["project_slug"]
}

func (h *DashboardHandler) sentryRequest(c *gin.Context, token, apiURL string) ([]byte, bool) {
	req, err := http.NewRequestWithContext(c.Request.Context(), "GET", apiURL, nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to build Sentry request"})
		return nil, false
	}
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "failed to reach Sentry API"})
		return nil, false
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		c.JSON(resp.StatusCode, gin.H{"error": "Sentry API error"})
		return nil, false
	}
	return data, true
}

func (h *DashboardHandler) SentryIssues(c *gin.Context) {
	_, orgID, ok := h.requireOrgMember(c)
	if !ok {
		return
	}
	integ, token, ok := h.loadIntegration(c, orgID, ProviderSentry)
	if !ok {
		return
	}
	orgSlug := sentryOrgSlug(integ.Metadata)
	if orgSlug == "" {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "sentry org_slug not configured"})
		return
	}
	params := url.Values{}
	params.Set("limit", "25")
	params.Set("query", "is:unresolved")
	if ps := sentryProjectSlug(integ.Metadata); ps != "" {
		params.Set("project", ps)
	}
	apiURL := "https://sentry.io/api/0/organizations/" + url.PathEscape(orgSlug) + "/issues/?" + params.Encode()
	data, ok := h.sentryRequest(c, token, apiURL)
	if !ok {
		return
	}
	var raw []sentryIssueAPI
	if err := json.Unmarshal(data, &raw); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to parse Sentry response"})
		return
	}
	out := make([]dashboardSentryIssue, 0, len(raw))
	for _, i := range raw {
		out = append(out, dashboardSentryIssue{
			ID:       i.ID,
			Title:    i.Title,
			Level:    i.Level,
			Project:  i.Project.Slug,
			Count:    i.Count,
			LastSeen: i.LastSeen,
		})
	}
	c.JSON(http.StatusOK, out)
}

func (h *DashboardHandler) SentryStats(c *gin.Context) {
	_, orgID, ok := h.requireOrgMember(c)
	if !ok {
		return
	}
	integ, token, ok := h.loadIntegration(c, orgID, ProviderSentry)
	if !ok {
		return
	}
	orgSlug := sentryOrgSlug(integ.Metadata)
	if orgSlug == "" {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "sentry org_slug not configured"})
		return
	}
	params := url.Values{}
	params.Set("limit", "100")
	if ps := sentryProjectSlug(integ.Metadata); ps != "" {
		params.Set("project", ps)
	}
	apiURL := "https://sentry.io/api/0/organizations/" + url.PathEscape(orgSlug) + "/issues/?" + params.Encode()
	data, ok := h.sentryRequest(c, token, apiURL)
	if !ok {
		return
	}
	var raw []sentryIssueAPI
	if err := json.Unmarshal(data, &raw); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to parse Sentry response"})
		return
	}
	unresolved := 0
	projects := map[string]struct{}{}
	for _, i := range raw {
		if i.Status == "unresolved" {
			unresolved++
		}
		if i.Project.Slug != "" {
			projects[i.Project.Slug] = struct{}{}
		}
	}
	c.JSON(http.StatusOK, dashboardSentryStats{
		TotalIssues:      len(raw),
		UnresolvedIssues: unresolved,
		ProjectCount:     len(projects),
	})
}

func (h *DashboardHandler) SentryReleases(c *gin.Context) {
	_, orgID, ok := h.requireOrgMember(c)
	if !ok {
		return
	}
	integ, token, ok := h.loadIntegration(c, orgID, ProviderSentry)
	if !ok {
		return
	}
	orgSlug := sentryOrgSlug(integ.Metadata)
	if orgSlug == "" {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "sentry org_slug not configured"})
		return
	}
	params := url.Values{}
	params.Set("limit", "10")
	if ps := sentryProjectSlug(integ.Metadata); ps != "" {
		params.Set("project", ps)
	}
	apiURL := "https://sentry.io/api/0/organizations/" + url.PathEscape(orgSlug) + "/releases/?" + params.Encode()
	data, ok := h.sentryRequest(c, token, apiURL)
	if !ok {
		return
	}
	var raw []sentryReleaseAPI
	if err := json.Unmarshal(data, &raw); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to parse Sentry response"})
		return
	}
	out := make([]dashboardSentryRelease, 0, len(raw))
	for _, r := range raw {
		project := ""
		if len(r.Projects) > 0 {
			project = r.Projects[0].Slug
		}
		var crashFree *float64
		if r.CrashFreeSessions != nil {
			v := *r.CrashFreeSessions * 100
			crashFree = &v
		}
		out = append(out, dashboardSentryRelease{
			ID:            r.Version,
			Version:       r.Version,
			Project:       project,
			CrashFreeRate: crashFree,
			NewIssues:     r.NewGroups,
			CreatedAt:     r.DateCreated,
		})
	}
	c.JSON(http.StatusOK, out)
}
