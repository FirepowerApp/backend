package liveactivity

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"net/http/httptest"
	"testing"
	"time"
)

// testP8Key generates a fresh P-256 private key and returns it base64-encoded
// in the format expected by APNS_AUTH_KEY.
func testP8Key(t *testing.T) string {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate P-256 key: %v", err)
	}
	pkcs8, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatalf("marshal PKCS8: %v", err)
	}
	pemBytes := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: pkcs8})
	return base64.StdEncoding.EncodeToString(pemBytes)
}

// testNotifier returns a LiveActivityNotifier wired to srv instead of real APNs.
func testNotifier(t *testing.T, srv *httptest.Server) *LiveActivityNotifier {
	t.Helper()
	signer, err := newJWTSigner(testP8Key(t), "KID", "TID")
	if err != nil {
		t.Fatalf("newJWTSigner: %v", err)
	}
	return &LiveActivityNotifier{
		client: &apnsClient{
			http:     srv.Client(),
			signer:   signer,
			host:     srv.Listener.Addr().String(),
			bundleID: "me.test.app",
		},
	}
}

// testEnvelope builds a minimal valid dispatch envelope JSON string.
func testEnvelope(t *testing.T, channels []string) string {
	t.Helper()
	payloadBytes, _ := json.Marshal(map[string]interface{}{
		"aps": map[string]interface{}{
			"timestamp":     time.Now().Unix(),
			"event":         "update",
			"stale-date":    time.Now().Add(90 * time.Second).Unix(),
			"content-state": map[string]interface{}{},
		},
	})
	b, err := json.Marshal(dispatchEnvelope{Channels: channels, Payload: payloadBytes})
	if err != nil {
		t.Fatalf("marshal test envelope: %v", err)
	}
	return string(b)
}
