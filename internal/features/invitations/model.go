package invitation

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

const (
	StatusPending  = "pending"
	StatusAccepted = "accepted"
	StatusExpired  = "expired"
)

type Invitation struct {
	ID             uuid.UUID      `gorm:"type:uuid;primaryKey" json:"id"`
	Token          string         `gorm:"type:text;uniqueIndex;not null" json:"token"`
	Email          string         `gorm:"type:text;not null" json:"email"`
	OrganizationID uuid.UUID      `gorm:"type:uuid;not null;index" json:"organization_id"`
	InvitedByID    uuid.UUID      `gorm:"type:uuid;not null;index" json:"invited_by_id"`
	Role           string         `gorm:"type:text;default:member" json:"role"`
	EnvironmentID  *uuid.UUID     `gorm:"type:uuid;index" json:"environment_id,omitempty"`
	ExpiresAt      time.Time      `gorm:"not null" json:"expires_at"`
	Status         string         `gorm:"type:text;not null" json:"status"`
	AcceptedAt     *time.Time     `json:"accepted_at,omitempty"`
	CreatedAt      time.Time      `json:"created_at"`
	DeletedAt      gorm.DeletedAt `gorm:"index" json:"-"`

	Teams []InvitationTeam `gorm:"foreignKey:InvitationID" json:"teams,omitempty"`
}

func (Invitation) TableName() string { return "invitations" }

func (i *Invitation) BeforeCreate(tx *gorm.DB) error {
	if i.ID == uuid.Nil {
		i.ID = uuid.New()
	}
	if i.Status == "" {
		i.Status = StatusPending
	}
	if i.Role == "" {
		i.Role = "member"
	}
	return nil
}

type InvitationTeam struct {
	InvitationID uuid.UUID `gorm:"type:uuid;not null;uniqueIndex:idx_inv_team" json:"invitation_id"`
	TeamID        uuid.UUID `gorm:"type:uuid;not null;uniqueIndex:idx_inv_team" json:"team_id"`
}

func (InvitationTeam) TableName() string { return "invitation_teams" }
