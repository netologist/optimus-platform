package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/optimus/projects/platform/internal/bootstrap"
	"github.com/optimus/projects/platform/internal/infra/outbox"
	"github.com/optimus/projects/platform/internal/telemetry"
)

const (
	pollInterval = 2 * time.Second
	batchSize    = 50
)

func main() {
	log.Println("Starting Optimus Transactional Outbox Relay...")

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	tp, err := telemetry.InitTracer(ctx, "outbox-relay")
	if err == nil && tp != nil {
		defer func() {
			shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer shutdownCancel()
			_ = tp.Shutdown(shutdownCtx)
		}()
	}

	storage, closeStorage := bootstrap.StorageFromEnv(ctx)
	defer closeStorage()

	publisher, closePublisher := buildPublisher(ctx)
	defer closePublisher()

	relay := outbox.NewRelay(storage, publisher, batchSize)
	go relay.Start(ctx, pollInterval)

	<-ctx.Done()
	log.Println("Outbox Relay shutdown cleanly.")
}

// buildPublisher returns a Redpanda publisher when brokers are configured, and an
// in-memory publisher otherwise. The fallback keeps unit tests and offline runs
// working, but it does not deliver events anywhere — hence the explicit warning.
func buildPublisher(ctx context.Context) (outbox.Publisher, func()) {
	brokers := parseBrokers(os.Getenv("REDPANDA_BROKERS"))
	if len(brokers) == 0 {
		log.Println("WARN: REDPANDA_BROKERS not set — domain events will be collected in memory only")
		return &outbox.MemoryPublisher{}, func() {}
	}

	publisher, err := outbox.NewRedpandaPublisher(brokers)
	if err != nil {
		log.Fatalf("Unable to create Redpanda publisher: %v", err)
	}

	if err := publisher.Ping(ctx); err != nil {
		// Not fatal: Redpanda may still be starting up. The relay retries every batch,
		// so unreachable brokers surface as repeated publish errors rather than a crash loop.
		log.Printf("WARN: Redpanda brokers %v not reachable yet (%v) — will retry per batch", brokers, err)
	} else {
		log.Printf("INFO: Publishing domain events to Redpanda brokers %v", brokers)
	}

	return publisher, publisher.Close
}

// parseBrokers splits a comma-separated broker list, tolerating surrounding whitespace.
func parseBrokers(raw string) []string {
	var brokers []string
	for _, b := range strings.Split(raw, ",") {
		if b = strings.TrimSpace(b); b != "" {
			brokers = append(brokers, b)
		}
	}
	return brokers
}
