package audit

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Service struct {
	db *gorm.DB
}

func NewService(db *gorm.DB) *Service {
	return &Service{db: db}
}

func (s *Service) Record(ctx context.Context, entry *Log) error {
	if err := s.db.WithContext(ctx).Create(entry).Error; err != nil {
		return fmt.Errorf("create audit log: %w", err)
	}
	return nil
}

type ListFilter struct {
	UserID    *uuid.UUID
	RiskLevel string
	StartTime *time.Time
	EndTime   *time.Time
}

func (s *Service) ListWithFilter(ctx context.Context, filter ListFilter, offset, limit int) ([]Log, int64, error) {
	query := s.db.WithContext(ctx).Model(&Log{})

	if filter.UserID != nil {
		query = query.Where("user_id = ?", *filter.UserID)
	}
	if filter.RiskLevel != "" {
		query = query.Where("risk_level = ?", filter.RiskLevel)
	}
	if filter.StartTime != nil {
		query = query.Where("created_at >= ?", *filter.StartTime)
	}
	if filter.EndTime != nil {
		query = query.Where("created_at <= ?", *filter.EndTime)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var logs []Log
	err := query.Order("created_at DESC").Offset(offset).Limit(limit).Find(&logs).Error
	return logs, total, err
}
