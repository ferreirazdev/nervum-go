package relationship

import (
	"time"

	"github.com/google/uuid"
	"github.com/nervum/nervum-go/internal/pkg/types"
	"gorm.io/gorm"
)

const (
	TypeDependsOn     = "depends_on"
	TypeOwnedBy       = "owned_by"
	TypeRunsOn        = "runs_on"
	TypeStoresDataIn  = "stores_data_in"
	TypeGeneratesCost = "generates_cost"
	TypeMonitoredBy   = "monitored_by"
)

type Relationship struct {
	ID             uuid.UUID      `gorm:"type:uuid;primaryKey" json:"id"`
	OrganizationID uuid.UUID      `gorm:"type:uuid;not null;index" json:"organization_id"`
	FromEntityID   uuid.UUID      `gorm:"type:uuid;not null;index" json:"from_entity_id"`
	ToEntityID     uuid.UUID      `gorm:"type:uuid;not null;index" json:"to_entity_id"`
	Type           string         `gorm:"type:text;not null" json:"type"`
	Metadata       types.JSONB    `gorm:"type:jsonb" json:"metadata,omitempty"`
	CreatedAt      time.Time      `json:"created_at"`
	DeletedAt      gorm.DeletedAt `gorm:"index" json:"-"`

	Organization interface{} `gorm:"-" json:"-"`
}

func (Relationship) TableName() string { return "relationships" }

func (r *Relationship) BeforeCreate(tx *gorm.DB) error {
	if r.ID == uuid.Nil {
		r.ID = uuid.New()
	}
	return nil
}
