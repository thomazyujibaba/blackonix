package domain

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"time"
)

type ChannelType string

const (
	ChannelWhatsApp ChannelType = "whatsapp"
	ChannelTelegram ChannelType = "telegram"
)

// ChannelCredentials stores platform-specific credentials as JSON in the database.
type ChannelCredentials map[string]string

func (cc ChannelCredentials) Value() (driver.Value, error) {
	return json.Marshal(cc)
}

func (cc *ChannelCredentials) Scan(value interface{}) error {
	bytes, ok := value.([]byte)
	if !ok {
		return fmt.Errorf("failed to scan ChannelCredentials: expected []byte, got %T", value)
	}
	return json.Unmarshal(bytes, cc)
}

// Get returns a credential value or empty string if not found.
func (cc ChannelCredentials) Get(key string) string {
	if cc == nil {
		return ""
	}
	return cc[key]
}

type Channel struct {
	ID          string             `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	TenantID    string             `gorm:"type:uuid;not null;index"`
	Type        ChannelType        `gorm:"type:varchar(20);not null"`
	Credentials ChannelCredentials `gorm:"type:jsonb;not null;default:'{}'"`
	ExternalID  string             `gorm:"uniqueIndex;not null"` // WABA ID or Telegram Bot ID
	Active      bool               `gorm:"default:true"`
	CreatedAt   time.Time
	UpdatedAt   time.Time

	Tenant Tenant `gorm:"foreignKey:TenantID"`
}
