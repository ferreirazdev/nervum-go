package billing

import (
	"time"

	"github.com/google/uuid"
	organization "github.com/nervum/nervum-go/internal/features/organizations"
	stripe "github.com/stripe/stripe-go/v81"
)

func applySubscriptionToOrganization(o *organization.Organization, sub *stripe.Subscription, planID *uuid.UUID) {
	if sub == nil {
		return
	}
	o.StripeSubscriptionID = sub.ID
	o.SubscriptionStatus = string(sub.Status)
	if planID != nil {
		o.BillingPlanID = planID
	}
	if sub.TrialEnd > 0 {
		t := time.Unix(sub.TrialEnd, 0).UTC()
		o.TrialEndsAt = &t
	} else {
		o.TrialEndsAt = nil
	}
	if sub.CurrentPeriodEnd > 0 {
		t := time.Unix(sub.CurrentPeriodEnd, 0).UTC()
		o.CurrentPeriodEnd = &t
	}
	if sub.Customer != nil && sub.Customer.ID != "" {
		o.StripeCustomerID = sub.Customer.ID
	}
}
