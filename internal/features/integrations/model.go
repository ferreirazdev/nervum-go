// Package integrations provides org-scoped external integrations (GitHub, GCloud).
// Tokens are stored encrypted; OAuth connect/callback and dashboard proxy live in handler.
package integrations

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

const (
	ProviderGitHub = "github"
	ProviderGCloud = "gcloud"
)

// Integration stores OAuth tokens and metadata for one provider per organization.
// access_token and refresh_token are encrypted at rest.
type Integration struct {
	ID                   uuid.UUID      `gorm:"type:uuid;primaryKey" json:"id"`
	OrganizationID       uuid.UUID      `gorm:"type:uuid;not null;uniqueIndex:idx_integrations_org_provider" json:"organization_id"`
	Provider             string         `gorm:"type:text;not null;uniqueIndex:idx_integrations_org_provider" json:"provider"` // github | gcloud
	AccessToken          string         `gorm:"type:text;not null" json:"-"`                                                     // encrypted
	RefreshToken         string         `gorm:"type:text" json:"-"`                                                             // encrypted, optional
	AccessTokenExpiresAt time.Time      `gorm:"type:timestamptz" json:"-"`                                                      // when the access token expires (GCloud only)
	Scopes               string         `gorm:"type:text" json:"scopes,omitempty"`
	ConnectedAt          time.Time      `gorm:"not null" json:"connected_at"`
	Metadata             datatypes.JSON `gorm:"type:jsonb" json:"metadata,omitempty"` // e.g. {"owner":"x","repo":"y"} or {"project_id":"..."}
	CreatedAt            time.Time      `json:"created_at"`
	UpdatedAt            time.Time      `json:"updated_at"`
}

func (Integration) TableName() string { return "integrations" }

func (i *Integration) BeforeCreate(tx *gorm.DB) error {
	if i.ID == uuid.Nil {
		i.ID = uuid.New()
	}
	return nil
}
