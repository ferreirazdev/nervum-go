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
	ID          uuid.UUID  `gorm:"type:uuid;primaryKey" json:"id"`
	Name        string     `gorm:"type:text;not null" json:"name"`
	Description string     `gorm:"type:text" json:"description"`
	Website     string     `gorm:"type:text" json:"website"`
	OwnerID     *uuid.UUID `gorm:"type:uuid;index" json:"owner_id,omitempty"`
	// Billing (Stripe): subscription is per organization.
	StripeCustomerID     string         `gorm:"type:text;not null;default:''" json:"stripe_customer_id,omitempty"`
	StripeSubscriptionID string         `gorm:"type:text;not null;default:''" json:"stripe_subscription_id,omitempty"`
	BillingPlanID        *uuid.UUID     `gorm:"type:uuid;index" json:"billing_plan_id,omitempty"`
	SubscriptionStatus   string         `gorm:"type:text;not null;default:'none'" json:"subscription_status"`
	TrialEndsAt          *time.Time     `json:"trial_ends_at,omitempty"`
	CurrentPeriodEnd     *time.Time     `json:"current_period_end,omitempty"`
	CreatedAt            time.Time      `json:"created_at"`
	UpdatedAt            time.Time      `json:"updated_at"`
	DeletedAt            gorm.DeletedAt `gorm:"index" json:"-"`
}

func (Organization) TableName() string { return "organizations" }

func (o *Organization) BeforeCreate(tx *gorm.DB) error {
	if o.ID == uuid.Nil {
		o.ID = uuid.New()
	}
	return nil
}
