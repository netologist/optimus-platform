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

	"github.com/optimus/projects/platform/internal/app"
	"github.com/optimus/projects/platform/internal/bootstrap"
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

	// Startup budget covers the dependency retries below: the database, then Temporal,
	// which is routinely the last dependency to become ready.
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
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
	storage, closeStorage, err := bootstrap.StorageFromEnv(ctx)
	if err != nil {
		// StorageFromEnv only fails when DATABASE_URL is set but unreachable, and serving
		// from memory then answers the API's own read endpoints — the audit trail among
		// them — out of a store no other process can see. Exiting lets Kubernetes restart
		// the pod once the database accepts connections.
		log.Fatalf("Unable to connect to PostgreSQL: %v", err)
	}
	defer closeStorage()

	// 2. Initialize Temporal Workflow Client
	wfClient, closeWorkflowClient, err := bootstrap.WorkflowClientFromEnv(ctx)
	if err != nil {
		// A no-op workflow client still answers every ingest and approval with success
		// while nothing durable happens, so an unreachable Temporal must not be survived.
		log.Fatalf("Unable to connect to Temporal: %v", err)
	}
	defer closeWorkflowClient()

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
