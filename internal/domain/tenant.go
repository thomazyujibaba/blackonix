package domain

import "time"

type Tenant struct {
	ID              string `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	Name            string `gorm:"not null"`
	RocketChatURL   string `gorm:"column:rocketchat_url"`
	RocketChatToken string `gorm:"column:rocketchat_token"`
	CreatedAt       time.Time
	UpdatedAt       time.Time
}
