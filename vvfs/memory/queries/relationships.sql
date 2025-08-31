-- name: CreateEntityFileRelation :one
-- Create a relationship between entity and file
INSERT INTO entity_file_relations (
        entity_name,
        file_id,
        relation_type,
        confidence,
        similarity_score,
        metadata,
        created_at
    )
VALUES (?, ?, ?, ?, ?, ?, ?)
RETURNING id,
    entity_name,
    file_id,
    relation_type,
    confidence,
    similarity_score,
    metadata,
    created_at;
-- name: GetEntityFileRelation :one
-- Get specific entity-file relationship
SELECT id,
    entity_name,
    file_id,
    relation_type,
    confidence,
    similarity_score,
    metadata,
    created_at
FROM entity_file_relations
WHERE entity_name = ?
    AND file_id = ?
    AND relation_type = ?;
-- name: UpdateEntityFileRelation :one
-- Update relationship confidence and metadata
UPDATE entity_file_relations
SET confidence = ?,
    similarity_score = ?,
    metadata = ?
WHERE id = ?
RETURNING id,
    entity_name,
    file_id,
    relation_type,
    confidence,
    similarity_score,
    metadata,
    created_at;
-- name: DeleteEntityFileRelation :exec
-- Delete specific relationship
DELETE FROM entity_file_relations
WHERE entity_name = ?
    AND file_id = ?
    AND relation_type = ?;
-- name: GetEntityRelations :many
-- Get all relationships for an entity
SELECT efr.id,
    efr.entity_name,
    efr.file_id,
    efr.relation_type,
    efr.confidence,
    efr.similarity_score,
    efr.metadata,
    efr.created_at,
    f.file_path,
    f.workspace_id
FROM entity_file_relations efr
    JOIN files f ON efr.file_id = f.id
WHERE efr.entity_name = ?
ORDER BY efr.confidence DESC,
    efr.similarity_score DESC;
-- name: GetFileRelations :many
-- Get all relationships for a file
SELECT efr.id,
    efr.entity_name,
    efr.file_id,
    efr.relation_type,
    efr.confidence,
    efr.similarity_score,
    efr.metadata,
    efr.created_at,
    e.entity_type,
    e.metadata as entity_metadata
FROM entity_file_relations efr
    JOIN entities e ON efr.entity_name = e.name
WHERE efr.file_id = ?
ORDER BY efr.confidence DESC,
    efr.similarity_score DESC;
-- name: GetRelationsByType :many
-- Get relationships of specific type with pagination
SELECT efr.id,
    efr.entity_name,
    efr.file_id,
    efr.relation_type,
    efr.confidence,
    efr.similarity_score,
    efr.metadata,
    efr.created_at,
    f.file_path,
    f.workspace_id,
    e.entity_type
FROM entity_file_relations efr
    JOIN files f ON efr.file_id = f.id
    JOIN entities e ON efr.entity_name = e.name
WHERE efr.relation_type = ?
ORDER BY efr.confidence DESC
LIMIT ? OFFSET ?;
-- name: GetHighConfidenceRelations :many
-- Get relationships above confidence threshold
SELECT efr.id,
    efr.entity_name,
    efr.file_id,
    efr.relation_type,
    efr.confidence,
    efr.similarity_score,
    efr.metadata,
    efr.created_at
FROM entity_file_relations efr
WHERE efr.confidence >= ?
ORDER BY efr.confidence DESC,
    efr.similarity_score DESC
LIMIT ? OFFSET ?;
-- name: GetSimilarEntitiesForFile :many
-- Find entities similar to a file based on embeddings (placeholder for vector similarity)
SELECT e.name,
    e.entity_type,
    e.embedding,
    e.metadata,
    e.created_at,
    -- Placeholder for vector similarity calculation
    0.0 as similarity_score
FROM entities e
WHERE e.embedding IS NOT NULL
ORDER BY e.created_at DESC
LIMIT ?;
-- name: GetFilesForEntityType :many
-- Get files related to entities of specific type
SELECT DISTINCT f.id,
    f.workspace_id,
    f.file_path,
    f.size,
    f.mod_time,
    f.is_dir,
    f.checksum,
    f.embedding,
    f.metadata,
    f.created_at,
    f.updated_at,
    efr.relation_type,
    efr.confidence,
    efr.similarity_score
