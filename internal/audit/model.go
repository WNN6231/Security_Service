package audit

import (
	"time"

	"github.com/google/uuid"
)

type Log struct {
	ID        uuid.UUID  `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	UserID    *uuid.UUID `json:"user_id" gorm:"type:uuid;index"`
	Action    string     `json:"action" gorm:"size:200;not null"`
	IP        string     `json:"ip" gorm:"size:45"`
	UserAgent string     `json:"user_agent" gorm:"size:255"`
	Status    int        `json:"status"`
	RiskLevel string     `json:"risk_level" gorm:"size:10;index"`
	CreatedAt time.Time  `json:"created_at" gorm:"index"`
}

func (Log) TableName() string {
	return "audit_logs"
}
