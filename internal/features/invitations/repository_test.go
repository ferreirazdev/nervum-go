package invitation

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	organization "github.com/nervum/nervum-go/internal/features/organizations"
	user "github.com/nervum/nervum-go/internal/features/users"
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
	if err := db.AutoMigrate(&organization.Organization{}, &user.User{}, &Invitation{}, &InvitationTeam{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

func TestRepository_CreateGetByIDGetByToken_Unit(t *testing.T) {
	db := testDB(t)
	sqlDB, _ := db.DB()
	defer sqlDB.Close()

	ctx := context.Background()
	orgRepo := organization.NewRepository(db)
	org := &organization.Organization{Name: "Acme"}
	if err := orgRepo.Create(ctx, org); err != nil {
		t.Fatalf("create org: %v", err)
	}
	userRepo := user.NewRepository(db)
	u := &user.User{Email: "a@b.com", PasswordHash: "h", Role: user.RoleAdmin}
	if err := userRepo.Create(ctx, u); err != nil {
		t.Fatalf("create user: %v", err)
	}

	repo := NewRepository(db)
	inv := &Invitation{
		Token:          "secret-token",
		Email:          "invitee@b.com",
		OrganizationID: org.ID,
		InvitedByID:    u.ID,
		Role:           user.RoleMember,
		ExpiresAt:      time.Now().Add(24 * time.Hour),
		Status:         StatusPending,
	}
	if err := repo.Create(ctx, inv, nil); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if inv.ID == uuid.Nil {
		t.Error("expected ID to be set")
	}

	got, err := repo.GetByID(ctx, inv.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.Email != "invitee@b.com" || got.Token != "secret-token" {
		t.Errorf("GetByID: got %+v", got)
	}

	byToken, err := repo.GetByToken(ctx, "secret-token")
	if err != nil {
		t.Fatalf("GetByToken: %v", err)
	}
	if byToken.ID != inv.ID {
		t.Errorf("GetByToken: got id %v", byToken.ID)
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
	userRepo := user.NewRepository(db)
	u := &user.User{Email: "a@b.com", PasswordHash: "h", Role: user.RoleAdmin}
	if err := userRepo.Create(ctx, u); err != nil {
		t.Fatalf("create user: %v", err)
	}

	repo := NewRepository(db)
	inv1 := &Invitation{Token: "t1", Email: "e1@b.com", OrganizationID: org.ID, InvitedByID: u.ID, ExpiresAt: time.Now().Add(time.Hour), Status: StatusPending}
	if err := repo.Create(ctx, inv1, nil); err != nil {
		t.Fatalf("Create 1: %v", err)
	}
	inv2 := &Invitation{Token: "t2", Email: "e2@b.com", OrganizationID: org.ID, InvitedByID: u.ID, ExpiresAt: time.Now().Add(time.Hour), Status: StatusPending}
	if err := repo.Create(ctx, inv2, nil); err != nil {
		t.Fatalf("Create 2: %v", err)
	}

	list, err := repo.ListByOrganization(ctx, org.ID, "")
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

	ctx := context.Background()
	orgRepo := organization.NewRepository(db)
	org := &organization.Organization{Name: "Acme"}
	if err := orgRepo.Create(ctx, org); err != nil {
		t.Fatalf("create org: %v", err)
	}
	userRepo := user.NewRepository(db)
	u := &user.User{Email: "a@b.com", PasswordHash: "h", Role: user.RoleAdmin}
	if err := userRepo.Create(ctx, u); err != nil {
		t.Fatalf("create user: %v", err)
	}

	repo := NewRepository(db)
	inv := &Invitation{Token: "t", Email: "e@b.com", OrganizationID: org.ID, InvitedByID: u.ID, ExpiresAt: time.Now().Add(time.Hour), Status: StatusPending}
	if err := repo.Create(ctx, inv, nil); err != nil {
		t.Fatalf("Create: %v", err)
	}
	inv.Status = StatusAccepted
	if err := repo.Update(ctx, inv); err != nil {
		t.Fatalf("Update: %v", err)
	}
	got, _ := repo.GetByID(ctx, inv.ID)
	if got.Status != StatusAccepted {
		t.Errorf("after update: got status %q", got.Status)
	}
	if err := repo.Delete(ctx, inv.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	_, err := repo.GetByID(ctx, inv.ID)
	if err == nil {
		t.Error("expected error after Delete")
	}
}
