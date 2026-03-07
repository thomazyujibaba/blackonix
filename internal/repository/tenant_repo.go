package repository

import (
	"context"

	"blackonix/internal/domain"
	"gorm.io/gorm"
)

type tenantRepo struct {
	db *gorm.DB
}

func NewTenantRepository(db *gorm.DB) TenantRepository {
	return &tenantRepo{db: db}
}

func (r *tenantRepo) FindByWabaID(ctx context.Context, wabaID string) (*domain.Tenant, error) {
	var tenant domain.Tenant
	if err := r.db.WithContext(ctx).Where("waba_id = ?", wabaID).First(&tenant).Error; err != nil {
		return nil, err
	}
	return &tenant, nil
}

func (r *tenantRepo) FindByID(ctx context.Context, id string) (*domain.Tenant, error) {
	var tenant domain.Tenant
	if err := r.db.WithContext(ctx).First(&tenant, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &tenant, nil
}

func (r *tenantRepo) List(ctx context.Context, params PaginationParams) (*PaginatedResult[domain.Tenant], error) {
	var tenants []domain.Tenant
	var total int64

	r.db.WithContext(ctx).Model(&domain.Tenant{}).Count(&total)

	offset := (params.Page - 1) * params.Limit
	if err := r.db.WithContext(ctx).Order("created_at DESC").Offset(offset).Limit(params.Limit).Find(&tenants).Error; err != nil {
		return nil, err
	}

	return &PaginatedResult[domain.Tenant]{
		Data: tenants, Total: total, Page: params.Page, Limit: params.Limit,
	}, nil
}

func (r *tenantRepo) Create(ctx context.Context, tenant *domain.Tenant) error {
	return r.db.WithContext(ctx).Create(tenant).Error
}

func (r *tenantRepo) Update(ctx context.Context, tenant *domain.Tenant) error {
	return r.db.WithContext(ctx).Save(tenant).Error
}

func (r *tenantRepo) Delete(ctx context.Context, id string) error {
	return r.db.WithContext(ctx).Delete(&domain.Tenant{}, "id = ?", id).Error
}
