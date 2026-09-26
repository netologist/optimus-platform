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
