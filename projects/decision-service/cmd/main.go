package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/optimus/projects/decision-service/internal/audit"
	"github.com/optimus/projects/decision-service/internal/decision"
	internalhttp "github.com/optimus/projects/decision-service/internal/http"
	"github.com/optimus/projects/decision-service/internal/policy"
	"github.com/optimus/projects/decision-service/internal/systemone"
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

	client := systemone.NewClient(ollayaURL)
	engine := policy.NewEngine()
	auditStore := audit.NewMemoryStore()

	svc := decision.NewService(client, engine, auditStore, modelName)
	handler := internalhttp.NewHandler(svc)

	addr := fmt.Sprintf(":%d", *port)
	log.Printf("Starting Decision Service on %s (Ollaya: %s, Model: %s)", addr, ollayaURL, modelName)
	if err := http.ListenAndServe(addr, handler); err != nil {
		log.Fatalf("Decision Service failed: %v", err)
	}
}
