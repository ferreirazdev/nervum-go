package organization

import (
	"context"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Repository persists organizations. Used by the handler and auth (e.g. auto-create org on register).
type Repository interface {
	Create(ctx context.Context, o *Organization) error
	GetByID(ctx context.Context, id uuid.UUID) (*Organization, error)
	GetByStripeSubscriptionID(ctx context.Context, stripeSubscriptionID string) (*Organization, error)
	List(ctx context.Context) ([]Organization, error)
	ListPaged(ctx context.Context, limit, offset int) ([]Organization, int64, error)
	Update(ctx context.Context, o *Organization) error
	Delete(ctx context.Context, id uuid.UUID) error
}

type repository struct {
	db *gorm.DB
}

// NewRepository returns an organization Repository backed by the given DB.
func NewRepository(db *gorm.DB) Repository {
	return &repository{db: db}
}

func (r *repository) Create(ctx context.Context, o *Organization) error {
	return r.db.WithContext(ctx).Create(o).Error
}

func (r *repository) GetByID(ctx context.Context, id uuid.UUID) (*Organization, error) {
	var o Organization
	err := r.db.WithContext(ctx).First(&o, "id = ?", id).Error
	if err != nil {
		return nil, err
	}
	return &o, nil
}

func (r *repository) GetByStripeSubscriptionID(ctx context.Context, stripeSubscriptionID string) (*Organization, error) {
	if stripeSubscriptionID == "" {
		return nil, gorm.ErrRecordNotFound
	}
	var o Organization
	err := r.db.WithContext(ctx).Where("stripe_subscription_id = ?", stripeSubscriptionID).First(&o).Error
	if err != nil {
		return nil, err
	}
	return &o, nil
}

func (r *repository) List(ctx context.Context) ([]Organization, error) {
	var list []Organization
	err := r.db.WithContext(ctx).Find(&list).Error
	return list, err
}

func (r *repository) ListPaged(ctx context.Context, limit, offset int) ([]Organization, int64, error) {
	if limit <= 0 {
		limit = 100
	}
	if limit > 500 {
		limit = 500
	}
	if offset < 0 {
		offset = 0
	}
	var total int64
	if err := r.db.WithContext(ctx).Model(&Organization{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var list []Organization
	err := r.db.WithContext(ctx).Order("created_at DESC").Limit(limit).Offset(offset).Find(&list).Error
	return list, total, err
}

func (r *repository) Update(ctx context.Context, o *Organization) error {
	return r.db.WithContext(ctx).Save(o).Error
}

func (r *repository) Delete(ctx context.Context, id uuid.UUID) error {
	return r.db.WithContext(ctx).Delete(&Organization{}, "id = ?", id).Error
}
