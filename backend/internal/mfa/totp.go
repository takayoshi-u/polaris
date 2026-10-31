package mfa

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/base32"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/takayoshi-u/polaris/backend/internal/domain"
)

var b32Encoding = base32.StdEncoding.WithPadding(base32.NoPadding)

// GenerateTOTPSecret generates a secure random 20-byte secret encoded in Base32.
func GenerateTOTPSecret() (string, error) {
	b := make([]byte, 20)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return b32Encoding.EncodeToString(b), nil
}

// GenerateTOTP creates a 6-digit TOTP code for the given secret at time t (RFC 6238).
func GenerateTOTP(secret string, t time.Time) (string, error) {
	cleanSecret := strings.ToUpper(strings.TrimSpace(secret))
	// Support with or without padding
	cleanSecret = strings.TrimRight(cleanSecret, "=")
	key, err := b32Encoding.DecodeString(cleanSecret)
	if err != nil {
		return "", fmt.Errorf("invalid base32 secret: %w", err)
	}

	counter := uint64(t.Unix() / 30)
	var buf [8]byte
	binary.BigEndian.PutUint64(buf[:], counter)

	mac := hmac.New(sha1.New, key)
	mac.Write(buf[:])
	h := mac.Sum(nil)

	offset := h[len(h)-1] & 0x0F
	code := binary.BigEndian.Uint32(h[offset : offset+4])
	code &= 0x7FFFFFFF
	code %= 1000000

	return fmt.Sprintf("%06d", code), nil
}

// ValidateTOTP checks if the provided 6-digit passcode matches the secret within ±1 time step.
func ValidateTOTP(secret string, passcode string, t time.Time) bool {
	passcode = strings.TrimSpace(passcode)
	if len(passcode) != 6 {
		return false
	}

	// ±1 step tolerance (90 seconds total window: -30s, 0s, +30s)
	steps := []time.Duration{-30 * time.Second, 0, 30 * time.Second}
	for _, step := range steps {
		expected, err := GenerateTOTP(secret, t.Add(step))
		if err == nil && hmac.Equal([]byte(expected), []byte(passcode)) {
			return true
		}
	}
	return false
}

// GenerateTOTPURI generates the otpauth:// URI for QR codes.
func GenerateTOTPURI(secret, email, issuer string) string {
	if issuer == "" {
		issuer = "Polaris"
	}
	label := fmt.Sprintf("%s:%s", issuer, email)
	u := url.URL{
		Scheme: "otpauth",
		Host:   "totp",
		Path:   "/" + label,
	}
	q := url.Values{}
	q.Set("secret", secret)
	q.Set("issuer", issuer)
	q.Set("algorithm", "SHA1")
	q.Set("digits", "6")
	q.Set("period", "30")
	u.RawQuery = q.Encode()
	return u.String()
}

// GenerateBackupCodes generates n backup codes (e.g. 8 codes of 8 hex/digits)
// and returns both the plain codes (for one-time display to user) and hashed entries (for storage).
func GenerateBackupCodes(count int) ([]string, []domain.BackupCodeEntry, error) {
	if count <= 0 {
		count = 8
	}
	plainCodes := make([]string, count)
	entries := make([]domain.BackupCodeEntry, count)

	for i := 0; i < count; i++ {
		b := make([]byte, 5) // 5 bytes = 10 hex characters
		if _, err := rand.Read(b); err != nil {
			return nil, nil, err
		}
		code := strings.ToUpper(hex.EncodeToString(b))
		// Format as 5-5 (e.g. 1A2B3-4C5D6)
		formatted := fmt.Sprintf("%s-%s", code[:5], code[5:])
		plainCodes[i] = formatted

		clean := strings.ReplaceAll(formatted, "-", "")
		h := sha256.Sum256([]byte(clean))
		entries[i] = domain.BackupCodeEntry{
			Hash: hex.EncodeToString(h[:]),
			Used: false,
		}
	}

	return plainCodes, entries, nil
}

// VerifyAndUseBackupCode checks if inputCode matches any unused backup code.
// If matched, marks it as used and returns true, updated entries.
func VerifyAndUseBackupCode(entries []domain.BackupCodeEntry, inputCode string) (bool, []domain.BackupCodeEntry, error) {
	clean := strings.ToUpper(strings.TrimSpace(inputCode))
	clean = strings.ReplaceAll(clean, "-", "")
	if len(clean) == 0 {
		return false, entries, errors.New("empty backup code")
	}

	h := sha256.Sum256([]byte(clean))
	targetHash := hex.EncodeToString(h[:])

	updated := make([]domain.BackupCodeEntry, len(entries))
	copy(updated, entries)

	for i, entry := range updated {
		if !entry.Used && hmac.Equal([]byte(entry.Hash), []byte(targetHash)) {
			updated[i].Used = true
			return true, updated, nil
		}
	}

	return false, entries, nil
}
