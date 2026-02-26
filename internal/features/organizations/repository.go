package organization

import (
	"context"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Repository interface {
	Create(ctx context.Context, o *Organization) error
	GetByID(ctx context.Context, id uuid.UUID) (*Organization, error)
	List(ctx context.Context) ([]Organization, error)
	Update(ctx context.Context, o *Organization) error
	Delete(ctx context.Context, id uuid.UUID) error
}

type repository struct {
	db *gorm.DB
}

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

func (r *repository) List(ctx context.Context) ([]Organization, error) {
	var list []Organization
	err := r.db.WithContext(ctx).Find(&list).Error
	return list, err
}

func (r *repository) Update(ctx context.Context, o *Organization) error {
	return r.db.WithContext(ctx).Save(o).Error
}

func (r *repository) Delete(ctx context.Context, id uuid.UUID) error {
	return r.db.WithContext(ctx).Delete(&Organization{}, "id = ?", id).Error
}
