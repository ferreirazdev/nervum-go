// Package userteam provides CRUD for user-team membership (linking users to teams).
package userteam

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// UserTeam represents a user's membership in a team. Stored in table user_teams.
type UserTeam struct {
	ID        uuid.UUID      `gorm:"type:uuid;primaryKey" json:"id"`
	UserID    uuid.UUID      `gorm:"type:uuid;not null;uniqueIndex:idx_user_team" json:"user_id"`
	TeamID    uuid.UUID      `gorm:"type:uuid;not null;uniqueIndex:idx_user_team" json:"team_id"`
	Role      string         `gorm:"type:text" json:"role,omitempty"` // optional
	CreatedAt time.Time      `json:"created_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`

	User interface{} `gorm:"-" json:"-"`
	Team interface{} `gorm:"-" json:"-"`
}

func (UserTeam) TableName() string { return "user_teams" }

func (u *UserTeam) BeforeCreate(tx *gorm.DB) error {
	if u.ID == uuid.Nil {
		u.ID = uuid.New()
	}
	return nil
}
