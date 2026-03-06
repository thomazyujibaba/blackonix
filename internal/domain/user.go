package domain

import "time"

type UserRole string

const (
	UserRoleAdmin  UserRole = "ADMIN"
	UserRoleTenant UserRole = "TENANT"
)

type User struct {
	ID           string   `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	Email        string   `gorm:"uniqueIndex;not null" json:"email"`
	PasswordHash string   `gorm:"not null" json:"-"`
	Role         UserRole `gorm:"type:varchar(10);not null;default:'TENANT'" json:"role"`
	TenantID     *string  `gorm:"type:uuid" json:"tenant_id,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`

	Tenant *Tenant `gorm:"foreignKey:TenantID" json:"tenant,omitempty"`
}
