package database

import (
	"testing"
)

func TestNewTestDB(t *testing.T) {
	db, err := NewTestDB()
	if err != nil {
		t.Fatalf("NewTestDB: %v", err)
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

func TestAutoMigrate_Unit(t *testing.T) {
	db, err := NewTestDB()
	if err != nil {
		t.Fatalf("NewTestDB: %v", err)
	}
	sqlDB, _ := db.DB()
	defer sqlDB.Close()
	if err := AutoMigrate(db); err != nil {
		t.Fatalf("AutoMigrate: %v", err)
	}
	// Idempotent
	if err := AutoMigrate(db); err != nil {
		t.Fatalf("AutoMigrate second run: %v", err)
	}
}
