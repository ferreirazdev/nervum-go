package environment

import (
	"context"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Repository interface {
	Create(ctx context.Context, e *Environment) error
	GetByID(ctx context.Context, id uuid.UUID) (*Environment, error)
	ListByOrganization(ctx context.Context, orgID uuid.UUID) ([]Environment, error)
	Update(ctx context.Context, e *Environment) error
	Delete(ctx context.Context, id uuid.UUID) error
}

type repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) Repository {
	return &repository{db: db}
}

func (r *repository) Create(ctx context.Context, e *Environment) error {
	return r.db.WithContext(ctx).Create(e).Error
}

func (r *repository) GetByID(ctx context.Context, id uuid.UUID) (*Environment, error) {
	var e Environment
	err := r.db.WithContext(ctx).First(&e, "id = ?", id).Error
	if err != nil {
		return nil, err
	}
	return &e, nil
}

func (r *repository) ListByOrganization(ctx context.Context, orgID uuid.UUID) ([]Environment, error) {
	var list []Environment
	err := r.db.WithContext(ctx).Where("organization_id = ?", orgID).Find(&list).Error
	return list, err
}

func (r *repository) Update(ctx context.Context, e *Environment) error {
	return r.db.WithContext(ctx).Save(e).Error
}

func (r *repository) Delete(ctx context.Context, id uuid.UUID) error {
	return r.db.WithContext(ctx).Delete(&Environment{}, "id = ?", id).Error
}
