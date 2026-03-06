package domain

import "time"

type Contact struct {
	ID          string `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	TenantID    string `gorm:"type:uuid;not null;index"`
	PhoneNumber string `gorm:"not null;index"`
	Name        string
	CreatedAt   time.Time
	UpdatedAt   time.Time

	Tenant Tenant `gorm:"foreignKey:TenantID"`
}
