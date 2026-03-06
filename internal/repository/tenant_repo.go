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
