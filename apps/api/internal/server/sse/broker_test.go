package sse

import (
	"strings"
	"testing"
	"time"
)

func TestSSEBrokerSubscriptionAndBroadcast(t *testing.T) {
	broker := NewBroker()
	runID := "test-run-123"

	ch, unsubscribe := broker.Subscribe(runID)
	defer unsubscribe()

	testData := map[string]string{"step": "plan", "status": "running"}
	broker.Broadcast(runID, "step_progress", testData)

	select {
	case msg := <-ch:
		msgStr := string(msg)
		if !strings.Contains(msgStr, "event: step_progress\n") {
			t.Fatalf("expected event header, got %s", msgStr)
		}
		if !strings.Contains(msgStr, `"step":"plan"`) {
			t.Fatalf("expected data payload, got %s", msgStr)
		}
	case <-time.After(1 * time.Second):
		t.Fatalf("timed out waiting for broadcast message")
	}
}
