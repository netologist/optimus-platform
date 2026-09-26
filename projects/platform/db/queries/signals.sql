-- name: SaveSignal :exec
INSERT INTO signals (id, tenant_id, asset_id, symptom, status, workflow_id, created_at)
VALUES ($1, $2, $3, $4, $5, $6, $7)
ON CONFLICT (tenant_id, id) DO UPDATE
SET status = EXCLUDED.status,
    workflow_id = EXCLUDED.workflow_id;

-- name: GetSignal :one
SELECT id, tenant_id, asset_id, symptom, status, workflow_id, created_at
FROM signals
WHERE tenant_id = $1 AND id = $2;
