package organization

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func testDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	if err := db.AutoMigrate(&Organization{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

func TestRepository_CreateGetByID_Unit(t *testing.T) {
	db := testDB(t)
	sqlDB, _ := db.DB()
	defer sqlDB.Close()

	repo := NewRepository(db)
	ctx := context.Background()

	o := &Organization{Name: "Acme Inc"}
	if err := repo.Create(ctx, o); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if o.ID == uuid.Nil {
		t.Error("expected ID to be set")
	}

	got, err := repo.GetByID(ctx, o.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.Name != "Acme Inc" {
		t.Errorf("got name %q", got.Name)
	}
}

func TestRepository_List_Unit(t *testing.T) {
	db := testDB(t)
	sqlDB, _ := db.DB()
	defer sqlDB.Close()

	repo := NewRepository(db)
	ctx := context.Background()

	if err := repo.Create(ctx, &Organization{Name: "A"}); err != nil {
		t.Fatalf("Create A: %v", err)
	}
	if err := repo.Create(ctx, &Organization{Name: "B"}); err != nil {
		t.Fatalf("Create B: %v", err)
	}

	list, err := repo.List(ctx)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 2 {
		t.Errorf("List: got %d, want 2", len(list))
	}
}

func TestRepository_UpdateDelete_Unit(t *testing.T) {
	db := testDB(t)
	sqlDB, _ := db.DB()
	defer sqlDB.Close()

	repo := NewRepository(db)
	ctx := context.Background()

	o := &Organization{Name: "Original"}
	if err := repo.Create(ctx, o); err != nil {
		t.Fatalf("Create: %v", err)
	}
	o.Name = "Updated"
	if err := repo.Update(ctx, o); err != nil {
		t.Fatalf("Update: %v", err)
	}
	got, _ := repo.GetByID(ctx, o.ID)
	if got.Name != "Updated" {
		t.Errorf("after update: got name %q", got.Name)
	}
	if err := repo.Delete(ctx, o.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	_, err := repo.GetByID(ctx, o.ID)
	if err == nil {
		t.Error("expected error after Delete")
	}
}
