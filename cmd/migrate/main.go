// Package main is the Nervum migration CLI.
// It applies or rolls back SQL migrations using the same database config as the API server.
//
// Usage:
//
//	go run ./cmd/migrate up              # apply all pending migrations
//	go run ./cmd/migrate down [N]        # roll back N migrations (default 1)
//	go run ./cmd/migrate version         # print current schema version
//	go run ./cmd/migrate force <V>       # force version to V (dirty state recovery)
package main

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"strconv"

	"github.com/golang-migrate/migrate/v4"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
	"github.com/nervum/nervum-go/internal/config"
	"github.com/nervum/nervum-go/internal/database"
)

func main() {
	_ = godotenv.Load()

	if len(os.Args) < 2 {
		usage()
		os.Exit(1)
	}

	cfg := config.Load()
	dsn := fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		cfg.Database.Host, cfg.Database.Port, cfg.Database.User,
		cfg.Database.Password, cfg.Database.DBName, cfg.Database.SSLMode,
	)
	sqlDB, err := sql.Open("postgres", dsn)
	if err != nil {
		log.Fatalf("open db: %v", err)
	}
	defer sqlDB.Close()

	m, err := database.MigratorFrom(sqlDB)
	if err != nil {
		log.Fatalf("migrator: %v", err)
	}
	defer m.Close()

	cmd := os.Args[1]
	switch cmd {
	case "up":
		if err := m.Up(); err != nil && err != migrate.ErrNoChange {
			log.Fatalf("up: %v", err)
		}
		log.Println("migrations applied")

	case "down":
		n := 1
		if len(os.Args) > 2 {
			n, err = strconv.Atoi(os.Args[2])
			if err != nil || n < 1 {
				log.Fatalf("down: invalid step count %q", os.Args[2])
			}
		}
		if err := m.Steps(-n); err != nil && err != migrate.ErrNoChange {
			log.Fatalf("down %d: %v", n, err)
		}
		log.Printf("rolled back %d migration(s)", n)

	case "version":
		v, dirty, err := m.Version()
		if err != nil {
			log.Fatalf("version: %v", err)
		}
		log.Printf("version=%d dirty=%v", v, dirty)

	case "force":
		if len(os.Args) < 3 {
			log.Fatal("force requires a version number: migrate force <V>")
		}
		v, err := strconv.Atoi(os.Args[2])
		if err != nil {
			log.Fatalf("force: invalid version %q", os.Args[2])
		}
		if err := m.Force(v); err != nil {
			log.Fatalf("force %d: %v", v, err)
		}
		log.Printf("forced version to %d", v)

	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", cmd)
		usage()
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, `Usage: migrate <command> [args]

Commands:
  up           Apply all pending migrations
  down [N]     Roll back N migrations (default 1)
  version      Print current schema version
  force <V>    Force schema version to V (use to recover from a failed migration)`)
}
