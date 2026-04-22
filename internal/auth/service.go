package auth

import (
	"context"
	"errors"
	"strings"
	"time"

	"security-service/internal/security"
	"security-service/internal/store"
	"security-service/internal/user"
)

type Service struct {
	userRepo   user.Repository
	jwtManager *security.JWTManager
	blacklist  *store.TokenBlacklist
}

func NewService(userRepo user.Repository, jwtManager *security.JWTManager, blacklist *store.TokenBlacklist) *Service {
	return &Service{
		userRepo:   userRepo,
		jwtManager: jwtManager,
		blacklist:  blacklist,
	}
}

func (s *Service) Register(ctx context.Context, req *RegisterRequest) (*user.User, error) {
	if existing, _ := s.userRepo.FindByEmail(ctx, req.Email); existing != nil {
		return nil, errors.New("email already registered")
	}
	if existing, _ := s.userRepo.FindByUsername(ctx, req.Username); existing != nil {
		return nil, errors.New("username already taken")
	}

	hashedPassword, err := security.HashPassword(req.Password)
	if err != nil {
		return nil, err
	}

	u := &user.User{
		Username: req.Username,
		Email:    req.Email,
		Password: hashedPassword,
	}

	if err := s.userRepo.Create(ctx, u); err != nil {
		return nil, err
	}

	u.Password = ""
	return u, nil
}

func (s *Service) Login(ctx context.Context, req *LoginRequest) (*TokenResponse, error) {
	u, err := s.userRepo.FindByEmail(ctx, req.Email)
	if err != nil {
		return nil, errors.New("invalid credentials")
	}

	if !security.CheckPassword(req.Password, u.Password) {
		return nil, errors.New("invalid credentials")
	}

	accessToken, _, err := s.jwtManager.GenerateAccessToken(u.ID, u.Email, u.Role)
	if err != nil {
		return nil, err
	}

	refreshToken, _, err := s.jwtManager.GenerateRefreshToken(u.ID)
	if err != nil {
		return nil, err
	}

	return &TokenResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		TokenType:    "Bearer",
	}, nil
}

func (s *Service) RefreshToken(ctx context.Context, refreshToken string) (*TokenResponse, error) {
	claims, err := s.jwtManager.ValidateToken(refreshToken)
	if err != nil {
		return nil, errors.New("invalid refresh token")
	}

	// Blacklist the old refresh token's jti
	if claims.ID != "" {
		ttl := time.Until(claims.ExpiresAt.Time)
		if ttl > 0 {
			_ = s.blacklist.Add(ctx, claims.ID, ttl)
		}
	}

	u, err := s.userRepo.FindByID(ctx, claims.UserID)
	if err != nil {
		return nil, errors.New("user not found")
	}

	newAccess, _, err := s.jwtManager.GenerateAccessToken(u.ID, u.Email, u.Role)
	if err != nil {
		return nil, err
	}

	newRefresh, _, err := s.jwtManager.GenerateRefreshToken(u.ID)
	if err != nil {
		return nil, err
	}

	return &TokenResponse{
		AccessToken:  newAccess,
		RefreshToken: newRefresh,
		TokenType:    "Bearer",
	}, nil
}

func (s *Service) Logout(ctx context.Context, authHeader string) error {
	tokenStr := strings.TrimPrefix(authHeader, "Bearer ")
	if tokenStr == "" || tokenStr == authHeader {
		return errors.New("missing bearer token")
	}

	claims, err := s.jwtManager.ValidateToken(tokenStr)
	if err != nil {
		return errors.New("invalid token")
	}

	if claims.ID == "" {
		return errors.New("token missing jti")
	}

	ttl := time.Until(claims.ExpiresAt.Time)
	if ttl <= 0 {
		return nil // already expired, no need to blacklist
	}

	return s.blacklist.Add(ctx, claims.ID, ttl)
}
