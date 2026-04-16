package billing

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/nervum/nervum-go/internal/config"
	"github.com/nervum/nervum-go/internal/features/auth"
	organization "github.com/nervum/nervum-go/internal/features/organizations"
	user "github.com/nervum/nervum-go/internal/features/users"
	stripe "github.com/stripe/stripe-go/v81"
	billingportalsession "github.com/stripe/stripe-go/v81/billingportal/session"
	checkoutsession "github.com/stripe/stripe-go/v81/checkout/session"
	"github.com/stripe/stripe-go/v81/subscription"
	"github.com/stripe/stripe-go/v81/webhook"
	"gorm.io/gorm"
)

// Handler serves billing HTTP routes (plans, Stripe Checkout, webhooks).
type Handler struct {
	planRepo      PlanRepository
	orgRepo       organization.Repository
	stripeSecret  string
	webhookSecret string
	trialDays     int64
	frontendURL   string
}

// NewHandler builds a billing Handler.
func NewHandler(planRepo PlanRepository, orgRepo organization.Repository, stripeCfg *config.StripeConfig, frontendURL string) *Handler {
	trial := stripeCfg.TrialPeriodDays
	if trial <= 0 {
		trial = 15
	}
	return &Handler{
		planRepo:      planRepo,
		orgRepo:       orgRepo,
		stripeSecret:  stripeCfg.SecretKey,
		webhookSecret: stripeCfg.WebhookSecret,
		trialDays:     trial,
		frontendURL:   strings.TrimRight(strings.TrimSpace(frontendURL), "/"),
	}
}

func (h *Handler) setStripeKey() {
	stripe.Key = h.stripeSecret
}

// Register mounts public billing routes on api (no auth).
func (h *Handler) RegisterPublic(api *gin.RouterGroup, sensitive ...gin.HandlerFunc) {
	g := api.Group("/billing")
	handlers := append(sensitive, h.ListPlans)
	g.GET("/plans", handlers...)
	g.POST("/webhook", h.Webhook)
}

// RegisterProtected mounts billing routes that require an authenticated session.
func (h *Handler) RegisterProtected(protected *gin.RouterGroup) {
	g := protected.Group("/billing")
	g.POST("/checkout-session", h.CreateCheckoutSession)
	g.POST("/portal-session", h.CreatePortalSession)
	g.POST("/confirm-session", h.ConfirmCheckoutSession)
	g.GET("/subscription", h.GetSubscription)
}

type planResponse struct {
	ID                 uuid.UUID       `json:"id"`
	Slug               string          `json:"slug"`
	Name               string          `json:"name"`
	Description        string          `json:"description"`
	Currency           string          `json:"currency"`
	PriceInterval      string          `json:"price_interval"`
	DisplayAmountCents *int            `json:"display_amount_cents,omitempty"`
	Features           json.RawMessage `json:"features,omitempty"`
}

func (h *Handler) ListPlans(c *gin.Context) {
	list, err := h.planRepo.ListActive(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	out := make([]planResponse, 0, len(list))
	for _, p := range list {
		pr := planResponse{
			ID:                 p.ID,
			Slug:               p.Slug,
			Name:               p.Name,
			Description:        p.Description,
			Currency:           p.Currency,
			PriceInterval:      p.PriceInterval,
			DisplayAmountCents: p.DisplayAmountCents,
		}
		if len(p.Features) > 0 {
			pr.Features = json.RawMessage(p.Features)
		}
		out = append(out, pr)
	}
	c.JSON(http.StatusOK, out)
}

type checkoutSessionRequest struct {
	PlanSlug string `json:"plan_slug" binding:"required"`
}

func (h *Handler) CreateCheckoutSession(c *gin.Context) {
	if h.stripeSecret == "" {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "stripe is not configured"})
		return
	}
	u, ok := currentUser(c)
	if !ok {
		return
	}
	if u.OrganizationID == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "organization required"})
		return
	}
	var req checkoutSessionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	plan, err := h.planRepo.GetActiveBySlug(c.Request.Context(), strings.TrimSpace(req.PlanSlug))
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "plan not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	org, err := h.orgRepo.GetByID(c.Request.Context(), *u.OrganizationID)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "organization not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	if org.OwnerID == nil || *org.OwnerID != u.ID {
		c.JSON(http.StatusForbidden, gin.H{"error": "only the organization owner can subscribe"})
		return
	}
	if org.SubscriptionStatus == "trialing" || org.SubscriptionStatus == "active" {
		c.JSON(http.StatusConflict, gin.H{"error": "organization already has an active subscription"})
		return
	}

	meta := map[string]string{
		"organization_id": org.ID.String(),
		"billing_plan_id": plan.ID.String(),
	}
	h.setStripeKey()
	params := &stripe.CheckoutSessionParams{
		Mode:       stripe.String(string(stripe.CheckoutSessionModeSubscription)),
		SuccessURL: stripe.String(h.frontendURL + "/onboarding?billing=success&session_id={CHECKOUT_SESSION_ID}"),
		CancelURL:  stripe.String(h.frontendURL + "/onboarding?billing=cancel"),
		LineItems: []*stripe.CheckoutSessionLineItemParams{
			{Price: stripe.String(plan.StripePriceID), Quantity: stripe.Int64(1)},
		},
		CustomerEmail: stripe.String(u.Email),
		Metadata:      meta,
		SubscriptionData: &stripe.CheckoutSessionSubscriptionDataParams{
			TrialPeriodDays: stripe.Int64(h.trialDays),
			Metadata:        meta,
		},
	}
	sess, err := checkoutsession.New(params)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "stripe error", "detail": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"url": sess.URL})
}

