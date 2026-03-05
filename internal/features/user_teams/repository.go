package userteam

import (
	"context"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Repository interface {
	Create(ctx context.Context, ut *UserTeam) error
	GetByUserAndTeam(ctx context.Context, userID, teamID uuid.UUID) (*UserTeam, error)
	ListByUser(ctx context.Context, userID uuid.UUID) ([]UserTeam, error)
	ListByTeam(ctx context.Context, teamID uuid.UUID) ([]UserTeam, error)
	Delete(ctx context.Context, userID, teamID uuid.UUID) error
	DeleteByTeam(ctx context.Context, teamID uuid.UUID) error
}

type repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) Repository {
	return &repository{db: db}
}

func (r *repository) Create(ctx context.Context, ut *UserTeam) error {
	return r.db.WithContext(ctx).Create(ut).Error
}

func (r *repository) GetByUserAndTeam(ctx context.Context, userID, teamID uuid.UUID) (*UserTeam, error) {
	var ut UserTeam
	err := r.db.WithContext(ctx).
		Where("user_id = ? AND team_id = ?", userID, teamID).
		First(&ut).Error
	if err != nil {
		return nil, err
	}
	return &ut, nil
}

func (r *repository) ListByUser(ctx context.Context, userID uuid.UUID) ([]UserTeam, error) {
	var list []UserTeam
	err := r.db.WithContext(ctx).Where("user_id = ?", userID).Find(&list).Error
	return list, err
}

func (r *repository) ListByTeam(ctx context.Context, teamID uuid.UUID) ([]UserTeam, error) {
	var list []UserTeam
	err := r.db.WithContext(ctx).Where("team_id = ?", teamID).Find(&list).Error
	return list, err
}

func (r *repository) Delete(ctx context.Context, userID, teamID uuid.UUID) error {
	return r.db.WithContext(ctx).
		Where("user_id = ? AND team_id = ?", userID, teamID).
		Delete(&UserTeam{}).Error
}

func (r *repository) DeleteByTeam(ctx context.Context, teamID uuid.UUID) error {
	return r.db.WithContext(ctx).Where("team_id = ?", teamID).Delete(&UserTeam{}).Error
}
