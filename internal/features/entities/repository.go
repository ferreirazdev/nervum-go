package entity

import (
	"context"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Repository interface {
	Create(ctx context.Context, e *Entity) error
	GetByID(ctx context.Context, id uuid.UUID) (*Entity, error)
	ListByOrganization(ctx context.Context, orgID uuid.UUID, envID *uuid.UUID) ([]Entity, error)
	Update(ctx context.Context, e *Entity) error
	Delete(ctx context.Context, id uuid.UUID) error
}

type repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) Repository {
	return &repository{db: db}
}

func (r *repository) Create(ctx context.Context, e *Entity) error {
	return r.db.WithContext(ctx).Create(e).Error
}

func (r *repository) GetByID(ctx context.Context, id uuid.UUID) (*Entity, error) {
	var e Entity
	err := r.db.WithContext(ctx).First(&e, "id = ?", id).Error
	if err != nil {
		return nil, err
	}
	return &e, nil
}

func (r *repository) ListByOrganization(ctx context.Context, orgID uuid.UUID, envID *uuid.UUID) ([]Entity, error) {
	var list []Entity
	q := r.db.WithContext(ctx).Where("organization_id = ?", orgID)
	if envID != nil {
		q = q.Where("environment_id = ?", *envID)
	}
	err := q.Find(&list).Error
	return list, err
}

func (r *repository) Update(ctx context.Context, e *Entity) error {
	return r.db.WithContext(ctx).Save(e).Error
}

func (r *repository) Delete(ctx context.Context, id uuid.UUID) error {
	return r.db.WithContext(ctx).Delete(&Entity{}, "id = ?", id).Error
}
