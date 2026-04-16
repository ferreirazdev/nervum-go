package config

import (
	"testing"
)

func TestLoad_Unit(t *testing.T) {
	t.Run("defaults", func(t *testing.T) {
		// Unset env vars that might be set in CI
		t.Setenv("PORT", "")
		t.Setenv("DB_PORT", "")
		t.Setenv("DB_HOST", "")
		t.Setenv("DB_USER", "")
		t.Setenv("DB_PASSWORD", "")
		t.Setenv("DB_NAME", "")
		t.Setenv("DB_SSLMODE", "")
		t.Setenv("SESSION_COOKIE_SECURE", "")
		t.Setenv("TRUSTED_PROXIES", "")
		t.Setenv("INTERNAL_ADMIN_EMAILS", "")
		cfg := Load()
		if cfg.Server.Port != 8080 {
			t.Errorf("Server.Port = %d, want 8080", cfg.Server.Port)
		}
		if cfg.Database.Port != 5432 {
			t.Errorf("Database.Port = %d, want 5432", cfg.Database.Port)
		}
		if cfg.Database.Host != "localhost" {
			t.Errorf("Database.Host = %q, want localhost", cfg.Database.Host)
		}
		if cfg.Database.User != "postgres" {
			t.Errorf("Database.User = %q, want postgres", cfg.Database.User)
		}
		if cfg.Database.Password != "postgres" {
			t.Errorf("Database.Password = %q, want postgres", cfg.Database.Password)
		}
		if cfg.Database.DBName != "nervum" {
			t.Errorf("Database.DBName = %q, want nervum", cfg.Database.DBName)
		}
		if cfg.Database.SSLMode != "disable" {
			t.Errorf("Database.SSLMode = %q, want disable", cfg.Database.SSLMode)
		}
		if !cfg.Server.SessionCookieSecure {
			t.Error("SessionCookieSecure: want true when SESSION_COOKIE_SECURE is unset")
		}
		if cfg.Server.TrustedProxies != nil {
			t.Errorf("TrustedProxies: want nil when unset, got %#v", cfg.Server.TrustedProxies)
		}
		if len(cfg.InternalAdminEmails) != 1 || cfg.InternalAdminEmails[0] != "ferreirazdev@gmail.com" {
			t.Errorf("InternalAdminEmails = %#v", cfg.InternalAdminEmails)
		}
	})

	t.Run("session cookie secure false", func(t *testing.T) {
		t.Setenv("SESSION_COOKIE_SECURE", "false")
		t.Setenv("TRUSTED_PROXIES", " 10.0.0.0/8 , 127.0.0.1 ")
		cfg := Load()
		if cfg.Server.SessionCookieSecure {
			t.Error("SessionCookieSecure: want false")
		}
		if len(cfg.Server.TrustedProxies) != 2 || cfg.Server.TrustedProxies[0] != "10.0.0.0/8" || cfg.Server.TrustedProxies[1] != "127.0.0.1" {
			t.Errorf("TrustedProxies = %#v", cfg.Server.TrustedProxies)
		}
	})

	t.Run("internal admin emails from env", func(t *testing.T) {
		t.Setenv("INTERNAL_ADMIN_EMAILS", " a@b.com , C@D.com ")
		cfg := Load()
		if len(cfg.InternalAdminEmails) != 2 {
			t.Fatalf("len = %d", len(cfg.InternalAdminEmails))
		}
		if cfg.InternalAdminEmails[0] != "a@b.com" || cfg.InternalAdminEmails[1] != "c@d.com" {
			t.Fatalf("got %#v", cfg.InternalAdminEmails)
		}
	})

	t.Run("from env", func(t *testing.T) {
		t.Setenv("INTERNAL_ADMIN_EMAILS", "")
		t.Setenv("PORT", "3000")
		t.Setenv("DB_PORT", "5433")
		t.Setenv("DB_HOST", "db.example.com")
		t.Setenv("DB_USER", "u")
		t.Setenv("DB_PASSWORD", "p")
		t.Setenv("DB_NAME", "mydb")
		t.Setenv("DB_SSLMODE", "require")
		cfg := Load()
		if cfg.Server.Port != 3000 {
			t.Errorf("Server.Port = %d, want 3000", cfg.Server.Port)
		}
		if cfg.Database.Port != 5433 {
			t.Errorf("Database.Port = %d, want 5433", cfg.Database.Port)
		}
		if cfg.Database.Host != "db.example.com" {
			t.Errorf("Database.Host = %q", cfg.Database.Host)
		}
		if cfg.Database.User != "u" {
			t.Errorf("Database.User = %q", cfg.Database.User)
		}
		if cfg.Database.Password != "p" {
			t.Errorf("Database.Password = %q", cfg.Database.Password)
		}
		if cfg.Database.DBName != "mydb" {
			t.Errorf("Database.DBName = %q", cfg.Database.DBName)
		}
		if cfg.Database.SSLMode != "require" {
			t.Errorf("Database.SSLMode = %q", cfg.Database.SSLMode)
		}
	})
}
