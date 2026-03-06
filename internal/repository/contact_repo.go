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
