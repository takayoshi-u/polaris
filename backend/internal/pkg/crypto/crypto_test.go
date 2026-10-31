package crypto_test

import (
	"testing"

	"github.com/takayoshi-u/polaris/backend/internal/pkg/crypto"
)

func TestPasswordHashAndCheck(t *testing.T) {
	if _, err := crypto.HashPassword("short"); err == nil {
		t.Fatal("expected error for password shorter than 8 characters")
	}

	hash, err := crypto.HashPassword("StrongPass123!")
	if err != nil {
		t.Fatalf("unexpected error hashing password: %v", err)
	}
	if !crypto.CheckPassword("StrongPass123!", hash) {
		t.Fatal("expected CheckPassword to succeed for matching password")
	}
	if crypto.CheckPassword("WrongPass123!", hash) {
		t.Fatal("expected CheckPassword to fail for wrong password")
	}
}

func TestRandomTokenAndHash(t *testing.T) {
	raw, hash, err := crypto.GenerateRandomToken(32)
	if err != nil {
		t.Fatalf("unexpected error generating token: %v", err)
	}
	if len(raw) != 64 {
		t.Fatalf("expected 64 hex chars, got %d", len(raw))
	}
	if crypto.HashToken(raw) != hash {
		t.Fatal("expected HashToken(raw) to match returned hash")
	}
}

func TestEncryptDecryptAESGCM(t *testing.T) {
	secret := "test-encryption-secret-key"
	plain := "smtp-secret-password-9876"

	enc, err := crypto.EncryptAESGCM(plain, secret)
	if err != nil {
		t.Fatalf("unexpected encrypt error: %v", err)
	}
	if enc == plain || enc == "" {
		t.Fatal("expected encrypted string to differ from plaintext")
	}

	dec, err := crypto.DecryptAESGCM(enc, secret)
	if err != nil {
		t.Fatalf("unexpected decrypt error: %v", err)
	}
	if dec != plain {
		t.Fatalf("expected decrypted string %q, got %q", plain, dec)
	}

	if _, err := crypto.DecryptAESGCM(enc, "wrong-secret"); err == nil {
		t.Fatal("expected decryption with wrong key to fail")
	}
}

