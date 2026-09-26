package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"

	"github.com/optimus/projects/integration-mocks/internal/eam"
	"github.com/optimus/projects/integration-mocks/internal/erp"
	"github.com/optimus/projects/integration-mocks/internal/fsm"
	"github.com/optimus/projects/integration-mocks/internal/mcpserver"
	"github.com/optimus/projects/integration-mocks/internal/oi"
	"github.com/optimus/projects/integration-mocks/internal/plm"
)

func main() {
	system := flag.String("system", "all", "System to mock (eam, plm, erp, fsm, oi, or all)")
	port := flag.Int("port", 8080, "HTTP server port")
	flag.Parse()

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

	addr := fmt.Sprintf(":%d", *port)
	log.Printf("Starting mock MCP server for [%s] on %s", *system, addr)
	if err := http.ListenAndServe(addr, srv); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
