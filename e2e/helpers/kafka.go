package helpers

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/twmb/franz-go/pkg/kgo"
)

// Event is one domain event consumed from Redpanda.
type Event struct {
	Topic     string
	Key       string
	Value     []byte
	Headers   map[string]string
	Partition int32
	Offset    int64
}

// Header returns the value of a record header, or "" when absent.
func (e *Event) Header(key string) string {
	return e.Headers[key]
}

// KafkaClient consumes the domain events the outbox relay publishes. It replaces the
// previous practice of asserting only on MCP call logs, which could not tell whether an
// event ever left the platform.
type KafkaClient struct {
	brokers []string
}

// NewKafkaClient reads brokers from KAFKA_BROKERS, defaulting to the port-forward the
// live E2E script establishes.
func NewKafkaClient() *KafkaClient {
	raw := os.Getenv("KAFKA_BROKERS")
	if raw == "" {
		raw = "127.0.0.1:19092"
	}

	var brokers []string
	for _, b := range strings.Split(raw, ",") {
		if b = strings.TrimSpace(b); b != "" {
			brokers = append(brokers, b)
		}
	}
	return &KafkaClient{brokers: brokers}
}

// WaitForEvent consumes topic from the beginning until a record with the given key
// appears, or the timeout elapses. Reading from the start keeps the assertion independent
// of how quickly the relay published relative to when the test began consuming.
func (k *KafkaClient) WaitForEvent(ctx context.Context, topic, key string, timeout time.Duration) (*Event, error) {
	consumer, err := kgo.NewClient(
		kgo.SeedBrokers(k.brokers...),
		kgo.ConsumeTopics(topic),
		kgo.ConsumeResetOffset(kgo.NewOffset().AtStart()),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create Kafka consumer: %w", err)
	}
	defer consumer.Close()

	deadline := time.Now().Add(timeout)
	var lastErr error

	for time.Now().Before(deadline) {
		pollCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
		fetches := consumer.PollFetches(pollCtx)
		cancel()

		if errs := fetches.Errors(); len(errs) > 0 {
			lastErr = errs[0].Err
		}

		var match *Event
		fetches.EachRecord(func(r *kgo.Record) {
			if match != nil || string(r.Key) != key {
				return
			}
			headers := make(map[string]string, len(r.Headers))
			for _, h := range r.Headers {
				headers[h.Key] = string(h.Value)
			}
			match = &Event{
				Topic:     r.Topic,
				Key:       string(r.Key),
				Value:     r.Value,
				Headers:   headers,
				Partition: r.Partition,
				Offset:    r.Offset,
			}
		})

		if match != nil {
			return match, nil
		}
	}

	if lastErr != nil {
		return nil, fmt.Errorf("no event with key %q on topic %s within %s (last error: %w)", key, topic, timeout, lastErr)
	}
	return nil, fmt.Errorf("no event with key %q on topic %s within %s", key, topic, timeout)
}
