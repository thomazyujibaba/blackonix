package domain

import "time"

type MessageDirection string

const (
	MessageDirectionInbound  MessageDirection = "INBOUND"
	MessageDirectionOutbound MessageDirection = "OUTBOUND"
)

type Message struct {
	ID        string           `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	TenantID  string           `gorm:"type:uuid;not null;index"`
	SessionID string           `gorm:"type:uuid;not null;index"`
	ContactID string           `gorm:"type:uuid;not null;index"`
	Direction MessageDirection `gorm:"type:varchar(10);not null"`
	Body      string           `gorm:"type:text;not null"`
	MediaURL  string
	CreatedAt time.Time

	Tenant  Tenant  `gorm:"foreignKey:TenantID"`
	Session Session `gorm:"foreignKey:SessionID"`
	Contact Contact `gorm:"foreignKey:ContactID"`
}
