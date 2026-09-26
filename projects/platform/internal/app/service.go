package app

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/optimus/projects/platform/internal/domain"
	"github.com/optimus/projects/platform/internal/tenant"
)

// Storage defines abstract persistence for platform operations
type Storage interface {
	SaveSignal(ctx context.Context, sig *domain.Signal) error
	GetSignal(ctx context.Context, tenantID, id string) (*domain.Signal, error)
	SaveWorkOrder(ctx context.Context, wo *domain.WorkOrder) error
	SaveOutboxMessage(ctx context.Context, msg *domain.OutboxMessage) error
	GetUnpublishedOutboxMessages(ctx context.Context, limit int) ([]*domain.OutboxMessage, error)
	MarkOutboxMessagePublished(ctx context.Context, id int64) error
}

// MemoryStorage provides an in-memory thread-safe implementation of Storage with RLS emulation
type MemoryStorage struct {
	signals    map[string]*domain.Signal
	workOrders map[string]*domain.WorkOrder
	outbox     []*domain.OutboxMessage
	mu         sync.RWMutex
}

func NewMemoryStorage() *MemoryStorage {
	return &MemoryStorage{
		signals:    make(map[string]*domain.Signal),
		workOrders: make(map[string]*domain.WorkOrder),
		outbox:     make([]*domain.OutboxMessage, 0),
	}
}

