-- name: CreateObservation :one
-- Create a new observation for an entity
INSERT INTO observations (entity_name, content, embedding, created_at)
VALUES (?, ?, ?, ?)
RETURNING id,
    entity_name,
    content,
    embedding,
    created_at;
-- name: GetObservation :one
-- Get observation by ID
SELECT id,
    entity_name,
    content,
    embedding,
    created_at
FROM observations
WHERE id = ?;
-- name: GetEntityObservations :many
-- Get all observations for an entity
SELECT id,
    entity_name,
    content,
    embedding,
    created_at
FROM observations
WHERE entity_name = ?
ORDER BY created_at DESC;
-- name: UpdateObservation :one
-- Update observation content and embedding
UPDATE observations
SET content = ?,
    embedding = ?
WHERE id = ?
RETURNING id,
    entity_name,
    content,
    embedding,
    created_at;
-- name: DeleteObservation :exec
-- Delete observation by ID
DELETE FROM observations
WHERE id = ?;
-- name: DeleteEntityObservations :exec
-- Delete all observations for an entity
DELETE FROM observations
WHERE entity_name = ?;
-- name: SearchObservations :many
-- Search observations by content (basic LIKE search)
SELECT o.id,
    o.entity_name,
    o.content,
    o.embedding,
    o.created_at
FROM observations o
WHERE o.content LIKE '%' || ? || '%'
    OR o.entity_name LIKE '%' || ? || '%'
ORDER BY o.created_at DESC
LIMIT ? OFFSET ?;
-- name: GetObservationsWithEmbeddings :many
-- Get observations that have embeddings for vector search
SELECT id,
    entity_name,
    content,
    embedding,
    created_at
FROM observations
WHERE embedding IS NOT NULL
ORDER BY created_at DESC
LIMIT ? OFFSET ?;
-- name: GetRecentObservations :many
-- Get recent observations across all entities
SELECT id,
    entity_name,
    content,
    embedding,
    created_at
FROM observations
ORDER BY created_at DESC
LIMIT ? OFFSET ?;
-- name: GetEntityObservationStats :one
-- Get observation statistics for an entity
SELECT COUNT(*) as total_observations,
    AVG(LENGTH(content)) as avg_content_length,
    MIN(created_at) as first_observation,
    MAX(created_at) as latest_observation,
    COUNT(
        CASE
            WHEN embedding IS NOT NULL THEN 1
        END
    ) as observations_with_embeddings
FROM observations
WHERE entity_name = ?;
-- name: BatchCreateObservations :exec
-- Batch create multiple observations
INSERT INTO observations (entity_name, content, embedding, created_at)
VALUES (?, ?, ?, ?);
-- name: BatchUpdateObservations :exec
-- Batch update multiple observations
UPDATE observations
SET content = ?,
    embedding = ?
WHERE id = ?;
-- name: BatchDeleteObservations :exec
-- Batch delete multiple observations
DELETE FROM observations
WHERE id = ?;
-- name: GetObservationsByTimeRange :many
-- Get observations within a time range
SELECT id,
    entity_name,
    content,
    embedding,
    created_at
FROM observations
WHERE created_at >= ?
    AND created_at <= ?
ORDER BY created_at DESC
LIMIT ? OFFSET ?;
-- name: GetObservationsByEntities :many
-- Get observations for multiple entities
SELECT o.id,
    o.entity_name,
    o.content,
    o.embedding,
    o.created_at
FROM observations o
WHERE o.entity_name IN (?, ?, ?) -- Placeholder for dynamic IN clause
ORDER BY o.entity_name,
    o.created_at DESC;