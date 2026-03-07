package repository

import (
	"context"

	"blackonix/internal/domain"
)

type PaginationParams struct {
	Page  int
	Limit int
}

type PaginatedResult[T any] struct {
	Data  []T   `json:"data"`
	Total int64 `json:"total"`
	Page  int   `json:"page"`
	Limit int   `json:"limit"`
}

type TenantRepository interface {
	FindByID(ctx context.Context, id string) (*domain.Tenant, error)
	List(ctx context.Context, params PaginationParams) (*PaginatedResult[domain.Tenant], error)
	Create(ctx context.Context, tenant *domain.Tenant) error
	Update(ctx context.Context, tenant *domain.Tenant) error
	Delete(ctx context.Context, id string) error
}

type ContactRepository interface {
	FindOrCreate(ctx context.Context, tenantID, phone, name string) (*domain.Contact, error)
	FindByID(ctx context.Context, id string) (*domain.Contact, error)
	List(ctx context.Context, tenantID string, params PaginationParams) (*PaginatedResult[domain.Contact], error)
}

type SessionRepository interface {
	FindActiveByContact(ctx context.Context, tenantID, contactID string) (*domain.Session, error)
	Create(ctx context.Context, session *domain.Session) error
	Update(ctx context.Context, session *domain.Session) error
	List(ctx context.Context, tenantID string, state string, params PaginationParams) (*PaginatedResult[domain.Session], error)
	FindByID(ctx context.Context, id string) (*domain.Session, error)
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

type ChannelRepository interface {
	FindByExternalID(ctx context.Context, externalID string) (*domain.Channel, error)
	FindByTenantAndType(ctx context.Context, tenantID string, channelType domain.ChannelType) (*domain.Channel, error)
	Create(ctx context.Context, channel *domain.Channel) error
	Update(ctx context.Context, channel *domain.Channel) error
	List(ctx context.Context, tenantID string, params PaginationParams) (*PaginatedResult[domain.Channel], error)
}
