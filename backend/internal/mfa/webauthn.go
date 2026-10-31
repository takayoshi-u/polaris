package mfa

import (
	"bytes"
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"strings"
)

// GenerateChallenge generates a 32-byte crypto-random challenge in Base64URL encoding (without padding).
func GenerateChallenge() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// WebAuthn Creation & Request Options
type PublicKeyCredentialCreationOptions struct {
	RP                     RPEntity                  `json:"rp"`
	User                   UserEntity                `json:"user"`
	Challenge              string                    `json:"challenge"`
	PubKeyCredParams       []PubKeyCredParam         `json:"pubKeyCredParams"`
	Timeout                int64                     `json:"timeout"`
	Attestation            string                    `json:"attestation"`
	AuthenticatorSelection AuthenticatorSelection    `json:"authenticatorSelection"`
}

type RPEntity struct {
	Name string `json:"name"`
	ID   string `json:"id,omitempty"`
}

type UserEntity struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	DisplayName string `json:"displayName"`
}

type PubKeyCredParam struct {
	Type string `json:"type"`
	Alg  int64  `json:"alg"`
}

type AuthenticatorSelection struct {
	ResidentKey      string `json:"residentKey,omitempty"`
	UserVerification string `json:"userVerification,omitempty"`
}

type PublicKeyCredentialRequestOptions struct {
	Challenge        string                 `json:"challenge"`
	Timeout          int64                  `json:"timeout"`
	RPID             string                 `json:"rpId,omitempty"`
	AllowCredentials []CredentialDescriptor `json:"allowCredentials"`
	UserVerification string                 `json:"userVerification,omitempty"`
}

type CredentialDescriptor struct {
	Type string `json:"type"`
	ID   string `json:"id"`
}

// ClientDataJSON structure
type ClientData struct {
	Type      string `json:"type"`
	Challenge string `json:"challenge"`
	Origin    string `json:"origin"`
}

// ParsedAttestation result
type ParsedAttestation struct {
	CredentialID    []byte
	PublicKey       []byte // COSE key bytes
	AAGUID          []byte
	AttestationType string
	SignCount       uint32
}

// ParseAttestation parses clientDataJSON and attestationObject from browser registration.
func ParseAttestation(clientDataJSON, attestationObject []byte, expectedChallenge string) (*ParsedAttestation, error) {
	// 1. Verify ClientData
	var cd ClientData
	if err := json.Unmarshal(clientDataJSON, &cd); err != nil {
		return nil, fmt.Errorf("invalid clientDataJSON: %w", err)
	}
	if cd.Type != "webauthn.create" {
		return nil, fmt.Errorf("expected clientData type 'webauthn.create', got %q", cd.Type)
	}
	if strings.TrimRight(cd.Challenge, "=") != strings.TrimRight(expectedChallenge, "=") {
		return nil, errors.New("challenge mismatch")
	}

	// 2. Decode AttestationObject (CBOR map)
	rawObj, err := UnmarshalCBOR(attestationObject)
	if err != nil {
		return nil, fmt.Errorf("failed to parse attestationObject CBOR: %w", err)
	}
	m, ok := rawObj.(map[any]any)
	if !ok {
		return nil, errors.New("attestationObject is not a CBOR map")
	}

	attType := "none"
	if fmtVal, ok := m["fmt"].(string); ok {
		attType = fmtVal
	}

	authDataBytes, ok := m["authData"].([]byte)
	if !ok || len(authDataBytes) < 37 {
		return nil, errors.New("missing or truncated authData in attestationObject")
	}

	// 3. Inspect authData
	// rpIdHash := authDataBytes[0:32]
	flags := authDataBytes[32]
	signCount := binary.BigEndian.Uint32(authDataBytes[33:37])

	// Check UP (User Present: bit 0)
	if flags&0x01 == 0 {
		return nil, errors.New("user present (UP) flag not set")
	}
	// Check AT (Attested credential data: bit 6)
	if flags&0x40 == 0 {
		return nil, errors.New("attested credential data (AT) flag not set")
	}

	// 4. Parse Attested Credential Data
	if len(authDataBytes) < 55 {
		return nil, errors.New("authData too short for attested credential data")
	}
	aaguid := authDataBytes[37:53]
	credIDLen := binary.BigEndian.Uint16(authDataBytes[53:55])

	offset := 55 + int(credIDLen)
	if len(authDataBytes) < offset {
		return nil, errors.New("authData truncated before credential ID ended")
	}
	credentialID := authDataBytes[55:offset]
	cosePubKey := authDataBytes[offset:]

	if len(cosePubKey) == 0 {
		return nil, errors.New("missing credential public key in authData")
	}

	return &ParsedAttestation{
		CredentialID:    credentialID,
		PublicKey:       cosePubKey,
		AAGUID:          aaguid,
		AttestationType: attType,
		SignCount:       signCount,
	}, nil
}