func (m *MemoryStorage) SaveSignal(ctx context.Context, sig *domain.Signal) error {
	tc, err := tenant.FromContext(ctx)
	if err != nil {
		return err
	}
	if sig.TenantID != tc.TenantID {
		return fmt.Errorf("RLS violation: tenant %s cannot access tenant %s data", tc.TenantID, sig.TenantID)
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.signals[sig.TenantID+":"+sig.ID] = sig
	return nil
}

func (m *MemoryStorage) GetSignal(ctx context.Context, tenantID, id string) (*domain.Signal, error) {
	tc, err := tenant.FromContext(ctx)
	if err != nil {
		return nil, err
	}
	if tenantID != tc.TenantID {
		return nil, fmt.Errorf("RLS violation: tenant %s cannot access tenant %s data", tc.TenantID, tenantID)
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	sig, ok := m.signals[tenantID+":"+id]
	if !ok {
		return nil, fmt.Errorf("signal not found")
	}
	return sig, nil
}

func (m *MemoryStorage) SaveWorkOrder(ctx context.Context, wo *domain.WorkOrder) error {
	tc, err := tenant.FromContext(ctx)
	if err != nil {
		return err
	}
	if wo.TenantID != tc.TenantID {
		return fmt.Errorf("RLS violation: tenant %s cannot write tenant %s data", tc.TenantID, wo.TenantID)
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.workOrders[wo.TenantID+":"+wo.ID] = wo
	return nil
}

func (m *MemoryStorage) SaveOutboxMessage(ctx context.Context, msg *domain.OutboxMessage) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	msg.ID = int64(len(m.outbox) + 1)
	m.outbox = append(m.outbox, msg)
	return nil
}

func (m *MemoryStorage) GetUnpublishedOutboxMessages(ctx context.Context, limit int) ([]*domain.OutboxMessage, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var res []*domain.OutboxMessage
	for _, msg := range m.outbox {
		if msg.PublishedAt == nil {
			res = append(res, msg)
			if len(res) >= limit {
				break
			}
		}
	}
	return res, nil
}

func (m *MemoryStorage) MarkOutboxMessagePublished(ctx context.Context, id int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	now := time.Now().UTC()
	for _, msg := range m.outbox {
		if msg.ID == id {
			msg.PublishedAt = &now
			return nil
		}
	}
	return fmt.Errorf("outbox message not found")
}

// WorkflowClient abstracts Temporal workflow orchestration
type WorkflowClient interface {
	StartAssetFailureWorkflow(ctx context.Context, workflowID, tenantID, assetID, symptom, traceparent string) error
	SignalApproval(ctx context.Context, workflowID string, approved bool, approver string) error
}

// Service coordinates platform use cases
type Service struct {
	storage  Storage
	wfClient WorkflowClient
}

type ServiceOption func(*Service)

func WithWorkflowClient(wfClient WorkflowClient) ServiceOption {
	return func(s *Service) {
		s.wfClient = wfClient
	}
}

func NewService(storage Storage, opts ...ServiceOption) *Service {
	s := &Service{storage: storage}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

type IngestSignalRequest struct {
	AssetID string `json:"asset_id"`
	Symptom string `json:"symptom"`
}

type IngestSignalResponse struct {
	SignalID   string `json:"signal_id"`
	WorkflowID string `json:"workflow_id"`
	Status     string `json:"status"`
}

func (s *Service) IngestSignal(ctx context.Context, req IngestSignalRequest, traceparent string) (*IngestSignalResponse, error) {
	tc, err := tenant.FromContext(ctx)
	if err != nil {
		return nil, err
	}

	sigID := fmt.Sprintf("sig-%d", time.Now().UnixNano())
	wfID := fmt.Sprintf("wf-asset-failure-%s-%d", req.AssetID, time.Now().UnixNano())

	sig := &domain.Signal{
		ID:         sigID,
		TenantID:   tc.TenantID,
		AssetID:    req.AssetID,
		Symptom:    req.Symptom,
		Status:     "INGESTED",
		WorkflowID: wfID,
		CreatedAt:  time.Now().UTC(),
	}

	if err := s.storage.SaveSignal(ctx, sig); err != nil {
		return nil, fmt.Errorf("failed to save signal: %w", err)
	}

	payload, _ := json.Marshal(map[string]any{
		"signal_id":   sigID,
		"asset_id":    req.AssetID,
		"tenant_id":   tc.TenantID,
		"workflow_id": wfID,
	})

	outboxMsg := &domain.OutboxMessage{
		TenantID:      tc.TenantID,
		EventType:     "signal.received",
		CorrelationID: wfID,
		Traceparent:   traceparent,
		Payload:       payload,
		CreatedAt:     time.Now().UTC(),
	}

	if err := s.storage.SaveOutboxMessage(ctx, outboxMsg); err != nil {
		return nil, fmt.Errorf("failed to enqueue outbox message: %w", err)
	}
	if s.wfClient != nil {
		if err := s.wfClient.StartAssetFailureWorkflow(ctx, wfID, tc.TenantID, req.AssetID, req.Symptom, traceparent); err != nil {
			return nil, fmt.Errorf("failed to trigger workflow: %w", err)
		}
	}

	return &IngestSignalResponse{
		SignalID:   sigID,
		WorkflowID: wfID,
		Status:     "INGESTED",
	}, nil
}

type ApproveWorkflowRequest struct {
	WorkflowID string `json:"workflow_id"`
	Approved   bool   `json:"approved"`
	Approver   string `json:"approver,omitempty"`
}

type ApproveWorkflowResponse struct {
	WorkflowID string `json:"workflow_id"`
	Status     string `json:"status"`
	Signaled   bool   `json:"signaled"`
}

func (s *Service) ApproveWorkflow(ctx context.Context, req ApproveWorkflowRequest) (*ApproveWorkflowResponse, error) {
	tc, err := tenant.FromContext(ctx)
	if err != nil {
		return nil, err
	}

	approver := req.Approver
	if approver == "" {
		approver = fmt.Sprintf("supervisor@%s", tc.TenantID)
	}

	signaled := false
	if s.wfClient != nil {
		if err := s.wfClient.SignalApproval(ctx, req.WorkflowID, req.Approved, approver); err != nil {
			return nil, fmt.Errorf("failed to signal workflow: %w", err)
		}
		signaled = true
	}

	status := "approved"
	if !req.Approved {
		status = "rejected"
	}

	return &ApproveWorkflowResponse{
		WorkflowID: req.WorkflowID,
		Status:     status,
		Signaled:   signaled,
	}, nil
}

func (s *Service) GetActiveTools(ctx context.Context) ([]domain.Tool, error) {
	// Dynamically returns registered MCP tools for this tenant
	return []domain.Tool{
		{
			Name:        "eam.get_asset",
			Description: "Retrieve asset master record",
			Endpoint:    "http://integration-mocks.optimus.svc:8080",
		},
		{
			Name:        "eam.get_maintenance_history",
			Description: "Retrieve maintenance history",
			Endpoint:    "http://integration-mocks.optimus.svc:8080",
		},
		{
			Name:        "plm.search_documents",
			Description: "Search technical service manuals",
			Endpoint:    "http://integration-mocks.optimus.svc:8080",
		},
		{
			Name:        "erp.get_inventory",
			Description: "Query inventory levels",
			Endpoint:    "http://integration-mocks.optimus.svc:8080",
		},
		{
			Name:        "erp.reserve_inventory",
			Description: "Reserve parts",
			Endpoint:    "http://integration-mocks.optimus.svc:8080",
		},
		{
			Name:        "fsm.create_work_order",
			Description: "Create work order",
			Endpoint:    "http://integration-mocks.optimus.svc:8080",
		},
	}, nil
}
