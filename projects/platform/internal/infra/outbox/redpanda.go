package outbox

import (
	"context"
	"fmt"
	"time"

	"github.com/twmb/franz-go/pkg/kgo"
)

// TraceparentHeader is the W3C trace context header attached to every published record,
// so consumers can continue the trace that produced the event (ADR-019).
const TraceparentHeader = "traceparent"

// RedpandaPublisher publishes domain events to Redpanda, which speaks the Kafka wire
// protocol, so the record layout is plain Kafka: key = correlation ID (keeps per-entity
// ordering), value = the event payload, traceparent carried as a record header.
type RedpandaPublisher struct {
	client *kgo.Client
}

// NewRedpandaPublisher connects to the given brokers and enables topic auto-creation,
// which keeps the local Kind environment free of manual topic provisioning. In a real
// deployment topics are created ahead of time and auto-creation would be disabled.
func NewRedpandaPublisher(brokers []string) (*RedpandaPublisher, error) {
	if len(brokers) == 0 {
		return nil, fmt.Errorf("no Redpanda brokers configured")
	}

	client, err := kgo.NewClient(
		kgo.SeedBrokers(brokers...),
		kgo.RequiredAcks(kgo.AllISRAcks()),
		kgo.ProducerBatchCompression(kgo.SnappyCompression()),
		kgo.AllowAutoTopicCreation(),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create Redpanda client: %w", err)
	}

	return &RedpandaPublisher{client: client}, nil
}

// Publish sends a single record and blocks until the broker acknowledges it, so the
// caller only marks the outbox row published once the event is durably accepted.
func (p *RedpandaPublisher) Publish(ctx context.Context, topic string, key string, payload []byte, traceparent string) error {
	record := &kgo.Record{
		Topic: topic,
		Key:   []byte(key),
		Value: payload,
	}

	if traceparent != "" {
		record.Headers = append(record.Headers, kgo.RecordHeader{
			Key:   TraceparentHeader,
			Value: []byte(traceparent),
		})
	}

	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	if err := p.client.ProduceSync(ctx, record).FirstErr(); err != nil {
		return fmt.Errorf("failed to produce to topic %s: %w", topic, err)
	}
	return nil
}

// Ping verifies broker reachability, used by the relay as a startup readiness gate.
func (p *RedpandaPublisher) Ping(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	return p.client.Ping(ctx)
}

// Close flushes buffered records and releases the client.
func (p *RedpandaPublisher) Close() {
	p.client.Close()
}
