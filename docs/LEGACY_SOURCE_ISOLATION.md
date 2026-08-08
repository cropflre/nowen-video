# Legacy Source Isolation

This phase is the first schema action after the legacy transcode retirement
review. It remains explicit, reversible and fail-closed.

It does **not** delete the old source. A successful isolation renames:

```text
transcode_tasks
    -> legacy_transcode_tasks_retired_v1
```

Every row, column and index remains in the same database. There is no scheduled
worker, date-triggered action or `DROP TABLE` statement.

## Preconditions

An administrator can isolate the source only when all of these conditions still
hold inside the same database transaction:

- the requested removal plan is the latest plan;
- the plan uses `legacy-source-removal-plan/v1`, is `prepared`, and has not
  reached its 24-hour expiration time;
- its approved retirement decision is still the latest decision;
- the current migration generation, complete source inventory, rollback-window
  evidence and evidence hash still match;
- the current portable column schema hash still matches;
- the source row count still equals the plan row count;
- `transcode_tasks` exists and the archive table does not;
- the exact confirmation phrase is supplied.

The rename and audit insert run in one transaction. After the rename, the service
checks the archive table name, row count and normalized schema hash again. Any
mismatch returns an error and rolls the transaction back.

## Administrator API

All endpoints require an authenticated administrator:

```text
GET  /api/admin/legacy-source-retirement/:source/isolation
POST /api/admin/legacy-source-retirement/:source/isolations
POST /api/admin/legacy-source-retirement/:source/isolation-rollbacks
```

Isolation request:

```json
{
  "expected_plan_id": "<latest removal plan id>",
  "expected_evidence_hash": "<plan evidence hash>",
  "expected_schema_hash": "<plan schema hash>",
  "confirmation": "ISOLATE transcode_tasks AS legacy_transcode_tasks_retired_v1",
  "reason": "approved maintenance change"
}
```

Rollback request:

```json
{
  "expected_isolation_id": "<isolation record id>",
  "expected_schema_hash": "<isolation schema hash>",
  "confirmation": "RESTORE legacy_transcode_tasks_retired_v1 AS transcode_tasks",
  "reason": "emergency downgrade"
}
```

Exact request retries are idempotent. The service returns the existing isolation
or rollback record instead of repeating DDL.

## Audit and rollback

`legacy_source_isolations` stores the removal plan, approval, evidence, schema,
backup proof, row count, reviewer and archive name. The record is append-only.

`legacy_source_isolation_rollbacks` records the explicit reverse rename. Rollback
is intentionally allowed after the removal plan expires because it is the
emergency downgrade path. Before restoration, the archive row count and schema
hash must still match the isolation record.

After rollback, the normal legacy projection source check can open another
bounded generation when an older server has changed legacy rows. Isolation never
pretends that a downgrade or restored source remained quiescent.

## Operational boundary

A later permanent-removal phase, if ever approved, must be a separate protocol
with a new backup and restore proof, a new observation period after isolation,
and another explicit administrator decision. This phase provides no permanent
removal endpoint.
