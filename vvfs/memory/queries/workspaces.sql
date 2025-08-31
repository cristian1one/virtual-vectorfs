-- name: CreateWorkspace :one
-- Create a new workspace
INSERT INTO workspaces (id, root_path, config, created_at, updated_at)
VALUES (?, ?, ?, ?, ?)
RETURNING id,
    root_path,
    config,
    created_at,
    updated_at;
-- name: GetWorkspace :one
-- Get workspace by ID
SELECT id,
    root_path,
    config,
    created_at,
    updated_at
FROM workspaces
WHERE id = ?;
-- name: GetWorkspaceByPath :one
-- Get workspace by root path
SELECT id,
    root_path,
    config,
    created_at,
    updated_at
FROM workspaces
WHERE root_path = ?;
-- name: UpdateWorkspace :one
-- Update workspace configuration
UPDATE workspaces
SET config = ?,
    updated_at = ?
WHERE id = ?
RETURNING id,
    root_path,
    config,
    created_at,
    updated_at;
-- name: DeleteWorkspace :exec
-- Delete workspace by ID
DELETE FROM workspaces
WHERE id = ?;
-- name: ListWorkspaces :many
-- List all workspaces with pagination
SELECT id,
    root_path,
    config,
    created_at,
    updated_at
FROM workspaces
ORDER BY created_at DESC
LIMIT ? OFFSET ?;
-- name: GetWorkspaceWithStats :one
-- Get workspace with comprehensive statistics using CTEs
WITH file_stats AS (
    SELECT workspace_id,
        COUNT(*) as total_files,
        SUM(size) as total_size,
        COUNT(
            CASE
                WHEN is_dir THEN 1
            END
        ) as directory_count,
        COUNT(
            CASE
                WHEN NOT is_dir THEN 1
            END
        ) as file_count,
        COUNT(
            CASE
                WHEN embedding IS NOT NULL THEN 1
            END
        ) as files_with_embeddings,
        AVG(size) as avg_file_size,
        MAX(mod_time) as latest_modification,
        MIN(created_at) as oldest_file
    FROM files
    WHERE files.workspace_id = ?
    GROUP BY files.workspace_id
),
snapshot_stats AS (
    SELECT workspace_id,
        COUNT(*) as snapshot_count,
        MAX(taken_at) as latest_snapshot
    FROM snapshots
    WHERE snapshots.workspace_id = ?
    GROUP BY snapshots.workspace_id
),
history_stats AS (
    SELECT workspace_id,
        COUNT(*) as total_operations,
        MAX(performed_at) as latest_activity
    FROM operation_history
    WHERE operation_history.workspace_id = ?
    GROUP BY operation_history.workspace_id
)
SELECT w.id,
    w.root_path,
    w.config,
    w.created_at,
    w.updated_at,
    COALESCE(f.total_files, 0) as total_files,
    COALESCE(f.total_size, 0) as total_size,
    COALESCE(f.directory_count, 0) as directory_count,
    COALESCE(f.file_count, 0) as file_count,
    COALESCE(f.files_with_embeddings, 0) as files_with_embeddings,
    COALESCE(f.avg_file_size, 0) as avg_file_size,
    f.latest_modification,
    f.oldest_file,
    COALESCE(s.snapshot_count, 0) as snapshot_count,
    s.latest_snapshot,
    COALESCE(h.total_operations, 0) as total_operations,
    h.latest_activity
FROM workspaces w
    LEFT JOIN file_stats f ON w.id = f.workspace_id
    LEFT JOIN snapshot_stats s ON w.id = s.workspace_id
    LEFT JOIN history_stats h ON w.id = h.workspace_id
WHERE w.id = ?;
-- name: GetWorkspaceSummary :many
-- Get summary of all workspaces with their statistics
WITH workspace_stats AS (
    SELECT w.id,
        w.root_path,
        w.created_at,
        COUNT(f.id) as file_count,
        SUM(f.size) as total_size,
        COUNT(
            CASE
                WHEN f.embedding IS NOT NULL THEN 1
            END
        ) as embedded_files,
        MAX(f.mod_time) as latest_activity
    FROM workspaces w
        LEFT JOIN files f ON w.id = f.workspace_id
    GROUP BY w.id,
        w.root_path,
        w.created_at
)
SELECT *
FROM workspace_stats
ORDER BY latest_activity DESC NULLS LAST,
    created_at DESC;
-- name: SearchWorkspaces :many
-- Search workspaces by root path
SELECT id,
    root_path,
    config,
    created_at,
    updated_at
FROM workspaces
WHERE root_path LIKE '%' || ? || '%'
ORDER BY root_path
LIMIT ? OFFSET ?;
-- name: GetWorkspaceActivity :many
-- Get recent activity for a workspace using window functions
WITH recent_operations AS (
    SELECT operation_type,
        entity_path,
        performed_by,
        performed_at,
        ROW_NUMBER() OVER (
            ORDER BY performed_at DESC
        ) as row_num
    FROM operation_history
    WHERE workspace_id = ?
    ORDER BY performed_at DESC
    LIMIT ?
)
SELECT *
FROM recent_operations
ORDER BY performed_at DESC;
-- name: GetWorkspaceFileDistribution :one
-- Get file type distribution and size statistics
SELECT COUNT(*) as total_files,
    SUM(
        CASE
            WHEN is_dir THEN 1
            ELSE 0
        END
    ) as directories,
    SUM(
        CASE
            WHEN NOT is_dir THEN 1
            ELSE 0
        END
    ) as regular_files,
    SUM(size) as total_size,
    AVG(size) as avg_file_size,
    MIN(size) as smallest_file,
    MAX(size) as largest_file,
    -- Size distribution
    COUNT(
        CASE
            WHEN size < 1024 THEN 1
        END
    ) as files_under_1kb,
    COUNT(
        CASE
            WHEN size BETWEEN 1024 AND 1048576 THEN 1
        END
    ) as files_1kb_to_1mb,
    COUNT(
        CASE
            WHEN size > 1048576 THEN 1
        END
    ) as files_over_1mb
FROM files
WHERE workspace_id = ?;
-- name: GetWorkspaceTopFiles :many
-- Get largest files in workspace
SELECT id,
    file_path,
    size,
    mod_time,
    is_dir,
    created_at
FROM files
WHERE workspace_id = ?
    AND NOT is_dir
ORDER BY size DESC
LIMIT ?;
-- name: BatchCreateWorkspaces :exec
-- Batch create multiple workspaces
INSERT INTO workspaces (id, root_path, config, created_at, updated_at)
VALUES (?, ?, ?, ?, ?);
-- name: BatchUpdateWorkspaceConfigs :exec
-- Batch update workspace configurations
UPDATE workspaces
SET config = ?,
    updated_at = ?
WHERE id = ?;
-- name: GetWorkspacesByActivity :many
-- Get workspaces ordered by recent activity
SELECT DISTINCT w.id,
    w.root_path,
    w.config,
    w.created_at,
    w.updated_at,
    MAX(
        COALESCE(f.mod_time, o.performed_at, w.updated_at)
    ) as latest_activity
FROM workspaces w
    LEFT JOIN files f ON w.id = f.workspace_id
    LEFT JOIN operation_history o ON w.id = o.workspace_id
GROUP BY w.id,
    w.root_path,
    w.config,
    w.created_at,
    w.updated_at
ORDER BY latest_activity DESC
LIMIT ? OFFSET ?;