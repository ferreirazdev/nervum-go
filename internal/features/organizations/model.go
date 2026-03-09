// Package organization provides CRUD for organizations (tenants). Used by the API server
// for multi-tenant isolation; each user can own or belong to an organization.
package organization

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Organization represents a tenant in the system. Stored in table organizations.
type Organization struct {
	ID          uuid.UUID      `gorm:"type:uuid;primaryKey" json:"id"`
	Name        string         `gorm:"type:text;not null" json:"name"`
	Description string         `gorm:"type:text" json:"description"`
	Website     string         `gorm:"type:text" json:"website"`
	OwnerID     *uuid.UUID     `gorm:"type:uuid;index" json:"owner_id,omitempty"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	DeletedAt   gorm.DeletedAt `gorm:"index" json:"-"`
}

func (Organization) TableName() string { return "organizations" }

func (o *Organization) BeforeCreate(tx *gorm.DB) error {
	if o.ID == uuid.Nil {
		o.ID = uuid.New()
	}
	return nil
}
