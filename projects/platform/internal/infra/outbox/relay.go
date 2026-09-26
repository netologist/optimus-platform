package outbox

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/optimus/projects/platform/internal/app"
	"github.com/optimus/projects/platform/internal/domain"
)

// Publisher interface for publishing domain events to Redpanda / Kafka
type Publisher interface {
	Publish(ctx context.Context, topic string, key string, payload []byte, traceparent string) error
}

// MemoryPublisher collects published events for testing
type MemoryPublisher struct {
	Published []domain.OutboxMessage
}

func (m *MemoryPublisher) Publish(ctx context.Context, topic string, key string, payload []byte, traceparent string) error {
	m.Published = append(m.Published, domain.OutboxMessage{
		EventType:     topic,
		CorrelationID: key,
		Traceparent:   traceparent,
		Payload:       payload,
	})
	return nil
}

// Relay handles polling outbox table and dispatching to publisher
type Relay struct {
	storage   app.Storage
	publisher Publisher
	batchSize int
}

func NewRelay(storage app.Storage, publisher Publisher, batchSize int) *Relay {
	return &Relay{
		storage:   storage,
		publisher: publisher,
		batchSize: batchSize,
	}
}

// RunOnce processes a single batch of unpublished messages
func (r *Relay) RunOnce(ctx context.Context) (int, error) {
	msgs, err := r.storage.GetUnpublishedOutboxMessages(ctx, r.batchSize)
	if err != nil {
		return 0, fmt.Errorf("failed to fetch outbox messages: %w", err)
	}

	for _, msg := range msgs {
		topic := fmt.Sprintf("events.%s", msg.EventType)
		if err := r.publisher.Publish(ctx, topic, msg.CorrelationID, msg.Payload, msg.Traceparent); err != nil {
			return 0, fmt.Errorf("failed to publish outbox message %d: %w", msg.ID, err)
		}
		if err := r.storage.MarkOutboxMessagePublished(ctx, msg.ID); err != nil {
			return 0, fmt.Errorf("failed to mark outbox message %d published: %w", msg.ID, err)
		}
	}

	return len(msgs), nil
}

// Start begins continuous background outbox relay
func (r *Relay) Start(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			count, err := r.RunOnce(ctx)
			if err != nil {
				log.Printf("[outbox-relay] error processing batch: %v", err)
			} else if count > 0 {
				log.Printf("[outbox-relay] dispatched %d domain event(s)", count)
			}
		}
	}
}
