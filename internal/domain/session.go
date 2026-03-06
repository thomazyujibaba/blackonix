package domain

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"time"
)

type SessionState string

const (
	SessionStateBot   SessionState = "BOT"
	SessionStateHuman SessionState = "HUMAN"
)

// ContextMemory armazena o histórico de conversa como JSON no banco.
type ContextMemory []ContextMessage

type ContextMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

func (cm ContextMemory) Value() (driver.Value, error) {
	return json.Marshal(cm)
}

func (cm *ContextMemory) Scan(value interface{}) error {
	bytes, ok := value.([]byte)
	if !ok {
		return fmt.Errorf("failed to scan ContextMemory: expected []byte, got %T", value)
	}
	return json.Unmarshal(bytes, cm)
}

type Session struct {
	ID               string       `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	TenantID         string       `gorm:"type:uuid;not null;index"`
	ContactID        string       `gorm:"type:uuid;not null;index"`
	State            SessionState `gorm:"type:varchar(10);not null;default:'BOT'"`
	ActiveDepartment string
	ContextMemory    ContextMemory `gorm:"type:jsonb;default:'[]'"`
	CreatedAt        time.Time
	UpdatedAt        time.Time

	Tenant  Tenant  `gorm:"foreignKey:TenantID"`
	Contact Contact `gorm:"foreignKey:ContactID"`
}
