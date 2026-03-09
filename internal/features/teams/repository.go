package teams

import (
	"context"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Repository persists teams and their environment links.
type Repository interface {
	Create(ctx context.Context, t *Team, environmentIDs []uuid.UUID) error
	GetByID(ctx context.Context, id uuid.UUID) (*Team, error)
	ListByOrganization(ctx context.Context, orgID uuid.UUID) ([]Team, error)
	ListTeamsForUserMember(ctx context.Context, orgID, userID uuid.UUID) ([]Team, error)
	Update(ctx context.Context, t *Team, environmentIDs []uuid.UUID) error
	Delete(ctx context.Context, id uuid.UUID) error
}

type repository struct {
	db *gorm.DB
}

// NewRepository returns a teams Repository backed by the given DB.
func NewRepository(db *gorm.DB) Repository {
	return &repository{db: db}
}

func (r *repository) Create(ctx context.Context, t *Team, environmentIDs []uuid.UUID) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(t).Error; err != nil {
			return err
		}
		for _, envID := range environmentIDs {
			if err := tx.Create(&TeamEnvironment{TeamID: t.ID, EnvironmentID: envID}).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func (r *repository) GetByID(ctx context.Context, id uuid.UUID) (*Team, error) {
	var t Team
	err := r.db.WithContext(ctx).Preload("Environments").First(&t, "id = ?", id).Error
	if err != nil {
		return nil, err
	}
	return &t, nil
}

func (r *repository) ListByOrganization(ctx context.Context, orgID uuid.UUID) ([]Team, error) {
	var list []Team
	err := r.db.WithContext(ctx).Preload("Environments").Where("organization_id = ?", orgID).Find(&list).Error
	return list, err
}

func (r *repository) ListTeamsForUserMember(ctx context.Context, orgID, userID uuid.UUID) ([]Team, error) {
	var list []Team
	err := r.db.WithContext(ctx).Preload("Environments").
		Joins("INNER JOIN user_teams ON user_teams.team_id = teams.id AND user_teams.deleted_at IS NULL").
		Where("teams.organization_id = ? AND user_teams.user_id = ?", orgID, userID).
		Find(&list).Error
	return list, err
}

func (r *repository) Update(ctx context.Context, t *Team, environmentIDs []uuid.UUID) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Save(t).Error; err != nil {
			return err
		}
		if err := tx.Where("team_id = ?", t.ID).Delete(&TeamEnvironment{}).Error; err != nil {
			return err
		}
		for _, envID := range environmentIDs {
			if err := tx.Create(&TeamEnvironment{TeamID: t.ID, EnvironmentID: envID}).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func (r *repository) Delete(ctx context.Context, id uuid.UUID) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("team_id = ?", id).Delete(&TeamEnvironment{}).Error; err != nil {
			return err
		}
		return tx.Delete(&Team{}, "id = ?", id).Error
	})
}
