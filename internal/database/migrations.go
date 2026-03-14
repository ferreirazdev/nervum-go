package database

import (
	"database/sql"
	"fmt"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	nervummigrations "github.com/nervum/nervum-go/migrations"
)

// MigratorFrom creates a *migrate.Migrate instance backed by the embedded SQL files.
// The caller must call m.Close() when done.
func MigratorFrom(db *sql.DB) (*migrate.Migrate, error) {
	src, err := iofs.New(nervummigrations.FS, ".")
	if err != nil {
		return nil, fmt.Errorf("migration source: %w", err)
	}
	driver, err := postgres.WithInstance(db, &postgres.Config{})
	if err != nil {
		return nil, fmt.Errorf("migration driver: %w", err)
	}
	m, err := migrate.NewWithInstance("iofs", src, "postgres", driver)
	if err != nil {
		return nil, fmt.Errorf("migrate instance: %w", err)
	}
	return m, nil
}

// RunMigrations applies all pending up-migrations. Safe to call on every startup;
// already-applied migrations are skipped automatically.
func RunMigrations(db *sql.DB) error {
	m, err := MigratorFrom(db)
	if err != nil {
		return err
	}
	defer m.Close()
	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("migrate up: %w", err)
	}
	return nil
}
