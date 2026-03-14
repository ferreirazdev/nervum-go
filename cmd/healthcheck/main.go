// Package main runs the Nervum health check CLI: fetches entities with health check
// configured, probes each endpoint, and updates entity status (healthy / critical).
//
// Two modes:
//   - API mode: set NERVUM_API_URL and NERVUM_SERVICE_TOKEN. The CLI calls the HTTP API (works from any host).
//   - DB mode: set DB_* env vars (same as the API). The CLI connects to Postgres directly and requires
//     NERVUM_ORGANIZATION_ID (optional NERVUM_ENVIRONMENT_ID). No token needed; use when the CLI runs where the DB is reachable (e.g. same host as API).
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/joho/godotenv"
	"github.com/nervum/nervum-go/internal/config"
	"github.com/nervum/nervum-go/internal/database"
	entity "github.com/nervum/nervum-go/internal/features/entities"
)

const (
	defaultTimeout         = 10 * time.Second
	expectedStatusDefault  = 200
	statusHealthy          = "healthy"
	statusCritical         = "critical"
)

type entityResponse struct {
	ID                        string                 `json:"id"`
	Name                      string                 `json:"name"`
	HealthCheckURL            string                 `json:"health_check_url"`
	HealthCheckMethod         string                 `json:"health_check_method"`
	HealthCheckHeaders        map[string]interface{} `json:"health_check_headers"`
	HealthCheckExpectedStatus int                    `json:"health_check_expected_status"`
}

func main() {
	envID := flag.String("env", "", "optional environment ID to scope health checks (default: all)")
	flag.Parse()

	_ = godotenv.Load()
	if envIDFromEnv := os.Getenv("NERVUM_ENVIRONMENT_ID"); *envID == "" && envIDFromEnv != "" {
		*envID = envIDFromEnv
	}

	apiURL := strings.TrimSuffix(os.Getenv("NERVUM_API_URL"), "/")
	token := os.Getenv("NERVUM_SERVICE_TOKEN")

	if apiURL != "" && token != "" {
		runAPIMode(apiURL, token, *envID)
		return
	}

	// DB mode: connect to database directly
	cfg := config.Load()
	if cfg.Database.Host == "" && cfg.Database.DBName == "" {
		log.Fatal("use either API mode (NERVUM_API_URL + NERVUM_SERVICE_TOKEN) or DB mode (DB_HOST, DB_NAME, etc.). For DB mode set DB_* env vars; NERVUM_ORGANIZATION_ID is optional to scope to one org.")
	}
	var orgIDPtr *uuid.UUID
	if orgIDStr := os.Getenv("NERVUM_ORGANIZATION_ID"); orgIDStr != "" {
		parsed, err := uuid.Parse(orgIDStr)
		if err != nil {
			log.Fatalf("invalid NERVUM_ORGANIZATION_ID: %v", err)
		}
		orgIDPtr = &parsed
	}
	var envIDPtr *uuid.UUID
	if *envID != "" {
		eid, err := uuid.Parse(*envID)
		if err != nil {
			log.Fatalf("invalid -env / NERVUM_ENVIRONMENT_ID: %v", err)
		}
		envIDPtr = &eid
	}
	db, err := database.NewDB(&cfg.Database)
	if err != nil {
		log.Fatalf("database: %v", err)
	}
	repo := entity.NewRepository(db)
	runDBMode(context.Background(), repo, orgIDPtr, envIDPtr)
}

