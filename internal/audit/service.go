package audit

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Service struct {
	db *gorm.DB
}

func NewService(db *gorm.DB) *Service {
	return &Service{db: db}
}

func (s *Service) Record(ctx context.Context, userID uuid.UUID, action, resource, detail, ip, userAgent string) error {
	log := &Log{
		UserID:    userID,
		Action:    action,
		Resource:  resource,
		Detail:    detail,
		IP:        ip,
		UserAgent: userAgent,
	}
	if err := s.db.WithContext(ctx).Create(log).Error; err != nil {
		return fmt.Errorf("create audit log: %w", err)
	}
	return nil
}

func (s *Service) List(ctx context.Context, offset, limit int) ([]Log, int64, error) {
	var logs []Log
	var total int64

	if err := s.db.WithContext(ctx).Model(&Log{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	err := s.db.WithContext(ctx).Order("created_at DESC").Offset(offset).Limit(limit).Find(&logs).Error
	return logs, total, err
}
