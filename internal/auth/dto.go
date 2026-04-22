package auth

import (
	"errors"

	"security-service/internal/validator"
)

type RegisterRequest struct {
	Username string `json:"username" binding:"required"`
	Email    string `json:"email" binding:"required"`
	Password string `json:"password" binding:"required"`
}

func (r *RegisterRequest) Sanitize() {
	r.Username = validator.SanitizeString(r.Username)
	r.Email = validator.SanitizeString(r.Email)
	r.Password = validator.SanitizeString(r.Password)
}

func (r *RegisterRequest) Validate() error {
	if !validator.IsValidUsername(r.Username) {
		return errors.New("username must be 3-32 characters, only letters, digits and underscore")
	}
	if !validator.IsValidEmail(r.Email) {
		return errors.New("invalid email format")
	}
	if !validator.IsValidPassword(r.Password) {
		return errors.New("password must be 8-72 characters with at least one uppercase, one lowercase and one digit")
	}
	return nil
}

type LoginRequest struct {
	Email    string `json:"email" binding:"required"`
	Password string `json:"password" binding:"required"`
}

func (r *LoginRequest) Sanitize() {
	r.Email = validator.SanitizeString(r.Email)
}

func (r *LoginRequest) Validate() error {
	if !validator.IsValidEmail(r.Email) {
		return errors.New("invalid email format")
	}
	if validator.IsBlank(r.Password) {
		return errors.New("password is required")
	}
	return nil
}

type RefreshRequest struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
}

type TokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	TokenType    string `json:"token_type"`
}
