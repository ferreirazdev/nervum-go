// Package orgservices provides storage for organization-tracked cloud services (e.g. GCloud Cloud Run).
package orgservices

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// OrganizationService represents a cloud service tracked by an organization (e.g. Cloud Run service name).
// Kind distinguishes resource types: cloud_run, cloud_sql, compute.
type OrganizationService struct {
	ID             uuid.UUID `gorm:"type:uuid;primaryKey" json:"id"`
	OrganizationID uuid.UUID `gorm:"type:uuid;not null;uniqueIndex:idx_org_svcs_provider_kind_name" json:"organization_id"`
	Provider       string    `gorm:"type:text;not null;uniqueIndex:idx_org_svcs_provider_kind_name" json:"provider"`       // e.g. "gcloud"
	Kind           string    `gorm:"type:text;not null;default:cloud_run;uniqueIndex:idx_org_svcs_provider_kind_name" json:"kind"` // cloud_run, cloud_sql, compute
	ServiceName    string    `gorm:"type:text;not null;uniqueIndex:idx_org_svcs_provider_kind_name" json:"service_name"`   // Cloud Run service name, instance name, or VM name
	Location       string    `gorm:"type:text" json:"location,omitempty"`                                                 // optional region/zone
	InstanceType   string    `gorm:"type:text" json:"instance_type,omitempty"`                                           // e.g. Cloud SQL databaseVersion, Compute machine type
	CreatedAt      time.Time `json:"created_at"`
}

func (OrganizationService) TableName() string { return "organization_services" }

func (s *OrganizationService) BeforeCreate(tx *gorm.DB) error {
	if s.ID == uuid.Nil {
		s.ID = uuid.New()
	}
	if s.Kind == "" {
		s.Kind = "cloud_run"
	}
	return nil
}