FROM files f
    JOIN entity_file_relations efr ON f.id = efr.file_id
    JOIN entities e ON efr.entity_name = e.name
WHERE e.entity_type = ?
ORDER BY efr.confidence DESC,
    f.updated_at DESC
LIMIT ? OFFSET ?;
-- name: GetEntityFileNetwork :many
-- Get network of relationships for analysis using CTEs
WITH entity_relations AS (
    SELECT er.entity_name,
        er.file_id,
        er.relation_type,
        er.confidence,
        er.similarity_score,
        ROW_NUMBER() OVER (
            PARTITION BY er.entity_name
            ORDER BY er.confidence DESC
        ) as rn
    FROM entity_file_relations er
    WHERE er.confidence >= ?
),
file_relations AS (
    SELECT fr.file_id,
        fr.entity_name,
        fr.relation_type,
        fr.confidence,
        fr.similarity_score,
        ROW_NUMBER() OVER (
            PARTITION BY fr.file_id
            ORDER BY fr.confidence DESC
        ) as rn
    FROM entity_file_relations fr
    WHERE fr.confidence >= ?
)
SELECT 'entity_to_file' as connection_type,
    er.entity_name as source,
    f.file_path as target,
    er.relation_type,
    er.confidence,
    er.similarity_score
FROM entity_relations er
    JOIN files f ON er.file_id = f.id
WHERE er.rn <= 5 -- Top 5 relations per entity
UNION ALL
SELECT 'file_to_entity' as connection_type,
    f.file_path as source,
    fr.entity_name as target,
    fr.relation_type,
    fr.confidence,
    fr.similarity_score
FROM file_relations fr
    JOIN files f ON fr.file_id = f.id
WHERE fr.rn <= 5 -- Top 5 relations per file
ORDER BY confidence DESC;
-- name: GetRelationStatistics :one
-- Get comprehensive statistics about entity-file relationships
SELECT COUNT(*) as total_relations,
    COUNT(DISTINCT entity_name) as unique_entities,
    COUNT(DISTINCT file_id) as unique_files,
    AVG(confidence) as avg_confidence,
    MIN(confidence) as min_confidence,
    MAX(confidence) as max_confidence,
    COUNT(
        CASE
            WHEN similarity_score IS NOT NULL THEN 1
        END
    ) as relations_with_similarity,
    AVG(similarity_score) as avg_similarity_score
FROM entity_file_relations;
-- name: BatchCreateEntityFileRelations :exec
-- Batch create multiple entity-file relationships
INSERT INTO entity_file_relations (
        entity_name,
        file_id,
        relation_type,
        confidence,
        similarity_score,
        metadata,
        created_at
    )
VALUES (?, ?, ?, ?, ?, ?, ?);
-- name: BatchUpdateRelationConfidence :exec
-- Batch update confidence scores
UPDATE entity_file_relations
SET confidence = ?,
    similarity_score = ?
WHERE id = ?;
-- name: CleanupLowConfidenceRelations :exec
-- Remove relationships below confidence threshold
DELETE FROM entity_file_relations
WHERE confidence < ?;
-- name: GetOrphanedRelations :many
-- Find relations where entity or file no longer exists
SELECT efr.id,
    efr.entity_name,
    efr.file_id,
    efr.relation_type,
    CASE
        WHEN e.name IS NULL THEN 'missing_entity'
        WHEN f.id IS NULL THEN 'missing_file'
        ELSE 'valid'
    END as status
FROM entity_file_relations efr
    LEFT JOIN entities e ON efr.entity_name = e.name
    LEFT JOIN files f ON efr.file_id = f.id
WHERE e.name IS NULL
    OR f.id IS NULL;
-- name: GetTopRelatedFiles :many
-- Get files most related to an entity with ranking
SELECT f.id,
    f.workspace_id,
    f.file_path,
    f.size,
    f.mod_time,
    efr.relation_type,
    efr.confidence,
    efr.similarity_score
FROM entity_file_relations efr
    JOIN files f ON efr.file_id = f.id
WHERE efr.entity_name = ?
ORDER BY efr.confidence DESC,
    efr.similarity_score DESC
LIMIT ?;