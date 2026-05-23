package liveactivity

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestSendNotification_EmptyMessage(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	_, err := testNotifier(t, srv).SendNotification(context.Background(), "")
	if err == nil {
		t.Fatal("expected error for empty message, got nil")
	}
}

func TestSendNotification_InvalidJSON(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	_, err := testNotifier(t, srv).SendNotification(context.Background(), "not-json")
	if err == nil {
		t.Fatal("expected error for non-JSON message, got nil")
	}
}

func TestSendNotification_EmptyChannels(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	_, err := testNotifier(t, srv).SendNotification(context.Background(), testEnvelope(t, []string{}))
	if err == nil {
		t.Fatal("expected error for empty channels list, got nil")
	}
}

func TestSendNotification_SuccessfulPush(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	ch, err := testNotifier(t, srv).SendNotification(context.Background(), testEnvelope(t, []string{"chan-1"}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	result := <-ch
	if !result.Success {
		t.Errorf("expected Success=true, got error: %v", result.Error)
	}
	if result.ID == "" {
		t.Error("expected non-empty result ID")
	}
}

func TestSendNotification_MultipleChannels(t *testing.T) {
	hits := 0
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	ch, err := testNotifier(t, srv).SendNotification(context.Background(), testEnvelope(t, []string{"chan-1", "chan-2"}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	result := <-ch
	if !result.Success {
		t.Errorf("expected Success=true, got error: %v", result.Error)
	}
	if hits != 2 {
		t.Errorf("expected 2 APNs requests (one per channel), got %d", hits)
	}
}

func TestPushWithRetry_NonRetryableStopsImmediately(t *testing.T) {
	calls := 0
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.WriteHeader(http.StatusBadRequest) // 400 — non-retryable
	}))
	defer srv.Close()

	err := testNotifier(t, srv).pushWithRetry(context.Background(), "chan-1", []byte(`{}`))
	if err == nil {
		t.Fatal("expected error for 400 response")
	}
	if calls != 1 {
		t.Errorf("expected exactly 1 call (no retry on non-retryable error), got %d", calls)
	}
}

func TestPushWithRetry_RetryableSucceedsOnSecondAttempt(t *testing.T) {
	calls := 0
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if calls < 2 {
			w.WriteHeader(http.StatusTooManyRequests) // 429 — retryable
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	err := testNotifier(t, srv).pushWithRetry(context.Background(), "chan-1", []byte(`{}`))
	if err != nil {
		t.Fatalf("expected success after retry, got: %v", err)
	}
	if calls != 2 {
		t.Errorf("expected 2 calls (retry once), got %d", calls)
	}
}

func TestPushWithRetry_ContextCancelAbortsRetry(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable) // 503 — retryable
	}))
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	err := testNotifier(t, srv).pushWithRetry(ctx, "chan-1", []byte(`{}`))
	if err == nil {
		t.Fatal("expected error when context is cancelled mid-retry")
	}
}

func TestPushWithRetry_GoneIs200(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusGone) // 410 — channel gone, treated as success
	}))
	defer srv.Close()

	err := testNotifier(t, srv).pushWithRetry(context.Background(), "chan-1", []byte(`{}`))
	if err != nil {
		t.Errorf("expected 410 Gone to be treated as non-error, got: %v", err)
	}
}
