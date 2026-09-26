package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/optimus/projects/integration-mocks/internal/eam"
	"github.com/optimus/projects/integration-mocks/internal/erp"
	"github.com/optimus/projects/integration-mocks/internal/fsm"
	"github.com/optimus/projects/integration-mocks/internal/mcpserver"
	"github.com/optimus/projects/integration-mocks/internal/oi"
	"github.com/optimus/projects/integration-mocks/internal/plm"
	"github.com/optimus/projects/integration-mocks/internal/telemetry"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func main() {
	system := flag.String("system", "all", "System to mock (eam, plm, erp, fsm, oi, or all)")
	port := flag.Int("port", 8080, "HTTP server port")
	flag.Parse()

	// Initialize OpenTelemetry Tracer for Integration Mocks
	tp, err := telemetry.InitTracer(context.Background(), "integration-mocks")
	if err == nil && tp != nil {
		defer func() {
			shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			_ = tp.Shutdown(shutdownCtx)
		}()
	}

	srv := mcpserver.New(*system)
	switch *system {
	case "eam":
		eam.Register(srv)
	case "plm":
		plm.Register(srv)
	case "erp":
		erp.Register(srv)
	case "fsm":
		fsm.Register(srv)
	case "oi":
		oi.Register(srv)
	case "all":
		eam.Register(srv)
		plm.Register(srv)
		erp.Register(srv)
		fsm.Register(srv)
		oi.Register(srv)
	default:
		log.Fatalf("Unknown system type: %s", *system)
	}

	mainMux := http.NewServeMux()
	mainMux.Handle("/metrics", promhttp.Handler())
	mainMux.Handle("/", srv)

	addr := fmt.Sprintf(":%d", *port)
	log.Printf("Starting mock MCP server for [%s] on %s", *system, addr)
	if err := http.ListenAndServe(addr, mainMux); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
