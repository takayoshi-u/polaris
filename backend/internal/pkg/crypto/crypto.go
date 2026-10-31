package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"io"

	"golang.org/x/crypto/bcrypt"
)

// HashPassword hashes a plaintext password using bcrypt.
func HashPassword(password string) (string, error) {
	if len(password) < 8 {
		return "", errors.New("password must be at least 8 characters")
	}
	b, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// CheckPassword verifies a plaintext password against a bcrypt hash.
func CheckPassword(password, hash string) bool {
	if hash == "" || password == "" {
		return false
	}
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}

// GenerateRandomToken generates a cryptographically secure random hex token and its SHA-256 hash.
func GenerateRandomToken(byteLen int) (rawToken string, tokenHash string, err error) {
	if byteLen <= 0 {
		byteLen = 32
	}
	buf := make([]byte, byteLen)
	if _, err := io.ReadFull(rand.Reader, buf); err != nil {
		return "", "", err
	}
	rawToken = hex.EncodeToString(buf)
	tokenHash = HashToken(rawToken)
	return rawToken, tokenHash, nil
}

// HashToken computes the SHA-256 hex digest of a token string.
func HashToken(rawToken string) string {
	sum := sha256.Sum256([]byte(rawToken))
	return hex.EncodeToString(sum[:])
}

// deriveAESKey derives a 32-byte AES-256 key from any passphrase using SHA-256.
func deriveAESKey(secret string) []byte {
	sum := sha256.Sum256([]byte(secret))
	return sum[:]
}

// EncryptAESGCM encrypts plaintext using AES-256-GCM and returns base64 ciphertext.
func EncryptAESGCM(plaintext, secret string) (string, error) {
	if plaintext == "" {
		return "", nil
	}
	key := deriveAESKey(secret)
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	sealed := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return base64.StdEncoding.EncodeToString(sealed), nil
}

// DecryptAESGCM decrypts a base64 ciphertext produced by EncryptAESGCM.
func DecryptAESGCM(ciphertextB64, secret string) (string, error) {
	if ciphertextB64 == "" {
		return "", nil
	}
	raw, err := base64.StdEncoding.DecodeString(ciphertextB64)
	if err != nil {
		return "", err
	}
	key := deriveAESKey(secret)
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonceSize := gcm.NonceSize()
	if len(raw) < nonceSize {
		return "", errors.New("ciphertext too short")
	}
	nonce, ciphertext := raw[:nonceSize], raw[nonceSize:]
	plain, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", err
	}
	return string(plain), nil
}

