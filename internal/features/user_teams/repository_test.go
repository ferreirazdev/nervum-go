package userteam

import (
	"context"
	"testing"

	"github.com/google/uuid"
	organization "github.com/nervum/nervum-go/internal/features/organizations"
	user "github.com/nervum/nervum-go/internal/features/users"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// teamRow avoids importing teams package (import cycle with handler).
type teamRow struct {
	ID             uuid.UUID `gorm:"type:uuid;primaryKey"`
	OrganizationID uuid.UUID `gorm:"type:uuid;not null;index"`
	Name           string    `gorm:"type:text;not null"`
}

func (teamRow) TableName() string { return "teams" }

func (t *teamRow) BeforeCreate(tx *gorm.DB) error {
	if t.ID == uuid.Nil {
		t.ID = uuid.New()
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
	if err := db.AutoMigrate(&organization.Organization{}, &teamRow{}, &user.User{}, &UserTeam{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

func TestRepository_CreateGetByUserAndTeam_Unit(t *testing.T) {
	db := testDB(t)
	sqlDB, _ := db.DB()
	defer sqlDB.Close()

	ctx := context.Background()
	orgRepo := organization.NewRepository(db)
	org := &organization.Organization{Name: "Acme"}
	if err := orgRepo.Create(ctx, org); err != nil {
		t.Fatalf("create org: %v", err)
	}
	team := &teamRow{OrganizationID: org.ID, Name: "Backend"}
	if err := db.WithContext(ctx).Create(team).Error; err != nil {
		t.Fatalf("create team: %v", err)
	}
	userRepo := user.NewRepository(db)
	u := &user.User{Email: "a@b.com", PasswordHash: "h", Role: user.RoleMember}
	if err := userRepo.Create(ctx, u); err != nil {
		t.Fatalf("create user: %v", err)
	}

	repo := NewRepository(db)
	ut := &UserTeam{UserID: u.ID, TeamID: team.ID}
	if err := repo.Create(ctx, ut); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if ut.ID == uuid.Nil {
		t.Error("expected ID to be set")
	}

	got, err := repo.GetByUserAndTeam(ctx, u.ID, team.ID)
	if err != nil {
		t.Fatalf("GetByUserAndTeam: %v", err)
	}
	if got.UserID != u.ID || got.TeamID != team.ID {
		t.Errorf("got %+v", got)
	}
}

func TestRepository_ListByUserListByTeam_Unit(t *testing.T) {
	db := testDB(t)
	sqlDB, _ := db.DB()
	defer sqlDB.Close()

	ctx := context.Background()
	orgRepo := organization.NewRepository(db)
	org := &organization.Organization{Name: "Acme"}
	if err := orgRepo.Create(ctx, org); err != nil {
		t.Fatalf("create org: %v", err)
	}
	team := &teamRow{OrganizationID: org.ID, Name: "Backend"}
	if err := db.WithContext(ctx).Create(team).Error; err != nil {
		t.Fatalf("create team: %v", err)
	}
	userRepo := user.NewRepository(db)
	u := &user.User{Email: "a@b.com", PasswordHash: "h", Role: user.RoleMember}
	if err := userRepo.Create(ctx, u); err != nil {
		t.Fatalf("create user: %v", err)
	}

	repo := NewRepository(db)
	if err := repo.Create(ctx, &UserTeam{UserID: u.ID, TeamID: team.ID}); err != nil {
		t.Fatalf("Create: %v", err)
	}

	byUser, err := repo.ListByUser(ctx, u.ID)
	if err != nil {
		t.Fatalf("ListByUser: %v", err)
	}
	if len(byUser) != 1 {
		t.Errorf("ListByUser: got %d, want 1", len(byUser))
	}
	byTeam, err := repo.ListByTeam(ctx, team.ID)
	if err != nil {
		t.Fatalf("ListByTeam: %v", err)
	}
	if len(byTeam) != 1 {
		t.Errorf("ListByTeam: got %d, want 1", len(byTeam))
	}
}

func TestRepository_DeleteDeleteByTeam_Unit(t *testing.T) {
	db := testDB(t)
	sqlDB, _ := db.DB()
	defer sqlDB.Close()

	ctx := context.Background()
	orgRepo := organization.NewRepository(db)
	org := &organization.Organization{Name: "Acme"}
	if err := orgRepo.Create(ctx, org); err != nil {
		t.Fatalf("create org: %v", err)
	}
	team := &teamRow{OrganizationID: org.ID, Name: "Backend"}
	if err := db.WithContext(ctx).Create(team).Error; err != nil {
		t.Fatalf("create team: %v", err)
	}
	userRepo := user.NewRepository(db)
	u := &user.User{Email: "a@b.com", PasswordHash: "h", Role: user.RoleMember}
	if err := userRepo.Create(ctx, u); err != nil {
		t.Fatalf("create user: %v", err)
	}

	repo := NewRepository(db)
	if err := repo.Create(ctx, &UserTeam{UserID: u.ID, TeamID: team.ID}); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if err := repo.Delete(ctx, u.ID, team.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	_, err := repo.GetByUserAndTeam(ctx, u.ID, team.ID)
	if err == nil {
		t.Error("expected error after Delete")
	}
}
