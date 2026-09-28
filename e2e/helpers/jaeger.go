package helpers

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"time"
)

// Span is the subset of a Jaeger span the assertions need.
type Span struct {
	OperationName string
	ServiceName   string
	TraceID       string
	SpanID        string
	Duration      time.Duration
	Tags          map[string]string
}

// Trace is one distributed trace as returned by the Jaeger query API.
type Trace struct {
	TraceID string
	Spans   []Span
}

// SpanNames returns the operation names in the trace, for readable failure output.
func (t *Trace) SpanNames() []string {
	names := make([]string, 0, len(t.Spans))
	for _, s := range t.Spans {
		names = append(names, s.OperationName)
	}
	return names
}

// HasSpan reports whether the trace contains a span with the given operation name.
func (t *Trace) HasSpan(name string) bool {
	for _, s := range t.Spans {
		if s.OperationName == name {
			return true
		}
	}
	return false
}

// HasAllSpans reports whether every named span is present.
func (t *Trace) HasAllSpans(names ...string) bool {
	for _, n := range names {
		if !t.HasSpan(n) {
			return false
		}
	}
	return true
}

// missingSpans returns the required spans the trace does not contain.
func missingSpans(t *Trace, names ...string) []string {
	var missing []string
	for _, n := range names {
		if !t.HasSpan(n) {
			missing = append(missing, n)
		}
	}
	return missing
}

// NewTraceparent builds a fresh W3C traceparent (version 00, sampled).
//
// Tests must not reuse a constant traceparent: Jaeger aggregates spans by trace id, so a
// fixed value merges every run into one trace and makes span assertions meaningless.
func NewTraceparent() string {
	b := make([]byte, 24)
	if _, err := rand.Read(b); err != nil {
		// A predictable trace id is still preferable to failing the test on entropy.
		now := time.Now().UnixNano()
		return fmt.Sprintf("00-%032x-%016x-01", now, now)
	}
	return fmt.Sprintf("00-%x-%x-01", b[:16], b[16:])
}

// JaegerClient queries the Jaeger query API to prove the trace genuinely spans services.
type JaegerClient struct {
	baseURL string
	http    *http.Client
}

// NewJaegerClient reads the query endpoint from JAEGER_URL, defaulting to the
// port-forward the live E2E script establishes.
func NewJaegerClient() *JaegerClient {
	base := os.Getenv("JAEGER_URL")
	if base == "" {
		base = "http://127.0.0.1:16686"
	}
	return &JaegerClient{
		baseURL: base,
		http:    &http.Client{Timeout: 10 * time.Second},
	}
}

// WaitForTrace polls until a trace for the workflow exists AND contains every required
// span. Waiting for the trace alone is not enough: Jaeger indexes spans individually, so
// a trace is routinely visible while its later spans are still in flight.
func (j *JaegerClient) WaitForTrace(ctx context.Context, service, workflowID string, timeout, lookback time.Duration, requiredSpans ...string) (*Trace, error) {
	deadline := time.Now().Add(timeout)
	var lastTrace *Trace
	var lastErr error

	for time.Now().Before(deadline) {
		trace, err := j.FindTraceByWorkflowID(ctx, service, workflowID, lookback)
		if err != nil {
			lastErr = err
		} else {
			lastTrace = trace
			if trace.HasAllSpans(requiredSpans...) {
				return trace, nil
			}
			lastErr = fmt.Errorf("trace %s is missing spans %v; has %v",
				trace.TraceID, missingSpans(trace, requiredSpans...), trace.SpanNames())
		}

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(2 * time.Second):
		}
	}

	if lastTrace != nil {
		return lastTrace, lastErr
	}
	return nil, lastErr
}

// FindTraceByWorkflowID returns the trace whose root carries the given workflow_id tag.
//
// The platform stamps that tag on the inbound signal span, so the workflow id is the
// handle that ties an HTTP request to the spans it produced across every service.
func (j *JaegerClient) FindTraceByWorkflowID(ctx context.Context, service, workflowID string, lookback time.Duration) (*Trace, error) {
	tags, _ := json.Marshal(map[string]string{"workflow_id": workflowID})

	query := url.Values{}
	query.Set("service", service)
	query.Set("tags", string(tags))
	query.Set("limit", "20")
	query.Set("lookback", fmt.Sprintf("%dh", int(lookback.Hours())+1))

	endpoint := fmt.Sprintf("%s/api/traces?%s", j.baseURL, query.Encode())

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}

	resp, err := j.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to query Jaeger: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("jaeger returned HTTP %d", resp.StatusCode)
	}

	var payload struct {
		Data []struct {
			TraceID string `json:"traceID"`
			Spans   []struct {
				OperationName string `json:"operationName"`
				SpanID        string `json:"spanID"`
				Duration      int64  `json:"duration"`
				Tags          []struct {
					Key   string `json:"key"`
					Value any    `json:"value"`
				} `json:"tags"`
				Process struct {
					ServiceName string `json:"serviceName"`
				} `json:"process"`
			} `json:"spans"`
		} `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, fmt.Errorf("failed to decode Jaeger response: %w", err)
	}

	if len(payload.Data) == 0 {
		return nil, fmt.Errorf("no trace found for workflow %s on service %s", workflowID, service)
	}

	// Jaeger may return several traces for the tag; prefer the one carrying the most
	// spans, which is the workflow's full chain rather than a fragment.
	best := payload.Data[0]
	for _, candidate := range payload.Data[1:] {
		if len(candidate.Spans) > len(best.Spans) {
			best = candidate
		}
	}

	trace := &Trace{TraceID: best.TraceID}
	for _, s := range best.Spans {
		tagsMap := make(map[string]string, len(s.Tags))
		for _, t := range s.Tags {
			tagsMap[t.Key] = fmt.Sprintf("%v", t.Value)
		}
		trace.Spans = append(trace.Spans, Span{
			OperationName: s.OperationName,
			ServiceName:   s.Process.ServiceName,
			TraceID:       best.TraceID,
			SpanID:        s.SpanID,
			Duration:      time.Duration(s.Duration) * time.Microsecond,
			Tags:          tagsMap,
		})
	}
	return trace, nil
}
