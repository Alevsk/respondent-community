-- Global timestamp index on observations.
--
-- The existing indexes are entity-scoped (entity_id, ts). The size-cap retention
-- routine selects the OLDEST observations across ALL entities (ORDER BY ts ASC);
-- without a standalone ts index that is a full table scan plus a temp B-tree sort
-- on every prune batch, which degrades quadratically as the table grows. This
-- index makes oldest-first selection an index range scan.
CREATE INDEX IF NOT EXISTS idx_observations_ts ON observations(ts);
