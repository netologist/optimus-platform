package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"go.temporal.io/sdk/client"

	"github.com/optimus/projects/platform/internal/app"
	"github.com/optimus/projects/platform/internal/infra/postgres"
	"github.com/optimus/projects/platform/internal/infra/temporal"
	"github.com/optimus/projects/platform/internal/telemetry"
	"github.com/optimus/projects/platform/internal/transport"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)
func main() {
	defaultPort := 8080
	if envPort := os.Getenv("PORT"); envPort != "" {
		if p, err := strconv.Atoi(envPort); err == nil {
			defaultPort = p
		}
	}

	port := flag.Int("port", defaultPort, "HTTP server port")
	flag.Parse()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// 0. Initialize OpenTelemetry Tracer
	tp, err := telemetry.InitTracer(context.Background(), "platform")
	if err == nil && tp != nil {
		defer func() {
			shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			_ = tp.Shutdown(shutdownCtx)
		}()
	}
	// 1. Storage setup
	var storage app.Storage
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL != "" {
		pool, err := pgxpool.New(ctx, dbURL)
		if err != nil {
			log.Printf("WARN: Failed to initialize Postgres pool (%v), falling back to MemoryStorage", err)
			storage = app.NewMemoryStorage()
		} else if err := pool.Ping(ctx); err != nil {
			log.Printf("WARN: Failed to ping Postgres at %s (%v), falling back to MemoryStorage", dbURL, err)
			pool.Close()
			storage = app.NewMemoryStorage()
		} else {
			log.Printf("INFO: Connected to PostgreSQL with RLS support")
			storage = postgres.NewStorage(pool)
			defer pool.Close()
		}
	} else {
		log.Println("INFO: DATABASE_URL not set, running with in-memory storage")
		storage = app.NewMemoryStorage()
	}

	// 2. Initialize Temporal Workflow Client
	var wfClient app.WorkflowClient
	temporalHost := os.Getenv("TEMPORAL_HOST")
	if temporalHost != "" {
		tc, err := client.Dial(client.Options{
			HostPort: temporalHost,
		})
		if err != nil {
			log.Printf("WARN: Unable to dial Temporal at %s (%v), falling back to NoopClient", temporalHost, err)
			wfClient = temporal.NewNoopClient()
		} else {
			log.Printf("INFO: Connected to Temporal cluster at %s", temporalHost)
			wfClient = temporal.NewClient(tc)
			defer tc.Close()
		}
	} else {
		log.Println("INFO: TEMPORAL_HOST not set, running with NoopClient")
		wfClient = temporal.NewNoopClient()
	}

	// 3. Assemble Service and Transport
	svc := app.NewService(storage, app.WithWorkflowClient(wfClient))
	handler := transport.NewHandler(svc)

	// Combine Platform API routes with Prometheus metrics endpoint
	mainMux := http.NewServeMux()
	mainMux.Handle("/metrics", promhttp.Handler())
	mainMux.Handle("/", handler)

	addr := fmt.Sprintf(":%d", *port)
	log.Printf("Starting Optimus Platform API on %s (docs available at http://localhost:%d/docs)", addr, *port)
	if err := http.ListenAndServe(addr, mainMux); err != nil {
		log.Fatalf("Server exited: %v", err)
	}
}