type confirmSessionRequest struct {
	SessionID string `json:"session_id" binding:"required"`
}

func (h *Handler) ConfirmCheckoutSession(c *gin.Context) {
	if h.stripeSecret == "" {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "stripe is not configured"})
		return
	}
	u, ok := currentUser(c)
	if !ok {
		return
	}
	if u.OrganizationID == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "organization required"})
		return
	}
	var req confirmSessionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	org, err := h.orgRepo.GetByID(c.Request.Context(), *u.OrganizationID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	if org.OwnerID == nil || *org.OwnerID != u.ID {
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
		return
	}

	h.setStripeKey()
	params := &stripe.CheckoutSessionParams{}
	params.AddExpand("subscription")
	sess, err := checkoutsession.Get(strings.TrimSpace(req.SessionID), params)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid session"})
		return
	}
	if sess.Metadata == nil || sess.Metadata["organization_id"] != org.ID.String() {
		c.JSON(http.StatusForbidden, gin.H{"error": "session does not belong to this organization"})
		return
	}
	if sess.Status != stripe.CheckoutSessionStatusComplete {
		c.JSON(http.StatusBadRequest, gin.H{"error": "checkout session not complete"})
		return
	}
	if sess.Mode != stripe.CheckoutSessionModeSubscription {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid checkout mode"})
		return
	}
	if sess.Subscription == nil || sess.Subscription.ID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "subscription missing from session"})
		return
	}
	sub, err := subscription.Get(sess.Subscription.ID, nil)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "stripe error"})
		return
	}
	var planID *uuid.UUID
	if s := sess.Metadata["billing_plan_id"]; s != "" {
		if id, err := uuid.Parse(s); err == nil {
			planID = &id
		}
	}
	applySubscriptionToOrganization(org, sub, planID)
	if err := h.orgRepo.Update(c.Request.Context(), org); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "subscription_status": org.SubscriptionStatus})
}

func (h *Handler) CreatePortalSession(c *gin.Context) {
	if h.stripeSecret == "" {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "stripe is not configured"})
		return
	}
	u, ok := currentUser(c)
	if !ok {
		return
	}
	if u.OrganizationID == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "organization required"})
		return
	}
	org, err := h.orgRepo.GetByID(c.Request.Context(), *u.OrganizationID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	if org.OwnerID == nil || *org.OwnerID != u.ID {
		c.JSON(http.StatusForbidden, gin.H{"error": "only the organization owner can manage billing"})
		return
	}
	if org.StripeCustomerID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "no Stripe customer for this organization"})
		return
	}
	h.setStripeKey()
	portal, err := billingportalsession.New(&stripe.BillingPortalSessionParams{
		Customer:  stripe.String(org.StripeCustomerID),
		ReturnURL: stripe.String(h.frontendURL + "/billing"),
	})
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "stripe error", "detail": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"url": portal.URL})
}

type subscriptionResponse struct {
	OrganizationID     uuid.UUID  `json:"organization_id"`
	SubscriptionStatus string     `json:"subscription_status"`
	TrialEndsAt        *time.Time `json:"trial_ends_at,omitempty"`
	CurrentPeriodEnd   *time.Time `json:"current_period_end,omitempty"`
	PlanSlug           string     `json:"plan_slug,omitempty"`
	PlanName           string     `json:"plan_name,omitempty"`
	IsOwner            bool       `json:"is_owner"`
}

