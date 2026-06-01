package provider

import (
	"strings"
	"testing"
)

func TestAPIKeyCipherEncryptsAndHintsWithoutPlaintext(t *testing.T) {
	cipher, err := NewAPIKeyCipher("0123456789abcdef0123456789abcdef", "test-key-v1")
	if err != nil {
		t.Fatalf("NewAPIKeyCipher returned error: %v", err)
	}

	encrypted, hint, err := cipher.Encrypt("sk-test-secret-value")
	if err != nil {
		t.Fatalf("Encrypt returned error: %v", err)
	}
	if strings.Contains(encrypted, "sk-test-secret-value") {
		t.Fatalf("encrypted payload contains plaintext key: %s", encrypted)
	}
	if !strings.HasPrefix(encrypted, "v1:test-key-v1:") {
		t.Fatalf("encrypted payload version/key id = %q", encrypted)
	}
	if hint != "****alue" {
		t.Fatalf("hint = %q, want ****alue", hint)
	}

	decrypted, err := cipher.Decrypt(encrypted)
	if err != nil {
		t.Fatalf("Decrypt returned error: %v", err)
	}
	if decrypted != "sk-test-secret-value" {
		t.Fatalf("decrypted = %q", decrypted)
	}
}

func TestAPIKeyCipherRejectsWeakConfigAndEmptyKeys(t *testing.T) {
	if _, err := NewAPIKeyCipher("short", "test-key-v1"); err == nil {
		t.Fatal("NewAPIKeyCipher accepted weak secret")
	}
	cipher, err := NewAPIKeyCipher("0123456789abcdef0123456789abcdef", "test-key-v1")
	if err != nil {
		t.Fatalf("NewAPIKeyCipher returned error: %v", err)
	}
	if _, _, err := cipher.Encrypt(" "); err == nil {
		t.Fatal("Encrypt accepted empty API key")
	}
}

func TestAPIKeyCipherDecryptRejectsPayloadForDifferentKeyID(t *testing.T) {
	const secret = "0123456789abcdef0123456789abcdef"

	payloadCipher, err := NewAPIKeyCipher(secret, "payload-key-v2")
	if err != nil {
		t.Fatalf("NewAPIKeyCipher payload cipher returned error: %v", err)
	}
	encrypted, _, err := payloadCipher.Encrypt("sk-test-secret-value")
	if err != nil {
		t.Fatalf("Encrypt returned error: %v", err)
	}

	instanceCipher, err := NewAPIKeyCipher(secret, "instance-key-v1")
	if err != nil {
		t.Fatalf("NewAPIKeyCipher instance cipher returned error: %v", err)
	}
	if _, err := instanceCipher.Decrypt(encrypted); err == nil {
		t.Fatal("Decrypt accepted payload for a different key id")
	}
}
