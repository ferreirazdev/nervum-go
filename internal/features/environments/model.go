package environment

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Environment struct {
	ID             uuid.UUID      `gorm:"type:uuid;primaryKey" json:"id"`
	OrganizationID uuid.UUID      `gorm:"type:uuid;not null;index" json:"organization_id"`
	Name           string         `gorm:"type:text" json:"name"` // prod, staging, dev
	CreatedAt      time.Time      `json:"created_at"`
	DeletedAt      gorm.DeletedAt `gorm:"index" json:"-"`

	Organization interface{} `gorm:"-" json:"-"`
}

func (Environment) TableName() string { return "environments" }

func (e *Environment) BeforeCreate(tx *gorm.DB) error {
	if e.ID == uuid.Nil {
		e.ID = uuid.New()
	}
	return nil
}
