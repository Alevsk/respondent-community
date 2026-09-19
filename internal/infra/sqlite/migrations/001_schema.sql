-- Community edition schema: SQLite equivalent of the PostgreSQL enterprise schema.
-- Uses TEXT for UUIDs, TEXT for JSON, TEXT (RFC3339Nano) for timestamps, REAL for coordinates.

CREATE TABLE IF NOT EXISTS entities (
    id TEXT PRIMARY KEY,
    external_id TEXT NOT NULL,
    layer_type TEXT NOT NULL,
    name TEXT NOT NULL DEFAULT '',
    metadata TEXT NOT NULL DEFAULT '{}',
    ai_metadata TEXT NOT NULL DEFAULT '{}',
    source TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_entities_layer_external ON entities(layer_type, external_id);
CREATE INDEX IF NOT EXISTS idx_entities_layer_type ON entities(layer_type);
CREATE INDEX IF NOT EXISTS idx_entities_name ON entities(name COLLATE NOCASE);

CREATE TABLE IF NOT EXISTS observations (
    id TEXT PRIMARY KEY,
    entity_id TEXT NOT NULL REFERENCES entities(id) ON DELETE CASCADE,
    ts TEXT NOT NULL,
    event_time TEXT,
    event_end TEXT,
    lat REAL,
    lon REAL,
    altitude_m REAL NOT NULL DEFAULT 0,
    velocity TEXT NOT NULL DEFAULT '{}',
    metadata TEXT NOT NULL DEFAULT '{}',
    ai_metadata TEXT NOT NULL DEFAULT '{}',
    source_type TEXT NOT NULL DEFAULT '',
    content_hash TEXT NOT NULL DEFAULT '',
    source TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_observations_entity_ts ON observations(entity_id, ts);
CREATE INDEX IF NOT EXISTS idx_observations_ts_desc ON observations(entity_id, ts DESC);
CREATE INDEX IF NOT EXISTS idx_observations_content_hash ON observations(entity_id, content_hash);
CREATE INDEX IF NOT EXISTS idx_observations_layer_bbox ON observations(entity_id, lat, lon);

CREATE TABLE IF NOT EXISTS ai_insights (
    id TEXT PRIMARY KEY,
    insight_type TEXT NOT NULL,
    source_name TEXT NOT NULL,
    operation_name TEXT NOT NULL,
    layer_type TEXT,
    attention TEXT,
    attention_rank INTEGER,
    dedup_key TEXT,
    result TEXT NOT NULL DEFAULT '{}',
    expires_at TEXT,
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))
);

CREATE INDEX IF NOT EXISTS idx_ai_insights_type ON ai_insights(insight_type);
CREATE INDEX IF NOT EXISTS idx_ai_insights_layer ON ai_insights(layer_type);
CREATE INDEX IF NOT EXISTS idx_ai_insights_expires ON ai_insights(expires_at);
CREATE INDEX IF NOT EXISTS idx_ai_insights_dedup ON ai_insights(source_name, operation_name, dedup_key);
CREATE INDEX IF NOT EXISTS idx_ai_insights_created ON ai_insights(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_ai_insights_attention ON ai_insights(attention_rank);

CREATE TABLE IF NOT EXISTS ai_insight_refs (
    id TEXT PRIMARY KEY,
    insight_id TEXT NOT NULL REFERENCES ai_insights(id) ON DELETE CASCADE,
    entity_id TEXT,
    observation_id TEXT
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_ai_insight_refs_unique ON ai_insight_refs(insight_id, COALESCE(entity_id, ''), COALESCE(observation_id, ''));
CREATE INDEX IF NOT EXISTS idx_ai_insight_refs_entity ON ai_insight_refs(entity_id);
CREATE INDEX IF NOT EXISTS idx_ai_insight_refs_observation ON ai_insight_refs(observation_id);

CREATE TABLE IF NOT EXISTS ai_enrichment_log (
    id TEXT PRIMARY KEY,
    entity_id TEXT NOT NULL,
    observation_id TEXT,
    source_name TEXT NOT NULL,
    operation_name TEXT NOT NULL,
    prompt_hash TEXT,
    status TEXT NOT NULL DEFAULT 'pending',
    provider TEXT NOT NULL DEFAULT '',
    model TEXT NOT NULL DEFAULT '',
    prompt_tokens INTEGER NOT NULL DEFAULT 0,
    completion_tokens INTEGER NOT NULL DEFAULT 0,
    latency_ms INTEGER NOT NULL DEFAULT 0,
    error_message TEXT NOT NULL DEFAULT '',
    result TEXT,
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    completed_at TEXT
);

CREATE INDEX IF NOT EXISTS idx_enrichment_entity_op ON ai_enrichment_log(entity_id, operation_name);

CREATE TABLE IF NOT EXISTS layers (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    type TEXT NOT NULL UNIQUE,
    enabled INTEGER NOT NULL DEFAULT 1,
    mode TEXT NOT NULL DEFAULT 'sparse',
    density INTEGER NOT NULL DEFAULT 0,
    source TEXT NOT NULL DEFAULT '',
    last_update TEXT,
    count INTEGER NOT NULL DEFAULT 0,
    config TEXT NOT NULL DEFAULT '{}',
    color TEXT NOT NULL DEFAULT '#ffffff',
    point_size INTEGER NOT NULL DEFAULT 2,
    rendering_mode TEXT NOT NULL DEFAULT 'map',
    filtering_mode TEXT NOT NULL DEFAULT '',
    display_config TEXT,
    history_config TEXT
);
