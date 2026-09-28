package bootstrap

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"go.temporal.io/sdk/client"
)

// A failing dialer is what a Temporal frontend that has not finished starting looks
// like, and the reason the API must not settle for a no-op client on the first refusal.
func TestConnectTemporalRetriesUntilTheFrontendAccepts(t *testing.T) {
	attempts := 0
	dial := func(string) (client.Client, error) {
		attempts++
		if attempts < 3 {
			return nil, errors.New("connection refused")
		}
		return nil, nil
	}

	wfClient, err := connectTemporal(context.Background(), "temporal:7233", 5, time.Millisecond, dial)
	if err != nil {
		t.Fatalf("expected the third attempt to succeed, got %v", err)
	}
	if wfClient == nil {
		t.Fatal("expected a workflow client")
	}
	if attempts != 3 {
		t.Fatalf("expected 3 dial attempts, got %d", attempts)
	}
}

// The failure mode this guards: a client that silently does nothing still answers every
// ingest and approval with success, so an unreachable Temporal must surface as an error.
func TestConnectTemporalFailsWhenTheFrontendNeverAccepts(t *testing.T) {
	dial := func(string) (client.Client, error) {
		return nil, errors.New("connection refused")
	}

	wfClient, err := connectTemporal(context.Background(), "temporal:7233", 2, time.Millisecond, dial)
	if err == nil {
		t.Fatal("expected an error when every dial attempt fails")
	}
	if wfClient != nil {
		t.Fatalf("expected no workflow client, got %v", wfClient)
	}
	for _, want := range []string{"temporal:7233", "connection refused"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("expected error %q to mention %q", err.Error(), want)
		}
	}
}

func TestConnectTemporalStopsRetryingWhenTheContextIsCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	attempts := 0
	dial := func(string) (client.Client, error) {
		attempts++
		return nil, errors.New("connection refused")
	}

	if _, err := connectTemporal(ctx, "temporal:7233", 30, time.Second, dial); err == nil {
		t.Fatal("expected an error from a cancelled start-up")
	}
	if attempts != 1 {
		t.Fatalf("expected the retry loop to stop after the first attempt, got %d", attempts)
	}
}

// An unset TEMPORAL_HOST is the explicit local run — the only case a no-op client is
// acceptable — and must not be reached through the configured-but-unreachable path.
func TestWorkflowClientFromEnvWithoutHostRunsWithoutTemporal(t *testing.T) {
	t.Setenv("TEMPORAL_HOST", "")

	wfClient, closeClient, err := WorkflowClientFromEnv(context.Background())
	if err != nil {
		t.Fatalf("expected no error without TEMPORAL_HOST, got %v", err)
	}
	defer closeClient()

	ctx := context.Background()
	if err := wfClient.StartAssetFailureWorkflow(ctx, "wf-1", "acme", "P-104", "overheating", "00-trace"); err != nil {
		t.Fatalf("expected no-op start to succeed, got %v", err)
	}
	if err := wfClient.SignalApproval(ctx, "wf-1", true, "j.smith"); err != nil {
		t.Fatalf("expected no-op signal to succeed, got %v", err)
	}
}
