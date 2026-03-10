package integrations

import (
	"bytes"
	"encoding/json"
	"net/http"
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
		ID              string  `json:"id"`
		BuildID         string  `json:"buildId"`
		Status          string  `json:"status"`
		DurationSeconds *int    `json:"durationSeconds,omitempty"`
		Trigger         string  `json:"trigger"`
		CreatedAt       string  `json:"created_at"`
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
	ID        int    `json:"id"`
	Number    int    `json:"number"`
	Title     string `json:"title"`
	State     string `json:"state"`
	CreatedAt string `json:"created_at"`
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
			TagName   string `json:"tagName"`
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
		c.JSON(resp.StatusCode, gin.H{"error": "Cloud Build API error"})
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
	// List services in us-central1 (default); could be made configurable
	location := "us-central1"
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
		c.JSON(resp.StatusCode, gin.H{"error": "Cloud Run API error"})
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
	Timestamp  string `json:"timestamp"`
	Severity   string `json:"severity"`
	TextPayload string `json:"textPayload"`
	JSONPayload  map[string]interface{} `json:"jsonPayload"`
	Resource   *struct {
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
		c.JSON(resp.StatusCode, gin.H{"error": "Logging API error"})
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
	location := "us-central1"
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
		c.JSON(resp.StatusCode, gin.H{"error": "Cloud Run API error"})
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
