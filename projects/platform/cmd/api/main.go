package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"

	"github.com/optimus/projects/platform/internal/app"
	"github.com/optimus/projects/platform/internal/transport"
)

func main() {
	port := flag.Int("port", 8080, "HTTP server port")
	flag.Parse()

	storage := app.NewMemoryStorage()
	svc := app.NewService(storage)
	handler := transport.NewHandler(svc)

	addr := fmt.Sprintf(":%d", *port)
	log.Printf("Starting Optimus Platform API on %s", addr)
	if err := http.ListenAndServe(addr, handler); err != nil {
		log.Fatalf("Server exited: %v", err)
	}
}
