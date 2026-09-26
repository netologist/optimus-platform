package outbox_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/optimus/projects/platform/internal/app"
	"github.com/optimus/projects/platform/internal/domain"
	"github.com/optimus/projects/platform/internal/infra/outbox"
)

func TestOutboxRelayDispatchesAndMarksPublished(t *testing.T) {
	storage := app.NewMemoryStorage()
	publisher := &outbox.MemoryPublisher{}
	relay := outbox.NewRelay(storage, publisher, 10)

	// Add unpublished message
	payload, _ := json.Marshal(map[string]string{"asset_id": "P-104"})
	_ = storage.SaveOutboxMessage(context.Background(), &domain.OutboxMessage{
		TenantID:      "acme",
		EventType:     "work_order.created",
		CorrelationID: "wf-104",
		Traceparent:   "00-trace-01",
		Payload:       payload,
		CreatedAt:     time.Now().UTC(),
	})

	// Run relay batch
	count, err := relay.RunOnce(context.Background())
	if err != nil {
		t.Fatalf("unexpected relay error: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected 1 message dispatched, got %d", count)
	}

	// Verify published
	if len(publisher.Published) != 1 {
		t.Fatalf("expected 1 published event, got %d", len(publisher.Published))
	}
	if publisher.Published[0].EventType != "events.work_order.created" {
		t.Errorf("expected topic events.work_order.created, got %s", publisher.Published[0].EventType)
	}

	// Verify no remaining unpublished messages
	remaining, err := storage.GetUnpublishedOutboxMessages(context.Background(), 10)
	if err != nil {
		t.Fatalf("failed to query outbox: %v", err)
	}
	if len(remaining) != 0 {
		t.Errorf("expected 0 remaining unpublished messages, got %d", len(remaining))
	}
}
