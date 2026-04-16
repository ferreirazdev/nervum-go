package billing

import (
	"context"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// PlanRepository loads billing plans from the database.
type PlanRepository interface {
	ListActive(ctx context.Context) ([]BillingPlan, error)
	GetActiveBySlug(ctx context.Context, slug string) (*BillingPlan, error)
	GetByID(ctx context.Context, id uuid.UUID) (*BillingPlan, error)
	GetByIDUnscoped(ctx context.Context, id uuid.UUID) (*BillingPlan, error)
	// Admin (all plans, including inactive and placeholder stripe ids)
	ListAll(ctx context.Context) ([]BillingPlan, error)
	Create(ctx context.Context, p *BillingPlan) error
	Update(ctx context.Context, p *BillingPlan) error
	Delete(ctx context.Context, id uuid.UUID) error
}

type planRepository struct {
	db *gorm.DB
}

// NewPlanRepository returns a PlanRepository backed by the given DB.
func NewPlanRepository(db *gorm.DB) PlanRepository {
	return &planRepository{db: db}
}

func (r *planRepository) ListActive(ctx context.Context) ([]BillingPlan, error) {
	var list []BillingPlan
	err := r.db.WithContext(ctx).
		Where("active = ? AND stripe_price_id NOT LIKE ?", true, "%REPLACE%").
		Order("sort_order ASC, name ASC").
		Find(&list).Error
	return list, err
}

func (r *planRepository) GetActiveBySlug(ctx context.Context, slug string) (*BillingPlan, error) {
	var p BillingPlan
	err := r.db.WithContext(ctx).
		Where("slug = ? AND active = ? AND stripe_price_id NOT LIKE ?", slug, true, "%REPLACE%").
		First(&p).Error
	if err != nil {
		return nil, err
	}
	return &p, nil
}

func (r *planRepository) GetByID(ctx context.Context, id uuid.UUID) (*BillingPlan, error) {
	var p BillingPlan
	err := r.db.WithContext(ctx).First(&p, "id = ?", id).Error
	if err != nil {
		return nil, err
	}
	return &p, nil
}

func (r *planRepository) GetByIDUnscoped(ctx context.Context, id uuid.UUID) (*BillingPlan, error) {
	var p BillingPlan
	err := r.db.WithContext(ctx).Unscoped().First(&p, "id = ?", id).Error
	if err != nil {
		return nil, err
	}
	return &p, nil
}

func (r *planRepository) ListAll(ctx context.Context) ([]BillingPlan, error) {
	var list []BillingPlan
	err := r.db.WithContext(ctx).Unscoped().Order("sort_order ASC, name ASC").Find(&list).Error
	return list, err
}

func (r *planRepository) Create(ctx context.Context, p *BillingPlan) error {
	return r.db.WithContext(ctx).Create(p).Error
}

func (r *planRepository) Update(ctx context.Context, p *BillingPlan) error {
	return r.db.WithContext(ctx).Save(p).Error
}

func (r *planRepository) Delete(ctx context.Context, id uuid.UUID) error {
	return r.db.WithContext(ctx).Delete(&BillingPlan{}, "id = ?", id).Error
}
