package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/optimus/projects/decision-service/internal/audit"
	"github.com/optimus/projects/decision-service/internal/decision"
	internalhttp "github.com/optimus/projects/decision-service/internal/http"
	"github.com/optimus/projects/decision-service/internal/policy"
	"github.com/optimus/projects/decision-service/internal/systemone"
	"github.com/optimus/projects/decision-service/internal/telemetry"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func main() {
	port := flag.Int("port", 8082, "Decision service port")
	flag.Parse()

	ollayaURL := os.Getenv("OLLAYA_URL")
	if ollayaURL == "" {
		ollayaURL = "http://127.0.0.1:11435"
	}

	modelName := os.Getenv("DECISION_MODEL")
	if modelName == "" {
		modelName = "laya"
	}
	// 0. Initialize OpenTelemetry Tracer
	tp, err := telemetry.InitTracer(context.Background(), "decision-service")
	if err == nil && tp != nil {
		defer func() {
			shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			_ = tp.Shutdown(shutdownCtx)
		}()
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// 1. Storage setup: PostgreSQL with RLS or in-memory fallback
	var auditStore audit.Store
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL != "" {
		pool, err := pgxpool.New(ctx, dbURL)
		if err != nil {
			log.Printf("WARN: Failed to initialize Postgres pool (%v), falling back to MemoryStore", err)
			auditStore = audit.NewMemoryStore()
		} else if err := pool.Ping(ctx); err != nil {
			log.Printf("WARN: Failed to ping Postgres at %s (%v), falling back to MemoryStore", dbURL, err)
			pool.Close()
			auditStore = audit.NewMemoryStore()
		} else {
			log.Printf("INFO: Connected to PostgreSQL decision_audit table with RLS support")
			auditStore = audit.NewPostgresStore(pool)
			defer pool.Close()
		}
	} else {
		log.Println("INFO: DATABASE_URL not set, using MemoryStore for decision audit")
		auditStore = audit.NewMemoryStore()
	}

	// 2. Ollaya SystemOne client & Policy Engine
	client := systemone.NewClient(ollayaURL)
	engine := policy.NewEngine()

	svc := decision.NewService(client, engine, auditStore, modelName)
	handler := internalhttp.NewHandler(svc)

	mainMux := http.NewServeMux()
	mainMux.Handle("/metrics", promhttp.Handler())
	mainMux.Handle("/", handler)

	addr := fmt.Sprintf(":%d", *port)
	log.Printf("Starting Decision Service on %s (Ollaya endpoint: %s, Default Model: %s)", addr, ollayaURL, modelName)
	if err := http.ListenAndServe(addr, mainMux); err != nil {
		log.Fatalf("Decision Service failed: %v", err)
	}
}
