package user

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

const (
	RoleAdmin  = "admin"
	RoleMember = "member"
)

type User struct {
	ID             uuid.UUID      `gorm:"type:uuid;primaryKey" json:"id"`
	Email          string         `gorm:"type:text;uniqueIndex" json:"email"`
	Name           string         `gorm:"type:text" json:"name"`
	Role           string         `gorm:"type:text" json:"role"` // admin, member
	OrganizationID *uuid.UUID     `gorm:"type:uuid;index" json:"organization_id,omitempty"`
	PasswordHash   string         `gorm:"type:text;not null" json:"-"`
	CreatedAt      time.Time      `json:"created_at"`
	UpdatedAt      time.Time      `json:"updated_at"`
	DeletedAt      gorm.DeletedAt `gorm:"index" json:"-"`
}

func (User) TableName() string { return "users" }

func (u *User) BeforeCreate(tx *gorm.DB) error {
	if u.ID == uuid.Nil {
		u.ID = uuid.New()
	}
	return nil
}
