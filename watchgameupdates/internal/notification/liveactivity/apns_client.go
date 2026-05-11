package liveactivity

// APNs HTTP/2 client for Live Activity broadcast push.
//
// Endpoint: POST /4/broadcasts/apps/{bundleID}
// Channel ID passed via apns-channel-id header (raw base64 from channels.go, no encoding).
// Required headers:
//   apns-push-type:  liveactivity
//   apns-channel-id: {channelID}
//   apns-priority:   10
//   apns-expiration: {unix timestamp}
//   authorization:   bearer {jwt}

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"
)

type apnsClient struct {
	http     *http.Client
	signer   *jwtSigner
	host     string
	bundleID string
}

func newAPNsClient(cfg *Config) (*apnsClient, error) {
	signer, err := newJWTSigner(cfg.AuthKey, cfg.KeyID, cfg.TeamID)
	if err != nil {
		return nil, fmt.Errorf("init JWT signer: %w", err)
	}

	httpClient := &http.Client{
		Timeout: 10 * time.Second,
	}

	return &apnsClient{
		http:     httpClient,
		signer:   signer,
		host:     cfg.Host,
		bundleID: cfg.Topic,
	}, nil
}

// Push sends a broadcast Live Activity push to the given channel ID.
// channelID is the raw base64 value stored in channels.go — passed as-is in the apns-channel-id header.
func (c *apnsClient) Push(ctx context.Context, channelID string, payload []byte) error {
	jwt, err := c.signer.Token()
	if err != nil {
		return fmt.Errorf("get JWT: %w", err)
	}

	expiration := fmt.Sprintf("%d", time.Now().Add(15*time.Minute).Unix())
	url := fmt.Sprintf("https://%s/4/broadcasts/apps/%s", c.host, c.bundleID)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("authorization", "bearer "+jwt)
	req.Header.Set("apns-push-type", "liveactivity")
	req.Header.Set("apns-channel-id", channelID)
	req.Header.Set("apns-priority", "10")
	req.Header.Set("apns-expiration", expiration)
	req.Header.Set("content-type", "application/json")

	start := time.Now()
	resp, err := c.http.Do(req)
	latencyMs := time.Since(start).Milliseconds()

	if err != nil {
		log.Printf("APNs push error: channel=%s latency_ms=%d err=%v", channelID, latencyMs, err)
		return fmt.Errorf("APNs push: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
	log.Printf("APNs push: channel=%s status=%d latency_ms=%d", channelID, resp.StatusCode, latencyMs)

	switch resp.StatusCode {
	case http.StatusOK:
		return nil
	case http.StatusGone:
		log.Printf("APNs push: channel %s is gone (410), dropping", channelID)
		return nil
	case http.StatusTooManyRequests:
		return &retryableError{status: resp.StatusCode, body: string(body)}
	case http.StatusInternalServerError, http.StatusServiceUnavailable:
		return &retryableError{status: resp.StatusCode, body: string(body)}
	default:
		return fmt.Errorf("APNs push unexpected status %d: %s", resp.StatusCode, body)
	}
}

// retryableError signals the caller should retry with backoff.
type retryableError struct {
	status int
	body   string
}

func (e *retryableError) Error() string {
	return fmt.Sprintf("APNs status %d: %s", e.status, e.body)
}

func isRetryable(err error) bool {
	_, ok := err.(*retryableError)
	return ok
}
