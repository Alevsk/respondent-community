-- Deduplicate any existing rows before enforcing uniqueness. NULL dedup_key rows
-- are distinct under SQLite NULL semantics and are intentionally left untouched.
DELETE FROM ai_insights
WHERE dedup_key IS NOT NULL
  AND id NOT IN (
    SELECT MIN(id) FROM ai_insights
    WHERE dedup_key IS NOT NULL
    GROUP BY source_name, operation_name, dedup_key
  );

DROP INDEX IF EXISTS idx_ai_insights_dedup;

CREATE UNIQUE INDEX IF NOT EXISTS idx_ai_insights_dedup
  ON ai_insights(source_name, operation_name, dedup_key);
