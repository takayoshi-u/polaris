package mfa

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"testing"
)

// Helper to encode a simple CBOR map
func encodeTestCOSEKey(pub *ecdsa.PublicKey) []byte {
	// Map with 5 entries:
	// 1: 2 (EC2)
	// 3: -7 (ES256)
	// -1: 1 (P-256)
	// -2: X bytes (32 bytes)
	// -3: Y bytes (32 bytes)
	var buf bytes.Buffer
	buf.WriteByte(0xa5) // Map of 5 items

	// 1: 2
	buf.Write([]byte{0x01, 0x02})
	// 3: -7 (negative int -1 - 6 = -7 -> 0x26)
	buf.Write([]byte{0x03, 0x26})
	// -1: 1 (-1 is 0x20)
	buf.Write([]byte{0x20, 0x01})
	// -2: x (-2 is 0x21, 32 bytes string 0x58 0x20)
	xBytes := pub.X.Bytes()
	if len(xBytes) < 32 {
		pad := make([]byte, 32-len(xBytes))
		xBytes = append(pad, xBytes...)
	}
	buf.Write([]byte{0x21, 0x58, 0x20})
	buf.Write(xBytes)
	// -3: y (-3 is 0x22, 32 bytes string 0x58 0x20)
	yBytes := pub.Y.Bytes()
	if len(yBytes) < 32 {
		pad := make([]byte, 32-len(yBytes))
		yBytes = append(pad, yBytes...)
	}
	buf.Write([]byte{0x22, 0x58, 0x20})
	buf.Write(yBytes)

	return buf.Bytes()
}

func TestWebAuthnLifecycle(t *testing.T) {
	challenge, err := GenerateChallenge()
	if err != nil {
		t.Fatalf("generate challenge: %v", err)
	}
	if len(challenge) < 32 {
		t.Fatalf("challenge too short")
	}

	// Generate key pair
	privKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate ecdsa key: %v", err)
	}
	cosePubKey := encodeTestCOSEKey(&privKey.PublicKey)

	// Create synthetic authData for registration
	var authData bytes.Buffer
	authData.Write(make([]byte, 32)) // rpIdHash
	authData.WriteByte(0x41)         // flags: UP (0x01) | AT (0x40)
	authData.Write([]byte{0, 0, 0, 1}) // signCount = 1

	aaguid := make([]byte, 16)
	authData.Write(aaguid)

	credID := []byte("test-credential-id-1234")
	var credIDLen [2]byte
	binary.BigEndian.PutUint16(credIDLen[:], uint16(len(credID)))
	authData.Write(credIDLen[:])
	authData.Write(credID)
	authData.Write(cosePubKey)

	// Create CBOR attestationObject map { "fmt": "none", "authData": authData.Bytes() }
	var attObjBuf bytes.Buffer
	attObjBuf.WriteByte(0xa2) // Map of 2 items
	// "fmt": "none"
	attObjBuf.Write([]byte{0x63, 'f', 'm', 't', 0x64, 'n', 'o', 'n', 'e'})
	// "authData": bytes
	ad := authData.Bytes()
	attObjBuf.Write([]byte{0x68, 'a', 'u', 't', 'h', 'D', 'a', 't', 'a'})
	attObjBuf.WriteByte(0x58)
	attObjBuf.WriteByte(byte(len(ad)))
	attObjBuf.Write(ad)

	// Create ClientDataJSON
	cdJSON, _ := json.Marshal(map[string]any{
		"type":      "webauthn.create",
		"challenge": challenge,
		"origin":    "http://localhost:3000",
	})

	// ParseAttestation
	parsed, err := ParseAttestation(cdJSON, attObjBuf.Bytes(), challenge)
	if err != nil {
		t.Fatalf("parse attestation failed: %v", err)
	}
	if string(parsed.CredentialID) != string(credID) {
		t.Fatalf("credential ID mismatch: got %s, want %s", string(parsed.CredentialID), string(credID))
	}
	if parsed.AttestationType != "none" {
		t.Fatalf("expected attestation type none, got %s", parsed.AttestationType)
	}

	// Assertion / Authentication Flow
	loginChallenge, _ := GenerateChallenge()
	loginCdJSON, _ := json.Marshal(map[string]any{
		"type":      "webauthn.get",
		"challenge": loginChallenge,
		"origin":    "http://localhost:3000",
	})

	var loginAuthData bytes.Buffer
	loginAuthData.Write(make([]byte, 32)) // rpIdHash
	loginAuthData.WriteByte(0x01)         // flags: UP (0x01)
	loginAuthData.Write([]byte{0, 0, 0, 2}) // signCount = 2

	cdHash := sha256.Sum256(loginCdJSON)
	var signedMsg bytes.Buffer
	signedMsg.Write(loginAuthData.Bytes())
	signedMsg.Write(cdHash[:])
	hashed := sha256.Sum256(signedMsg.Bytes())

	sig, err := ecdsa.SignASN1(rand.Reader, privKey, hashed[:])
	if err != nil {
		t.Fatalf("sign failed: %v", err)
	}

	// VerifyAssertion
	newCount, err := VerifyAssertion(loginCdJSON, loginAuthData.Bytes(), sig, cosePubKey, loginChallenge, 1)
	if err != nil {
		t.Fatalf("verify assertion failed: %v", err)
	}
	if newCount != 2 {
		t.Fatalf("expected sign count 2, got %d", newCount)
	}

	// Test invalid signature
	invalidSig := make([]byte, len(sig))
	copy(invalidSig, sig)
	invalidSig[len(invalidSig)-1] ^= 0xFF
	_, err = VerifyAssertion(loginCdJSON, loginAuthData.Bytes(), invalidSig, cosePubKey, loginChallenge, 1)
	if err == nil {
		t.Fatalf("expected invalid signature to fail")
	}

	// Test challenge mismatch
	_, err = VerifyAssertion(loginCdJSON, loginAuthData.Bytes(), sig, cosePubKey, "wrong-challenge", 1)
	if err == nil {
		t.Fatalf("expected wrong challenge to fail")
	}
}
