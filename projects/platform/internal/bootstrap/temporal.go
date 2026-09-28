package bootstrap

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"go.temporal.io/sdk/client"

	"github.com/optimus/projects/platform/internal/app"
	"github.com/optimus/projects/platform/internal/infra/temporal"
)

const (
	// Temporal is normally the last dependency to become ready: it runs its own
	// migrations and history service before it accepts a frontend connection, while the
	// API pod is scheduled at the same time as everything else. The retry budget is
	// therefore wider than the database's.
	temporalConnectAttempts = 30
	temporalConnectBackoff  = 2 * time.Second
)

// Dialer connects to a Temporal frontend. It is a parameter rather than a direct call so
// the retry behaviour can be exercised without a live cluster.
type Dialer func(hostPort string) (client.Client, error)

func defaultDialer(hostPort string) (client.Client, error) {
	return client.Dial(client.Options{HostPort: hostPort})
}

// WorkflowClientFromEnv returns a Temporal-backed workflow client when TEMPORAL_HOST is
// set, and a no-op client when it is deliberately unset (local runs without a cluster).
//
// A configured-but-unreachable Temporal is an error, never a no-op client: the signal
// and approval endpoints report the same success whether the workflow client did the
// work or did nothing at all, so falling back turns the entire durable pipeline into a
// silent no-op whose only symptom is a workflow that never runs. Callers that fail hard
// on the returned error let Kubernetes restart them instead.
func WorkflowClientFromEnv(ctx context.Context) (app.WorkflowClient, func(), error) {
	hostPort := os.Getenv("TEMPORAL_HOST")
	if hostPort == "" {
		log.Println("INFO: TEMPORAL_HOST not set, workflows are disabled (NoopClient)")
		return temporal.NewNoopClient(), func() {}, nil
	}

	wfClient, err := connectTemporal(ctx, hostPort, temporalConnectAttempts, temporalConnectBackoff, defaultDialer)
	if err != nil {
		return nil, func() {}, err
	}

	log.Printf("INFO: Connected to Temporal cluster at %s", hostPort)
	return wfClient, wfClient.Close, nil
}

// connectTemporal dials with retries, because a single refused connection at startup
// must not decide the process's configuration for the rest of its life.
func connectTemporal(ctx context.Context, hostPort string, attempts int, backoff time.Duration, dial Dialer) (*temporal.Client, error) {
	var lastErr error

	for attempt := 1; attempt <= attempts; attempt++ {
		c, err := dial(hostPort)
		if err == nil {
			return temporal.NewClient(c), nil
		}

		lastErr = err
		if ctx.Err() != nil || attempt == attempts {
			break
		}

		log.Printf("WARN: Temporal not ready (attempt %d/%d): %v", attempt, attempts, err)
		select {
		case <-ctx.Done():
		case <-time.After(backoff):
		}
	}

	return nil, fmt.Errorf("unable to connect to Temporal at %s after %d attempts: %w", hostPort, attempts, lastErr)
}
