package validator

import (
	"regexp"
	"strings"
)

var (
	usernameRegex = regexp.MustCompile(`^[a-zA-Z0-9_]+$`)
	emailRegex    = regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`)
	upperRegex    = regexp.MustCompile(`[A-Z]`)
	lowerRegex    = regexp.MustCompile(`[a-z]`)
	digitRegex    = regexp.MustCompile(`[0-9]`)
)

// IsValidUsername checks 3-32 characters, alphanumeric and underscore only.
func IsValidUsername(s string) bool {
	return len(s) >= 3 && len(s) <= 32 && usernameRegex.MatchString(s)
}

// IsValidEmail checks standard email format.
func IsValidEmail(s string) bool {
	return emailRegex.MatchString(s)
}

// IsValidPassword checks 8-72 characters, must contain uppercase, lowercase, and digit.
func IsValidPassword(s string) bool {
	if len(s) < 8 || len(s) > 72 {
		return false
	}
	return upperRegex.MatchString(s) && lowerRegex.MatchString(s) && digitRegex.MatchString(s)
}

// SanitizeString trims leading/trailing whitespace.
func SanitizeString(s string) string {
	return strings.TrimSpace(s)
}

// IsBlank returns true if the string is empty or only whitespace.
func IsBlank(s string) bool {
	return strings.TrimSpace(s) == ""
}
