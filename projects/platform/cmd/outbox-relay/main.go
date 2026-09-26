package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/optimus/projects/platform/internal/app"
	"github.com/optimus/projects/platform/internal/infra/outbox"
	"github.com/optimus/projects/platform/internal/infra/postgres"
)

func main() {
	log.Println("Starting Optimus Transactional Outbox Relay...")

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	var storage app.Storage
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL != "" {
		poolCtx, poolCancel := context.WithTimeout(ctx, 10*time.Second)
		pool, err := pgxpool.New(poolCtx, dbURL)
		poolCancel()

		if err != nil {
			log.Printf("WARN: Failed to initialize Postgres pool (%v), falling back to MemoryStorage", err)
			storage = app.NewMemoryStorage()
		} else if err := pool.Ping(ctx); err != nil {
			log.Printf("WARN: Failed to ping Postgres (%v), falling back to MemoryStorage", err)
			pool.Close()
			storage = app.NewMemoryStorage()
		} else {
			log.Printf("INFO: Outbox Relay connected to PostgreSQL")
			storage = postgres.NewStorage(pool)
			defer pool.Close()
		}
	} else {
		log.Println("INFO: DATABASE_URL not set, running Outbox Relay with in-memory storage")
		storage = app.NewMemoryStorage()
	}

	publisher := &outbox.MemoryPublisher{}
	relay := outbox.NewRelay(storage, publisher, 50)

	go relay.Start(ctx, 2*time.Second)

	<-ctx.Done()
	log.Println("Outbox Relay shutdown cleanly.")
}
