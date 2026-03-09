package userenvironmentaccess

import (
	"context"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Repository persists user-environment access and lists by user or environment.
type Repository interface {
	Create(ctx context.Context, u *UserEnvironmentAccess) error
	GetByID(ctx context.Context, id uuid.UUID) (*UserEnvironmentAccess, error)
	GetByUserAndEnvironment(ctx context.Context, userID, envID uuid.UUID) (*UserEnvironmentAccess, error)
	ListByUser(ctx context.Context, userID uuid.UUID) ([]UserEnvironmentAccess, error)
	ListByEnvironment(ctx context.Context, envID uuid.UUID) ([]UserEnvironmentAccess, error)
	Update(ctx context.Context, u *UserEnvironmentAccess) error
	Delete(ctx context.Context, id uuid.UUID) error
}

type repository struct {
	db *gorm.DB
}

// NewRepository returns a user_environment_access Repository backed by the given DB.
func NewRepository(db *gorm.DB) Repository {
	return &repository{db: db}
}

func (r *repository) Create(ctx context.Context, u *UserEnvironmentAccess) error {
	return r.db.WithContext(ctx).Create(u).Error
}

func (r *repository) GetByID(ctx context.Context, id uuid.UUID) (*UserEnvironmentAccess, error) {
	var u UserEnvironmentAccess
	err := r.db.WithContext(ctx).First(&u, "id = ?", id).Error
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func (r *repository) GetByUserAndEnvironment(ctx context.Context, userID, envID uuid.UUID) (*UserEnvironmentAccess, error) {
	var u UserEnvironmentAccess
	err := r.db.WithContext(ctx).
		Where("user_id = ? AND environment_id = ?", userID, envID).
		First(&u).Error
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func (r *repository) ListByUser(ctx context.Context, userID uuid.UUID) ([]UserEnvironmentAccess, error) {
	var list []UserEnvironmentAccess
	err := r.db.WithContext(ctx).Where("user_id = ?", userID).Find(&list).Error
	return list, err
}

func (r *repository) ListByEnvironment(ctx context.Context, envID uuid.UUID) ([]UserEnvironmentAccess, error) {
	var list []UserEnvironmentAccess
	err := r.db.WithContext(ctx).Where("environment_id = ?", envID).Find(&list).Error
	return list, err
}

func (r *repository) Update(ctx context.Context, u *UserEnvironmentAccess) error {
	return r.db.WithContext(ctx).Save(u).Error
}

func (r *repository) Delete(ctx context.Context, id uuid.UUID) error {
	return r.db.WithContext(ctx).Delete(&UserEnvironmentAccess{}, "id = ?", id).Error
}
