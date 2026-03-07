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

func (r *sessionRepo) List(ctx context.Context, tenantID string, state string, params PaginationParams) (*PaginatedResult[domain.Session], error) {
	var sessions []domain.Session
	var total int64

	q := r.db.WithContext(ctx).Model(&domain.Session{})
	if tenantID != "" {
		q = q.Where("tenant_id = ?", tenantID)
	}
	if state != "" {
		q = q.Where("state = ?", state)
	}

	q.Count(&total)

	offset := (params.Page - 1) * params.Limit
	if err := q.Preload("Contact").Order("updated_at DESC").Offset(offset).Limit(params.Limit).Find(&sessions).Error; err != nil {
		return nil, err
	}

	return &PaginatedResult[domain.Session]{
		Data: sessions, Total: total, Page: params.Page, Limit: params.Limit,
	}, nil
}

func (r *sessionRepo) FindByID(ctx context.Context, id string) (*domain.Session, error) {
	var session domain.Session
	if err := r.db.WithContext(ctx).Preload("Contact").Preload("Tenant").First(&session, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &session, nil
}
