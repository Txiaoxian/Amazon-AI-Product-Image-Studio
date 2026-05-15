package sse

import (
	"context"
	"testing"
	"time"

	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/database"
)

func TestBrokerFanoutAndUnsubscribeCleanup(t *testing.T) {
	broker := NewBroker(1)
	ctx, cancel := context.WithCancel(context.Background())
	events, unsubscribe := broker.Subscribe(ctx)
	if broker.SubscriberCount() != 1 {
		t.Fatalf("subscriber count = %d, want 1", broker.SubscriberCount())
	}

	broker.PublishTaskEvent(context.Background(), database.TaskEvent{Sequence: 7, ID: "evt_00000000000000000007"})
	select {
	case event := <-events:
		if event.Sequence != 7 {
			t.Fatalf("fanout sequence = %d, want 7", event.Sequence)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for fanout event")
	}

	unsubscribe()
	cancel()
	if broker.SubscriberCount() != 0 {
		t.Fatalf("subscriber count after unsubscribe = %d, want 0", broker.SubscriberCount())
	}
}
