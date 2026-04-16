// Package billing exposes HTTP handlers for Stripe Checkout, webhooks, and billing plans.
package billing

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// BillingPlan is a sellable tier; stripe_price_id must exist in Stripe Dashboard.
type BillingPlan struct {
	ID                 uuid.UUID      `gorm:"type:uuid;primaryKey" json:"id"`
	Slug               string         `gorm:"type:text;uniqueIndex" json:"slug"`
	Name               string         `gorm:"type:text;not null" json:"name"`
	Description        string         `gorm:"type:text;not null;default:''" json:"description"`
	StripePriceID      string         `gorm:"column:stripe_price_id;type:text;not null" json:"stripe_price_id"`
	Currency           string         `gorm:"type:text;not null;default:'usd'" json:"currency"`
	PriceInterval      string         `gorm:"column:price_interval;type:text;not null;default:'month'" json:"price_interval"`
	DisplayAmountCents *int           `json:"display_amount_cents,omitempty"`
	Features           datatypes.JSON `gorm:"type:jsonb" json:"features"`
	Active             bool           `gorm:"not null;default:true" json:"active"`
	SortOrder          int            `gorm:"not null;default:0" json:"sort_order"`
	CreatedAt          time.Time      `json:"created_at"`
	UpdatedAt          time.Time      `json:"updated_at"`
	DeletedAt          gorm.DeletedAt `gorm:"index" json:"-"`
}

func (BillingPlan) TableName() string { return "billing_plans" }

func (p *BillingPlan) BeforeCreate(tx *gorm.DB) error {
	if p.ID == uuid.Nil {
		p.ID = uuid.New()
	}
	return nil
}
