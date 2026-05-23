package liveactivity

import (
	"encoding/base64"
	"encoding/pem"
	"strings"
	"testing"
	"time"
)

func TestParseP8Key_Valid(t *testing.T) {
	key, err := parseP8Key(testP8Key(t))
	if err != nil || key == nil {
		t.Fatalf("expected valid key, got err=%v", err)
	}
}

func TestParseP8Key_InvalidBase64(t *testing.T) {
	_, err := parseP8Key("not!!valid!!base64@@")
	if err == nil {
		t.Fatal("expected error for invalid base64 input")
	}
}

func TestParseP8Key_ValidBase64NoPEMBlock(t *testing.T) {
	_, err := parseP8Key(base64.StdEncoding.EncodeToString([]byte("not a pem block")))
	if err == nil {
		t.Fatal("expected error when base64 decodes but contains no PEM block")
	}
}

func TestParseP8Key_InvalidPKCS8Data(t *testing.T) {
	pemBytes := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: []byte("garbage pkcs8")})
	_, err := parseP8Key(base64.StdEncoding.EncodeToString(pemBytes))
	if err == nil {
		t.Fatal("expected error for invalid PKCS8 data inside PEM block")
	}
}

func TestJWTSigner_TokenHasThreeParts(t *testing.T) {
	signer, err := newJWTSigner(testP8Key(t), "KEYID12345", "TEAMID1234")
	if err != nil {
		t.Fatalf("newJWTSigner: %v", err)
	}
	token, err := signer.Token()
	if err != nil {
		t.Fatalf("Token(): %v", err)
	}
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		t.Fatalf("want 3 JWT parts (header.payload.sig), got %d in %q", len(parts), token)
	}
	// ES256 for P-256: r||s each 32 bytes = 64 bytes raw, 86 base64url chars
	sig, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		t.Fatalf("decode signature: %v", err)
	}
	if len(sig) != 64 {
		t.Errorf("want 64-byte ES256 signature, got %d bytes", len(sig))
	}
}

func TestJWTSigner_CachesTokenWithinInterval(t *testing.T) {
	signer, _ := newJWTSigner(testP8Key(t), "KID", "TID")
	tok1, _ := signer.Token()
	tok2, _ := signer.Token()
	if tok1 != tok2 {
		t.Error("want same cached token on consecutive calls within refresh interval")
	}
}

func TestJWTSigner_RefreshesTokenAfterInterval(t *testing.T) {
	signer, _ := newJWTSigner(testP8Key(t), "KID", "TID")
	tok1, _ := signer.Token()

	// Backdate issuedAt past the refresh threshold.
	signer.mu.Lock()
	signer.issuedAt = time.Now().Add(-(jwtRefreshInterval + time.Second))
	signer.mu.Unlock()

	tok2, err := signer.Token()
	if err != nil {
		t.Fatalf("Token() after expiry: %v", err)
	}
	if tok1 == tok2 {
		t.Error("want new token after refresh interval elapsed, got same token")
	}
}
