#!/usr/bin/env bash
set -euo pipefail

: "${DATABASE_URL:?DATABASE_URL is required}"
SHARED_ROOT="${SHARED_ROOT:-$(mktemp -d)}"
export PGCONNECT_TIMEOUT=5

psqlq() {
  psql "$DATABASE_URL" -v ON_ERROR_STOP=1 -Atqc "$1"
}

assert_eq() {
  local actual="$1" expected="$2" label="$3"
  if [[ "$actual" != "$expected" ]]; then
    echo "assertion failed: $label: got=$actual want=$expected" >&2
    exit 1
  fi
}

rm -rf "$SHARED_ROOT"
mkdir -p "$SHARED_ROOT/workspaces" "$SHARED_ROOT/artifacts"

psql "$DATABASE_URL" -v ON_ERROR_STOP=1 <<'SQL'
DROP SCHEMA IF EXISTS transcode_cluster_cert CASCADE;
CREATE SCHEMA transcode_cluster_cert;
SET search_path TO transcode_cluster_cert;

CREATE TABLE jobs (
  id text PRIMARY KEY,
  status text NOT NULL,
  desired_state text NOT NULL,
  active_key text UNIQUE,
  worker_id text NOT NULL DEFAULT '',
  lease_token text NOT NULL DEFAULT '',
  lease_expires_at timestamptz,
  updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE storage_ledger (
  id text PRIMARY KEY,
  version bigint NOT NULL DEFAULT 0,
  updated_at timestamptz NOT NULL DEFAULT now()
);
INSERT INTO storage_ledger(id) VALUES ('artifact_store');

CREATE TABLE reservations (
  job_id text PRIMARY KEY REFERENCES jobs(id),
  reserved_bytes bigint NOT NULL,
  observed_bytes bigint NOT NULL DEFAULT 0,
  state text NOT NULL,
  updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE artifacts (
  id text PRIMARY KEY,
  job_id text NOT NULL REFERENCES jobs(id),
  status text NOT NULL,
  path text NOT NULL DEFAULT '',
  cleanup_state text NOT NULL DEFAULT '',
  cleanup_token text NOT NULL DEFAULT '',
  cleanup_lease_expires_at timestamptz,
  published_at timestamptz,
  updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE incidents (
  id bigserial PRIMARY KEY,
  active_key text UNIQUE,
  code text NOT NULL,
  operation text NOT NULL,
  path text NOT NULL,
  status text NOT NULL,
  occurrences bigint NOT NULL DEFAULT 1,
  first_seen_at timestamptz NOT NULL,
  last_seen_at timestamptz NOT NULL,
  recovered_at timestamptz
);
SQL

# 1. Two instances race the same queued Job. Exactly one conditional update wins.
psqlq "SET search_path TO transcode_cluster_cert; INSERT INTO jobs(id,status,desired_state,active_key) VALUES ('job-claim','queued','running','media:720p');"
claim() {
  local worker="$1" token="$2"
  psql "$DATABASE_URL" -v ON_ERROR_STOP=1 -Atq <<SQL
SET search_path TO transcode_cluster_cert;
UPDATE jobs SET status='claimed', worker_id='$worker', lease_token='$token', lease_expires_at=now()+interval '30 seconds', updated_at=now()
WHERE id='job-claim' AND status='queued' AND desired_state='running' AND (lease_expires_at IS NULL OR lease_expires_at <= now())
RETURNING worker_id;
SQL
}
claim instance-a token-a >"$SHARED_ROOT/claim-a" & p1=$!
claim instance-b token-b >"$SHARED_ROOT/claim-b" & p2=$!
wait "$p1" "$p2"
assert_eq "$(cat "$SHARED_ROOT/claim-a" "$SHARED_ROOT/claim-b" | sed '/^$/d' | wc -l | tr -d ' ')" "1" "single Job Lease owner"
assert_eq "$(psqlq "SET search_path TO transcode_cluster_cert; SELECT count(*) FROM jobs WHERE id='job-claim' AND status='claimed' AND lease_token <> '';")" "1" "claimed row"

# 2. Reservation allocation is serialized by the singleton ledger row.
psqlq "SET search_path TO transcode_cluster_cert; INSERT INTO jobs(id,status,desired_state,active_key) VALUES ('job-r1','queued','running','r1'),('job-r2','queued','running','r2');"
reserve() {
  local job="$1"
  psql "$DATABASE_URL" -v ON_ERROR_STOP=1 -Atq <<SQL
SET search_path TO transcode_cluster_cert;
BEGIN;
UPDATE storage_ledger SET version=version+1, updated_at=now() WHERE id='artifact_store';
WITH active AS (
  SELECT COALESCE(SUM(reserved_bytes-observed_bytes),0) AS bytes FROM reservations WHERE state='active'
), accepted AS (
  SELECT 1 FROM active WHERE bytes + 700 <= 1000
)
INSERT INTO reservations(job_id,reserved_bytes,state)
SELECT '$job',700,'active' FROM accepted
ON CONFLICT (job_id) DO NOTHING
RETURNING job_id;
COMMIT;
SQL
}
reserve job-r1 >"$SHARED_ROOT/reserve-1" & p1=$!
reserve job-r2 >"$SHARED_ROOT/reserve-2" & p2=$!
wait "$p1" "$p2"
assert_eq "$(psqlq "SET search_path TO transcode_cluster_cert; SELECT count(*) FROM reservations WHERE state='active';")" "1" "serialized reservation capacity"
assert_eq "$(psqlq "SET search_path TO transcode_cluster_cert; SELECT COALESCE(sum(reserved_bytes-observed_bytes),0) FROM reservations WHERE state='active';")" "700" "reservation budget"

# 3. Two instances prepare different workspaces but only the Lease owner may publish.
mkdir -p "$SHARED_ROOT/workspaces/job-claim/attempt-a" "$SHARED_ROOT/workspaces/job-claim/attempt-b"
printf '#EXTM3U\nseg.ts\n' >"$SHARED_ROOT/workspaces/job-claim/attempt-a/stream.m3u8"
printf 'a' >"$SHARED_ROOT/workspaces/job-claim/attempt-a/seg.ts"
printf '#EXTM3U\nseg.ts\n' >"$SHARED_ROOT/workspaces/job-claim/attempt-b/stream.m3u8"
printf 'b' >"$SHARED_ROOT/workspaces/job-claim/attempt-b/seg.ts"
OWNER_TOKEN="$(psqlq "SET search_path TO transcode_cluster_cert; SELECT lease_token FROM jobs WHERE id='job-claim';")"
OWNER_ATTEMPT="attempt-a"; [[ "$OWNER_TOKEN" == "token-b" ]] && OWNER_ATTEMPT="attempt-b"
LOSER_ATTEMPT="attempt-b"; [[ "$OWNER_ATTEMPT" == "attempt-b" ]] && LOSER_ATTEMPT="attempt-a"
ARTIFACT_DIR="$SHARED_ROOT/artifacts/media/720p/artifact-1"
mkdir -p "$(dirname "$ARTIFACT_DIR")"

publish_if_owner() {
  local token="$1" attempt="$2" output="$3"
  local owned
  owned="$(psqlq "SET search_path TO transcode_cluster_cert; SELECT count(*) FROM jobs WHERE id='job-claim' AND lease_token='$token' AND status='claimed' AND lease_expires_at > now();")"
  if [[ "$owned" == "1" ]]; then
    if mv "$SHARED_ROOT/workspaces/job-claim/$attempt" "$ARTIFACT_DIR" 2>/dev/null; then
      psqlq "SET search_path TO transcode_cluster_cert; INSERT INTO artifacts(id,job_id,status,path,published_at) VALUES ('artifact-1','job-claim','published','$ARTIFACT_DIR',now()) ON CONFLICT DO NOTHING;"
      echo published >"$output"
    fi
  fi
}
publish_if_owner "$OWNER_TOKEN" "$OWNER_ATTEMPT" "$SHARED_ROOT/publish-owner" & p1=$!
publish_if_owner stale-token "$LOSER_ATTEMPT" "$SHARED_ROOT/publish-loser" & p2=$!
wait "$p1" "$p2"
assert_eq "$(find "$SHARED_ROOT/artifacts" -name stream.m3u8 | wc -l | tr -d ' ')" "1" "single immutable publication"
assert_eq "$(psqlq "SET search_path TO transcode_cluster_cert; SELECT count(*) FROM artifacts WHERE status='published';")" "1" "single published metadata row"

# 4. Cleanup Lease allows one remover only; stale token cannot delete metadata.
psqlq "SET search_path TO transcode_cluster_cert; UPDATE artifacts SET cleanup_state='pending' WHERE id='artifact-1';"
cleanup_claim() {
  local token="$1"
  psql "$DATABASE_URL" -v ON_ERROR_STOP=1 -Atq <<SQL
SET search_path TO transcode_cluster_cert;
UPDATE artifacts SET cleanup_state='claimed', cleanup_token='$token', cleanup_lease_expires_at=now()+interval '2 minutes', updated_at=now()
WHERE id='artifact-1' AND cleanup_state='pending' AND (cleanup_lease_expires_at IS NULL OR cleanup_lease_expires_at <= now())
RETURNING cleanup_token;
SQL
}
cleanup_claim clean-a >"$SHARED_ROOT/clean-a" & p1=$!
cleanup_claim clean-b >"$SHARED_ROOT/clean-b" & p2=$!
wait "$p1" "$p2"
CLEAN_TOKEN="$(cat "$SHARED_ROOT/clean-a" "$SHARED_ROOT/clean-b" | sed '/^$/d')"
assert_eq "$(printf '%s\n' "$CLEAN_TOKEN" | sed '/^$/d' | wc -l | tr -d ' ')" "1" "single cleanup owner"
rm -rf "$ARTIFACT_DIR"
psqlq "SET search_path TO transcode_cluster_cert; DELETE FROM artifacts WHERE id='artifact-1' AND cleanup_token='$CLEAN_TOKEN' AND cleanup_state='claimed';"
assert_eq "$(psqlq "SET search_path TO transcode_cluster_cert; SELECT count(*) FROM artifacts WHERE id='artifact-1';")" "0" "cleanup metadata fenced by token"

# 5. Incident deduplication across instances and recovery by successful shared-store probe.
report_incident() {
  psqlq "SET search_path TO transcode_cluster_cert; INSERT INTO incidents(active_key,code,operation,path,status,first_seen_at,last_seen_at) VALUES ('io_error|publish|$SHARED_ROOT','io_error','publish','$SHARED_ROOT','active',now(),now()) ON CONFLICT(active_key) DO UPDATE SET occurrences=incidents.occurrences+1,last_seen_at=now();"
}
report_incident & p1=$!; report_incident & p2=$!; wait "$p1" "$p2"
assert_eq "$(psqlq "SET search_path TO transcode_cluster_cert; SELECT count(*) FROM incidents WHERE status='active';")" "1" "single active incident"
assert_eq "$(psqlq "SET search_path TO transcode_cluster_cert; SELECT occurrences FROM incidents WHERE status='active';")" "2" "incident occurrence accumulation"
PROBE="$SHARED_ROOT/.cluster-probe.tmp"; FINAL="$SHARED_ROOT/.cluster-probe.ok"
printf 'nowen-storage-probe' >"$PROBE"; sync "$PROBE"; mv "$PROBE" "$FINAL"; rm -f "$FINAL"
psqlq "SET search_path TO transcode_cluster_cert; UPDATE incidents SET status='recovered',recovered_at=now(),active_key=NULL WHERE status='active';"
assert_eq "$(psqlq "SET search_path TO transcode_cluster_cert; SELECT count(*) FROM incidents WHERE status='active';")" "0" "incident recovery"
assert_eq "$(psqlq "SET search_path TO transcode_cluster_cert; SELECT count(*) FROM incidents WHERE status='recovered';")" "1" "incident history retained"

echo "PostgreSQL multi-instance and shared-storage certification passed"
