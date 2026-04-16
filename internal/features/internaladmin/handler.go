// Package internaladmin exposes HTTP routes for operators (email allowlist only).
package internaladmin

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/nervum/nervum-go/internal/features/billing"
	organization "github.com/nervum/nervum-go/internal/features/organizations"
	user "github.com/nervum/nervum-go/internal/features/users"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// Handler serves internal admin APIs.
type Handler struct {
	planRepo billing.PlanRepository
	userRepo user.Repository
	orgRepo  organization.Repository
}

// NewHandler returns an internal admin Handler.
func NewHandler(planRepo billing.PlanRepository, userRepo user.Repository, orgRepo organization.Repository) *Handler {
	return &Handler{planRepo: planRepo, userRepo: userRepo, orgRepo: orgRepo}
}

// Register mounts routes on g (e.g. /api/v1/internal after middleware).
func (h *Handler) Register(g *gin.RouterGroup) {
	g.GET("/admin/status", h.AdminStatus)
	g.GET("/plans", h.ListPlans)
	g.POST("/plans", h.CreatePlan)
	g.PUT("/plans/:id", h.UpdatePlan)
	g.DELETE("/plans/:id", h.DeletePlan)
	g.GET("/users", h.ListUsers)
	g.GET("/users/:id", h.GetUser)
	g.PUT("/users/:id", h.UpdateUser)
	g.GET("/organizations", h.ListOrganizations)
	g.GET("/organizations/:id", h.GetOrganization)
	g.PUT("/organizations/:id", h.UpdateOrganization)
}

