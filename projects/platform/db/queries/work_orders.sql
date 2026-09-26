-- name: SaveWorkOrder :exec
INSERT INTO work_orders (id, tenant_id, asset_id, priority, status, idempotency_key, created_at)
VALUES ($1, $2, $3, $4, $5, $6, $7)
ON CONFLICT (tenant_id, id) DO UPDATE
SET status = EXCLUDED.status;

-- name: GetWorkOrder :one
SELECT id, tenant_id, asset_id, priority, status, idempotency_key, created_at
FROM work_orders
WHERE tenant_id = $1 AND id = $2;
