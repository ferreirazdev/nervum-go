package relationship

import (
	"context"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Repository persists relationships and lists by organization.
type Repository interface {
	Create(ctx context.Context, r *Relationship) error
	GetByID(ctx context.Context, id uuid.UUID) (*Relationship, error)
	ListByOrganization(ctx context.Context, orgID uuid.UUID) ([]Relationship, error)
	Update(ctx context.Context, r *Relationship) error
	Delete(ctx context.Context, id uuid.UUID) error
}

type repository struct {
	db *gorm.DB
}

// NewRepository returns a relationship Repository backed by the given DB.
func NewRepository(db *gorm.DB) Repository {
	return &repository{db: db}
}

func (r *repository) Create(ctx context.Context, rel *Relationship) error {
	return r.db.WithContext(ctx).Create(rel).Error
}

func (r *repository) GetByID(ctx context.Context, id uuid.UUID) (*Relationship, error) {
	var rel Relationship
	err := r.db.WithContext(ctx).First(&rel, "id = ?", id).Error
	if err != nil {
		return nil, err
	}
	return &rel, nil
}

func (r *repository) ListByOrganization(ctx context.Context, orgID uuid.UUID) ([]Relationship, error) {
	var list []Relationship
	err := r.db.WithContext(ctx).Where("organization_id = ?", orgID).Find(&list).Error
	return list, err
}

func (r *repository) Update(ctx context.Context, rel *Relationship) error {
	return r.db.WithContext(ctx).Save(rel).Error
}

func (r *repository) Delete(ctx context.Context, id uuid.UUID) error {
	return r.db.WithContext(ctx).Delete(&Relationship{}, "id = ?", id).Error
}
