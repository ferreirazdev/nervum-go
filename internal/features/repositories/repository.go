package repositories

import (
	"context"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Repository persists organization repositories.
type Repository interface {
	ListByOrganization(ctx context.Context, orgID uuid.UUID) ([]OrganizationRepository, error)
	Create(ctx context.Context, r *OrganizationRepository) error
	GetByID(ctx context.Context, id uuid.UUID) (*OrganizationRepository, error)
	Delete(ctx context.Context, id uuid.UUID) error
}

type repository struct {
	db *gorm.DB
}

// NewRepository returns a Repository backed by the given DB.
func NewRepository(db *gorm.DB) Repository {
	return &repository{db: db}
}

func (r *repository) ListByOrganization(ctx context.Context, orgID uuid.UUID) ([]OrganizationRepository, error) {
	var list []OrganizationRepository
	err := r.db.WithContext(ctx).Where("organization_id = ?", orgID).Order("created_at ASC").Find(&list).Error
	return list, err
}

func (r *repository) Create(ctx context.Context, repo *OrganizationRepository) error {
	return r.db.WithContext(ctx).Create(repo).Error
}

func (r *repository) GetByID(ctx context.Context, id uuid.UUID) (*OrganizationRepository, error) {
	var repo OrganizationRepository
	err := r.db.WithContext(ctx).First(&repo, "id = ?", id).Error
	if err != nil {
		return nil, err
	}
	return &repo, nil
}

func (r *repository) Delete(ctx context.Context, id uuid.UUID) error {
	return r.db.WithContext(ctx).Delete(&OrganizationRepository{}, "id = ?", id).Error
}
