// Package config provides environment-based configuration for the Nervum API.
// Used by the main server to load server and database settings.
package config

import (
	"os"
	"strconv"
)

// Config holds server and database configuration loaded from the environment.
type Config struct {
	Server   ServerConfig
	Database DatabaseConfig
}

// ServerConfig holds HTTP server settings (e.g. port).
type ServerConfig struct {
	Port int
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

	return &Config{
		Server: ServerConfig{
			Port: port,
		},
		Database: DatabaseConfig{
			Host:     getEnv("DB_HOST", "localhost"),
			Port:     dbPort,
			User:     getEnv("DB_USER", "postgres"),
			Password: getEnv("DB_PASSWORD", "postgres"),
			DBName:   getEnv("DB_NAME", "nervum"),
			SSLMode:  getEnv("DB_SSLMODE", "disable"),
		},
	}
}

func getEnv(key, defaultVal string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultVal
}
