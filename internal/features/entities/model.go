package entity

import (
	"time"

	"github.com/google/uuid"
	"github.com/nervum/nervum-go/internal/pkg/types"
	"gorm.io/gorm"
)

const (
	TypeService  = "service"
	TypeDatabase = "database"
	TypeInfra   = "infra"
	TypeTeam    = "team"
	TypeRoadmap = "roadmap"
	TypeCost    = "cost"
	TypeMetric  = "metric"
)

const (
	StatusHealthy  = "healthy"
	StatusWarning  = "warning"
	StatusCritical = "critical"
)

type Entity struct {
	ID             uuid.UUID      `gorm:"type:uuid;primaryKey" json:"id"`
	OrganizationID uuid.UUID      `gorm:"type:uuid;not null;index" json:"organization_id"`
	EnvironmentID  uuid.UUID      `gorm:"type:uuid;not null;index" json:"environment_id"`
	Type           string         `gorm:"type:text;not null" json:"type"`
	Name           string         `gorm:"type:text;not null" json:"name"`
	Status         string         `gorm:"type:text" json:"status"` // healthy, warning, critical
	OwnerTeamID    *uuid.UUID     `gorm:"type:uuid" json:"owner_team_id,omitempty"`
	Metadata       types.JSONB    `gorm:"type:jsonb" json:"metadata,omitempty"`
	CreatedAt      time.Time      `json:"created_at"`
	UpdatedAt      time.Time      `json:"updated_at"`
	DeletedAt      gorm.DeletedAt `gorm:"index" json:"-"`

	Organization interface{} `gorm:"-" json:"-"`
	Environment  interface{} `gorm:"-" json:"-"`
}

func (Entity) TableName() string { return "entities" }

func (e *Entity) BeforeCreate(tx *gorm.DB) error {
	if e.ID == uuid.Nil {
		e.ID = uuid.New()
	}
	return nil
}
