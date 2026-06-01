package provider

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"io"
	"strings"
)

const encryptedPayloadVersion = "v1"

type APIKeyCipher struct {
	aead  cipher.AEAD
	keyID string
}

func NewAPIKeyCipher(secret string, keyID string) (APIKeyCipher, error) {
	key, err := deriveEncryptionKey(secret)
	if err != nil {
		return APIKeyCipher{}, err
	}

	keyID = strings.TrimSpace(keyID)
	if keyID == "" || strings.Contains(keyID, ":") {
		return APIKeyCipher{}, ErrEncryption
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return APIKeyCipher{}, fmt.Errorf("%w: %v", ErrEncryption, err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return APIKeyCipher{}, fmt.Errorf("%w: %v", ErrEncryption, err)
	}

	return APIKeyCipher{aead: aead, keyID: keyID}, nil
}

func deriveEncryptionKey(secret string) ([]byte, error) {
	secret = strings.TrimSpace(secret)
	if len(secret) < 32 {
		return nil, ErrEncryption
	}

	decoded, err := base64.StdEncoding.DecodeString(secret)
	if err == nil && len(decoded) == 32 {
		return decoded, nil
	}

	sum := sha256.Sum256([]byte(secret))
	return sum[:], nil
}

func (c APIKeyCipher) Encrypt(plain string) (string, string, error) {
	plain = strings.TrimSpace(plain)
	if plain == "" || len(plain) > 4096 {
		return "", "", ErrValidation
	}
	if c.aead == nil {
		return "", "", ErrEncryption
	}

	nonce := make([]byte, c.aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", "", fmt.Errorf("%w: %v", ErrEncryption, err)
	}

	ciphertext := c.aead.Seal(nil, nonce, []byte(plain), []byte(c.keyID))
	payload := append(nonce, ciphertext...)
	return encryptedPayloadVersion + ":" + c.keyID + ":" + base64.StdEncoding.EncodeToString(payload), apiKeyHint(plain), nil
}

func (c APIKeyCipher) Decrypt(encrypted string) (string, error) {
	encrypted = strings.TrimSpace(encrypted)
	if encrypted == "" || c.aead == nil {
		return "", ErrEncryption
	}

	parts := strings.Split(encrypted, ":")
	if len(parts) != 3 || parts[0] != encryptedPayloadVersion || parts[1] == "" || parts[1] != c.keyID {
		return "", ErrEncryption
	}

	payload, err := base64.StdEncoding.DecodeString(parts[2])
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrEncryption, err)
	}
	if len(payload) <= c.aead.NonceSize() {
		return "", ErrEncryption
	}

	nonce := payload[:c.aead.NonceSize()]
	ciphertext := payload[c.aead.NonceSize():]
	plain, err := c.aead.Open(nil, nonce, ciphertext, []byte(parts[1]))
	if err != nil {
		return "", ErrEncryption
	}

	return string(plain), nil
}

func apiKeyHint(apiKey string) string {
	apiKey = strings.TrimSpace(apiKey)
	runes := []rune(apiKey)
	if len(runes) <= 4 {
		return "****"
	}
	return "****" + string(runes[len(runes)-4:])
}