func runAPIMode(apiURL, token, envID string) {
	client := &http.Client{Timeout: 30 * time.Second}
	listURL := apiURL + "/api/v1/entities/with-health-check"
	if envID != "" {
		listURL += "?environment_id=" + envID
	}
	req, err := http.NewRequest(http.MethodGet, listURL, nil)
	if err != nil {
		log.Fatalf("build request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		log.Fatalf("fetch entities: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		log.Fatalf("entities/with-health-check returned %d: %s", resp.StatusCode, string(body))
	}
	var list []entityResponse
	if err := json.NewDecoder(resp.Body).Decode(&list); err != nil {
		log.Fatalf("decode entities: %v", err)
	}
	if len(list) == 0 {
		log.Println("no entities with health check configured")
		os.Exit(0)
	}
	var failed int
	for _, e := range list {
		if e.HealthCheckURL == "" {
			continue
		}
		ok, err := runCheck(e.HealthCheckURL, e.HealthCheckMethod, e.HealthCheckHeaders, e.HealthCheckExpectedStatus)
		newStatus := statusHealthy
		if !ok || err != nil {
			newStatus = statusCritical
			failed++
			if err != nil {
				log.Printf("[%s] %s FAIL: %v", e.ID, e.Name, err)
			} else {
				log.Printf("[%s] %s FAIL: response not OK", e.ID, e.Name)
			}
		} else {
			log.Printf("[%s] %s OK", e.ID, e.Name)
		}
		if err := updateStatusAPI(client, apiURL, token, e.ID, newStatus); err != nil {
			log.Printf("[%s] %s update status: %v", e.ID, e.Name, err)
			failed++
		}
	}
	if failed > 0 {
		os.Exit(1)
	}
	os.Exit(0)
}

func runDBMode(ctx context.Context, repo entity.Repository, orgID *uuid.UUID, envID *uuid.UUID) {
	var list []entity.Entity
	var err error
	if orgID != nil {
		list, err = repo.ListWithHealthCheck(ctx, *orgID, envID)
	} else {
		list, err = repo.ListAllWithHealthCheck(ctx, envID)
	}
	if err != nil {
		log.Fatalf("list entities: %v", err)
	}
	if len(list) == 0 {
		log.Println("no entities with health check configured")
		os.Exit(0)
	}
	var failed int
	for i := range list {
		e := &list[i]
		if e.HealthCheckURL == "" {
			continue
		}
		headers := make(map[string]interface{})
		if e.HealthCheckHeaders != nil {
			for k, v := range e.HealthCheckHeaders {
				headers[k] = v
			}
		}
		ok, err := runCheck(e.HealthCheckURL, e.HealthCheckMethod, headers, e.HealthCheckExpectedStatus)
		newStatus := statusHealthy
		if !ok || err != nil {
			newStatus = statusCritical
			failed++
			if err != nil {
				log.Printf("[%s] %s FAIL: %v", e.ID, e.Name, err)
			} else {
				log.Printf("[%s] %s FAIL: response not OK", e.ID, e.Name)
			}
		} else {
			log.Printf("[%s] %s OK", e.ID, e.Name)
		}
		e.Status = newStatus
		if err := repo.Update(ctx, e); err != nil {
			log.Printf("[%s] %s update status: %v", e.ID, e.Name, err)
			failed++
		}
	}
	if failed > 0 {
		os.Exit(1)
	}
	os.Exit(0)
}

func runCheck(url, method string, headers map[string]interface{}, expectedStatus int) (ok bool, err error) {
	method = strings.TrimSpace(method)
	if method == "" {
		method = http.MethodGet
	}
	if expectedStatus == 0 {
		expectedStatus = expectedStatusDefault
	}
	req, err := http.NewRequest(method, url, nil)
	if err != nil {
		return false, err
	}
	for k, v := range headers {
		if s, ok := v.(string); ok {
			req.Header.Set(k, s)
		} else {
			req.Header.Set(k, fmt.Sprint(v))
		}
	}
	client := &http.Client{Timeout: defaultTimeout}
	resp, err := client.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()
	return resp.StatusCode == expectedStatus, nil
}

func updateStatusAPI(client *http.Client, apiURL, token, entityID, status string) error {
	body := map[string]string{"status": status}
	raw, _ := json.Marshal(body)
	req, err := http.NewRequest(http.MethodPut, apiURL+"/api/v1/entities/"+entityID, bytes.NewReader(raw))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("PUT returned %d: %s", resp.StatusCode, string(b))
	}
	return nil
}
