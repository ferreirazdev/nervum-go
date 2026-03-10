package integrations

import (
	"context"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Repository persists integrations. Tokens are stored encrypted; encryption is handled by the handler/service layer.
type Repository interface {
	Create(ctx context.Context, i *Integration) error
	GetByID(ctx context.Context, id uuid.UUID) (*Integration, error)
	GetByOrganizationAndProvider(ctx context.Context, orgID uuid.UUID, provider string) (*Integration, error)
	ListByOrganization(ctx context.Context, orgID uuid.UUID) ([]Integration, error)
	Update(ctx context.Context, i *Integration) error
	Delete(ctx context.Context, id uuid.UUID) error
}

type repository struct {
	db *gorm.DB
}

// NewRepository returns an integrations Repository backed by the given DB.
func NewRepository(db *gorm.DB) Repository {
	return &repository{db: db}
}

func (r *repository) Create(ctx context.Context, i *Integration) error {
	return r.db.WithContext(ctx).Create(i).Error
}

func (r *repository) GetByID(ctx context.Context, id uuid.UUID) (*Integration, error) {
	var i Integration
	err := r.db.WithContext(ctx).First(&i, "id = ?", id).Error
	if err != nil {
		return nil, err
	}
	return &i, nil
}

func (r *repository) GetByOrganizationAndProvider(ctx context.Context, orgID uuid.UUID, provider string) (*Integration, error) {
	var i Integration
	err := r.db.WithContext(ctx).First(&i, "organization_id = ? AND provider = ?", orgID, provider).Error
	if err != nil {
		return nil, err
	}
	return &i, nil
}

func (r *repository) ListByOrganization(ctx context.Context, orgID uuid.UUID) ([]Integration, error) {
	var list []Integration
	err := r.db.WithContext(ctx).Where("organization_id = ?", orgID).Find(&list).Error
	return list, err
}

func (r *repository) Update(ctx context.Context, i *Integration) error {
	return r.db.WithContext(ctx).Save(i).Error
}

func (r *repository) Delete(ctx context.Context, id uuid.UUID) error {
	return r.db.WithContext(ctx).Delete(&Integration{}, "id = ?", id).Error
}
