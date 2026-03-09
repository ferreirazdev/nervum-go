package invitation

import (
	"context"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Repository persists invitations and looks them up by ID or token.
type Repository interface {
	Create(ctx context.Context, inv *Invitation, teamIDs []uuid.UUID) error
	GetByID(ctx context.Context, id uuid.UUID) (*Invitation, error)
	GetByToken(ctx context.Context, token string) (*Invitation, error)
	ListByOrganization(ctx context.Context, orgID uuid.UUID, status string) ([]Invitation, error)
	Update(ctx context.Context, inv *Invitation) error
	Delete(ctx context.Context, id uuid.UUID) error
}

type repository struct {
	db *gorm.DB
}

// NewRepository returns an invitation Repository backed by the given DB.
func NewRepository(db *gorm.DB) Repository {
	return &repository{db: db}
}

func (r *repository) Create(ctx context.Context, inv *Invitation, teamIDs []uuid.UUID) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(inv).Error; err != nil {
			return err
		}
		for _, teamID := range teamIDs {
			if err := tx.Create(&InvitationTeam{InvitationID: inv.ID, TeamID: teamID}).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func (r *repository) GetByID(ctx context.Context, id uuid.UUID) (*Invitation, error) {
	var inv Invitation
	err := r.db.WithContext(ctx).Preload("Teams").First(&inv, "id = ?", id).Error
	if err != nil {
		return nil, err
	}
	return &inv, nil
}

func (r *repository) GetByToken(ctx context.Context, token string) (*Invitation, error) {
	var inv Invitation
	err := r.db.WithContext(ctx).Preload("Teams").Where("token = ?", token).First(&inv).Error
	if err != nil {
		return nil, err
	}
	return &inv, nil
}

func (r *repository) ListByOrganization(ctx context.Context, orgID uuid.UUID, status string) ([]Invitation, error) {
	var list []Invitation
	q := r.db.WithContext(ctx).Preload("Teams").Where("organization_id = ?", orgID)
	if status != "" {
		q = q.Where("status = ?", status)
	}
	err := q.Find(&list).Error
	return list, err
}

func (r *repository) Update(ctx context.Context, inv *Invitation) error {
	return r.db.WithContext(ctx).Save(inv).Error
}

func (r *repository) Delete(ctx context.Context, id uuid.UUID) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("invitation_id = ?", id).Delete(&InvitationTeam{}).Error; err != nil {
			return err
		}
		return tx.Delete(&Invitation{}, "id = ?", id).Error
	})
}
