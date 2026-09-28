// Package bootstrap wires infrastructure adapters from environment configuration.
// The cmd/ entrypoints share it so the fallback behaviour cannot drift between the
// API, the Temporal worker and the outbox relay.
package bootstrap

import (
	"context"
	"log"
	"os"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/optimus/projects/platform/internal/app"
	"github.com/optimus/projects/platform/internal/infra/postgres"
)

// StorageFromEnv returns a PostgreSQL-backed Storage when DATABASE_URL is set and the
// server is reachable, and an in-memory Storage otherwise. The fallback keeps unit tests
// and offline development working; it is deliberately loud in the log because it means
// nothing written will survive the process.
//
// The returned function releases the connection pool and must be called on shutdown;
// it is a no-op when the in-memory fallback is used.
func StorageFromEnv(ctx context.Context) (app.Storage, func()) {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		log.Println("INFO: DATABASE_URL not set, running with in-memory storage")
		return app.NewMemoryStorage(), func() {}
	}

	poolCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	pool, err := pgxpool.New(poolCtx, dbURL)
	if err != nil {
		log.Printf("WARN: Failed to initialize Postgres pool (%v), falling back to MemoryStorage", err)
		return app.NewMemoryStorage(), func() {}
	}

	if err := pool.Ping(poolCtx); err != nil {
		log.Printf("WARN: Failed to ping Postgres (%v), falling back to MemoryStorage", err)
		pool.Close()
		return app.NewMemoryStorage(), func() {}
	}

	log.Println("INFO: Connected to PostgreSQL with RLS support")
	return postgres.NewStorage(pool), pool.Close
}
