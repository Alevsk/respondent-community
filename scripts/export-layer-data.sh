#!/usr/bin/env bash
# export-layer-data.sh
# Exports entities and observations for specified layer types from a
# respondent-postgres container into CSV files.
#
# Usage:
#   ./scripts/export-layer-data.sh <layer_type> [layer_type...] [-- output_dir]
#
# Examples:
#   ./scripts/export-layer-data.sh earthquakes radiation
#   ./scripts/export-layer-data.sh fires_active weather_alerts -- /tmp/backup
#   ./scripts/export-layer-data.sh --all
#   ./scripts/export-layer-data.sh --all -- /tmp/full-backup
#
# Environment:
#   RESPONDENT_PG_CONTAINER  Container name (default: respondent-postgres)
#   RESPONDENT_PG_USER       Postgres user (default: respondent)
#   RESPONDENT_PG_DB         Postgres database (default: respondent)
#   RESPONDENT_CONTAINER_RT  Container runtime: docker | podman (default: docker)
#   RESPONDENT_REMOTE_HOST   If set, run via SSH (e.g. devops@m910-1)

set -euo pipefail

CONTAINER="${RESPONDENT_PG_CONTAINER:-respondent-postgres}"
DB_USER="${RESPONDENT_PG_USER:-respondent}"
DB_NAME="${RESPONDENT_PG_DB:-respondent}"
RUNTIME="${RESPONDENT_CONTAINER_RT:-docker}"
REMOTE="${RESPONDENT_REMOTE_HOST:-}"
OUTPUT_DIR="./data-export"

# Parse arguments: layer types before --, output dir after --
LAYER_TYPES=()
EXPORT_ALL=false
while [[ $# -gt 0 ]]; do
  case "$1" in
    --all) EXPORT_ALL=true; shift ;;
    --)    shift; OUTPUT_DIR="${1:-./ data-export}"; shift ;;
    *)     LAYER_TYPES+=("$1"); shift ;;
  esac
done

# Helper: run a command inside the postgres container (local or remote)
run_pg() {
  if [[ -n "$REMOTE" ]]; then
    ssh "$REMOTE" "$RUNTIME exec $CONTAINER $*"
  else
    $RUNTIME exec "$CONTAINER" "$@"
  fi
}

# If --all, discover layer types from the database
if [[ "$EXPORT_ALL" == true ]]; then
  echo "--- Discovering layer types from database..."
  DISCOVERED=$(run_pg psql -U "$DB_USER" -d "$DB_NAME" -t -A -c \
    "SELECT DISTINCT layer_type FROM entities ORDER BY layer_type;")
  while IFS= read -r lt; do
    [[ -n "$lt" ]] && LAYER_TYPES+=("$lt")
  done <<< "$DISCOVERED"
fi

if [[ ${#LAYER_TYPES[@]} -eq 0 ]]; then
  echo "Usage: $0 <layer_type> [layer_type...] [-- output_dir]"
  echo "       $0 --all [-- output_dir]"
  exit 1
fi

mkdir -p "$OUTPUT_DIR"

echo "==> Exporting layer data from container: $CONTAINER"
[[ -n "$REMOTE" ]] && echo "    Remote: $REMOTE"
echo "    Layer types: ${LAYER_TYPES[*]}"
echo "    Output dir:  $OUTPUT_DIR"
echo ""

# Build the IN clause
IN_CLAUSE=$(printf "'%s'," "${LAYER_TYPES[@]}")
IN_CLAUSE="(${IN_CLAUSE%,})"

# 1. Export entities
echo "--- Exporting entities..."
run_pg psql -U "$DB_USER" -d "$DB_NAME" -c "\
COPY (
  SELECT id, external_id, layer_type, name, metadata, created_at
  FROM entities
  WHERE layer_type IN $IN_CLAUSE
  ORDER BY layer_type, external_id
) TO STDOUT WITH (FORMAT csv, HEADER true, FORCE_QUOTE *)
" > "$OUTPUT_DIR/entities.csv"

ENTITY_COUNT=$(tail -n +2 "$OUTPUT_DIR/entities.csv" | wc -l | tr -d ' ')
echo "    Exported $ENTITY_COUNT entities"

# 2. Export observations
echo "--- Exporting observations..."
run_pg psql -U "$DB_USER" -d "$DB_NAME" -c "\
COPY (
  SELECT o.id, o.entity_id, o.ts, o.position::text, o.altitude_m,
         o.velocity, o.metadata, o.created_at, o.lat, o.lon,
         o.source_type, o.content_hash
  FROM observations o
  INNER JOIN entities e ON e.id = o.entity_id
  WHERE e.layer_type IN $IN_CLAUSE
  ORDER BY o.entity_id, o.ts
) TO STDOUT WITH (FORMAT csv, HEADER true, FORCE_QUOTE *)
" > "$OUTPUT_DIR/observations.csv"

OBS_COUNT=$(tail -n +2 "$OUTPUT_DIR/observations.csv" | wc -l | tr -d ' ')
echo "    Exported $OBS_COUNT observations"

echo ""
echo "==> Export complete!"
echo "    $OUTPUT_DIR/entities.csv     ($ENTITY_COUNT rows)"
echo "    $OUTPUT_DIR/observations.csv ($OBS_COUNT rows)"
