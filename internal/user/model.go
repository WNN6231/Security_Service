package user

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type User struct {
	ID        uuid.UUID      `json:"id" gorm:"type:uuid;primaryKey"`
	Username  string         `json:"username" gorm:"uniqueIndex;size:50;not null"`
	Email     string         `json:"email" gorm:"uniqueIndex;size:255;not null"`
	Password  string         `json:"-" gorm:"not null"`
	Role      string         `json:"role" gorm:"size:20;default:'user'"`
	IsActive  bool           `json:"is_active" gorm:"default:true"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `json:"-" gorm:"index"`
}

func (User) TableName() string { return "users" }

func (u *User) BeforeCreate(tx *gorm.DB) error {
	if u.ID == uuid.Nil {
		u.ID = uuid.New()
	}
	return nil
}
