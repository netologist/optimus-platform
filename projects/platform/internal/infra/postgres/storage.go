package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/optimus/projects/platform/internal/domain"
	"github.com/optimus/projects/platform/internal/infra/postgres/db"
	"github.com/optimus/projects/platform/internal/tenant"
)

// Storage implements app.Storage using PostgreSQL 17 with Row-Level Security (RLS) and typed sqlc queries
type Storage struct {
	pool *pgxpool.Pool
}

func NewStorage(pool *pgxpool.Pool) *Storage {
	return &Storage{pool: pool}
}

// withTenantTx executes a function inside a transaction with RLS app.current_tenant set
func (s *Storage) withTenantTx(ctx context.Context, tenantID string, fn func(tx pgx.Tx) error) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	// Set local tenant context for RLS policy enforcement
	if tenantID != "" {
		if _, err := tx.Exec(ctx, "SELECT set_config('app.current_tenant', $1, true)", tenantID); err != nil {
			return fmt.Errorf("failed to set local tenant context %s: %w", tenantID, err)
		}
	}

	if err := fn(tx); err != nil {
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}
	return nil
}

func (s *Storage) SaveSignal(ctx context.Context, sig *domain.Signal) error {
	tc, err := tenant.FromContext(ctx)
	if err != nil {
		return err
	}
	if sig.TenantID != tc.TenantID {
		return fmt.Errorf("RLS violation: tenant context %s does not match signal tenant %s", tc.TenantID, sig.TenantID)
	}

	return s.withTenantTx(ctx, tc.TenantID, func(tx pgx.Tx) error {
		q := db.New(tx)
		return q.SaveSignal(ctx, db.SaveSignalParams{
			ID:         sig.ID,
			TenantID:   sig.TenantID,
			AssetID:    sig.AssetID,
			Symptom:    sig.Symptom,
			Status:     sig.Status,
			WorkflowID: pgtype.Text{String: sig.WorkflowID, Valid: sig.WorkflowID != ""},
			CreatedAt:  pgtype.Timestamptz{Time: sig.CreatedAt, Valid: true},
		})
	})
}

