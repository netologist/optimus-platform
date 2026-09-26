package systemone

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
)

// Request defines the SystemOne API request shape
type Request struct {
	Model     string              `json:"model"`
	State     any                 `json:"state"`
	Questions map[string]Question `json:"questions"`
}

// Question defines a typed question for SystemOne/Ollaya
type Question struct {
	Type         string `json:"type"` // "choice", "score", "noul"
	Instructions string `json:"instructions,omitempty"`
	Criteria     any    `json:"criteria,omitempty"` // list or map
}

// Response defines the SystemOne API response shape
type Response struct {
	Model      string                    `json:"model"`
	Answers    map[string]QuestionAnswer `json:"answers"`
	Confidence float64                   `json:"confidence"`
	TimingMS   float64                   `json:"timing_ms,omitempty"`
}

// QuestionAnswer holds the evaluated answer and probability
type QuestionAnswer struct {
	Value       any     `json:"value"`
	Score       float64 `json:"score,omitempty"`
	Probability float64 `json:"probability,omitempty"`
}

// Client talks to Ollaya SystemOne endpoint
type Client struct {
	baseURL    string
	httpClient *http.Client
}

func NewClient(baseURL string) *Client {
	return &Client{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

func (c *Client) Decide(ctx context.Context, req Request) (*Response, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	url := fmt.Sprintf("%s/v1/systemone", c.baseURL)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create http request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	otel.GetTextMapPropagator().Inject(ctx, propagation.HeaderCarrier(httpReq.Header))
	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("systemone request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("systemone returned non-200: %d (%s)", resp.StatusCode, string(respBody))
	}

	var res Response
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &res, nil
}
