package integrations

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
)

// EncodeState produces a state value for OAuth: base64url(orgID) + "." + base64url(HMAC(key, orgID)).
// Key must be at least 32 bytes (use first 32 of encryption key or full key).
func EncodeState(key []byte, orgID uuid.UUID) (string, error) {
	if len(key) < 32 {
		return "", errors.New("key must be at least 32 bytes for state encoding")
	}
	s := orgID.String()
	mac := hmac.New(sha256.New, key[:32])
	mac.Write([]byte(s))
	sum := mac.Sum(nil)
	return base64.URLEncoding.EncodeToString([]byte(s)) + "." + base64.URLEncoding.EncodeToString(sum), nil
}

// DecodeState verifies and decodes state, returning the organization ID.
func DecodeState(key []byte, state string) (uuid.UUID, error) {
	if len(key) < 32 {
		return uuid.Nil, errors.New("key must be at least 32 bytes for state decoding")
	}
	parts := strings.SplitN(state, ".", 2)
	if len(parts) != 2 {
		return uuid.Nil, errors.New("invalid state format")
	}
	orgIDBytes, err := base64.URLEncoding.DecodeString(parts[0])
	if err != nil {
		return uuid.Nil, fmt.Errorf("decode state org_id: %w", err)
	}
	sig, err := base64.URLEncoding.DecodeString(parts[1])
	if err != nil {
		return uuid.Nil, fmt.Errorf("decode state signature: %w", err)
	}
	mac := hmac.New(sha256.New, key[:32])
	mac.Write(orgIDBytes)
	expected := mac.Sum(nil)
	if !hmac.Equal(sig, expected) {
		return uuid.Nil, errors.New("invalid state signature")
	}
	orgID, err := uuid.Parse(string(orgIDBytes))
	if err != nil {
		return uuid.Nil, fmt.Errorf("parse org_id: %w", err)
	}
	return orgID, nil
}

// EncodeStateWithReturn produces a state value for OAuth that includes an optional return path.
// Payload is orgID + "|" + returnTo; state = base64(payload) + "." + base64(HMAC(key, payload)).
// Key must be at least 32 bytes. returnTo should be allowlisted by the caller (e.g. "onboarding", "integrations").
func EncodeStateWithReturn(key []byte, orgID uuid.UUID, returnTo string) (string, error) {
	if len(key) < 32 {
		return "", errors.New("key must be at least 32 bytes for state encoding")
	}
	payload := orgID.String() + "|" + returnTo
	mac := hmac.New(sha256.New, key[:32])
	mac.Write([]byte(payload))
	sum := mac.Sum(nil)
	return base64.URLEncoding.EncodeToString([]byte(payload)) + "." + base64.URLEncoding.EncodeToString(sum), nil
}

// DecodeStateWithReturn verifies and decodes state, returning the organization ID and return path.
// If the payload does not contain "|", returnTo is empty (backward compatible with old state).
func DecodeStateWithReturn(key []byte, state string) (uuid.UUID, string, error) {
	if len(key) < 32 {
		return uuid.Nil, "", errors.New("key must be at least 32 bytes for state decoding")
	}
	parts := strings.SplitN(state, ".", 2)
	if len(parts) != 2 {
		return uuid.Nil, "", errors.New("invalid state format")
	}
	payloadBytes, err := base64.URLEncoding.DecodeString(parts[0])
	if err != nil {
		return uuid.Nil, "", fmt.Errorf("decode state payload: %w", err)
	}
	sig, err := base64.URLEncoding.DecodeString(parts[1])
	if err != nil {
		return uuid.Nil, "", fmt.Errorf("decode state signature: %w", err)
	}
	mac := hmac.New(sha256.New, key[:32])
	mac.Write(payloadBytes)
	expected := mac.Sum(nil)
	if !hmac.Equal(sig, expected) {
		return uuid.Nil, "", errors.New("invalid state signature")
	}
	payloadStr := string(payloadBytes)
	sep := strings.Index(payloadStr, "|")
	var orgIDStr, returnTo string
	if sep >= 0 {
		orgIDStr = payloadStr[:sep]
		returnTo = payloadStr[sep+1:]
	} else {
		orgIDStr = payloadStr
	}
	orgID, err := uuid.Parse(orgIDStr)
	if err != nil {
		return uuid.Nil, "", fmt.Errorf("parse org_id: %w", err)
	}
	return orgID, returnTo, nil
}
