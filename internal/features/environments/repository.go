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
	ListEnvironmentsForMember(ctx context.Context, orgID, userID uuid.UUID) ([]Environment, error)
	UserCanAccessEnvironment(ctx context.Context, envID, userID uuid.UUID) (bool, error)
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

func (r *repository) ListEnvironmentsForMember(ctx context.Context, orgID, userID uuid.UUID) ([]Environment, error) {
	var list []Environment
	subqAccess := r.db.WithContext(ctx).Table("user_environment_access").Where("user_id = ? AND deleted_at IS NULL", userID).Select("environment_id")
	subqTeamIDs := r.db.WithContext(ctx).Table("user_teams").Where("user_id = ? AND deleted_at IS NULL", userID).Select("team_id")
	subqEnvFromTeams := r.db.WithContext(ctx).Table("team_environments").Where("team_id IN (?)", subqTeamIDs).Select("environment_id")
	err := r.db.WithContext(ctx).Where("organization_id = ? AND (id IN (?) OR id IN (?))", orgID, subqAccess, subqEnvFromTeams).Find(&list).Error
	return list, err
}

func (r *repository) UserCanAccessEnvironment(ctx context.Context, envID, userID uuid.UUID) (bool, error) {
	var count int64
	if err := r.db.WithContext(ctx).Table("user_environment_access").
		Where("user_id = ? AND environment_id = ? AND deleted_at IS NULL", userID, envID).
		Count(&count).Error; err != nil {
		return false, err
	}
	if count > 0 {
		return true, nil
	}
	err := r.db.WithContext(ctx).Table("team_environments").
		Joins("INNER JOIN user_teams ON user_teams.team_id = team_environments.team_id AND user_teams.deleted_at IS NULL").
		Where("team_environments.environment_id = ? AND user_teams.user_id = ?", envID, userID).
		Count(&count).Error
	return count > 0, err
}

func (r *repository) Update(ctx context.Context, e *Environment) error {
	return r.db.WithContext(ctx).Save(e).Error
}

func (r *repository) Delete(ctx context.Context, id uuid.UUID) error {
	return r.db.WithContext(ctx).Delete(&Environment{}, "id = ?", id).Error
}
