package liveactivity

// APNs JWT lifecycle:
//
//   sign once → cache up to 50 min → auto-refresh
//   │
//   ├── header:  {"alg":"ES256","kid":KEY_ID}
//   ├── payload: {"iss":TEAM_ID,"iat":unix_ts}
//   └── signature: ES256(header.payload, p8_key)
//
// Apple accepts tokens up to 1 hour old. We refresh at 50 min to stay safe.

import (
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"sync"
	"time"
)

const jwtRefreshInterval = 50 * time.Minute

type jwtSigner struct {
	key      *ecdsa.PrivateKey
	keyID    string
	teamID   string
	mu       sync.Mutex
	token    string
	issuedAt time.Time
}

func newJWTSigner(authKeyPEM, keyID, teamID string) (*jwtSigner, error) {
	key, err := parseP8Key(authKeyPEM)
	if err != nil {
		return nil, fmt.Errorf("parse APNs auth key: %w", err)
	}
	return &jwtSigner{key: key, keyID: keyID, teamID: teamID}, nil
}

// Token returns a valid JWT, refreshing if the current one is near expiry.
func (s *jwtSigner) Token() (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.token != "" && time.Since(s.issuedAt) < jwtRefreshInterval {
		return s.token, nil
	}

	token, err := s.buildToken()
	if err != nil {
		return "", err
	}
	s.token = token
	s.issuedAt = time.Now()
	return token, nil
}

func (s *jwtSigner) buildToken() (string, error) {
	header, err := base64JSON(map[string]string{
		"alg": "ES256",
		"kid": s.keyID,
	})
	if err != nil {
		return "", err
	}

	payload, err := base64JSON(map[string]interface{}{
		"iss": s.teamID,
		"iat": time.Now().Unix(),
	})
	if err != nil {
		return "", err
	}

	msg := header + "." + payload
	digest := sha256.Sum256([]byte(msg))

	r, sig, err := ecdsa.Sign(rand.Reader, s.key, digest[:])
	if err != nil {
		return "", fmt.Errorf("ecdsa sign: %w", err)
	}

	// ES256 signature encoding: r || s, each left-padded to 32 bytes
	rb := leftPad(r.Bytes(), 32)
	sb := leftPad(sig.Bytes(), 32)
	sigEncoded := base64.RawURLEncoding.EncodeToString(append(rb, sb...))

	return msg + "." + sigEncoded, nil
}

func parseP8Key(pemStr string) (*ecdsa.PrivateKey, error) {
	decoded, err := base64.StdEncoding.DecodeString(pemStr)
	if err != nil {
		return nil, fmt.Errorf("base64-decode APNS_AUTH_KEY: %w (value must be base64-encoded .p8 contents)", err)
	}
	block, _ := pem.Decode(decoded)
	if block == nil {
		return nil, fmt.Errorf("failed to PEM-decode APNS_AUTH_KEY (base64 decoded but no PEM block found)")
	}
	raw, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("parse PKCS8 private key: %w", err)
	}
	ec, ok := raw.(*ecdsa.PrivateKey)
	if !ok {
		return nil, fmt.Errorf("APNS_AUTH_KEY is not an EC private key")
	}
	return ec, nil
}

func base64JSON(v interface{}) (string, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return "", fmt.Errorf("marshal JWT part: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func leftPad(b []byte, size int) []byte {
	if len(b) >= size {
		return b
	}
	out := make([]byte, size)
	copy(out[size-len(b):], b)
	return out
}