// VerifyAssertion verifies the login assertion response from the browser against the stored credential.
func VerifyAssertion(clientDataJSON, authData, signature, cosePublicKey []byte, expectedChallenge string, currentSignCount int64) (uint32, error) {
	// 1. Verify ClientData
	var cd ClientData
	if err := json.Unmarshal(clientDataJSON, &cd); err != nil {
		return 0, fmt.Errorf("invalid clientDataJSON: %w", err)
	}
	if cd.Type != "webauthn.get" {
		return 0, fmt.Errorf("expected clientData type 'webauthn.get', got %q", cd.Type)
	}
	if strings.TrimRight(cd.Challenge, "=") != strings.TrimRight(expectedChallenge, "=") {
		return 0, errors.New("challenge mismatch")
	}

	// 2. Verify AuthenticatorData
	if len(authData) < 37 {
		return 0, errors.New("authenticatorData is too short (< 37 bytes)")
	}
	flags := authData[32]
	if flags&0x01 == 0 {
		return 0, errors.New("user present (UP) flag not set")
	}
	newSignCount := binary.BigEndian.Uint32(authData[33:37])

	// 3. Signature Verification
	// Signed message is authData || SHA-256(clientDataJSON)
	clientDataHash := sha256.Sum256(clientDataJSON)
	var signedData bytes.Buffer
	signedData.Write(authData)
	signedData.Write(clientDataHash[:])
	hashedMsg := sha256.Sum256(signedData.Bytes())

	if err := verifySignature(cosePublicKey, hashedMsg[:], signature); err != nil {
		return 0, fmt.Errorf("signature verification failed: %w", err)
	}

	return newSignCount, nil
}

// verifySignature parses COSE key bytes and verifies ECDSA or RSA signature.
func verifySignature(coseKeyBytes, hash, signature []byte) error {
	coseObj, err := UnmarshalCBOR(coseKeyBytes)
	if err != nil {
		return fmt.Errorf("invalid COSE key CBOR: %w", err)
	}
	coseMap, ok := coseObj.(map[any]any)
	if !ok {
		return errors.New("COSE key is not a CBOR map")
	}

	// 1: KeyType (2 = EC2, 3 = RSA), 3: Alg (-7 = ES256, -257 = RS256)
	kty, _ := coseMap[int64(1)].(int64)
	alg, _ := coseMap[int64(3)].(int64)

	switch {
	case kty == 2 || alg == -7: // EC2 / ES256
		// -1: crv (1 = P-256), -2: x (32 bytes), -3: y (32 bytes)
		xBytes, okX := coseMap[int64(-2)].([]byte)
		yBytes, okY := coseMap[int64(-3)].([]byte)
		if !okX || !okY {
			return errors.New("missing x or y coordinate in EC COSE key")
		}

		pubKey := ecdsa.PublicKey{
			Curve: elliptic.P256(),
			X:     new(big.Int).SetBytes(xBytes),
			Y:     new(big.Int).SetBytes(yBytes),
		}

		// WebAuthn signature is ASN.1 DER encoded ECDSA-Sig-Value
		if ecdsa.VerifyASN1(&pubKey, hash, signature) {
			return nil
		}

		// Fallback: check raw r||s format (64 bytes)
		if len(signature) == 64 {
			r := new(big.Int).SetBytes(signature[:32])
			s := new(big.Int).SetBytes(signature[32:])
			if ecdsa.Verify(&pubKey, hash, r, s) {
				return nil
			}
		}

		return errors.New("invalid ECDSA signature")

	case kty == 3 || alg == -257: // RSA / RS256
		// -1: n (modulus bytes), -2: e (exponent bytes)
		nBytes, okN := coseMap[int64(-1)].([]byte)
		eBytes, okE := coseMap[int64(-2)].([]byte)
		if !okN || !okE {
			return errors.New("missing n or e in RSA COSE key")
		}
		var eInt int
		for _, b := range eBytes {
			eInt = (eInt << 8) | int(b)
		}
		pubKey := rsa.PublicKey{
			N: new(big.Int).SetBytes(nBytes),
			E: eInt,
		}

		return rsa.VerifyPKCS1v15(&pubKey, crypto.SHA256, hash, signature)

	default:
		return fmt.Errorf("unsupported COSE key type %d / alg %d", kty, alg)
	}
}

// GenerateRegistrationOptions creates options for navigator.credentials.create().
func GenerateRegistrationOptions(userID, userEmail, userDisplayName, rpID, rpName string) (*PublicKeyCredentialCreationOptions, string, error) {
	challenge, err := GenerateChallenge()
	if err != nil {
		return nil, "", err
	}
	opts := &PublicKeyCredentialCreationOptions{
		RP: RPEntity{
			Name: rpName,
			ID:   rpID,
		},
		User: UserEntity{
			ID:          base64.RawURLEncoding.EncodeToString([]byte(userID)),
			Name:        userEmail,
			DisplayName: userDisplayName,
		},
		Challenge: challenge,
		PubKeyCredParams: []PubKeyCredParam{
			{Type: "public-key", Alg: -7},   // ES256
			{Type: "public-key", Alg: -257}, // RS256
		},
		Timeout:     60000,
		Attestation: "none",
		AuthenticatorSelection: AuthenticatorSelection{
			UserVerification: "preferred",
		},
	}
	return opts, challenge, nil
}

// GenerateRequestOptions creates options for navigator.credentials.get().
func GenerateRequestOptions(allowedCredIDs [][]byte, rpID string) (*PublicKeyCredentialRequestOptions, string, error) {
	challenge, err := GenerateChallenge()
	if err != nil {
		return nil, "", err
	}
	var descriptors []CredentialDescriptor
	for _, id := range allowedCredIDs {
		descriptors = append(descriptors, CredentialDescriptor{
			Type: "public-key",
			ID:   base64.RawURLEncoding.EncodeToString(id),
		})
	}
	opts := &PublicKeyCredentialRequestOptions{
		Challenge:        challenge,
		Timeout:          60000,
		RPID:             rpID,
		AllowCredentials: descriptors,
		UserVerification: "preferred",
	}
	return opts, challenge, nil
}

