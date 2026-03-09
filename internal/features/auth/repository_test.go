package auth

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
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
	if err := db.AutoMigrate(&user.User{}, &Session{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

func TestSessionRepository_CreateGetByIDDelete_Unit(t *testing.T) {
	db := testDB(t)
	sqlDB, _ := db.DB()
	defer sqlDB.Close()

	userRepo := user.NewRepository(db)
	u := &user.User{Email: "a@b.com", PasswordHash: "h", Role: user.RoleMember}
	if err := userRepo.Create(context.Background(), u); err != nil {
		t.Fatalf("create user: %v", err)
	}

	repo := NewSessionRepository(db)
	ctx := context.Background()
	s := &Session{UserID: u.ID, ExpiresAt: time.Now().Add(time.Hour)}
	if err := repo.Create(ctx, s); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if s.ID == uuid.Nil {
		t.Error("expected ID to be set")
	}

	got, err := repo.GetByID(ctx, s.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.UserID != u.ID {
		t.Errorf("GetByID: got UserID %v", got.UserID)
	}

	if err := repo.Delete(ctx, s.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	_, err = repo.GetByID(ctx, s.ID)
	if err == nil {
		t.Error("expected error after Delete")
	}
}
