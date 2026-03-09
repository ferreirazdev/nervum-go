package userenvironmentaccess

import (
	"context"
	"testing"

	"github.com/google/uuid"
	environment "github.com/nervum/nervum-go/internal/features/environments"
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
	if err := db.AutoMigrate(&organization.Organization{}, &environment.Environment{}, &user.User{}, &UserEnvironmentAccess{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

func TestRepository_CreateGetByIDGetByUserAndEnvironment_Unit(t *testing.T) {
	db := testDB(t)
	sqlDB, _ := db.DB()
	defer sqlDB.Close()

	ctx := context.Background()
	orgRepo := organization.NewRepository(db)
	org := &organization.Organization{Name: "Acme"}
	if err := orgRepo.Create(ctx, org); err != nil {
		t.Fatalf("create org: %v", err)
	}
	envRepo := environment.NewRepository(db)
	env := &environment.Environment{OrganizationID: org.ID, Name: "Prod"}
	if err := envRepo.Create(ctx, env); err != nil {
		t.Fatalf("create env: %v", err)
	}
	userRepo := user.NewRepository(db)
	u := &user.User{Email: "a@b.com", PasswordHash: "h", Role: user.RoleMember}
	if err := userRepo.Create(ctx, u); err != nil {
		t.Fatalf("create user: %v", err)
	}

	repo := NewRepository(db)
	uea := &UserEnvironmentAccess{UserID: u.ID, EnvironmentID: env.ID}
	if err := repo.Create(ctx, uea); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if uea.ID == uuid.Nil {
		t.Error("expected ID to be set")
	}

	got, err := repo.GetByID(ctx, uea.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.UserID != u.ID || got.EnvironmentID != env.ID {
		t.Errorf("GetByID: got %+v", got)
	}

	byUserEnv, err := repo.GetByUserAndEnvironment(ctx, u.ID, env.ID)
	if err != nil {
		t.Fatalf("GetByUserAndEnvironment: %v", err)
	}
	if byUserEnv.ID != uea.ID {
		t.Errorf("GetByUserAndEnvironment: got id %v", byUserEnv.ID)
	}
}

func TestRepository_ListByUserListByEnvironment_Unit(t *testing.T) {
	db := testDB(t)
	sqlDB, _ := db.DB()
	defer sqlDB.Close()

	ctx := context.Background()
	orgRepo := organization.NewRepository(db)
	org := &organization.Organization{Name: "Acme"}
	if err := orgRepo.Create(ctx, org); err != nil {
		t.Fatalf("create org: %v", err)
	}
	envRepo := environment.NewRepository(db)
	env := &environment.Environment{OrganizationID: org.ID, Name: "Prod"}
	if err := envRepo.Create(ctx, env); err != nil {
		t.Fatalf("create env: %v", err)
	}
	userRepo := user.NewRepository(db)
	u := &user.User{Email: "a@b.com", PasswordHash: "h", Role: user.RoleMember}
	if err := userRepo.Create(ctx, u); err != nil {
		t.Fatalf("create user: %v", err)
	}

	repo := NewRepository(db)
	if err := repo.Create(ctx, &UserEnvironmentAccess{UserID: u.ID, EnvironmentID: env.ID}); err != nil {
		t.Fatalf("Create: %v", err)
	}

	byUser, err := repo.ListByUser(ctx, u.ID)
	if err != nil {
		t.Fatalf("ListByUser: %v", err)
	}
	if len(byUser) != 1 {
		t.Errorf("ListByUser: got %d, want 1", len(byUser))
	}
	byEnv, err := repo.ListByEnvironment(ctx, env.ID)
	if err != nil {
		t.Fatalf("ListByEnvironment: %v", err)
	}
	if len(byEnv) != 1 {
		t.Errorf("ListByEnvironment: got %d, want 1", len(byEnv))
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
	envRepo := environment.NewRepository(db)
	env := &environment.Environment{OrganizationID: org.ID, Name: "Prod"}
	if err := envRepo.Create(ctx, env); err != nil {
		t.Fatalf("create env: %v", err)
	}
	userRepo := user.NewRepository(db)
	u := &user.User{Email: "a@b.com", PasswordHash: "h", Role: user.RoleMember}
	if err := userRepo.Create(ctx, u); err != nil {
		t.Fatalf("create user: %v", err)
	}

	repo := NewRepository(db)
	uea := &UserEnvironmentAccess{UserID: u.ID, EnvironmentID: env.ID}
	if err := repo.Create(ctx, uea); err != nil {
		t.Fatalf("Create: %v", err)
	}
	uea.Role = "viewer"
	if err := repo.Update(ctx, uea); err != nil {
		t.Fatalf("Update: %v", err)
	}
	got, _ := repo.GetByID(ctx, uea.ID)
	if got.Role != "viewer" {
		t.Errorf("after update: got role %q", got.Role)
	}
	if err := repo.Delete(ctx, uea.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	_, err := repo.GetByID(ctx, uea.ID)
	if err == nil {
		t.Error("expected error after Delete")
	}
}
