package repository

import (
	"context"

	"blackonix/internal/domain"
	"gorm.io/gorm"
)

type sessionRepo struct {
	db *gorm.DB
}

func NewSessionRepository(db *gorm.DB) SessionRepository {
	return &sessionRepo{db: db}
}

func (r *sessionRepo) FindActiveByContact(ctx context.Context, tenantID, contactID string) (*domain.Session, error) {
	var session domain.Session
	err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND contact_id = ?", tenantID, contactID).
		Order("created_at DESC").
		First(&session).Error

	if err != nil {
		return nil, err
	}
	return &session, nil
}

func (r *sessionRepo) Create(ctx context.Context, session *domain.Session) error {
	return r.db.WithContext(ctx).Create(session).Error
}

func (r *sessionRepo) Update(ctx context.Context, session *domain.Session) error {
	return r.db.WithContext(ctx).Save(session).Error
}
