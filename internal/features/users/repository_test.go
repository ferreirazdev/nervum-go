package user

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// orgRow avoids importing organization package (import cycle with handler).
type orgRow struct {
	ID   uuid.UUID `gorm:"type:uuid;primaryKey"`
	Name string    `gorm:"type:text;not null"`
}

func (orgRow) TableName() string { return "organizations" }

func (o *orgRow) BeforeCreate(tx *gorm.DB) error {
	if o.ID == uuid.Nil {
		o.ID = uuid.New()
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
	if err := db.AutoMigrate(&orgRow{}, &User{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

func TestRepository_CreateGetByIDGetByEmail_Unit(t *testing.T) {
	db := testDB(t)
	sqlDB, _ := db.DB()
	defer sqlDB.Close()

	repo := NewRepository(db)
	ctx := context.Background()
	u := &User{Email: "a@b.com", Name: "Alice", PasswordHash: "hash", Role: RoleMember}
	if err := repo.Create(ctx, u); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if u.ID == uuid.Nil {
		t.Error("expected ID to be set")
	}

	byID, err := repo.GetByID(ctx, u.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if byID.Email != "a@b.com" {
		t.Errorf("GetByID: got email %q", byID.Email)
	}

	byEmail, err := repo.GetByEmail(ctx, "a@b.com")
	if err != nil {
		t.Fatalf("GetByEmail: %v", err)
	}
	if byEmail.ID != u.ID {
		t.Errorf("GetByEmail: got id %v", byEmail.ID)
	}
}

func TestRepository_ListListByOrganization_Unit(t *testing.T) {
	db := testDB(t)
	sqlDB, _ := db.DB()
	defer sqlDB.Close()

	ctx := context.Background()
	org := &orgRow{Name: "Acme"}
	if err := db.WithContext(ctx).Create(org).Error; err != nil {
		t.Fatalf("create org: %v", err)
	}
	orgID := org.ID

	repo := NewRepository(db)
	if err := repo.Create(ctx, &User{Email: "1@b.com", PasswordHash: "h", Role: RoleMember, OrganizationID: &orgID}); err != nil {
		t.Fatalf("Create 1: %v", err)
	}
	if err := repo.Create(ctx, &User{Email: "2@b.com", PasswordHash: "h", Role: RoleMember, OrganizationID: &orgID}); err != nil {
		t.Fatalf("Create 2: %v", err)
	}

	list, err := repo.List(ctx)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 2 {
		t.Errorf("List: got %d, want 2", len(list))
	}

	byOrg, err := repo.ListByOrganization(ctx, org.ID)
	if err != nil {
		t.Fatalf("ListByOrganization: %v", err)
	}
	if len(byOrg) != 2 {
		t.Errorf("ListByOrganization: got %d, want 2", len(byOrg))
	}
}

func TestRepository_UpdateDelete_Unit(t *testing.T) {
	db := testDB(t)
	sqlDB, _ := db.DB()
	defer sqlDB.Close()

	repo := NewRepository(db)
	ctx := context.Background()
	u := &User{Email: "x@b.com", Name: "Original", PasswordHash: "h", Role: RoleMember}
	if err := repo.Create(ctx, u); err != nil {
		t.Fatalf("Create: %v", err)
	}
	u.Name = "Updated"
	if err := repo.Update(ctx, u); err != nil {
		t.Fatalf("Update: %v", err)
	}
	got, _ := repo.GetByID(ctx, u.ID)
	if got.Name != "Updated" {
		t.Errorf("after update: got name %q", got.Name)
	}
	if err := repo.Delete(ctx, u.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	_, err := repo.GetByID(ctx, u.ID)
	if err == nil {
		t.Error("expected error after Delete")
	}
}
