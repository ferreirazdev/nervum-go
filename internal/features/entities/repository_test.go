package entity

import (
	"context"
	"testing"

	"github.com/google/uuid"
	organization "github.com/nervum/nervum-go/internal/features/organizations"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// envRow avoids importing environments package (import cycle with handler).
type envRow struct {
	ID             uuid.UUID `gorm:"type:uuid;primaryKey"`
	OrganizationID uuid.UUID `gorm:"type:uuid;not null;index"`
	Name           string    `gorm:"type:text"`
}

func (envRow) TableName() string { return "environments" }

func (e *envRow) BeforeCreate(tx *gorm.DB) error {
	if e.ID == uuid.Nil {
		e.ID = uuid.New()
	}
	return nil
}

func testDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	if err := db.AutoMigrate(&organization.Organization{}, &envRow{}, &Entity{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

func TestRepository_CreateGetByID_Unit(t *testing.T) {
	db := testDB(t)
	sqlDB, _ := db.DB()
	defer sqlDB.Close()

	ctx := context.Background()
	orgRepo := organization.NewRepository(db)
	org := &organization.Organization{Name: "Acme"}
	if err := orgRepo.Create(ctx, org); err != nil {
		t.Fatalf("create org: %v", err)
	}
	env := &envRow{OrganizationID: org.ID, Name: "Prod"}
	if err := db.WithContext(ctx).Create(env).Error; err != nil {
		t.Fatalf("create env: %v", err)
	}

	repo := NewRepository(db)
	e := &Entity{OrganizationID: org.ID, EnvironmentID: env.ID, Type: TypeService, Name: "api"}
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
	if got.Name != "api" || got.Type != TypeService {
		t.Errorf("got %+v", got)
	}
}

func TestRepository_ListByOrganization_Unit(t *testing.T) {
	db := testDB(t)
	sqlDB, _ := db.DB()
	defer sqlDB.Close()

	ctx := context.Background()
	orgRepo := organization.NewRepository(db)
	org := &organization.Organization{Name: "Acme"}
	if err := orgRepo.Create(ctx, org); err != nil {
		t.Fatalf("create org: %v", err)
	}
	env := &envRow{OrganizationID: org.ID, Name: "Prod"}
	if err := db.WithContext(ctx).Create(env).Error; err != nil {
		t.Fatalf("create env: %v", err)
	}

	repo := NewRepository(db)
	if err := repo.Create(ctx, &Entity{OrganizationID: org.ID, EnvironmentID: env.ID, Type: TypeService, Name: "A"}); err != nil {
		t.Fatalf("Create A: %v", err)
	}
	if err := repo.Create(ctx, &Entity{OrganizationID: org.ID, EnvironmentID: env.ID, Type: TypeDatabase, Name: "B"}); err != nil {
		t.Fatalf("Create B: %v", err)
	}

	list, err := repo.ListByOrganization(ctx, org.ID, nil)
	if err != nil {
		t.Fatalf("ListByOrganization: %v", err)
	}
	if len(list) != 2 {
		t.Errorf("ListByOrganization: got %d, want 2", len(list))
	}

	listEnv, err := repo.ListByOrganization(ctx, org.ID, &env.ID)
	if err != nil {
		t.Fatalf("ListByOrganization(env): %v", err)
	}
	if len(listEnv) != 2 {
		t.Errorf("ListByOrganization(env): got %d, want 2", len(listEnv))
	}
}

func TestRepository_CountByEnvironment_Unit(t *testing.T) {
	db := testDB(t)
	sqlDB, _ := db.DB()
	defer sqlDB.Close()

	ctx := context.Background()
	orgRepo := organization.NewRepository(db)
	org := &organization.Organization{Name: "Acme"}
	if err := orgRepo.Create(ctx, org); err != nil {
		t.Fatalf("create org: %v", err)
	}
	env := &envRow{OrganizationID: org.ID, Name: "Prod"}
	if err := db.WithContext(ctx).Create(env).Error; err != nil {
		t.Fatalf("create env: %v", err)
	}

	repo := NewRepository(db)
	if err := repo.Create(ctx, &Entity{OrganizationID: org.ID, EnvironmentID: env.ID, Type: TypeService, Name: "A"}); err != nil {
		t.Fatalf("Create: %v", err)
	}
	count, err := repo.CountByEnvironment(ctx, env.ID)
	if err != nil {
		t.Fatalf("CountByEnvironment: %v", err)
	}
	if count != 1 {
		t.Errorf("CountByEnvironment: got %d, want 1", count)
	}
}

func TestRepository_UpdateDelete_Unit(t *testing.T) {
	db := testDB(t)
	sqlDB, _ := db.DB()
	defer sqlDB.Close()

	ctx := context.Background()
	orgRepo := organization.NewRepository(db)
	org := &organization.Organization{Name: "Acme"}
	if err := orgRepo.Create(ctx, org); err != nil {
		t.Fatalf("create org: %v", err)
	}
	env := &envRow{OrganizationID: org.ID, Name: "Prod"}
	if err := db.WithContext(ctx).Create(env).Error; err != nil {
		t.Fatalf("create env: %v", err)
	}

	repo := NewRepository(db)
	e := &Entity{OrganizationID: org.ID, EnvironmentID: env.ID, Type: TypeService, Name: "Original"}
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
