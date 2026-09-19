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

var attentionRanks = map[string]int{
	"info":     0,
	"low":      1,
	"medium":   2,
	"high":     3,
	"critical": 4,
}

type aiInsightRepository struct {
	db     *sql.DB
	logger zerolog.Logger
}

// NewAIInsightRepository creates a new SQLite-backed AIInsightRepository.
func NewAIInsightRepository(db *sql.DB, logger zerolog.Logger) domain.AIInsightRepository {
	return &aiInsightRepository{db: db, logger: logger}
}

func (r *aiInsightRepository) Create(ctx context.Context, insight *domain.AIInsight) (string, error) {
	if insight.ID == "" {
		insight.ID = uuid.New().String()
	}

	var rank sql.NullInt64
	if insight.Attention != nil {
		if v, ok := attentionRanks[*insight.Attention]; ok {
			rank = sql.NullInt64{Int64: int64(v), Valid: true}
		}
	}

	// INSERT OR IGNORE makes Create idempotent against the UNIQUE dedup index
	// (idx_ai_insights_dedup on source_name, operation_name, dedup_key). A
	// duplicate dedup_key is silently ignored — the pre-generated insight.ID is
	// still returned so callers behave identically whether the row was new or a dup.
	_, err := r.db.ExecContext(ctx,
		`INSERT OR IGNORE INTO ai_insights
		 (id, insight_type, source_name, operation_name, layer_type, attention, attention_rank,
		  dedup_key, result, expires_at, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		insight.ID, insight.InsightType, insight.SourceName, insight.OperationName,
		nullString(insight.LayerType), nullString(insight.Attention), rank,
		nullString(insight.DedupKey), marshalJSON(insight.Result),
		nullTime(insight.ExpiresAt), formatTime(insight.CreatedAt),
	)
	if err != nil {
		return "", translateError("ai_insight", err)
	}
	return insight.ID, nil
}

func (r *aiInsightRepository) CreateRef(ctx context.Context, insightID string, entityID, observationID *string) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT OR IGNORE INTO ai_insight_refs (id, insight_id, entity_id, observation_id)
		 VALUES (?, ?, ?, ?)`,
		uuid.New().String(), insightID, nullString(entityID), nullString(observationID),
	)
	return translateError("ai_insight_ref", err)
}

func (r *aiInsightRepository) GetByID(ctx context.Context, id string) (*domain.AIInsight, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT id, insight_type, source_name, operation_name, layer_type, attention, attention_rank,
		        dedup_key, result, expires_at, created_at
		 FROM ai_insights WHERE id = ?`, id)

	insight, err := r.scanInsight(row)
	if err != nil {
		return nil, err
	}

	if err := r.populateRefs(ctx, []*domain.AIInsight{insight}); err != nil {
		return nil, err
	}
	return insight, nil
}

func (r *aiInsightRepository) List(ctx context.Context, filter domain.InsightFilter) ([]*domain.AIInsight, int, error) {
	where, args := r.buildWhere(filter)

	var total int
	countQuery := "SELECT COUNT(*) FROM ai_insights"
	if where != "" {
		countQuery += " WHERE " + where
	}
	if err := r.db.QueryRowContext(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, translateError("ai_insight", err)
	}

	limit := filter.Limit
	if limit <= 0 {
		limit = 50
	}
	offset := filter.Offset
	if offset < 0 {
		offset = 0
	}

	args = append(args, limit, offset)
	listQuery := `SELECT id, insight_type, source_name, operation_name, layer_type, attention, attention_rank,
	                     dedup_key, result, expires_at, created_at
	              FROM ai_insights`
	if where != "" {
		listQuery += " WHERE " + where
	}
	listQuery += " ORDER BY created_at DESC LIMIT ? OFFSET ?"

	rows, err := r.db.QueryContext(ctx, listQuery, args...)
	if err != nil {
		return nil, 0, translateError("ai_insight", err)
	}
	defer func() { _ = rows.Close() }()

	var insights []*domain.AIInsight
	for rows.Next() {
		insight, err := r.scanInsightFromRows(rows)
		if err != nil {
			return nil, 0, err
		}
		insights = append(insights, insight)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, translateError("ai_insight", err)
	}

	if err := r.populateRefs(ctx, insights); err != nil {
		return nil, 0, err
	}
	return insights, total, nil
}

func (r *aiInsightRepository) DeleteExpired(ctx context.Context) (int64, error) {
	result, err := r.db.ExecContext(ctx,
		"DELETE FROM ai_insights WHERE expires_at IS NOT NULL AND expires_at <= ?",
		formatTime(time.Now().UTC()),
	)
	if err != nil {
		return 0, translateError("ai_insight", err)
	}
	n, _ := result.RowsAffected()
	return n, nil
}

func (r *aiInsightRepository) GetRecentDedupKeys(ctx context.Context, sourceName, operationName string, since time.Time) (map[string]bool, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT dedup_key FROM ai_insights
		 WHERE source_name = ? AND operation_name = ? AND dedup_key IS NOT NULL AND created_at >= ?`,
		sourceName, operationName, formatTime(since),
	)
	if err != nil {
		return nil, translateError("ai_insight", err)
	}
	defer func() { _ = rows.Close() }()

	keys := make(map[string]bool)
	for rows.Next() {
		var key string
		if err := rows.Scan(&key); err != nil {
			return nil, translateError("ai_insight", err)
		}
		keys[key] = true
	}
	return keys, rows.Err()
}

