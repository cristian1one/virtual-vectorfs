-- name: SearchEntitiesByContent :many
-- Search entities by observation content
SELECT DISTINCT e.name,
    e.entity_type,
    e.embedding,
    e.metadata,
    e.created_at,
    e.updated_at
FROM entities e
    JOIN observations o ON o.entity_name = e.name
WHERE o.content LIKE '%' || ? || '%'
ORDER BY e.created_at DESC
LIMIT ? OFFSET ?;
-- name: SearchEntitiesByName :many
-- Search entities by name
SELECT name,
    entity_type,
    e.embedding,
    metadata,
    created_at,
    updated_at
FROM entities e
WHERE name LIKE '%' || ? || '%'
ORDER BY created_at DESC
LIMIT ? OFFSET ?;
-- name: SearchEntitiesByType :many
-- Search entities by type
SELECT name,
    entity_type,
    e.embedding,
    metadata,
    created_at,
    updated_at
FROM entities e
WHERE entity_type = ?
ORDER BY created_at DESC
LIMIT ? OFFSET ?;
-- name: GetEntitiesWithEmbeddings :many
-- Get entities that have embeddings for vector operations
SELECT name,
    entity_type,
    embedding,
    metadata,
    created_at,
    updated_at
FROM entities
WHERE embedding IS NOT NULL
ORDER BY created_at DESC
LIMIT ? OFFSET ?;
-- name: GetEntitiesByMetadata :many
-- Search entities by metadata content
SELECT name,
    entity_type,
    embedding,
    metadata,
    created_at,
    updated_at
FROM entities
WHERE metadata LIKE '%' || ? || '%'
ORDER BY created_at DESC
LIMIT ? OFFSET ?;
-- name: GetRecentEntities :many
-- Get recently created entities
SELECT name,
    entity_type,
    embedding,
    metadata,
    created_at,
    updated_at
FROM entities
ORDER BY created_at DESC
LIMIT ? OFFSET ?;
-- name: GetEntitiesByObservationCount :many
-- Get entities ordered by number of observations
SELECT e.name,
    e.entity_type,
    e.embedding,
    e.metadata,
    e.created_at,
    e.updated_at,
    COUNT(o.id) as observation_count
FROM entities e
    LEFT JOIN observations o ON o.entity_name = e.name
GROUP BY e.name,
    e.entity_type,
    e.embedding,
    e.metadata,
    e.created_at,
    e.updated_at
ORDER BY observation_count DESC,
    e.created_at DESC
LIMIT ? OFFSET ?;