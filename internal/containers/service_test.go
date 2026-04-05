package containers

import (
	"testing"
	"time"
)

func TestSuspendUsesStopSemantics(t *testing.T) {
	t.Parallel()

	if got := requestedToIncusAction("suspend"); got != "stop" {
		t.Fatalf("expected suspend to map to stop, got %s", got)
	}

	instanceStatus, serviceStatus, suspendedAt := statusAfterAction("suspend", time.Now().UTC())
	if instanceStatus != "stopped" || serviceStatus != "stopped" {
		t.Fatalf("unexpected statuses: %s %s", instanceStatus, serviceStatus)
	}
	if suspendedAt != nil {
		t.Fatalf("expected suspended_at to be nil")
	}

	if eventType := actionToEventType("suspend"); eventType != "container_stopped" {
		t.Fatalf("unexpected event type: %s", eventType)
	}
}

func TestNormalizeLegacyStatusMapsSuspendedToStopped(t *testing.T) {
	t.Parallel()

	if got := normalizeLegacyStatus("suspended"); got != "stopped" {
		t.Fatalf("expected stopped, got %s", got)
	}
	if got := normalizeLegacyStatus("running"); got != "running" {
		t.Fatalf("expected running to stay running, got %s", got)
	}
}
