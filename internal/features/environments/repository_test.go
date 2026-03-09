package environment

import (
	"context"
	"testing"

	"github.com/google/uuid"
	organization "github.com/nervum/nervum-go/internal/features/organizations"
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
	if err := db.AutoMigrate(&organization.Organization{}, &Environment{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

func TestRepository_CreateGetByID_Unit(t *testing.T) {
	db := testDB(t)
	sqlDB, _ := db.DB()
	defer sqlDB.Close()

	orgRepo := organization.NewRepository(db)
	org := &organization.Organization{Name: "Acme"}
	if err := orgRepo.Create(context.Background(), org); err != nil {
		t.Fatalf("create org: %v", err)
	}

	repo := NewRepository(db)
	ctx := context.Background()
	e := &Environment{OrganizationID: org.ID, Name: "Prod", Status: "healthy"}
	if err := repo.Create(ctx, e); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if e.ID == uuid.Nil {
		t.Error("expected ID to be set")
	}

	got, err := repo.GetByID(ctx, e.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.Name != "Prod" || got.OrganizationID != org.ID {
		t.Errorf("got %+v", got)
	}
}

func TestRepository_ListByOrganization_Unit(t *testing.T) {
	db := testDB(t)
	sqlDB, _ := db.DB()
	defer sqlDB.Close()

	orgRepo := organization.NewRepository(db)
	org := &organization.Organization{Name: "Acme"}
	if err := orgRepo.Create(context.Background(), org); err != nil {
		t.Fatalf("create org: %v", err)
	}

	repo := NewRepository(db)
	ctx := context.Background()
	if err := repo.Create(ctx, &Environment{OrganizationID: org.ID, Name: "A"}); err != nil {
		t.Fatalf("Create A: %v", err)
	}
	if err := repo.Create(ctx, &Environment{OrganizationID: org.ID, Name: "B"}); err != nil {
		t.Fatalf("Create B: %v", err)
	}

	list, err := repo.ListByOrganization(ctx, org.ID)
	if err != nil {
		t.Fatalf("ListByOrganization: %v", err)
	}
	if len(list) != 2 {
		t.Errorf("ListByOrganization: got %d, want 2", len(list))
	}
}

func TestRepository_UpdateDelete_Unit(t *testing.T) {
	db := testDB(t)
	sqlDB, _ := db.DB()
	defer sqlDB.Close()

	orgRepo := organization.NewRepository(db)
	org := &organization.Organization{Name: "Acme"}
	if err := orgRepo.Create(context.Background(), org); err != nil {
		t.Fatalf("create org: %v", err)
	}

	repo := NewRepository(db)
	ctx := context.Background()
	e := &Environment{OrganizationID: org.ID, Name: "Original"}
	if err := repo.Create(ctx, e); err != nil {
		t.Fatalf("Create: %v", err)
	}
	e.Name = "Updated"
	if err := repo.Update(ctx, e); err != nil {
		t.Fatalf("Update: %v", err)
	}
	got, _ := repo.GetByID(ctx, e.ID)
	if got.Name != "Updated" {
		t.Errorf("after update: got name %q", got.Name)
	}
	if err := repo.Delete(ctx, e.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	_, err := repo.GetByID(ctx, e.ID)
	if err == nil {
		t.Error("expected error after Delete")
	}
}
