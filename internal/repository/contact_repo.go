package repository

import (
	"context"

	"blackonix/internal/domain"
	"gorm.io/gorm"
)

type contactRepo struct {
	db *gorm.DB
}

func NewContactRepository(db *gorm.DB) ContactRepository {
	return &contactRepo{db: db}
}

func (r *contactRepo) FindOrCreate(ctx context.Context, tenantID, phone, name string) (*domain.Contact, error) {
	var contact domain.Contact
	err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND phone_number = ?", tenantID, phone).
		First(&contact).Error

	if err == gorm.ErrRecordNotFound {
		contact = domain.Contact{
			TenantID:    tenantID,
			PhoneNumber: phone,
			Name:        name,
		}
		if err := r.db.WithContext(ctx).Create(&contact).Error; err != nil {
			return nil, err
		}
		return &contact, nil
	}
	if err != nil {
		return nil, err
	}
	return &contact, nil
}

func (r *contactRepo) FindByID(ctx context.Context, id string) (*domain.Contact, error) {
	var contact domain.Contact
	if err := r.db.WithContext(ctx).First(&contact, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &contact, nil
}

func (r *contactRepo) List(ctx context.Context, tenantID string, params PaginationParams) (*PaginatedResult[domain.Contact], error) {
	var contacts []domain.Contact
	var total int64

	q := r.db.WithContext(ctx).Model(&domain.Contact{})
	if tenantID != "" {
		q = q.Where("tenant_id = ?", tenantID)
	}

	q.Count(&total)

	offset := (params.Page - 1) * params.Limit
	if err := q.Order("created_at DESC").Offset(offset).Limit(params.Limit).Find(&contacts).Error; err != nil {
		return nil, err
	}

	return &PaginatedResult[domain.Contact]{
		Data: contacts, Total: total, Page: params.Page, Limit: params.Limit,
	}, nil
}
