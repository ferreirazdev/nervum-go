package auth

import (
	"context"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// SessionRepository persists and looks up sessions by ID.
type SessionRepository interface {
	Create(ctx context.Context, s *Session) error
	GetByID(ctx context.Context, id uuid.UUID) (*Session, error)
	Delete(ctx context.Context, id uuid.UUID) error
}

type sessionRepository struct {
	db *gorm.DB
}

// NewSessionRepository returns a SessionRepository backed by the given DB.
func NewSessionRepository(db *gorm.DB) SessionRepository {
	return &sessionRepository{db: db}
}

func (r *sessionRepository) Create(ctx context.Context, s *Session) error {
	return r.db.WithContext(ctx).Create(s).Error
}

func (r *sessionRepository) GetByID(ctx context.Context, id uuid.UUID) (*Session, error) {
	var s Session
	err := r.db.WithContext(ctx).
		Where("id = ? AND expires_at > ? AND deleted_at IS NULL", id, time.Now()).
		First(&s).Error
	if err != nil {
		return nil, err
	}
	return &s, nil
}

func (r *sessionRepository) Delete(ctx context.Context, id uuid.UUID) error {
	return r.db.WithContext(ctx).Delete(&Session{}, "id = ?", id).Error
}
