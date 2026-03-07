package repository

import (
	"context"

	"blackonix/internal/domain"

	"gorm.io/gorm"
)

type channelRepo struct {
	db *gorm.DB
}

func NewChannelRepository(db *gorm.DB) ChannelRepository {
	return &channelRepo{db: db}
}

func (r *channelRepo) FindByExternalID(ctx context.Context, externalID string) (*domain.Channel, error) {
	var channel domain.Channel
	if err := r.db.WithContext(ctx).Preload("Tenant").Where("external_id = ? AND active = true", externalID).First(&channel).Error; err != nil {
		return nil, err
	}
	return &channel, nil
}

func (r *channelRepo) FindByTenantAndType(ctx context.Context, tenantID string, channelType domain.ChannelType) (*domain.Channel, error) {
	var channel domain.Channel
	if err := r.db.WithContext(ctx).Where("tenant_id = ? AND type = ? AND active = true", tenantID, channelType).First(&channel).Error; err != nil {
		return nil, err
	}
	return &channel, nil
}

func (r *channelRepo) Create(ctx context.Context, channel *domain.Channel) error {
	return r.db.WithContext(ctx).Create(channel).Error
}

func (r *channelRepo) Update(ctx context.Context, channel *domain.Channel) error {
	return r.db.WithContext(ctx).Save(channel).Error
}

func (r *channelRepo) List(ctx context.Context, tenantID string, params PaginationParams) (*PaginatedResult[domain.Channel], error) {
	var channels []domain.Channel
	var total int64

	q := r.db.WithContext(ctx).Model(&domain.Channel{})
	if tenantID != "" {
		q = q.Where("tenant_id = ?", tenantID)
	}

	q.Count(&total)

	offset := (params.Page - 1) * params.Limit
	if err := q.Preload("Tenant").Order("created_at DESC").Offset(offset).Limit(params.Limit).Find(&channels).Error; err != nil {
		return nil, err
	}

	return &PaginatedResult[domain.Channel]{
		Data: channels, Total: total, Page: params.Page, Limit: params.Limit,
	}, nil
}
