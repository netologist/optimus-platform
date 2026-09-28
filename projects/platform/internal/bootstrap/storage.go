// Package bootstrap wires infrastructure adapters from environment configuration.
// The cmd/ entrypoints share it so the fallback behaviour cannot drift between the
// API, the Temporal worker and the outbox relay.
package bootstrap

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/optimus/projects/platform/internal/app"
	"github.com/optimus/projects/platform/internal/infra/postgres"
)

const (
	connectAttempts = 10
	connectBackoff  = 2 * time.Second
)

// StorageFromEnv returns a PostgreSQL-backed Storage when DATABASE_URL is set.
//
// It returns an error when DATABASE_URL is configured but unreachable, rather than
// silently degrading to memory storage. A relay that falls back keeps running while
// doing nothing at all — no events dispatched, no rows claimed — which is far harder
// to notice than a crash loop. Callers that genuinely tolerate memory storage (the API
// during local development) fall back explicitly at the call site.
//
// When DATABASE_URL is unset the in-memory storage is returned with a nil error, since
// that is an explicit choice rather than a failure.
func StorageFromEnv(ctx context.Context) (app.Storage, func(), error) {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		log.Println("INFO: DATABASE_URL not set, running with in-memory storage")
		return app.NewMemoryStorage(), func() {}, nil
	}

	pool, err := connect(ctx, dbURL)
	if err != nil {
		return nil, func() {}, err
	}

	log.Println("INFO: Connected to PostgreSQL with RLS support")
	return postgres.NewStorage(pool), pool.Close, nil
}

// connect retries, because the relay and the worker are routinely scheduled before
// PostgreSQL has finished becoming ready — a single refused connection at startup must
// not decide the process's configuration for the rest of its life.
func connect(ctx context.Context, dbURL string) (*pgxpool.Pool, error) {
	var lastErr error

	for attempt := 1; attempt <= connectAttempts; attempt++ {
		pool, err := pgxpool.New(ctx, dbURL)
		if err == nil {
			pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
			err = pool.Ping(pingCtx)
			cancel()

			if err == nil {
				return pool, nil
			}
			pool.Close()
		}

		lastErr = err
		if ctx.Err() != nil || attempt == connectAttempts {
			break
		}

		log.Printf("WARN: PostgreSQL not ready (attempt %d/%d): %v", attempt, connectAttempts, err)
		select {
		case <-ctx.Done():
		case <-time.After(connectBackoff):
		}
	}

	return nil, fmt.Errorf("unable to connect to PostgreSQL after %d attempts: %w", connectAttempts, lastErr)
}