func (h *Handler) AdminStatus(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (h *Handler) ListPlans(c *gin.Context) {
	list, err := h.planRepo.ListAll(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	c.JSON(http.StatusOK, list)
}

type createPlanBody struct {
	Slug               string          `json:"slug" binding:"required"`
	Name               string          `json:"name" binding:"required"`
	Description        string          `json:"description"`
	StripePriceID      string          `json:"stripe_price_id" binding:"required"`
	Currency           string          `json:"currency"`
	PriceInterval      string          `json:"price_interval"`
	DisplayAmountCents *int            `json:"display_amount_cents"`
	Features           json.RawMessage `json:"features"`
	Active             *bool           `json:"active"`
	SortOrder          *int            `json:"sort_order"`
}

func (h *Handler) CreatePlan(c *gin.Context) {
	var body createPlanBody
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	p := &billing.BillingPlan{
		Slug:               strings.TrimSpace(body.Slug),
		Name:               strings.TrimSpace(body.Name),
		Description:        strings.TrimSpace(body.Description),
		StripePriceID:      strings.TrimSpace(body.StripePriceID),
		Currency:           strings.TrimSpace(defaultStr(body.Currency, "usd")),
		PriceInterval:      strings.TrimSpace(defaultStr(body.PriceInterval, "month")),
		DisplayAmountCents: body.DisplayAmountCents,
		Active:             body.Active == nil || *body.Active,
		SortOrder:          0,
	}
	if body.SortOrder != nil {
		p.SortOrder = *body.SortOrder
	}
	if len(body.Features) > 0 {
		p.Features = datatypes.JSON(body.Features)
	}
	if err := h.planRepo.Create(c.Request.Context(), p); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	c.JSON(http.StatusCreated, p)
}

type updatePlanBody struct {
	Slug               *string          `json:"slug"`
	Name               *string          `json:"name"`
	Description        *string          `json:"description"`
	StripePriceID      *string          `json:"stripe_price_id"`
	Currency           *string          `json:"currency"`
	PriceInterval      *string          `json:"price_interval"`
	DisplayAmountCents *int             `json:"display_amount_cents"`
	Features           *json.RawMessage `json:"features"`
	Active             *bool            `json:"active"`
	SortOrder          *int             `json:"sort_order"`
}

func (h *Handler) UpdatePlan(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	p, err := h.planRepo.GetByIDUnscoped(c.Request.Context(), id)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	var body updatePlanBody
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if body.Slug != nil {
		p.Slug = strings.TrimSpace(*body.Slug)
	}
	if body.Name != nil {
		p.Name = strings.TrimSpace(*body.Name)
	}
	if body.Description != nil {
		p.Description = strings.TrimSpace(*body.Description)
	}
	if body.StripePriceID != nil {
		p.StripePriceID = strings.TrimSpace(*body.StripePriceID)
	}
	if body.Currency != nil {
		p.Currency = strings.TrimSpace(*body.Currency)
	}
	if body.PriceInterval != nil {
		p.PriceInterval = strings.TrimSpace(*body.PriceInterval)
	}
	if body.DisplayAmountCents != nil {
		p.DisplayAmountCents = body.DisplayAmountCents
	}
	if body.Features != nil {
		p.Features = datatypes.JSON(*body.Features)
	}
	if body.Active != nil {
		p.Active = *body.Active
	}
	if body.SortOrder != nil {
		p.SortOrder = *body.SortOrder
	}
	if err := h.planRepo.Update(c.Request.Context(), p); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	c.JSON(http.StatusOK, p)
}

func (h *Handler) DeletePlan(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	if err := h.planRepo.Delete(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *Handler) ListUsers(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "100"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	list, total, err := h.userRepo.ListPaged(c.Request.Context(), limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"users": list, "total": total})
}

func (h *Handler) GetUser(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	u, err := h.userRepo.GetByID(c.Request.Context(), id)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	c.JSON(http.StatusOK, u)
}

type updateUserBody struct {
	Name           *string    `json:"name"`
	OrganizationID *uuid.UUID `json:"organization_id"`
	Role           *string    `json:"role"`
	Onboarding     *bool      `json:"onboarding"`
}

func (h *Handler) UpdateUser(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	u, err := h.userRepo.GetByID(c.Request.Context(), id)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	var body updateUserBody
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if body.Name != nil {
		u.Name = strings.TrimSpace(*body.Name)
	}
	if body.OrganizationID != nil {
		u.OrganizationID = body.OrganizationID
	}
	if body.Role != nil {
		r := strings.TrimSpace(*body.Role)
		if r != user.RoleAdmin && r != user.RoleManager && r != user.RoleMember {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid role"})
			return
		}
		u.Role = r
	}
	if body.Onboarding != nil {
		u.OnboardingCompleted = *body.Onboarding
	}
	if err := h.userRepo.Update(c.Request.Context(), u); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	c.JSON(http.StatusOK, u)
}

func (h *Handler) ListOrganizations(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "100"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	list, total, err := h.orgRepo.ListPaged(c.Request.Context(), limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"organizations": list, "total": total})
}

func (h *Handler) GetOrganization(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	o, err := h.orgRepo.GetByID(c.Request.Context(), id)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	c.JSON(http.StatusOK, o)
}

type updateOrgBody struct {
	Name                 *string    `json:"name"`
	Description          *string    `json:"description"`
	Website              *string    `json:"website"`
	OwnerID              *uuid.UUID `json:"owner_id"`
	StripeCustomerID     *string    `json:"stripe_customer_id"`
	StripeSubscriptionID *string    `json:"stripe_subscription_id"`
	BillingPlanID        *uuid.UUID `json:"billing_plan_id"`
	SubscriptionStatus   *string    `json:"subscription_status"`
	TrialEndsAt          *string    `json:"trial_ends_at"`
	CurrentPeriodEnd     *string    `json:"current_period_end"`
}

func (h *Handler) UpdateOrganization(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	o, err := h.orgRepo.GetByID(c.Request.Context(), id)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	var body updateOrgBody
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if body.Name != nil {
		o.Name = strings.TrimSpace(*body.Name)
	}
	if body.Description != nil {
		o.Description = *body.Description
	}
	if body.Website != nil {
		o.Website = *body.Website
	}
	if body.OwnerID != nil {
		o.OwnerID = body.OwnerID
	}
	if body.StripeCustomerID != nil {
		o.StripeCustomerID = *body.StripeCustomerID
	}
	if body.StripeSubscriptionID != nil {
		o.StripeSubscriptionID = *body.StripeSubscriptionID
	}
	if body.BillingPlanID != nil {
		o.BillingPlanID = body.BillingPlanID
	}
	if body.SubscriptionStatus != nil {
		o.SubscriptionStatus = strings.TrimSpace(*body.SubscriptionStatus)
	}
	if body.TrialEndsAt != nil {
		if strings.TrimSpace(*body.TrialEndsAt) == "" {
			o.TrialEndsAt = nil
		} else {
			t, err := time.Parse(time.RFC3339, strings.TrimSpace(*body.TrialEndsAt))
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "trial_ends_at must be RFC3339"})
				return
			}
			o.TrialEndsAt = &t
		}
	}
	if body.CurrentPeriodEnd != nil {
		if strings.TrimSpace(*body.CurrentPeriodEnd) == "" {
			o.CurrentPeriodEnd = nil
		} else {
			t, err := time.Parse(time.RFC3339, strings.TrimSpace(*body.CurrentPeriodEnd))
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "current_period_end must be RFC3339"})
				return
			}
			o.CurrentPeriodEnd = &t
		}
	}
	if err := h.orgRepo.Update(c.Request.Context(), o); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	c.JSON(http.StatusOK, o)
}

func defaultStr(s, def string) string {
	if strings.TrimSpace(s) == "" {
		return def
	}
	return s
}
