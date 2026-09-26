package audit

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/optimus/projects/decision-service/internal/policy"
	"github.com/optimus/projects/decision-service/internal/systemone"
)

// PostgresStore implements Store using PostgreSQL 17 with Row-Level Security (RLS)
type PostgresStore struct {
	pool *pgxpool.Pool
}

func NewPostgresStore(pool *pgxpool.Pool) *PostgresStore {
	return &PostgresStore{pool: pool}
}

func (s *PostgresStore) withTenantTx(ctx context.Context, tenantID string, fn func(tx pgx.Tx) error) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	if tenantID != "" {
		if _, err := tx.Exec(ctx, "SET LOCAL app.current_tenant = $1", tenantID); err != nil {
			return fmt.Errorf("failed to set local tenant %s: %w", tenantID, err)
		}
	}

	if err := fn(tx); err != nil {
		return err
	}

	return tx.Commit(ctx)
}

func (s *PostgresStore) Save(ctx context.Context, rec *Record) error {
	rawJSON, err := json.Marshal(rec.RawResponse)
	if err != nil {
		return fmt.Errorf("failed to marshal raw response: %w", err)
	}

	govJSON, err := json.Marshal(rec.GovernedDecision)
	if err != nil {
		return fmt.Errorf("failed to marshal governed decision: %w", err)
	}

	createdAt := rec.CreatedAt
	if createdAt.IsZero() {
		createdAt = time.Now().UTC()
	}

	return s.withTenantTx(ctx, rec.TenantID, func(tx pgx.Tx) error {
		query := `
			INSERT INTO decision_audit (
				tenant_id, decision_id, asset_id, model,
				state_payload, raw_response, governed_decision,
				policy_version, requires_approval, created_at
			) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
			RETURNING id
		`
		return tx.QueryRow(ctx, query,
			rec.TenantID,
			rec.DecisionID,
			rec.AssetID,
			rec.Model,
			rec.StatePayload,
			rawJSON,
			govJSON,
			rec.PolicyVersion,
			rec.RequiresApproval,
			createdAt,
		).Scan(&rec.ID)
	})
}

func (s *PostgresStore) Get(ctx context.Context, tenantID, decisionID string) (*Record, error) {
	var rec Record
	var rawJSON, govJSON []byte

	err := s.withTenantTx(ctx, tenantID, func(tx pgx.Tx) error {
		query := `
			SELECT id, tenant_id, decision_id, asset_id, model,
			       state_payload, raw_response, governed_decision,
			       policy_version, requires_approval, created_at
			FROM decision_audit
			WHERE tenant_id = $1 AND decision_id = $2
		`
		return tx.QueryRow(ctx, query, tenantID, decisionID).Scan(
			&rec.ID,
			&rec.TenantID,
			&rec.DecisionID,
			&rec.AssetID,
			&rec.Model,
			&rec.StatePayload,
			&rawJSON,
			&govJSON,
			&rec.PolicyVersion,
			&rec.RequiresApproval,
			&rec.CreatedAt,
		)
	})
	if err != nil {
		return nil, err
	}

	var rawResp systemone.Response
	if err := json.Unmarshal(rawJSON, &rawResp); err == nil {
		rec.RawResponse = &rawResp
	}

	var govDec policy.GovernedDecision
	if err := json.Unmarshal(govJSON, &govDec); err == nil {
		rec.GovernedDecision = govDec
	}

	return &rec, nil
}
