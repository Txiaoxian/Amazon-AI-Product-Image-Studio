package auth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

var (
	ErrInvalidToken = errors.New("invalid auth token")
	ErrExpiredToken = errors.New("expired auth token")
)

type tokenClaims struct {
	Issuer    string `json:"iss"`
	Subject   string `json:"sub"`
	TenantID  string `json:"tenantId"`
	Email     string `json:"email,omitempty"`
	CSRFToken string `json:"csrf"`
	IssuedAt  int64  `json:"iat"`
	ExpiresAt int64  `json:"exp"`
}

func createToken(secret string, issuer string, principal Principal, ttl time.Duration, now time.Time) (string, error) {
	claims := tokenClaims{
		Issuer:    issuer,
		Subject:   principal.UserID,
		TenantID:  principal.TenantID,
		Email:     principal.Email,
		CSRFToken: principal.CSRFToken,
		IssuedAt:  now.Unix(),
		ExpiresAt: now.Add(ttl).Unix(),
	}

	headerJSON, err := json.Marshal(map[string]string{
		"alg": "HS256",
		"typ": "JWT",
	})
	if err != nil {
		return "", err
	}

	claimsJSON, err := json.Marshal(claims)
	if err != nil {
		return "", err
	}

	header := base64.RawURLEncoding.EncodeToString(headerJSON)
	payload := base64.RawURLEncoding.EncodeToString(claimsJSON)
	signingInput := header + "." + payload
	signature := sign(secret, signingInput)
	return signingInput + "." + signature, nil
}

func parseToken(secret string, issuer string, token string, now time.Time) (tokenClaims, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return tokenClaims{}, ErrInvalidToken
	}

	expectedSignature := sign(secret, parts[0]+"."+parts[1])
	if !hmac.Equal([]byte(expectedSignature), []byte(parts[2])) {
		return tokenClaims{}, ErrInvalidToken
	}

	headerJSON, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return tokenClaims{}, ErrInvalidToken
	}
	var header struct {
		Algorithm string `json:"alg"`
		Type      string `json:"typ"`
	}
	if err := json.Unmarshal(headerJSON, &header); err != nil {
		return tokenClaims{}, ErrInvalidToken
	}
	if header.Algorithm != "HS256" || header.Type != "JWT" {
		return tokenClaims{}, ErrInvalidToken
	}

	claimsJSON, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return tokenClaims{}, ErrInvalidToken
	}
	var claims tokenClaims
	if err := json.Unmarshal(claimsJSON, &claims); err != nil {
		return tokenClaims{}, ErrInvalidToken
	}
	if claims.Issuer != issuer || claims.Subject == "" || claims.TenantID == "" || claims.CSRFToken == "" {
		return tokenClaims{}, ErrInvalidToken
	}
	if now.Unix() >= claims.ExpiresAt {
		return tokenClaims{}, ErrExpiredToken
	}

	return claims, nil
}

func sign(secret string, signingInput string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(signingInput))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func newCSRFToken() (string, error) {
	var b [32]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", fmt.Errorf("create csrf token: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(b[:]), nil
}
