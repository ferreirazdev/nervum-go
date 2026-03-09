// Package teams provides CRUD for teams and team-environment links. Teams belong to an
// organization and can be granted access to specific environments; members are via user_teams.
package teams

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Team represents a team within an organization. Stored in table teams.
type Team struct {
	ID             uuid.UUID      `gorm:"type:uuid;primaryKey" json:"id"`
	OrganizationID uuid.UUID      `gorm:"type:uuid;not null;index" json:"organization_id"`
	Name           string         `gorm:"type:text;not null" json:"name"`
	Icon           string         `gorm:"type:text" json:"icon"` // emoji or icon name e.g. "users"
	CreatedAt      time.Time      `json:"created_at"`
	UpdatedAt      time.Time      `json:"updated_at"`
	DeletedAt      gorm.DeletedAt `gorm:"index" json:"-"`

	Environments []TeamEnvironment `gorm:"foreignKey:TeamID" json:"environments,omitempty"`
}

func (Team) TableName() string { return "teams" }

func (t *Team) BeforeCreate(tx *gorm.DB) error {
	if t.ID == uuid.Nil {
		t.ID = uuid.New()
	}
	return nil
}

// TeamEnvironment links a team to environments it can access. Stored in team_environments.
type TeamEnvironment struct {
	TeamID        uuid.UUID `gorm:"type:uuid;not null;uniqueIndex:idx_team_env" json:"team_id"`
	EnvironmentID uuid.UUID `gorm:"type:uuid;not null;uniqueIndex:idx_team_env" json:"environment_id"`
}

func (TeamEnvironment) TableName() string { return "team_environments" }
