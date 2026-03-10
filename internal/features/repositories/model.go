// Package repositories provides storage for organization-tracked repositories (e.g. GitHub owner/repo).
package repositories

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// OrganizationRepository represents a repository tracked by an organization (e.g. for dashboard).
type OrganizationRepository struct {
	ID             uuid.UUID `gorm:"type:uuid;primaryKey" json:"id"`
	OrganizationID uuid.UUID `gorm:"type:uuid;not null;uniqueIndex:idx_org_repos_provider_full" json:"organization_id"`
	Provider       string    `gorm:"type:text;not null;uniqueIndex:idx_org_repos_provider_full" json:"provider"` // e.g. "github"
	FullName       string    `gorm:"type:text;not null;uniqueIndex:idx_org_repos_provider_full" json:"full_name"` // "owner/repo"
	CreatedAt      time.Time `json:"created_at"`
}

func (OrganizationRepository) TableName() string { return "organization_repositories" }

func (r *OrganizationRepository) BeforeCreate(tx *gorm.DB) error {
	if r.ID == uuid.Nil {
		r.ID = uuid.New()
	}
	return nil
}
