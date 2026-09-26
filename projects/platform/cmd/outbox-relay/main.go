package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/optimus/projects/platform/internal/app"
	"github.com/optimus/projects/platform/internal/infra/outbox"
)

func main() {
	log.Println("Starting Optimus Transactional Outbox Relay...")

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	storage := app.NewMemoryStorage()
	publisher := &outbox.MemoryPublisher{}
	relay := outbox.NewRelay(storage, publisher, 50)

	go relay.Start(ctx, 2*time.Second)

	<-ctx.Done()
	log.Println("Outbox Relay shutdown cleanly.")
}
