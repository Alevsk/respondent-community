package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"

	"github.com/Alevsk/respondent/internal/domain"
)

type aiEnrichmentLogRepository struct {
	db     *sql.DB
	logger zerolog.Logger
}

// NewAIEnrichmentLogRepository creates a new SQLite-backed AIEnrichmentLogRepository.
func NewAIEnrichmentLogRepository(db *sql.DB, logger zerolog.Logger) domain.AIEnrichmentLogRepository {
	return &aiEnrichmentLogRepository{db: db, logger: logger}
}

func (r *aiEnrichmentLogRepository) Create(ctx context.Context, log *domain.AIEnrichmentLog) error {
	if log.ID == "" {
		log.ID = uuid.New().String()
	}
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO ai_enrichment_log
		 (id, entity_id, observation_id, source_name, operation_name, prompt_hash,
		  status, provider, model, prompt_tokens, completion_tokens, latency_ms,
		  error_message, result, created_at, completed_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		log.ID, log.EntityID, nullString(log.ObservationID),
		log.SourceName, log.OperationName, nullString(log.PromptHash),
		log.Status, log.Provider, log.Model,
		log.PromptTokens, log.CompletionTokens, log.LatencyMS,
		log.ErrorMessage, marshalJSON(log.Result),
		formatTime(log.CreatedAt), nullTime(log.CompletedAt),
	)
	return translateError("ai_enrichment_log", err)
}

func (r *aiEnrichmentLogRepository) UpdateStatus(ctx context.Context, id string, status string, result map[string]any, usage *domain.AIUsage, errMsg string) error {
	now := time.Now().UTC()
	var provider, model string
	var promptTokens, completionTokens, latencyMS int

	if usage != nil {
		provider = usage.Provider
		model = usage.Model
		promptTokens = usage.PromptTokens
		completionTokens = usage.CompletionTokens
		latencyMS = usage.LatencyMS
	}

	res, err := r.db.ExecContext(ctx,
		`UPDATE ai_enrichment_log
		 SET status = ?, result = ?, provider = ?, model = ?,
		     prompt_tokens = ?, completion_tokens = ?, latency_ms = ?,
		     error_message = ?, completed_at = ?
		 WHERE id = ?`,
		status, marshalJSON(result), provider, model,
		promptTokens, completionTokens, latencyMS,
		errMsg, formatTime(now), id,
	)
	if err != nil {
		return translateError("ai_enrichment_log", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return domain.NewNotFoundError("ai_enrichment_log not found", nil)
	}
	return nil
}

// DeleteStrandedLogs reconciles audit rows left in a non-terminal status by a
// crash/timeout/panic before a terminal UpdateStatus. Rather than deleting the
// rows (which would lose the audit trail) it transitions them to 'failed' with a
// diagnostic error message. Age is computed from created_at because the schema
// has no updated_at column.
func (r *aiEnrichmentLogRepository) DeleteStrandedLogs(ctx context.Context, olderThan time.Duration, statuses []string) (int64, error) {
	if len(statuses) == 0 {
		return 0, nil
	}
	placeholders := make([]string, len(statuses))
	args := make([]any, 0, len(statuses)+2)
	args = append(args, "orphaned: restart/timeout", formatTime(time.Now().UTC()))
	for i, s := range statuses {
		placeholders[i] = "?"
		args = append(args, s)
	}
	cutoff := formatTime(time.Now().UTC().Add(-olderThan))
	args = append(args, cutoff)
	q := fmt.Sprintf(
		`UPDATE ai_enrichment_log SET status = 'failed', error_message = ?, completed_at = ?
		 WHERE status IN (%s) AND created_at < ?`, strings.Join(placeholders, ","))
	res, err := r.db.ExecContext(ctx, q, args...)
	if err != nil {
		return 0, translateError("ai_enrichment_log", err)
	}
	n, _ := res.RowsAffected()
	return n, nil
}

func (r *aiEnrichmentLogRepository) GetByEntityAndOperation(ctx context.Context, entityID, operationName string) (*domain.AIEnrichmentLog, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT id, entity_id, observation_id, source_name, operation_name, prompt_hash,
		        status, provider, model, prompt_tokens, completion_tokens, latency_ms,
		        error_message, result, created_at, completed_at
		 FROM ai_enrichment_log
		 WHERE entity_id = ? AND operation_name = ?
		 ORDER BY created_at DESC
		 LIMIT 1`,
		entityID, operationName,
	)
	return r.scanLog(row)
}

func (r *aiEnrichmentLogRepository) scanLog(row *sql.Row) (*domain.AIEnrichmentLog, error) {
	var (
		log                       domain.AIEnrichmentLog
		observationID, promptHash sql.NullString
		resultStr, createdAtStr   string
		completedAt               sql.NullString
	)
	err := row.Scan(
		&log.ID, &log.EntityID, &observationID,
		&log.SourceName, &log.OperationName, &promptHash,
		&log.Status, &log.Provider, &log.Model,
		&log.PromptTokens, &log.CompletionTokens, &log.LatencyMS,
		&log.ErrorMessage, &resultStr, &createdAtStr, &completedAt,
	)
	if err != nil {
		return nil, translateError("ai_enrichment_log", err)
	}
	log.ObservationID = stringPtr(observationID)
	log.PromptHash = stringPtr(promptHash)
	log.Result = unmarshalAnyMap(resultStr)
	log.CreatedAt, _ = parseTime(createdAtStr)
	log.CompletedAt = timePtr(completedAt)
	return &log, nil
}
