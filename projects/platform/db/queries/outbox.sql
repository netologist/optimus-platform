-- name: SaveOutboxMessage :one
INSERT INTO outbox (tenant_id, event_type, correlation_id, traceparent, payload, created_at)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING id;

-- name: GetUnpublishedOutboxMessages :many
SELECT id, tenant_id, event_type, correlation_id, COALESCE(traceparent, '')::varchar AS traceparent, payload, created_at
FROM outbox
WHERE published_at IS NULL
ORDER BY created_at ASC
LIMIT $1;

-- name: MarkOutboxMessagePublished :exec
UPDATE outbox
SET published_at = $1
WHERE id = $2;

-- name: ListAuditEntries :many
-- The tenant's business-event trail, optionally narrowed to a single correlation id.
-- The outbox is already the append-only, tenant-scoped record of every domain event,
-- so the audit view reads it instead of maintaining a second, drift-prone log.
SELECT id, tenant_id, event_type, correlation_id,
       COALESCE(traceparent, '')::varchar AS traceparent,
       payload, published_at, created_at
FROM outbox
WHERE tenant_id = $1
  AND (sqlc.narg('correlation_id')::varchar IS NULL
       OR correlation_id = sqlc.narg('correlation_id')::varchar)
ORDER BY created_at ASC, id ASC
LIMIT $2;
