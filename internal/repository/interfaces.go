package repository

import (
	"context"

	"blackonix/internal/domain"
)

type TenantRepository interface {
	FindByWabaID(ctx context.Context, wabaID string) (*domain.Tenant, error)
	FindByID(ctx context.Context, id string) (*domain.Tenant, error)
}

type ContactRepository interface {
	FindOrCreate(ctx context.Context, tenantID, phone, name string) (*domain.Contact, error)
	FindByID(ctx context.Context, id string) (*domain.Contact, error)
}

type SessionRepository interface {
	FindActiveByContact(ctx context.Context, tenantID, contactID string) (*domain.Session, error)
	Create(ctx context.Context, session *domain.Session) error
	Update(ctx context.Context, session *domain.Session) error
}

type UserRepository interface {
	FindByEmail(ctx context.Context, email string) (*domain.User, error)
	FindByID(ctx context.Context, id string) (*domain.User, error)
	Create(ctx context.Context, user *domain.User) error
}

type MessageRepository interface {
	Create(ctx context.Context, msg *domain.Message) error
	FindBySession(ctx context.Context, sessionID string, limit int) ([]domain.Message, error)
}
