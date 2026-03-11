// Package user provides CRUD for users and role-based permission helpers (CanInvite, CanManageTeams, etc.).
// Used by the API server and by auth/invitations for session and invite flows.
package user

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Role constants for user.Role (admin, manager, member).
const (
	RoleAdmin   = "admin"
	RoleManager = "manager"
	RoleMember  = "member"
)

// User represents an account. Stored in table users; PasswordHash is never serialized to JSON.
type User struct {
	ID                   uuid.UUID      `gorm:"type:uuid;primaryKey" json:"id"`
	Email                string         `gorm:"type:text;uniqueIndex" json:"email"`
	Name                 string         `gorm:"type:text" json:"name"`
	Role                 string         `gorm:"type:text" json:"role"` // admin, manager, member
	OrganizationID       *uuid.UUID     `gorm:"type:uuid;index" json:"organization_id,omitempty"`
	OnboardingCompleted  bool           `gorm:"type:boolean;default:false" json:"onboarding"`
	PasswordHash         string         `gorm:"type:text;not null" json:"-"`
	CreatedAt            time.Time      `json:"created_at"`
	UpdatedAt            time.Time      `json:"updated_at"`
	DeletedAt            gorm.DeletedAt `gorm:"index" json:"-"`
}

func (User) TableName() string { return "users" }

func (u *User) BeforeCreate(tx *gorm.DB) error {
	if u.ID == uuid.Nil {
		u.ID = uuid.New()
	}
	return nil
}
