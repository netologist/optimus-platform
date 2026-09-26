package helpers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

type TestClient struct {
	PlatformURL string
	DecisionURL string
	MocksURL    string
	HTTP        *http.Client
}

func NewTestClient() *TestClient {
	return &TestClient{
		PlatformURL: "http://127.0.0.1:8080",
		DecisionURL: "http://127.0.0.1:8082",
		MocksURL:    "http://127.0.0.1:8081",
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
