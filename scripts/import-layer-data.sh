#!/usr/bin/env bash
# import-layer-data.sh
# Imports entities and observations from CSV dumps into a respondent-postgres
# container. Supports both local and remote (SSH) targets.
#
# Usage:
#   ./scripts/import-layer-data.sh [data_dir]
#   data_dir defaults to ./data-export/
#
# Environment:
#   RESPONDENT_PG_CONTAINER  Container name (default: respondent-postgres)
#   RESPONDENT_PG_USER       Postgres user (default: respondent)
#   RESPONDENT_PG_DB         Postgres database (default: respondent)
#   RESPONDENT_CONTAINER_RT  Container runtime: docker | podman (default: docker)
#   RESPONDENT_REMOTE_HOST   If set, import via SSH (e.g. devops@m910-1)
#
# Import behavior:
#   - Entities: ON CONFLICT (layer_type, external_id) DO NOTHING
#   - Observations: ON CONFLICT (entity_id, ts) DO NOTHING
#   - Entity IDs are remapped via (layer_type, external_id) lookup to handle
#     UUID mismatches between environments
#   - Safe to re-run (idempotent)

set -euo pipefail

REMOTE="${RESPONDENT_REMOTE_HOST:-}"
CONTAINER="${RESPONDENT_PG_CONTAINER:-respondent-postgres}"
RUNTIME="${RESPONDENT_CONTAINER_RT:-docker}"
DB_USER="${RESPONDENT_PG_USER:-respondent}"
DB_NAME="${RESPONDENT_PG_DB:-respondent}"
DATA_DIR="${1:-./data-export}"

ENTITIES_CSV="$DATA_DIR/entities.csv"
OBSERVATIONS_CSV="$DATA_DIR/observations.csv"

for f in "$ENTITIES_CSV" "$OBSERVATIONS_CSV"; do
  if [[ ! -f "$f" ]]; then
    echo "ERROR: $f not found. Run export-layer-data.sh first."
    exit 1
  fi
done

ENTITY_COUNT=$(tail -n +2 "$ENTITIES_CSV" | wc -l | tr -d ' ')
OBS_COUNT=$(tail -n +2 "$OBSERVATIONS_CSV" | wc -l | tr -d ' ')

echo "==> Importing layer data"
[[ -n "$REMOTE" ]] && echo "    Target: $REMOTE ($RUNTIME)"
[[ -z "$REMOTE" ]] && echo "    Target: local ($RUNTIME)"
echo "    Entities:     $ENTITY_COUNT"
echo "    Observations: $OBS_COUNT"
echo ""

# Helper: copy a file into the postgres container
copy_to_container() {
  local src="$1" dest="$2"
  if [[ -n "$REMOTE" ]]; then
    scp "$src" "$REMOTE:/tmp/$(basename "$dest")"
    ssh "$REMOTE" "$RUNTIME cp /tmp/$(basename "$dest") $CONTAINER:$dest"
  else
    $RUNTIME cp "$src" "$CONTAINER:$dest"
  fi
}

# Helper: run psql in the postgres container
run_psql() {
  if [[ -n "$REMOTE" ]]; then
    ssh "$REMOTE" "$RUNTIME exec -i $CONTAINER psql -U $DB_USER -d $DB_NAME"
  else
    $RUNTIME exec -i "$CONTAINER" psql -U "$DB_USER" -d "$DB_NAME"
  fi
}

# Helper: clean up a file from the container (and remote /tmp if applicable)
cleanup_file() {
  local path="$1"
  if [[ -n "$REMOTE" ]]; then
    ssh "$REMOTE" "rm -f /tmp/$(basename "$path")"
    ssh "$REMOTE" "$RUNTIME exec $CONTAINER rm -f $path"
  else
    $RUNTIME exec "$CONTAINER" rm -f "$path"
  fi
}

# 1. Copy CSV files into the container
echo "--- Copying CSV files..."
copy_to_container "$ENTITIES_CSV" "/tmp/import_entities.csv"
copy_to_container "$OBSERVATIONS_CSV" "/tmp/import_observations.csv"

# 2. Import in a single psql session (single transaction)
echo "--- Importing data (single transaction)..."
run_psql <<'EOSQL'
BEGIN;

-- === ENTITIES ===
CREATE TEMP TABLE _stg_entities (
  id UUID,
  external_id TEXT,
  layer_type TEXT,
  name TEXT,
  metadata JSONB,
  created_at TIMESTAMPTZ
);

\copy _stg_entities FROM '/tmp/import_entities.csv' WITH (FORMAT csv, HEADER true);

INSERT INTO entities (id, external_id, layer_type, name, metadata, created_at)
SELECT id, external_id, layer_type, name, metadata, created_at
FROM _stg_entities
ON CONFLICT (layer_type, external_id) DO NOTHING;

DO $$
DECLARE cnt BIGINT;
BEGIN
  SELECT COUNT(*) INTO cnt FROM _stg_entities;
  RAISE NOTICE 'Staged % entities', cnt;
END $$;

-- === OBSERVATIONS ===
CREATE TEMP TABLE _stg_observations (
  id UUID,
  entity_id UUID,
  ts TIMESTAMPTZ,
  position TEXT,
  altitude_m DOUBLE PRECISION,
  velocity JSONB,
  metadata JSONB,
  created_at TIMESTAMPTZ,
  lat DOUBLE PRECISION,
  lon DOUBLE PRECISION,
  source_type TEXT,
  content_hash TEXT
);

\copy _stg_observations FROM '/tmp/import_observations.csv' WITH (FORMAT csv, HEADER true);

-- Remap entity_id via (layer_type, external_id) to handle UUID differences
INSERT INTO observations (id, entity_id, ts, position, altitude_m, velocity, metadata, created_at, lat, lon, source_type, content_hash)
SELECT
  o.id,
  COALESCE(re.id, o.entity_id) AS entity_id,
  o.ts,
  CASE WHEN o.position IS NOT NULL AND o.position != '' THEN o.position::geography ELSE NULL END,
  o.altitude_m,
  o.velocity,
  o.metadata,
  o.created_at,
  o.lat,
  o.lon,
  o.source_type,
  o.content_hash
FROM _stg_observations o
LEFT JOIN _stg_entities se ON se.id = o.entity_id
LEFT JOIN entities re ON re.layer_type = se.layer_type AND re.external_id = se.external_id
ON CONFLICT (entity_id, ts) DO NOTHING;

DO $$
DECLARE cnt BIGINT;
BEGIN
  SELECT COUNT(*) INTO cnt FROM _stg_observations;
  RAISE NOTICE 'Staged % observations', cnt;
END $$;

COMMIT;
EOSQL

# 3. Clean up
echo "--- Cleaning up temp files..."
cleanup_file "/tmp/import_entities.csv"
cleanup_file "/tmp/import_observations.csv"

echo ""
echo "==> Import complete!"

# 4. Verify
echo "--- Verifying data..."
run_psql <<'EOSQL'
SELECT layer_type, COUNT(*) AS entities FROM entities GROUP BY layer_type ORDER BY layer_type;
EOSQL
