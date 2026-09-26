package audit

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"github.com/optimus/projects/decision-service/internal/policy"
	"github.com/optimus/projects/decision-service/internal/systemone"
)

// Record represents a persisted decision audit entry
type Record struct {
	ID               int64                   `json:"id"`
	DecisionID       string                  `json:"decision_id"`
	TenantID         string                  `json:"tenant_id"`
	AssetID          string                  `json:"asset_id"`
	Model            string                  `json:"model"`
	StatePayload     json.RawMessage         `json:"state_payload"`
	RawResponse      *systemone.Response     `json:"raw_response"`
	GovernedDecision policy.GovernedDecision `json:"governed_decision"`
	PolicyVersion    string                  `json:"policy_version"`
	RequiresApproval bool                    `json:"requires_approval"`
	CreatedAt        time.Time               `json:"created_at"`
}

// Store abstracts decision audit storage
type Store interface {
	Save(ctx context.Context, rec *Record) error
	Get(ctx context.Context, tenantID, decisionID string) (*Record, error)
}

// MemoryStore provides in-memory thread-safe storage for tests
type MemoryStore struct {
	records map[string]*Record
	mu      sync.RWMutex
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		records: make(map[string]*Record),
	}
}

func (m *MemoryStore) Save(ctx context.Context, rec *Record) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	rec.ID = int64(len(m.records) + 1)
	m.records[rec.TenantID+":"+rec.DecisionID] = rec
	return nil
}

func (m *MemoryStore) Get(ctx context.Context, tenantID, decisionID string) (*Record, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	rec, ok := m.records[tenantID+":"+decisionID]
	if !ok {
		return nil, nil
	}
	return rec, nil
}
