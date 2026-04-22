package rbac

import (
	"context"
	"fmt"

	"gorm.io/gorm"
)

type Service struct {
	db *gorm.DB
}

func NewService(db *gorm.DB) *Service {
	return &Service{db: db}
}

func (s *Service) HasPermission(ctx context.Context, roleName, resource, action string) (bool, error) {
	var count int64
	err := s.db.WithContext(ctx).
		Model(&Permission{}).
		Joins("JOIN role_permissions ON role_permissions.permission_id = permissions.id").
		Joins("JOIN roles ON roles.id = role_permissions.role_id").
		Where("roles.name = ? AND permissions.resource = ? AND permissions.action = ?", roleName, resource, action).
		Count(&count).Error
	if err != nil {
		return false, fmt.Errorf("check permission: %w", err)
	}
	return count > 0, nil
}

func (s *Service) AssignRole(ctx context.Context, roleName string) (*Role, error) {
	var role Role
	err := s.db.WithContext(ctx).Where("name = ?", roleName).First(&role).Error
	if err != nil {
		return nil, fmt.Errorf("find role: %w", err)
	}
	return &role, nil
}
