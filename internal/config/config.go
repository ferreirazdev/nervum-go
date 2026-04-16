// Package config provides environment-based configuration for the Nervum API.
// Used by the main server to load server and database settings.
package config

import (
	"encoding/base64"
	"encoding/hex"
	"net/http"
	"os"
	"strconv"
	"strings"
)

// Config holds server and database configuration loaded from the environment.
type Config struct {
	Server              ServerConfig
	Database            DatabaseConfig
	Integrations        IntegrationsConfig
	Stripe              StripeConfig
	InternalAdminEmails []string // INTERNAL_ADMIN_EMAILS (comma-separated); default ferreirazdev@gmail.com when unset
}

// StripeConfig holds Stripe Billing API keys (Checkout, Customer Portal, webhooks).
type StripeConfig struct {
	SecretKey       string // STRIPE_SECRET_KEY
	WebhookSecret   string // STRIPE_WEBHOOK_SECRET (optional in dev; required for webhook verification)
	TrialPeriodDays int64  // STRIPE_TRIAL_PERIOD_DAYS (default 15)
}

// IntegrationsConfig holds OAuth and encryption settings for org integrations.
type IntegrationsConfig struct {
	EncryptionKey      []byte // 32 bytes for AES-256; from INTEGRATION_ENCRYPTION_KEY (hex or base64)
	FrontendURL        string // Redirect after OAuth callback
	APIBaseURL         string // Backend base URL for OAuth callback (e.g. http://localhost:8080)
	GitHubClientID     string
	GitHubClientSecret string
	GoogleClientID     string
	GoogleClientSecret string
}

// ServerConfig holds HTTP server settings.
type ServerConfig struct {
	Port                  int
	SessionCookieSameSite http.SameSite // from SESSION_COOKIE_SAMESITE: strict (default), lax, none — use none for SPA on another origin
	SessionCookieSecure   bool          // from SESSION_COOKIE_SECURE (default true); set false only for local HTTP API
	TrustedProxies        []string      // from TRUSTED_PROXIES (comma-separated CIDRs); empty means trust no proxy headers (direct RemoteAddr)
	CORSAllowedOrigins    []string      // from CORS_ALLOWED_ORIGINS (comma-separated); defaults to localhost:5173
	// Service token auth for CLI/automation: when Authorization: Bearer <token> matches ServiceToken,
	// request is treated as authenticated as ServiceUserID (UUID of an existing user in that org).
	ServiceToken  string // NERVUM_SERVICE_TOKEN
	ServiceUserID string // NERVUM_SERVICE_USER_ID (UUID)
}

// DatabaseConfig holds Postgres connection settings. Corresponds to env vars
// PORT, DB_HOST, DB_PORT, DB_USER, DB_PASSWORD, DB_NAME, DB_SSLMODE.
type DatabaseConfig struct {
	Host     string
	Port     int
	User     string
	Password string
	DBName   string
	SSLMode  string
}

// Load reads configuration from the environment and returns a Config.
// Uses defaults for PORT (8080), DB_* (localhost, postgres, nervum, etc.) when unset.
func Load() *Config {
	port, _ := strconv.Atoi(getEnv("PORT", "8080"))
	dbPort, _ := strconv.Atoi(getEnv("DB_PORT", "5432"))

	corsOrigins := []string{"http://localhost:5173"}
	if raw := os.Getenv("CORS_ALLOWED_ORIGINS"); raw != "" {
		corsOrigins = strings.Split(raw, ",")
		for i, o := range corsOrigins {
			corsOrigins[i] = strings.TrimSpace(o)
		}
	}

	integrations := IntegrationsConfig{
		FrontendURL:        getEnv("FRONTEND_URL", "http://localhost:5173"),
		APIBaseURL:         getEnv("API_BASE_URL", "http://localhost:8080"),
		GitHubClientID:     getEnv("GITHUB_CLIENT_ID", ""),
		GitHubClientSecret: getEnv("GITHUB_CLIENT_SECRET", ""),
		GoogleClientID:     getEnv("GOOGLE_CLIENT_ID", ""),
		GoogleClientSecret: getEnv("GOOGLE_CLIENT_SECRET", ""),
	}
	if k := getEnv("INTEGRATION_ENCRYPTION_KEY", ""); k != "" {
		if raw, err := hex.DecodeString(k); err == nil && len(raw) == 32 {
			integrations.EncryptionKey = raw
		} else if raw, err := base64.StdEncoding.DecodeString(k); err == nil && len(raw) == 32 {
			integrations.EncryptionKey = raw
		}
		// If decode fails or length != 32, EncryptionKey stays nil; connect will fail with clear error
	}

	trialDays, _ := strconv.ParseInt(getEnv("STRIPE_TRIAL_PERIOD_DAYS", "15"), 10, 64)
	if trialDays <= 0 {
		trialDays = 15
	}

	internalAdmins := parseCommaSeparated(os.Getenv("INTERNAL_ADMIN_EMAILS"))
	for i := range internalAdmins {
		internalAdmins[i] = strings.ToLower(strings.TrimSpace(internalAdmins[i]))
	}
	if len(internalAdmins) == 0 {
		internalAdmins = []string{"ferreirazdev@gmail.com"}
	}

	return &Config{
		InternalAdminEmails: internalAdmins,
		Stripe: StripeConfig{
			SecretKey:       getEnv("STRIPE_SECRET_KEY", ""),
			WebhookSecret:   getEnv("STRIPE_WEBHOOK_SECRET", ""),
			TrialPeriodDays: trialDays,
		},
		Server: ServerConfig{
			Port:                  port,
			SessionCookieSameSite: parseSessionCookieSameSite(getEnv("SESSION_COOKIE_SAMESITE", "strict")),
			SessionCookieSecure:   parseEnvBoolDefaultTrue("SESSION_COOKIE_SECURE"),
			TrustedProxies:        parseCommaSeparated(getEnv("TRUSTED_PROXIES", "")),
			CORSAllowedOrigins:    corsOrigins,
			ServiceToken:          getEnv("NERVUM_SERVICE_TOKEN", ""),
			ServiceUserID:         getEnv("NERVUM_SERVICE_USER_ID", ""),
		},
		Database: DatabaseConfig{
			Host:     getEnv("DB_HOST", "localhost"),
			Port:     dbPort,
			User:     getEnv("DB_USER", "postgres"),
			Password: getEnv("DB_PASSWORD", "postgres"),
			DBName:   getEnv("DB_NAME", "nervum"),
			SSLMode:  getEnv("DB_SSLMODE", "disable"),
		},
		Integrations: integrations,
	}
}

func parseCommaSeparated(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// parseEnvBoolDefaultTrue returns true when unset or invalid; false only for "0", "false", "no" (case-insensitive).
func parseEnvBoolDefaultTrue(key string) bool {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return true
	}
	switch strings.ToLower(v) {
	case "0", "false", "no", "off":
		return false
	default:
		b, err := strconv.ParseBool(v)
		if err != nil {
			return true
		}
		return b
	}
}

func parseSessionCookieSameSite(raw string) http.SameSite {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "none":
		return http.SameSiteNoneMode
	case "lax":
		return http.SameSiteLaxMode
	default:
		return http.SameSiteStrictMode
	}
}

func getEnv(key, defaultVal string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultVal
}
