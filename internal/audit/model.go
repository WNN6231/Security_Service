package audit

import (
	"time"

	"github.com/google/uuid"
)

type Log struct {
	ID        uuid.UUID `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	UserID    uuid.UUID `json:"user_id" gorm:"type:uuid;index"`
	Action    string    `json:"action" gorm:"size:100;not null"`
	Resource  string    `json:"resource" gorm:"size:100"`
	Detail    string    `json:"detail" gorm:"type:text"`
	IP        string    `json:"ip" gorm:"size:45"`
	UserAgent string    `json:"user_agent" gorm:"size:255"`
	CreatedAt time.Time `json:"created_at" gorm:"index"`
}

func (Log) TableName() string {
	return "audit_logs"
}
