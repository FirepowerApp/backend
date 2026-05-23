package notification

import (
	"context"
	"testing"
	"time"
)

type mockNotifier struct {
	keys []string
}

func (m *mockNotifier) GetRequiredDataKeys() []string { return m.keys }
func (m *mockNotifier) FormatMessage(_ NotificationRequest) string { return "mock" }
func (m *mockNotifier) SendNotification(_ context.Context, _ string) (<-chan NotificationResult, error) {
	ch := make(chan NotificationResult, 1)
	ch <- NotificationResult{ID: "mock-id", Success: true, Timestamp: time.Now()}
	return ch, nil
}
func (m *mockNotifier) Close() error { return nil }

func TestRegisterNotifier_AddsRequiredKeys(t *testing.T) {
	svc := NewServiceWithNotificationFlag(false)
	svc.RegisterNotifier(&mockNotifier{keys: []string{"homeTeamGoals", "awayTeamGoals"}})

	keys := svc.GetAllRequiredDataKeys()
	want := map[string]bool{"homeTeamGoals": false, "awayTeamGoals": false}
	for _, k := range keys {
		want[k] = true
	}
	for k, found := range want {
		if !found {
			t.Errorf("key %q missing from GetAllRequiredDataKeys after RegisterNotifier", k)
		}
	}
}

func TestRegisterNotifier_MultipleNotifiers(t *testing.T) {
	svc := NewServiceWithNotificationFlag(false)
	svc.RegisterNotifier(&mockNotifier{keys: []string{"a"}})
	svc.RegisterNotifier(&mockNotifier{keys: []string{"b"}})

	keys := svc.GetAllRequiredDataKeys()
	found := map[string]bool{}
	for _, k := range keys {
		found[k] = true
	}
	if !found["a"] || !found["b"] {
		t.Errorf("expected both 'a' and 'b' in required keys, got %v", keys)
	}
}
