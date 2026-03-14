package orgservices

import (
	"context"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Repository persists organization cloud services.
type Repository interface {
	ListByOrganization(ctx context.Context, orgID uuid.UUID) ([]OrganizationService, error)
	ListByOrganizationAndKind(ctx context.Context, orgID uuid.UUID, kind string) ([]OrganizationService, error)
	Create(ctx context.Context, s *OrganizationService) error
	GetByID(ctx context.Context, id uuid.UUID) (*OrganizationService, error)
	Delete(ctx context.Context, id uuid.UUID) error
}

type repository struct {
	db *gorm.DB
}

// NewRepository returns a Repository backed by the given DB.
func NewRepository(db *gorm.DB) Repository {
	return &repository{db: db}
}

func (r *repository) ListByOrganization(ctx context.Context, orgID uuid.UUID) ([]OrganizationService, error) {
	return r.ListByOrganizationAndKind(ctx, orgID, "")
}

func (r *repository) ListByOrganizationAndKind(ctx context.Context, orgID uuid.UUID, kind string) ([]OrganizationService, error) {
	var list []OrganizationService
	q := r.db.WithContext(ctx).Where("organization_id = ?", orgID)
	if kind != "" {
		if kind == "cloud_run" {
			q = q.Where("kind = ? OR kind = ?", "cloud_run", "")
		} else {
			q = q.Where("kind = ?", kind)
		}
	}
	err := q.Order("created_at ASC").Find(&list).Error
	return list, err
}

func (r *repository) Create(ctx context.Context, s *OrganizationService) error {
	return r.db.WithContext(ctx).Create(s).Error
}

func (r *repository) GetByID(ctx context.Context, id uuid.UUID) (*OrganizationService, error) {
	var svc OrganizationService
	err := r.db.WithContext(ctx).First(&svc, "id = ?", id).Error
	if err != nil {
		return nil, err
	}
	return &svc, nil
}

func (r *repository) Delete(ctx context.Context, id uuid.UUID) error {
	return r.db.WithContext(ctx).Delete(&OrganizationService{}, "id = ?", id).Error
}