// buildWhere constructs the WHERE clause and argument slice from an InsightFilter.
func (r *aiInsightRepository) buildWhere(filter domain.InsightFilter) (string, []any) {
	var clauses []string
	var args []any

	if filter.InsightType != "" {
		clauses = append(clauses, "insight_type = ?")
		args = append(args, filter.InsightType)
	}
	if filter.LayerType != "" {
		clauses = append(clauses, "layer_type = ?")
		args = append(args, filter.LayerType)
	}
	if filter.Attention != "" {
		clauses = append(clauses, "attention = ?")
		args = append(args, filter.Attention)
	}
	if filter.MinAttention != "" {
		if rank, ok := attentionRanks[filter.MinAttention]; ok {
			clauses = append(clauses, "attention_rank >= ?")
			args = append(args, rank)
		}
	}
	if filter.EntityID != "" {
		clauses = append(clauses, "id IN (SELECT insight_id FROM ai_insight_refs WHERE entity_id = ?)")
		args = append(args, filter.EntityID)
	}
	if filter.ObservationID != "" {
		clauses = append(clauses, "id IN (SELECT insight_id FROM ai_insight_refs WHERE observation_id = ?)")
		args = append(args, filter.ObservationID)
	}

	return strings.Join(clauses, " AND "), args
}

// populateRefs loads entity refs for a batch of insights.
func (r *aiInsightRepository) populateRefs(ctx context.Context, insights []*domain.AIInsight) error {
	if len(insights) == 0 {
		return nil
	}

	idIndex := make(map[string]*domain.AIInsight, len(insights))
	placeholders := make([]string, len(insights))
	args := make([]any, len(insights))
	for i, ins := range insights {
		idIndex[ins.ID] = ins
		placeholders[i] = "?"
		args[i] = ins.ID
	}

	query := fmt.Sprintf(
		`SELECT r.insight_id, r.entity_id, r.observation_id,
		        e.id, e.external_id, e.name, e.layer_type
		 FROM ai_insight_refs r
		 LEFT JOIN entities e ON e.id = r.entity_id
		 WHERE r.insight_id IN (%s)`,
		strings.Join(placeholders, ","),
	)

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return translateError("ai_insight_ref", err)
	}
	defer func() { _ = rows.Close() }()

	for rows.Next() {
		var (
			insightID     string
			entityID      sql.NullString
			observationID sql.NullString
			eID           sql.NullString
			eExternalID   sql.NullString
			eName         sql.NullString
			eLayerType    sql.NullString
		)
		if err := rows.Scan(&insightID, &entityID, &observationID, &eID, &eExternalID, &eName, &eLayerType); err != nil {
			return translateError("ai_insight_ref", err)
		}
		ins, ok := idIndex[insightID]
		if !ok {
			continue
		}
		if entityID.Valid {
			ins.EntityIDs = appendUnique(ins.EntityIDs, entityID.String)
			if eID.Valid {
				ins.Entities = appendUniqueRef(ins.Entities, domain.InsightEntityRef{
					ID:         eID.String,
					ExternalID: eExternalID.String,
					Name:       eName.String,
					LayerType:  eLayerType.String,
				})
			}
		}
		if observationID.Valid {
			ins.ObservationIDs = appendUnique(ins.ObservationIDs, observationID.String)
		}
	}
	return rows.Err()
}

func appendUnique(slice []string, s string) []string {
	for _, v := range slice {
		if v == s {
			return slice
		}
	}
	return append(slice, s)
}

func appendUniqueRef(slice []domain.InsightEntityRef, ref domain.InsightEntityRef) []domain.InsightEntityRef {
	for _, v := range slice {
		if v.ID == ref.ID {
			return slice
		}
	}
	return append(slice, ref)
}

// scanInsight scans one row from a *sql.Row into an AIInsight.
func (r *aiInsightRepository) scanInsight(row *sql.Row) (*domain.AIInsight, error) {
	var (
		ins                            domain.AIInsight
		layerType, attention, dedupKey sql.NullString
		attentionRank                  sql.NullInt64
		resultStr, createdAtStr        string
		expiresAt                      sql.NullString
	)
	err := row.Scan(
		&ins.ID, &ins.InsightType, &ins.SourceName, &ins.OperationName,
		&layerType, &attention, &attentionRank,
		&dedupKey, &resultStr, &expiresAt, &createdAtStr,
	)
	if err != nil {
		return nil, translateError("ai_insight", err)
	}
	ins.LayerType = stringPtr(layerType)
	ins.Attention = stringPtr(attention)
	ins.AttentionRank = intPtr(attentionRank)
	ins.DedupKey = stringPtr(dedupKey)
	ins.Result = unmarshalAnyMap(resultStr)
	ins.ExpiresAt = timePtr(expiresAt)
	ins.CreatedAt, _ = parseTime(createdAtStr)
	return &ins, nil
}

// scanInsightFromRows scans one row from *sql.Rows.
func (r *aiInsightRepository) scanInsightFromRows(rows *sql.Rows) (*domain.AIInsight, error) {
	var (
		ins                            domain.AIInsight
		layerType, attention, dedupKey sql.NullString
		attentionRank                  sql.NullInt64
		resultStr, createdAtStr        string
		expiresAt                      sql.NullString
	)
	err := rows.Scan(
		&ins.ID, &ins.InsightType, &ins.SourceName, &ins.OperationName,
		&layerType, &attention, &attentionRank,
		&dedupKey, &resultStr, &expiresAt, &createdAtStr,
	)
	if err != nil {
		return nil, translateError("ai_insight", err)
	}
	ins.LayerType = stringPtr(layerType)
	ins.Attention = stringPtr(attention)
	ins.AttentionRank = intPtr(attentionRank)
	ins.DedupKey = stringPtr(dedupKey)
	ins.Result = unmarshalAnyMap(resultStr)
	ins.ExpiresAt = timePtr(expiresAt)
	ins.CreatedAt, _ = parseTime(createdAtStr)
	return &ins, nil
}
