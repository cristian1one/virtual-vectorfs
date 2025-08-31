-- name: CreateSnapshot :one
-- Create a new snapshot for a workspace
INSERT INTO snapshots (
        id,
        workspace_id,
        taken_at,
        directory_state,
        description,
        created_at
    )
VALUES (?, ?, ?, ?, ?, ?)
RETURNING id,
    workspace_id,
    taken_at,
    directory_state,
    description,
    created_at;
-- name: GetSnapshot :one
-- Get snapshot by ID
SELECT id,
    workspace_id,
    taken_at,
    directory_state,
    description,
    created_at
FROM snapshots
WHERE id = ?;
-- name: GetWorkspaceSnapshots :many
-- Get all snapshots for a workspace
SELECT id,
    workspace_id,
    taken_at,
    directory_state,
    description,
    created_at
FROM snapshots
WHERE workspace_id = ?
ORDER BY taken_at DESC;
-- name: DeleteSnapshot :exec
-- Delete snapshot by ID
DELETE FROM snapshots
WHERE id = ?;
-- name: GetLatestSnapshot :one
-- Get the most recent snapshot for a workspace
SELECT id,
    workspace_id,
    taken_at,
    directory_state,
    description,
    created_at
FROM snapshots
WHERE workspace_id = ?
ORDER BY taken_at DESC
LIMIT 1;
-- name: CreateOperation :one
-- Create a new operation history entry
INSERT INTO operation_history (
        id,
        workspace_id,
        operation_type,
        entity_path,
        old_path,
        new_path,
        metadata,
        performed_by,
        performed_at
    )
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
RETURNING id,
    workspace_id,
    operation_type,
    entity_path,
    old_path,
    new_path,
    metadata,
    performed_by,
    performed_at;
-- name: GetOperation :one
-- Get operation by ID
SELECT id,
    workspace_id,
    operation_type,
    entity_path,
    old_path,
    new_path,
    metadata,
    performed_by,
    performed_at
FROM operation_history
WHERE id = ?;
-- name: GetWorkspaceOperations :many
-- Get operation history for a workspace
SELECT id,
    workspace_id,
    operation_type,
    entity_path,
    old_path,
    new_path,
    metadata,
    performed_by,
    performed_at
FROM operation_history
WHERE workspace_id = ?
ORDER BY performed_at DESC
LIMIT ? OFFSET ?;
-- name: GetOperationsByType :many
-- Get operations of a specific type
SELECT id,
    workspace_id,
    operation_type,
    entity_path,
    old_path,
    new_path,
    metadata,
    performed_by,
    performed_at
FROM operation_history
WHERE operation_type = ?
ORDER BY performed_at DESC
LIMIT ? OFFSET ?;
-- name: GetOperationsByUser :many
-- Get operations performed by a specific user
SELECT id,
    workspace_id,
    operation_type,
    entity_path,
    old_path,
    new_path,
    metadata,
    performed_by,
    performed_at
FROM operation_history
WHERE performed_by = ?
ORDER BY performed_at DESC
LIMIT ? OFFSET ?;
-- name: GetRecentOperations :many
-- Get recent operations across all workspaces
SELECT id,
    workspace_id,
    operation_type,
    entity_path,
    old_path,
    new_path,
    metadata,
    performed_by,
    performed_at
FROM operation_history
ORDER BY performed_at DESC
LIMIT ? OFFSET ?;
-- name: GetOperationsByTimeRange :many
-- Get operations within a time range
SELECT id,
    workspace_id,
    operation_type,
    entity_path,
    old_path,
    new_path,
    metadata,
    performed_by,
    performed_at
FROM operation_history
WHERE performed_at >= ?
    AND performed_at <= ?
ORDER BY performed_at DESC
LIMIT ? OFFSET ?;
-- name: GetOperationStats :one
-- Get operation statistics for a workspace
SELECT COUNT(*) as total_operations,
    COUNT(DISTINCT operation_type) as unique_operation_types,
    COUNT(DISTINCT performed_by) as unique_users,
    MIN(performed_at) as first_operation,
    MAX(performed_at) as latest_operation
FROM operation_history
WHERE workspace_id = ?;
-- name: BatchCreateOperations :exec
-- Batch create multiple operation records
INSERT INTO operation_history (
        id,
        workspace_id,
        operation_type,
        entity_path,
        old_path,
        new_path,
        metadata,
        performed_by,
        performed_at
    )
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?);
-- name: CleanupOldOperations :exec
-- Remove operations older than specified date
DELETE FROM operation_history
WHERE performed_at < ?;
-- name: GetOperationSummary :many
-- Get operation summary by type and user
SELECT operation_type,
    performed_by,
    COUNT(*) as operation_count,
    MIN(performed_at) as first_operation,
    MAX(performed_at) as last_operation
FROM operation_history
WHERE workspace_id = ?
GROUP BY operation_type,
    performed_by
ORDER BY operation_count DESC;