package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	sqlsafety "github.com/Alevsk/respondent/internal/sqlsafety"
)

const (
	// defaultAnalysisRowCap bounds rows returned to the analysis/LLM stage even if
	// a query lacks its own LIMIT — a hard backstop against unbounded result sets.
	defaultAnalysisRowCap = 500
	// defaultAnalysisQueryTimeout bounds a single analysis query so a runaway
	// spatial join cannot hang an analysis tick.
	defaultAnalysisQueryTimeout = 30 * time.Second
)

// ReadOnlyQueryExecutor runs validated, read-only SQL against the SQLite database
// and returns rows as generic column->value maps. It satisfies the analysis
// engine's QueryExecutor interface structurally (no import of the analysis
// package), preserving the infra -> app dependency direction.
type ReadOnlyQueryExecutor struct {
	db      *sql.DB
	rowCap  int
	timeout time.Duration
}

// NewReadOnlyQueryExecutor builds an executor over the given DB with default
// row cap and timeout.
func NewReadOnlyQueryExecutor(db *sql.DB) *ReadOnlyQueryExecutor {
	return &ReadOnlyQueryExecutor{
		db:      db,
		rowCap:  defaultAnalysisRowCap,
		timeout: defaultAnalysisQueryTimeout,
	}
}

// QueryRows validates the SQL is read-only (single SELECT/CTE — no writes, PRAGMA,
// ATTACH or multiple statements), bounds it by row cap and timeout, and returns
// the rows. The row cap is enforced by wrapping the query as a subquery with an
// outer LIMIT (parameterized) rather than string-appending, so it composes safely
// with the query's own ORDER BY/LIMIT and with CTEs.
func (e *ReadOnlyQueryExecutor) QueryRows(ctx context.Context, query string) ([]map[string]any, error) {
	if err := sqlsafety.ValidateReadOnlySQL(query); err != nil {
		return nil, fmt.Errorf("read-only validation: %w", err)
	}

	ctx, cancel := context.WithTimeout(ctx, e.timeout)
	defer cancel()

	wrapped := fmt.Sprintf("SELECT * FROM (%s) LIMIT ?", query)
	rows, err := e.db.QueryContext(ctx, wrapped, e.rowCap)
	if err != nil {
		return nil, translateError("analysis-query", err)
	}
	defer func() { _ = rows.Close() }()

	cols, err := rows.Columns()
	if err != nil {
		return nil, translateError("analysis-query", err)
	}

	out := make([]map[string]any, 0)
	for rows.Next() {
		vals := make([]any, len(cols))
		ptrs := make([]any, len(cols))
		for i := range vals {
			ptrs[i] = &vals[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			return nil, translateError("analysis-query", err)
		}
		row := make(map[string]any, len(cols))
		for i, c := range cols {
			// SQLite TEXT often scans as []byte; surface it as string so CEL and
			// JSON encoding see a plain string.
			if b, ok := vals[i].([]byte); ok {
				row[c] = string(b)
			} else {
				row[c] = vals[i]
			}
		}
		out = append(out, row)
	}
	return out, rows.Err()
}