func (s *Storage) GetSignal(ctx context.Context, tenantID, id string) (*domain.Signal, error) {
	tc, err := tenant.FromContext(ctx)
	if err != nil {
		return nil, err
	}
	if tenantID != tc.TenantID {
		return nil, fmt.Errorf("RLS violation: tenant context %s does not match requested tenant %s", tc.TenantID, tenantID)
	}

	var sig domain.Signal
	err = s.withTenantTx(ctx, tc.TenantID, func(tx pgx.Tx) error {
		q := db.New(tx)
		row, err := q.GetSignal(ctx, db.GetSignalParams{
			TenantID: tenantID,
			ID:       id,
		})
		if err != nil {
			return err
		}

		sig = domain.Signal{
			ID:         row.ID,
			TenantID:   row.TenantID,
			AssetID:    row.AssetID,
			Symptom:    row.Symptom,
			Status:     row.Status,
			WorkflowID: row.WorkflowID.String,
			CreatedAt:  row.CreatedAt.Time,
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &sig, nil
}

func (s *Storage) SaveWorkOrder(ctx context.Context, wo *domain.WorkOrder) error {
	tc, err := tenant.FromContext(ctx)
	if err != nil {
		return err
	}
	if wo.TenantID != tc.TenantID {
		return fmt.Errorf("RLS violation: tenant context %s does not match work order tenant %s", tc.TenantID, wo.TenantID)
	}

	return s.withTenantTx(ctx, tc.TenantID, func(tx pgx.Tx) error {
		q := db.New(tx)
		return q.SaveWorkOrder(ctx, db.SaveWorkOrderParams{
			ID:             wo.ID,
			TenantID:       wo.TenantID,
			AssetID:        wo.AssetID,
			Priority:       wo.Priority,
			Status:         wo.Status,
			IdempotencyKey: wo.IdempotencyKey,
			CreatedAt:      pgtype.Timestamptz{Time: wo.CreatedAt, Valid: true},
		})
	})
}

func (s *Storage) GetWorkOrder(ctx context.Context, tenantID, id string) (*domain.WorkOrder, error) {
	tc, err := tenant.FromContext(ctx)
	if err != nil {
		return nil, err
	}
	if tenantID != tc.TenantID {
		return nil, fmt.Errorf("RLS violation: tenant context %s does not match requested tenant %s", tc.TenantID, tenantID)
	}

	var wo domain.WorkOrder
	err = s.withTenantTx(ctx, tc.TenantID, func(tx pgx.Tx) error {
		q := db.New(tx)
		row, err := q.GetWorkOrder(ctx, db.GetWorkOrderParams{
			TenantID: tenantID,
			ID:       id,
		})
		if err != nil {
			return err
		}

		wo = domain.WorkOrder{
			ID:             row.ID,
			TenantID:       row.TenantID,
			AssetID:        row.AssetID,
			Priority:       row.Priority,
			Status:         row.Status,
			IdempotencyKey: row.IdempotencyKey,
			CreatedAt:      row.CreatedAt.Time,
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &wo, nil
}

// SaveWorkOrderWithEvent writes the work order row and its domain event inside one
// transaction. Either both land or neither does, which is what lets the relay treat a
// missing outbox row as "no event" rather than "an event that may have been lost".
func (s *Storage) SaveWorkOrderWithEvent(ctx context.Context, wo *domain.WorkOrder, msg *domain.OutboxMessage) error {
	tc, err := tenant.FromContext(ctx)
	if err != nil {
		return err
	}
	if wo.TenantID != tc.TenantID {
		return fmt.Errorf("RLS violation: tenant context %s does not match work order tenant %s", tc.TenantID, wo.TenantID)
	}
	if msg.TenantID != "" && msg.TenantID != tc.TenantID {
		return fmt.Errorf("RLS violation: tenant context %s does not match outbox tenant %s", tc.TenantID, msg.TenantID)
	}
	msg.TenantID = tc.TenantID

	return s.withTenantTx(ctx, tc.TenantID, func(tx pgx.Tx) error {
		q := db.New(tx)

		if err := q.SaveWorkOrder(ctx, db.SaveWorkOrderParams{
			ID:             wo.ID,
			TenantID:       wo.TenantID,
			AssetID:        wo.AssetID,
			Priority:       wo.Priority,
			Status:         wo.Status,
			IdempotencyKey: wo.IdempotencyKey,
			CreatedAt:      pgtype.Timestamptz{Time: wo.CreatedAt, Valid: true},
		}); err != nil {
			return fmt.Errorf("failed to save work order: %w", err)
		}

		id, err := q.SaveOutboxMessage(ctx, db.SaveOutboxMessageParams{
			TenantID:      msg.TenantID,
			EventType:     msg.EventType,
			CorrelationID: msg.CorrelationID,
			Traceparent:   pgtype.Text{String: msg.Traceparent, Valid: msg.Traceparent != ""},
			Payload:       msg.Payload,
			CreatedAt:     pgtype.Timestamptz{Time: msg.CreatedAt, Valid: true},
		})
		if err != nil {
			return fmt.Errorf("failed to save outbox message: %w", err)
		}
		msg.ID = id
		return nil
	})
}

func (s *Storage) SaveOutboxMessage(ctx context.Context, msg *domain.OutboxMessage) error {
	tenantID := msg.TenantID
	if tc, err := tenant.FromContext(ctx); err == nil && tc.TenantID != "" {
		if tenantID != "" && tenantID != tc.TenantID {
			return fmt.Errorf("RLS violation: tenant context %s does not match outbox tenant %s", tc.TenantID, tenantID)
		}
		tenantID = tc.TenantID
	}

	return s.withTenantTx(ctx, tenantID, func(tx pgx.Tx) error {
		q := db.New(tx)
		id, err := q.SaveOutboxMessage(ctx, db.SaveOutboxMessageParams{
			TenantID:      tenantID,
			EventType:     msg.EventType,
			CorrelationID: msg.CorrelationID,
			Traceparent:   pgtype.Text{String: msg.Traceparent, Valid: msg.Traceparent != ""},
			Payload:       msg.Payload,
			CreatedAt:     pgtype.Timestamptz{Time: msg.CreatedAt, Valid: true},
		})
		if err != nil {
			return err
		}
		msg.ID = id
		return nil
	})
}

func (s *Storage) GetUnpublishedOutboxMessages(ctx context.Context, limit int) ([]*domain.OutboxMessage, error) {
	if limit <= 0 {
		limit = 50
	}

	q := db.New(s.pool)
	rows, err := q.GetUnpublishedOutboxMessages(ctx, int32(limit))
	if err != nil {
		return nil, fmt.Errorf("failed to query unpublished outbox messages: %w", err)
	}

	messages := make([]*domain.OutboxMessage, len(rows))
	for i, r := range rows {
		messages[i] = &domain.OutboxMessage{
			ID:            r.ID,
			TenantID:      r.TenantID,
			EventType:     r.EventType,
			CorrelationID: r.CorrelationID,
			Traceparent:   r.Traceparent,
			Payload:       r.Payload,
			CreatedAt:     r.CreatedAt.Time,
		}
	}
	return messages, nil
}

func (s *Storage) MarkOutboxMessagePublished(ctx context.Context, id int64) error {
	q := db.New(s.pool)
	err := q.MarkOutboxMessagePublished(ctx, db.MarkOutboxMessagePublishedParams{
		PublishedAt: pgtype.Timestamptz{Time: time.Now().UTC(), Valid: true},
		ID:          id,
	})
	if err != nil {
		return fmt.Errorf("failed to mark outbox message %d as published: %w", id, err)
	}
	return nil
}
