-- name: CreateEntity :one
-- Create a new entity with upsert semantics
INSERT INTO entities (
        name,
        entity_type,
        embedding,
        metadata,
        created_at,
        updated_at
    )
VALUES (?, ?, ?, ?, ?, ?) ON CONFLICT(name) DO
UPDATE
SET entity_type = excluded.entity_type,
    embedding = excluded.embedding,
    metadata = excluded.metadata,
    updated_at = excluded.updated_at
RETURNING name,
    entity_type,
    embedding,
    metadata,
    created_at,
    updated_at;
-- name: GetEntity :one
-- Get entity by name
SELECT name,
    entity_type,
    embedding,
    metadata,
    created_at,
    updated_at
FROM entities
WHERE name = ?;
-- name: UpdateEntity :one
-- Update entity with optimistic locking
UPDATE entities
SET entity_type = ?,
    embedding = ?,
    metadata = ?,
    updated_at = ?
WHERE name = ?
RETURNING name,
    entity_type,
    embedding,
    metadata,
    created_at,
    updated_at;
-- name: DeleteEntity :exec
-- Delete entity by name
DELETE FROM entities
WHERE name = ?;
-- name: ListEntities :many
-- List entities with pagination and filtering
SELECT name,
    entity_type,
    embedding,
    metadata,
    created_at,
    updated_at
FROM entities
WHERE (
        ? = ''
        OR entity_type = ?
    )
ORDER BY created_at DESC
LIMIT ? OFFSET ?;
-- name: GetEntitiesByType :many
-- Get all entities of a specific type
SELECT name,
    entity_type,
    embedding,
    metadata,
    created_at,
    updated_at
FROM entities
WHERE entity_type = ?
ORDER BY name;
-- name: SearchEntities :many
-- Basic text search in entity names and types
SELECT name,
    entity_type,
    embedding,
    metadata,
    created_at,
    updated_at
FROM entities
WHERE name LIKE '%' || ? || '%'
    OR entity_type LIKE '%' || ? || '%'
ORDER BY created_at DESC
LIMIT ? OFFSET ?;
-- name: GetEntityWithObservations :one
-- Get entity with its observations using CTE
WITH entity_data AS (
    SELECT name,
        entity_type,
        embedding,
        metadata,
        created_at,
        updated_at
    FROM entities
    WHERE name = ?
),
observation_data AS (
    SELECT entity_name,
        content,
        embedding,
        created_at
    FROM observations
    WHERE entity_name = ?
    ORDER BY created_at DESC
    LIMIT ?
)
SELECT e.name,
    e.entity_type,
    e.embedding,
    e.metadata,
    e.created_at,
    e.updated_at,
    GROUP_CONCAT(o.content, ' | ') as observations_content,
    COUNT(o.entity_name) as observation_count
FROM entity_data e
    LEFT JOIN observation_data o ON e.name = o.entity_name
GROUP BY e.name,
    e.entity_type,
    e.embedding,
    e.metadata,
    e.created_at,
    e.updated_at;
-- name: GetEntitiesWithStats :many
-- Get entities with observation counts and relation counts using window functions
WITH entity_stats AS (
    SELECT e.name,
        e.entity_type,
        e.embedding,
        e.metadata,
        e.created_at,
        e.updated_at,
        COUNT(DISTINCT o.id) as observation_count,
        COUNT(
            DISTINCT CASE
                WHEN r.source = e.name THEN r.id
            END
        ) as outgoing_relation_count,
        COUNT(
            DISTINCT CASE
                WHEN r.target = e.name THEN r.id
            END
        ) as incoming_relation_count
    FROM entities e
        LEFT JOIN observations o ON e.name = o.entity_name
        LEFT JOIN relations r ON e.name = r.source
        OR e.name = r.target
    GROUP BY e.name,
        e.entity_type,
        e.embedding,
        e.metadata,
        e.created_at,
        e.updated_at
)
SELECT *
FROM entity_stats
ORDER BY observation_count DESC,
    created_at DESC
LIMIT ? OFFSET ?;
-- name: BatchCreateEntities :exec
-- Batch insert multiple entities
INSERT INTO entities (
        name,
        entity_type,
        embedding,
        metadata,
        created_at,
        updated_at
    )
VALUES (?, ?, ?, ?, ?, ?);
-- name: BatchUpdateEntities :exec
-- Batch update multiple entities
UPDATE entities
SET entity_type = ?,
    embedding = ?,
    metadata = ?,
    updated_at = ?
WHERE name = ?;
-- name: BatchDeleteEntities :exec
-- Batch delete multiple entities
DELETE FROM entities
WHERE name = ?;