func (h *Handler) GetSubscription(c *gin.Context) {
	u, ok := currentUser(c)
	if !ok {
		return
	}
	if u.OrganizationID == nil {
		c.JSON(http.StatusOK, subscriptionResponse{IsOwner: false})
		return
	}
	org, err := h.orgRepo.GetByID(c.Request.Context(), *u.OrganizationID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	isOwner := org.OwnerID != nil && *org.OwnerID == u.ID
	resp := subscriptionResponse{
		OrganizationID:     org.ID,
		SubscriptionStatus: org.SubscriptionStatus,
		TrialEndsAt:        org.TrialEndsAt,
		CurrentPeriodEnd:   org.CurrentPeriodEnd,
		IsOwner:            isOwner,
	}
	if org.BillingPlanID != nil {
		if plan, err := h.planRepo.GetByID(c.Request.Context(), *org.BillingPlanID); err == nil {
			resp.PlanSlug = plan.Slug
			resp.PlanName = plan.Name
		}
	}
	c.JSON(http.StatusOK, resp)
}

func (h *Handler) Webhook(c *gin.Context) {
	if h.webhookSecret == "" {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "webhook not configured"})
		return
	}
	payload, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "read body"})
		return
	}
	sig := c.GetHeader("Stripe-Signature")
	ev, err := webhook.ConstructEvent(payload, sig, h.webhookSecret)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid signature"})
		return
	}
	h.setStripeKey()
	switch ev.Type {
	case "checkout.session.completed":
		var sess stripe.CheckoutSession
		if err := json.Unmarshal(ev.Data.Raw, &sess); err != nil {
			break
		}
		if sess.Mode != stripe.CheckoutSessionModeSubscription {
			c.JSON(http.StatusOK, gin.H{"received": true})
			return
		}
		if sess.Metadata == nil {
			c.JSON(http.StatusOK, gin.H{"received": true})
			return
		}
		orgID, err := uuid.Parse(sess.Metadata["organization_id"])
		if err != nil {
			c.JSON(http.StatusOK, gin.H{"received": true})
			return
		}
		org, err := h.orgRepo.GetByID(c.Request.Context(), orgID)
		if err != nil {
			c.JSON(http.StatusOK, gin.H{"received": true})
			return
		}
		subID := ""
		if sess.Subscription != nil {
			subID = sess.Subscription.ID
		}
		if subID == "" {
			c.JSON(http.StatusOK, gin.H{"received": true})
			return
		}
		sub, err := subscription.Get(subID, nil)
		if err != nil {
			c.JSON(http.StatusOK, gin.H{"received": true})
			return
		}
		var planID *uuid.UUID
		if s := sess.Metadata["billing_plan_id"]; s != "" {
			if id, err := uuid.Parse(s); err == nil {
				planID = &id
			}
		}
		applySubscriptionToOrganization(org, sub, planID)
		if sess.Customer != nil && sess.Customer.ID != "" {
			org.StripeCustomerID = sess.Customer.ID
		}
		_ = h.orgRepo.Update(c.Request.Context(), org)

	case "customer.subscription.updated", "customer.subscription.deleted":
		var sub stripe.Subscription
		if err := json.Unmarshal(ev.Data.Raw, &sub); err != nil {
			break
		}
		org, err := h.orgRepo.GetByStripeSubscriptionID(c.Request.Context(), sub.ID)
		if err != nil {
			c.JSON(http.StatusOK, gin.H{"received": true})
			return
		}
		planID := org.BillingPlanID
		applySubscriptionToOrganization(org, &sub, planID)
		if ev.Type == "customer.subscription.deleted" {
			org.SubscriptionStatus = "canceled"
		}
		_ = h.orgRepo.Update(c.Request.Context(), org)

	case "invoice.payment_failed":
		var inv stripe.Invoice
		if err := json.Unmarshal(ev.Data.Raw, &inv); err != nil {
			break
		}
		subID := ""
		if inv.Subscription != nil {
			subID = inv.Subscription.ID
		}
		if subID == "" {
			break
		}
		org, err := h.orgRepo.GetByStripeSubscriptionID(c.Request.Context(), subID)
		if err != nil {
			break
		}
		org.SubscriptionStatus = "past_due"
		_ = h.orgRepo.Update(c.Request.Context(), org)
	}
	c.JSON(http.StatusOK, gin.H{"received": true})
}

func currentUser(c *gin.Context) (*user.User, bool) {
	val, ok := c.Get(auth.ContextUser)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
		return nil, false
	}
	u, ok := val.(*user.User)
	if !ok || u == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
		return nil, false
	}
	return u, true
}
