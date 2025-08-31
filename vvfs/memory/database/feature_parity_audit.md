# FEATURE PARITY AUDIT: Old Manual vs New sqlc Implementation (Updated)

## EXECUTIVE SUMMARY

This updated audit re-verifies the new sqlc + goose implementation against the legacy manual implementation to ensure we can safely remove `vvfs/memory/database-old/`.

Status: ✅ Parity achieved for all functional behaviors used by the old layer. One convenience wrapper (`GetRelations`) is not present as a named method, but its behavior is fully covered via `GetRelationsForEntities`. Schema initialization now relies on goose migrations rather than programmatic `initialize`.

---

## 🗂️ DETAILED FUNCTION-BY-FUNCTION ANALYSIS

### 1. Core Database Operations

| Old Function        | New Equivalent                         | Status  | Notes                                               |
| ------------------- | -------------------------------------- | ------- | --------------------------------------------------- |
| `Close()`           | `DBManager.Close()`                    | ✅ EXACT | Closes prepared statements and DBs                  |
| `GetRelations()`    | Covered by `GetRelationsForEntities()` | ✅ COVER | Wrapper not present; behavior available via helper  |
| `ensureFTSSchema()` | `DBManager.ensureFTSSchema()`          | ✅ EXACT | Creates `fts_observations` + triggers (best-effort) |

### 2. Search & Vector Operations

| Old Function       | New Equivalent                              | Status  | Notes                                                                     |
| ------------------ | ------------------------------------------- | ------- | ------------------------------------------------------------------------- |
| `SearchNodes()`    | `DBManager.SearchNodes()`                   | ✅ EXACT | Routes text vs vector queries                                             |
| `SearchEntities()` | `DBManager.SearchEntities()` + sqlc queries | ✅ EXACT | FTS5 (BM25) or LIKE fallback                                              |
| `SearchSimilar()`  | `DBManager.SearchSimilar()`                 | ✅ EXACT | Uses `vector_top_k` when available; client-side cosine fallback otherwise |

### 3. Vector Processing

| Old Function             | New Equivalent                 | Status  | Notes                           |
| ------------------------ | ------------------------------ | ------- | ------------------------------- |
| `vectorZeroString()`     | `DBManager.vectorZeroString()` | ✅ EXACT |                                 |
| `vectorToString()`       | `DBManager.vectorToString()`   | ✅ EXACT | Formats libSQL `vector32([..])` |
| `ExtractVector()`        | `DBManager.ExtractVector()`    | ✅ EXACT | Parses F32_BLOB to `[]float32`  |
| `coerceToFloat32Slice()` | `coerceToFloat32Slice()`       | ✅ EXACT | Utility maintained              |

### 4. Capabilities Detection

| Old Function                     | New Equivalent                             | Status  | Notes                                  |
| -------------------------------- | ------------------------------------------ | ------- | -------------------------------------- |
| `detectCapabilitiesForProject()` | `DBManager.detectCapabilitiesForProject()` | ✅ EXACT | Probes `vector_top_k` and FTS5 support |

### 5. Prepared Statement Caching

| Old Function        | New Equivalent                                     | Status  | Notes                         |
| ------------------- | -------------------------------------------------- | ------- | ----------------------------- |
| `getPreparedStmt()` | `DBManager.getPreparedStmt(ctx, project, db, sql)` | ✅ EXACT | Per-project SQL -> stmt cache |

### 6. Graph Operations

| Old Function                | New Equivalent                        | Status  | Notes                             |
| --------------------------- | ------------------------------------- | ------- | --------------------------------- |
| `GetRelationsForEntities()` | `DBManager.GetRelationsForEntities()` | ✅ EXACT | Same behavior (dynamic IN clause) |

### 7. Relationship Management

| Old Function        | New Equivalent                | Status  | Notes                      |
| ------------------- | ----------------------------- | ------- | -------------------------- |
| `CreateRelations()` | `DBManager.CreateRelations()` | ✅ EXACT | Inserts relations via sqlc |
| `UpdateRelations()` | `DBManager.UpdateRelations()` | ✅ EXACT | Delete/insert semantics    |

### 8. Entity CRUD Operations

| Old Function              | New Equivalent                      | Status  | Notes                                   |
| ------------------------- | ----------------------------------- | ------- | --------------------------------------- |
| `getEntityObservations()` | `DBManager.getEntityObservations()` | ✅ EXACT | Uses sqlc `GetEntityObservations`       |
| `CreateEntities()`        | `DBManager.CreateEntities()`        | ✅ EXACT | Upsert + observations rewrite           |
| `GetEntities()`           | `DBManager.GetEntities()`           | ✅ EXACT | Per-name retrieval via sqlc `GetEntity` |

### 9. Configuration Management

| Old Function  | New Equivalent         | Status  | Notes                       |
| ------------- | ---------------------- | ------- | --------------------------- |
| `NewConfig()` | `database.NewConfig()` | ✅ EXACT | Reads env, maps to `Config` |

### 10. Connection Management

| Old Function              | New Equivalent            | Status      | Notes                                                                      |
| ------------------------- | ------------------------- | ----------- | -------------------------------------------------------------------------- |
| `NewDBManager()`          | `database.NewDBManager()` | ✅ EXACT     | Validates dims, primes default project                                     |
| `getDB()`                 | `DBManager.getDB()`       | ✅ EXACT     | Per-project DB, pool tuning, caps detection, querier init                  |
| `detectDBEmbeddingDims()` | `detectDBEmbeddingDims()` | ✅ EXACT     | Schema/table-introspective fallback                                        |
| `initialize()`            | `DBManager.initialize()`  | ✅ DIFFERENT | No direct schema exec; relies on goose migrations (expected design change) |

---

## DELTAS AND NOTES

- `GetRelations` wrapper: Not present by name; covered by `GetRelationsForEntities`. If a 100% API name match is required, add a trivial wrapper delegating to `GetRelationsForEntities`.
- Schema initialization: Old code executed schema statements programmatically. New code assumes goose-managed migrations. Ensure migrations run prior to usage.
- Search enhancements: New layer adds `AdvancedSearch`, `HybridSearch` (RRF), and `FuzzySearch` helpers; they do not reduce parity.

---

## 📊 PARITY SUMMARY

All legacy behaviors are implemented or covered in the new layer. There are no functional blockers to removing `vvfs/memory/database-old/` provided migrations are applied before runtime.

---

## ✅ NEXT STEPS

1. Confirm goose migrations are applied in all environments (CI and runtime bootstrap).
2. Optionally add `GetRelations(ctx, projectName, entityNames []string)` wrapper for perfect API name continuity.
3. Proceed to remove `vvfs/memory/database-old/` after step 1 (and step 2 if name parity is desired).

Build status: go build ./... → ✅

