package rbac

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

const (
	permCachePrefix = "rbac:perms:user:"
	permCacheTTL    = 5 * time.Minute
)

type Service struct {
	db  *gorm.DB
	rdb *redis.Client
}

func NewService(db *gorm.DB, rdb *redis.Client) *Service {
	return &Service{db: db, rdb: rdb}
}

// --------------- CheckPermission (with Redis cache) ---------------

func (s *Service) CheckPermission(ctx context.Context, userID uuid.UUID, permCode string) (bool, error) {
	cacheKey := permCachePrefix + userID.String()

	// Try cache first
	if cached, err := s.rdb.Get(ctx, cacheKey).Result(); err == nil {
		if cached == "" {
			return false, nil
		}
		for _, p := range strings.Split(cached, ",") {
			if p == permCode {
				return true, nil
			}
		}
		return false, nil
	}

	// Cache miss — query DB
	perms, err := s.loadUserPermissions(ctx, userID)
	if err != nil {
		return false, err
	}

	// Populate cache (best-effort)
	_ = s.rdb.Set(ctx, cacheKey, strings.Join(perms, ","), permCacheTTL).Err()

	for _, p := range perms {
		if p == permCode {
			return true, nil
		}
	}
	return false, nil
}

// loadUserPermissions: user_roles → role_permissions → permissions
func (s *Service) loadUserPermissions(ctx context.Context, userID uuid.UUID) ([]string, error) {
	var perms []Permission
	err := s.db.WithContext(ctx).
		Distinct("permissions.*").
		Model(&Permission{}).
		Joins("JOIN role_permissions ON role_permissions.permission_id = permissions.id").
		Joins("JOIN user_roles ON user_roles.role_id = role_permissions.role_id").
		Where("user_roles.user_id = ?", userID).
		Find(&perms).Error
	if err != nil {
		return nil, fmt.Errorf("load user permissions: %w", err)
	}

	codes := make([]string, len(perms))
	for i, p := range perms {
		codes[i] = p.Code
	}
	return codes, nil
}

func (s *Service) InvalidatePermCache(ctx context.Context, userID uuid.UUID) {
	_ = s.rdb.Del(ctx, permCachePrefix+userID.String()).Err()
}

// --------------- CRUD ---------------

func (s *Service) CreateRole(ctx context.Context, name, description string, permCodes []string) (*Role, error) {
	var perms []Permission
	if len(permCodes) > 0 {
		if err := s.db.WithContext(ctx).Where("code IN ?", permCodes).Find(&perms).Error; err != nil {
			return nil, fmt.Errorf("find permissions: %w", err)
		}
	}

	role := Role{
		Name:        name,
		Description: description,
		Permissions: perms,
	}
	if err := s.db.WithContext(ctx).Create(&role).Error; err != nil {
		return nil, fmt.Errorf("create role: %w", err)
	}
	return &role, nil
}

func (s *Service) ListRoles(ctx context.Context) ([]Role, error) {
	var roles []Role
	err := s.db.WithContext(ctx).Preload("Permissions").Find(&roles).Error
	return roles, err
}

func (s *Service) AssignRoleToUser(ctx context.Context, userID, roleID uuid.UUID) error {
	var role Role
	if err := s.db.WithContext(ctx).First(&role, "id = ?", roleID).Error; err != nil {
		return fmt.Errorf("role not found")
	}

	ur := UserRole{UserID: userID, RoleID: roleID}
	if err := s.db.WithContext(ctx).Where(ur).FirstOrCreate(&ur).Error; err != nil {
		return fmt.Errorf("assign role: %w", err)
	}

	s.InvalidatePermCache(ctx, userID)
	return nil
}

// --------------- Seed ---------------

func Seed(db *gorm.DB) {
	permCodes := []string{
		"user:create", "user:update", "user:read", "user:delete",
		"role:manage", "permission:manage", "log:read", "policy:manage",
	}

	perms := make(map[string]Permission, len(permCodes))
	for _, code := range permCodes {
		p := Permission{Code: code}
		db.Where("code = ?", code).FirstOrCreate(&p)
		perms[code] = p
	}

	seedRole := func(name, desc string, codes []string) {
		var role Role
		db.Where("name = ?", name).FirstOrCreate(&role, Role{Name: name, Description: desc})
		if len(codes) > 0 {
			var rp []Permission
			for _, c := range codes {
				rp = append(rp, perms[c])
			}
			if err := db.Model(&role).Association("Permissions").Replace(rp); err != nil {
				log.Printf("seed role %s permissions: %v", name, err)
			}
		}
	}

	seedRole("admin", "Full access", permCodes)
	seedRole("auditor", "Audit read-only", []string{"log:read"})
	seedRole("developer", "Developer access", []string{"user:read"})
	seedRole("guest", "No permissions", nil)

	log.Println("RBAC seed data initialized")
}
