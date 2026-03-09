// Package userenvironmentaccess provides CRUD for user-environment access (RBAC per environment).
package userenvironmentaccess

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// UserEnvironmentAccess grants a user access to an environment. Stored in user_environment_access.
type UserEnvironmentAccess struct {
	ID            uuid.UUID      `gorm:"type:uuid;primaryKey" json:"id"`
	UserID        uuid.UUID      `gorm:"type:uuid;not null;uniqueIndex:idx_user_env" json:"user_id"`
	EnvironmentID uuid.UUID      `gorm:"type:uuid;not null;uniqueIndex:idx_user_env" json:"environment_id"`
	Role          string         `gorm:"type:text" json:"role,omitempty"` // optional override per environment
	CreatedAt     time.Time      `json:"created_at"`
	DeletedAt     gorm.DeletedAt `gorm:"index" json:"-"`

	User        interface{} `gorm:"-" json:"-"`
	Environment interface{} `gorm:"-" json:"-"`
}

func (UserEnvironmentAccess) TableName() string { return "user_environment_access" }

func (u *UserEnvironmentAccess) BeforeCreate(tx *gorm.DB) error {
	if u.ID == uuid.Nil {
		u.ID = uuid.New()
	}
	return nil
}
