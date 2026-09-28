package helpers

import (
	"context"
	"errors"
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
	// Timestamp is the producer's clock as recorded by the broker, so the order the relay
	// published in is observable independently of when this process read the record.
	Timestamp time.Time
}

// Header returns the value of a record header, or "" when absent.
func (e *Event) Header(key string) string {
	return e.Headers[key]
}

// KafkaClient consumes the domain events the outbox relay publishes. Asserting on these
// records rather than on the API's intent is what tells a delivered event from a promised
// one.
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

// WaitForEvents consumes topic from the beginning and returns every record carrying the
// given key. It returns once the first match has arrived and no further match has appeared
// for settle, so a redelivery lands in the result instead of being missed.
//
// Broker reachability is proven before the wait begins: a port-forward that never came up
// must say so, rather than surface as a missing event once the timeout expires.
func (k *KafkaClient) WaitForEvents(ctx context.Context, topic, key string, timeout, settle time.Duration) ([]Event, error) {
	consumer, err := kgo.NewClient(
		kgo.SeedBrokers(k.brokers...),
		kgo.ConsumeTopics(topic),
		kgo.ConsumeResetOffset(kgo.NewOffset().AtStart()),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create Kafka consumer: %w", err)
	}
	defer consumer.Close()

	pingCtx, cancelPing := context.WithTimeout(ctx, 5*time.Second)
	err = consumer.Ping(pingCtx)
	cancelPing()
	if err != nil {
		return nil, fmt.Errorf("cannot reach Redpanda at %v: %w", k.brokers, err)
	}

	deadline := time.Now().Add(timeout)
	lastArrival := time.Now()
	var matches []Event
	var lastBrokerErr error

	for time.Now().Before(deadline) {
		pollCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
		fetches := consumer.PollFetches(pollCtx)
		cancel()

		for _, fetchErr := range fetches.Errors() {
			// An empty poll ends on our own deadline; that is not a broker fault, and
			// reporting it as one would hide the faults that are.
			if !errors.Is(fetchErr.Err, context.DeadlineExceeded) && !errors.Is(fetchErr.Err, context.Canceled) {
				lastBrokerErr = fetchErr.Err
			}
		}

		fetches.EachRecord(func(r *kgo.Record) {
			if string(r.Key) != key {
				return
			}
			headers := make(map[string]string, len(r.Headers))
			for _, h := range r.Headers {
				headers[h.Key] = string(h.Value)
			}
			matches = append(matches, Event{
				Topic:     r.Topic,
				Key:       string(r.Key),
				Value:     r.Value,
				Headers:   headers,
				Partition: r.Partition,
				Offset:    r.Offset,
				Timestamp: r.Timestamp,
			})
			lastArrival = time.Now()
		})

		if len(matches) > 0 && time.Since(lastArrival) >= settle {
			return matches, nil
		}
	}

	if len(matches) > 0 {
		return matches, nil
	}
	if lastBrokerErr != nil {
		return nil, fmt.Errorf("no event with key %q on topic %s within %s (broker error: %w)", key, topic, timeout, lastBrokerErr)
	}
	return nil, fmt.Errorf("no event with key %q on topic %s within %s", key, topic, timeout)
}
