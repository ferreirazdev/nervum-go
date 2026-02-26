//go:build integration

package database

import (
	"os"
	"testing"

	"github.com/nervum/nervum-go/internal/config"
)

func TestNewDB_Integration(t *testing.T) {
	cfg := &config.DatabaseConfig{
		Host:     getEnv("DB_HOST", "localhost"),
		Port:     5432,
		User:     getEnv("DB_USER", "postgres"),
		Password: getEnv("DB_PASSWORD", "postgres"),
		DBName:   getEnv("DB_NAME", "nervum_test"),
		SSLMode:  "disable",
	}
	db, err := NewDB(cfg)
	if err != nil {
		t.Skipf("skip integration test (no postgres): %v", err)
		return
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("db.DB(): %v", err)
	}
	defer sqlDB.Close()
	if err := sqlDB.Ping(); err != nil {
		t.Fatalf("ping: %v", err)
	}
}

func TestAutoMigrate_Integration(t *testing.T) {
	cfg := &config.DatabaseConfig{
		Host:     getEnv("DB_HOST", "localhost"),
		Port:     5432,
		User:     getEnv("DB_USER", "postgres"),
		Password: getEnv("DB_PASSWORD", "postgres"),
		DBName:   getEnv("DB_NAME", "nervum_test"),
		SSLMode:  "disable",
	}
	db, err := NewDB(cfg)
	if err != nil {
		t.Skipf("skip integration test (no postgres): %v", err)
		return
	}
	sqlDB, _ := db.DB()
	defer sqlDB.Close()
	if err := AutoMigrate(db); err != nil {
		t.Fatalf("AutoMigrate: %v", err)
	}
	// Run again (idempotent)
	if err := AutoMigrate(db); err != nil {
		t.Fatalf("AutoMigrate second run: %v", err)
	}
}

func getEnv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}