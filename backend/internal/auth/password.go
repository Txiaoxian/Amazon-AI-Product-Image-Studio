package auth

import (
	"errors"
	"strings"

	"golang.org/x/crypto/bcrypt"
)

var (
	ErrWeakPassword   = errors.New("password must be 12 to 128 characters")
	dummyPasswordHash []byte
)

func init() {
	hash, err := bcrypt.GenerateFromPassword([]byte("not-the-request-password"), bcrypt.DefaultCost)
	if err == nil {
		dummyPasswordHash = hash
	}
}

func HashPassword(password string) (string, error) {
	if err := ValidatePassword(password); err != nil {
		return "", err
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

func CheckPassword(hash string, password string) bool {
	if strings.TrimSpace(hash) == "" && len(dummyPasswordHash) > 0 {
		return bcrypt.CompareHashAndPassword(dummyPasswordHash, []byte(password)) == nil
	}

	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}

func ValidatePassword(password string) error {
	if len(password) < 12 || len(password) > 128 {
		return ErrWeakPassword
	}
	return nil
}
