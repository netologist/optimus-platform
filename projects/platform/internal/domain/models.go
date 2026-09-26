package domain

import (
	"encoding/json"
	"time"
)

// Signal represents an operational incident or asset anomaly
type Signal struct {
	ID         string    `json:"id"`
	TenantID   string    `json:"tenant_id"`
	AssetID    string    `json:"asset_id"`
	Symptom    string    `json:"symptom"`
	Status     string    `json:"status"`
	WorkflowID string    `json:"workflow_id,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
}

// WorkOrder represents a dispatched task in the maintenance or FSM system
type WorkOrder struct {
	ID             string    `json:"id"`
	TenantID       string    `json:"tenant_id"`
	AssetID        string    `json:"asset_id"`
	Priority       string    `json:"priority"`
	Status         string    `json:"status"`
	IdempotencyKey string    `json:"idempotency_key"`
	CreatedAt      time.Time `json:"created_at"`
}

// OutboxMessage represents a domain event queued for reliable asynchronous dispatch
type OutboxMessage struct {
	ID            int64           `json:"id"`
	TenantID      string          `json:"tenant_id"`
	EventType     string          `json:"event_type"`
	CorrelationID string          `json:"correlation_id"`
	Traceparent   string          `json:"traceparent,omitempty"`
	Payload       json.RawMessage `json:"payload"`
	PublishedAt   *time.Time      `json:"published_at,omitempty"`
	CreatedAt     time.Time       `json:"created_at"`
}

// Tool represents a registered MCP tool available for a tenant
type Tool struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	InputSchema json.RawMessage `json:"inputSchema"`
	Endpoint    string          `json:"endpoint"`
}
