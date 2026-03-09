package relationship

import (
	"context"
	"testing"

	"github.com/google/uuid"
	entity "github.com/nervum/nervum-go/internal/features/entities"
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
	if err := db.AutoMigrate(&organization.Organization{}, &entity.Entity{}, &Relationship{}); err != nil {
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
	entityRepo := entity.NewRepository(db)
	envID := uuid.New()
	e1 := &entity.Entity{OrganizationID: org.ID, EnvironmentID: envID, Type: entity.TypeService, Name: "A"}
	if err := entityRepo.Create(ctx, e1); err != nil {
		t.Fatalf("create entity 1: %v", err)
	}
	e2 := &entity.Entity{OrganizationID: org.ID, EnvironmentID: envID, Type: entity.TypeDatabase, Name: "B"}
	if err := entityRepo.Create(ctx, e2); err != nil {
		t.Fatalf("create entity 2: %v", err)
	}

	repo := NewRepository(db)
	rel := &Relationship{OrganizationID: org.ID, FromEntityID: e1.ID, ToEntityID: e2.ID, Type: TypeDependsOn}
	if err := repo.Create(ctx, rel); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if rel.ID == uuid.Nil {
		t.Error("expected ID to be set")
	}

	got, err := repo.GetByID(ctx, rel.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.FromEntityID != e1.ID || got.ToEntityID != e2.ID || got.Type != TypeDependsOn {
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
	entityRepo := entity.NewRepository(db)
	envID := uuid.New()
	e1 := &entity.Entity{OrganizationID: org.ID, EnvironmentID: envID, Type: entity.TypeService, Name: "A"}
	if err := entityRepo.Create(ctx, e1); err != nil {
		t.Fatalf("create entity 1: %v", err)
	}
	e2 := &entity.Entity{OrganizationID: org.ID, EnvironmentID: envID, Type: entity.TypeDatabase, Name: "B"}
	if err := entityRepo.Create(ctx, e2); err != nil {
		t.Fatalf("create entity 2: %v", err)
	}

	repo := NewRepository(db)
	if err := repo.Create(ctx, &Relationship{OrganizationID: org.ID, FromEntityID: e1.ID, ToEntityID: e2.ID, Type: TypeDependsOn}); err != nil {
		t.Fatalf("Create: %v", err)
	}

	list, err := repo.ListByOrganization(ctx, org.ID)
	if err != nil {
		t.Fatalf("ListByOrganization: %v", err)
	}
	if len(list) != 1 {
		t.Errorf("ListByOrganization: got %d, want 1", len(list))
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
	entityRepo := entity.NewRepository(db)
	envID := uuid.New()
	e1 := &entity.Entity{OrganizationID: org.ID, EnvironmentID: envID, Type: entity.TypeService, Name: "A"}
	if err := entityRepo.Create(ctx, e1); err != nil {
		t.Fatalf("create entity 1: %v", err)
	}
	e2 := &entity.Entity{OrganizationID: org.ID, EnvironmentID: envID, Type: entity.TypeDatabase, Name: "B"}
	if err := entityRepo.Create(ctx, e2); err != nil {
		t.Fatalf("create entity 2: %v", err)
	}

	repo := NewRepository(db)
	rel := &Relationship{OrganizationID: org.ID, FromEntityID: e1.ID, ToEntityID: e2.ID, Type: TypeDependsOn}
	if err := repo.Create(ctx, rel); err != nil {
		t.Fatalf("Create: %v", err)
	}
	rel.Type = TypeOwnedBy
	if err := repo.Update(ctx, rel); err != nil {
		t.Fatalf("Update: %v", err)
	}
	got, _ := repo.GetByID(ctx, rel.ID)
	if got.Type != TypeOwnedBy {
		t.Errorf("after update: got type %q", got.Type)
	}
	if err := repo.Delete(ctx, rel.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	_, err := repo.GetByID(ctx, rel.ID)
	if err == nil {
		t.Error("expected error after Delete")
	}
}
