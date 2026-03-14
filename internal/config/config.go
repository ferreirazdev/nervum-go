// Package config provides environment-based configuration for the Nervum API.
// Used by the main server to load server and database settings.
package config

import (
	"encoding/base64"
	"encoding/hex"
	"os"
	"strconv"
	"strings"
)

// Config holds server and database configuration loaded from the environment.
type Config struct {
	Server       ServerConfig
	Database     DatabaseConfig
	Integrations IntegrationsConfig
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
	Port               int
	CORSAllowedOrigins []string // from CORS_ALLOWED_ORIGINS (comma-separated); defaults to localhost:5173
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

	return &Config{
		Server: ServerConfig{
			Port:               port,
			CORSAllowedOrigins: corsOrigins,
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

func getEnv(key, defaultVal string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultVal
}
