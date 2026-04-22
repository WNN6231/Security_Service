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
	ID   uuid.UUID `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	Code string    `json:"code" gorm:"uniqueIndex;size:100;not null"`
}

type UserRole struct {
	UserID    uuid.UUID `json:"user_id" gorm:"type:uuid;primaryKey"`
	RoleID    uuid.UUID `json:"role_id" gorm:"type:uuid;primaryKey"`
	CreatedAt time.Time `json:"created_at"`
}

func (Role) TableName() string       { return "roles" }
func (Permission) TableName() string { return "permissions" }
func (UserRole) TableName() string   { return "user_roles" }
