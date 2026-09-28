package helpers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"time"
)

type TestClient struct {
	PlatformURL string
	DecisionURL string
	MocksURL    string
	HTTP        *http.Client
}

func NewTestClient() *TestClient {
	pURL := os.Getenv("PLATFORM_URL")
	if pURL == "" {
		pURL = "http://127.0.0.1:8080"
	}

	dURL := os.Getenv("DECISION_URL")
	if dURL == "" {
		dURL = "http://127.0.0.1:8082"
	}

	mURL := os.Getenv("MOCKS_URL")
	if mURL == "" {
		mURL = "http://127.0.0.1:8081"
	}

	return &TestClient{
		PlatformURL: pURL,
		DecisionURL: dURL,
		MocksURL:    mURL,
		HTTP:        &http.Client{Timeout: 10 * time.Second},
	}
}

func (c *TestClient) IngestSignal(tenantID, assetID, symptom, traceparent string) (map[string]any, error) {
	url := fmt.Sprintf("%s/v1/tenants/%s/signals", c.PlatformURL, tenantID)
	payload, _ := json.Marshal(map[string]string{
		"asset_id": assetID,
		"symptom":  symptom,
	})

	req, _ := http.NewRequest(http.MethodPost, url, bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Tenant-ID", tenantID)
	if traceparent != "" {
		req.Header.Set("traceparent", traceparent)
	}

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var res map[string]any
	body, _ := io.ReadAll(resp.Body)
	_ = json.Unmarshal(body, &res)
	return res, nil
}

func (c *TestClient) ApproveWorkflow(tenantID, workflowID string) (map[string]any, error) {
	url := fmt.Sprintf("%s/v1/tenants/%s/approvals/%s/approve", c.PlatformURL, tenantID, workflowID)
	req, _ := http.NewRequest(http.MethodPost, url, nil)
	req.Header.Set("X-Tenant-ID", tenantID)

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var res map[string]any
	body, _ := io.ReadAll(resp.Body)
	_ = json.Unmarshal(body, &res)
	return res, nil
}

func (c *TestClient) GetMockCallLog() ([]map[string]any, error) {
	url := fmt.Sprintf("%s/call-log", c.MocksURL)
	resp, err := c.HTTP.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var logs []map[string]any
	body, _ := io.ReadAll(resp.Body)
	_ = json.Unmarshal(body, &logs)
	return logs, nil
}

// AuditEntry is one node of the platform's business-event trail.
type AuditEntry struct {
	ID            int64          `json:"id"`
	EventType     string         `json:"event_type"`
	CorrelationID string         `json:"correlation_id"`
	Traceparent   string         `json:"traceparent"`
	Payload       map[string]any `json:"payload"`
	Published     bool           `json:"published"`
	CreatedAt     time.Time      `json:"created_at"`
}

// GetAuditTrail fetches the tenant's audit trail, optionally filtered to one workflow id.
func (c *TestClient) GetAuditTrail(tenantID, correlationID string) ([]AuditEntry, error) {
	endpoint := fmt.Sprintf("%s/v1/tenants/%s/audit", c.PlatformURL, tenantID)
	if correlationID != "" {
		endpoint += "?correlation_id=" + url.QueryEscape(correlationID)
	}

	req, _ := http.NewRequest(http.MethodGet, endpoint, nil)
	req.Header.Set("X-Tenant-ID", tenantID)

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("audit request returned HTTP %d: %s", resp.StatusCode, string(body))
	}

	var trail struct {
		TenantID string       `json:"tenant_id"`
		Entries  []AuditEntry `json:"entries"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&trail); err != nil {
		return nil, err
	}
	return trail.Entries, nil
}

// DecisionAudit is the persisted decision record, including the raw model answer and the
// policy version that governed it.
type DecisionAudit struct {
	DecisionID       string         `json:"decision_id"`
	TenantID         string         `json:"tenant_id"`
	AssetID          string         `json:"asset_id"`
	Model            string         `json:"model"`
	PolicyVersion    string         `json:"policy_version"`
	RequiresApproval bool           `json:"requires_approval"`
	GovernedDecision map[string]any `json:"governed_decision"`
}

// GetDecisionAudit reads a persisted decision back from the decision service.
func (c *TestClient) GetDecisionAudit(tenantID, decisionID string) (*DecisionAudit, error) {
	url := fmt.Sprintf("%s/v1/tenants/%s/decisions/%s", c.DecisionURL, tenantID, decisionID)

	req, _ := http.NewRequest(http.MethodGet, url, nil)
	req.Header.Set("X-Tenant-ID", tenantID)

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("decision audit request returned HTTP %d: %s", resp.StatusCode, string(body))
	}

	var rec DecisionAudit
	if err := json.NewDecoder(resp.Body).Decode(&rec); err != nil {
		return nil, err
	}
	return &rec, nil
}
