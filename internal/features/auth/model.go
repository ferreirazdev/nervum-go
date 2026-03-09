// Package auth provides session-based authentication: login, register, logout, and
// RequireAuth middleware for protected routes. Sessions are stored in the database
// and identified by a cookie (nervum_session).
package auth

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Session represents a user session stored in the database. Used for cookie-based auth.
type Session struct {
	ID        uuid.UUID      `gorm:"type:uuid;primaryKey" json:"id"`
	UserID    uuid.UUID      `gorm:"type:uuid;not null;index" json:"user_id"`
	ExpiresAt time.Time      `gorm:"not null" json:"expires_at"`
	CreatedAt time.Time      `json:"created_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

func (Session) TableName() string { return "sessions" }

func (s *Session) BeforeCreate(tx *gorm.DB) error {
	if s.ID == uuid.Nil {
		s.ID = uuid.New()
	}
	return nil
}
