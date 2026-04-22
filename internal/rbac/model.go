package rbac

import (
	"time"

	"github.com/google/uuid"
)

type Role struct {
	ID          uuid.UUID    `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	Name        string       `json:"name" gorm:"uniqueIndex;size:50;not null"`
	Description string       `json:"description" gorm:"size:255"`
	Permissions []Permission `json:"permissions" gorm:"many2many:role_permissions;"`
	CreatedAt   time.Time    `json:"created_at"`
	UpdatedAt   time.Time    `json:"updated_at"`
}

type Permission struct {
	ID       uuid.UUID `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	Resource string    `json:"resource" gorm:"size:100;not null"`
	Action   string    `json:"action" gorm:"size:50;not null"`
}

func (Permission) TableName() string {
	return "permissions"
}

func (Role) TableName() string {
	return "roles"
}
