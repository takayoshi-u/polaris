package mfa

import (
	"strings"
	"testing"
	"time"
)

func TestTOTPGenerationAndValidation(t *testing.T) {
	secret, err := GenerateTOTPSecret()
	if err != nil {
		t.Fatalf("failed to generate secret: %v", err)
	}
	if len(secret) == 0 {
		t.Fatalf("secret is empty")
	}

	now := time.Now()
	code, err := GenerateTOTP(secret, now)
	if err != nil {
		t.Fatalf("failed to generate TOTP: %v", err)
	}
	if len(code) != 6 {
		t.Fatalf("expected 6-digit code, got %s", code)
	}

	// Validate exact time
	if !ValidateTOTP(secret, code, now) {
		t.Fatalf("expected code %s to be valid at now", code)
	}

	// Validate within +25 seconds (same or next step)
	if !ValidateTOTP(secret, code, now.Add(25*time.Second)) {
		t.Fatalf("expected code %s to be valid within tolerance (+25s)", code)
	}

	// Validate within -25 seconds
	if !ValidateTOTP(secret, code, now.Add(-25*time.Second)) {
		t.Fatalf("expected code %s to be valid within tolerance (-25s)", code)
	}

	// Invalid code
	if ValidateTOTP(secret, "000000", now) && code != "000000" {
		t.Fatalf("expected invalid code to fail")
	}

	// Way out of tolerance (+2 hours)
	if ValidateTOTP(secret, code, now.Add(2*time.Hour)) {
		t.Fatalf("expected code to fail after 2 hours")
	}
}

func TestTOTPURI(t *testing.T) {
	uri := GenerateTOTPURI("JBSWY3DPEHPK3PXP", "alice@example.com", "Polaris")
	if !strings.HasPrefix(uri, "otpauth://totp/Polaris:alice@example.com?") {
		t.Fatalf("unexpected uri: %s", uri)
	}
	if !strings.Contains(uri, "secret=JBSWY3DPEHPK3PXP") {
		t.Fatalf("missing secret in uri: %s", uri)
	}
	if !strings.Contains(uri, "issuer=Polaris") {
		t.Fatalf("missing issuer in uri: %s", uri)
	}
}

func TestBackupCodesLifecycle(t *testing.T) {
	plainCodes, entries, err := GenerateBackupCodes(8)
	if err != nil {
		t.Fatalf("generate backup codes: %v", err)
	}
	if len(plainCodes) != 8 || len(entries) != 8 {
		t.Fatalf("expected 8 codes, got %d", len(plainCodes))
	}

	// Test using first code
	codeToUse := plainCodes[0]
	matched, updated, err := VerifyAndUseBackupCode(entries, codeToUse)
	if err != nil || !matched {
		t.Fatalf("expected first code to match, matched=%v, err=%v", matched, err)
	}
	if !updated[0].Used {
		t.Fatalf("expected code 0 to be marked as used")
	}

	// Test reusing same code should fail
	matched2, _, err := VerifyAndUseBackupCode(updated, codeToUse)
	if err != nil || matched2 {
		t.Fatalf("expected reused code to fail, matched=%v", matched2)
	}

	// Test invalid code
	matched3, _, _ := VerifyAndUseBackupCode(updated, "INVALID-CODE")
	if matched3 {
		t.Fatalf("expected random code to fail")
	}
}
