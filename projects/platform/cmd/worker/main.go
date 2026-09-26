package main

import (
	"log"
	"os"

	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"

	"github.com/optimus/projects/platform/internal/workflow"
)

func main() {
	temporalHost := os.Getenv("TEMPORAL_HOST")
	if temporalHost == "" {
		temporalHost = "127.0.0.1:7233"
	}

	c, err := client.Dial(client.Options{
		HostPort: temporalHost,
	})
	if err != nil {
		log.Fatalf("Unable to create Temporal client: %v", err)
	}
	defer c.Close()

	w := worker.New(c, "optimus-task-queue", worker.Options{})

	acts := workflow.NewActivities(
		os.Getenv("AI_RUNTIME_URL"),
		os.Getenv("DECISION_SERVICE_URL"),
		os.Getenv("MCP_SERVER_URL"),
	)

	w.RegisterWorkflow(workflow.AssetFailureWorkflow)
	w.RegisterActivity(acts.InvestigateFailure)
	w.RegisterActivity(acts.RunDecision)
	w.RegisterActivity(acts.ReserveSparePart)
	w.RegisterActivity(acts.ReleaseSparePartReservation)
	w.RegisterActivity(acts.CreateFieldWorkOrder)

	log.Printf("Starting Optimus Temporal Worker on task queue 'optimus-task-queue' (host: %s)", temporalHost)
	if err := w.Run(worker.InterruptCh()); err != nil {
		log.Fatalf("Unable to start worker: %v", err)
	}
}
