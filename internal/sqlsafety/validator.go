// Package query provides SQL validation for LLM-generated queries.
// It uses pg_query_go (the real PostgreSQL parser via cgo) to parse SQL
// into an AST and verify every statement is a read-only SELECT.
package sqlsafety

import (
	"fmt"
	"time"

	pg_query "github.com/pganalyze/pg_query_go/v6"
)

// ValidateReadOnlySQL parses SQL into a PostgreSQL AST and verifies every
// statement is a read-only SELECT. Returns an error if any mutation,
// DDL, or utility statement is found.
//
// This is a critical security boundary: LLM-generated SQL must pass this
// validation before execution. It is one layer in a defense-in-depth strategy
// that also includes read-only DB connections, statement timeouts, row limits,
// and restricted DB users.
func ValidateReadOnlySQL(sql string) error {
	if sql == "" {
		return fmt.Errorf("empty SQL input")
	}

	result, err := pg_query.Parse(sql)
	if err != nil {
		return fmt.Errorf("SQL parse error: %w", err)
	}

	if len(result.Stmts) == 0 {
		return fmt.Errorf("no statements found")
	}

	// Reject multiple statements to prevent piggybacking attacks
	// (e.g., "SELECT 1; DROP TABLE entities").
	if len(result.Stmts) > 1 {
		return fmt.Errorf("multiple statements are not allowed: found %d statements", len(result.Stmts))
	}

	for _, rawStmt := range result.Stmts {
		stmt := rawStmt.GetStmt()
		if stmt == nil {
			return fmt.Errorf("empty statement")
		}

		if err := validateStatement(stmt); err != nil {
			return err
		}
	}

	return nil
}

// validateStatement checks a single AST node to ensure it is a read-only SELECT.
// Any statement type other than SELECT is rejected.
func validateStatement(stmt *pg_query.Node) error {
	switch {
	case stmt.GetSelectStmt() != nil:
		return validateSelectStmt(stmt.GetSelectStmt())

	// Reject all mutation statements.
	case stmt.GetInsertStmt() != nil:
		return fmt.Errorf("INSERT statements are not allowed")
	case stmt.GetUpdateStmt() != nil:
		return fmt.Errorf("UPDATE statements are not allowed")
	case stmt.GetDeleteStmt() != nil:
		return fmt.Errorf("DELETE statements are not allowed")
	case stmt.GetMergeStmt() != nil:
		return fmt.Errorf("MERGE statements are not allowed")

	// Reject DDL.
	case stmt.GetCreateStmt() != nil:
		return fmt.Errorf("CREATE statements are not allowed")
	case stmt.GetDropStmt() != nil:
		return fmt.Errorf("DROP statements are not allowed")
	case stmt.GetAlterTableStmt() != nil:
		return fmt.Errorf("ALTER statements are not allowed")
	case stmt.GetTruncateStmt() != nil:
		return fmt.Errorf("TRUNCATE statements are not allowed")

	// Reject privilege/role commands.
	case stmt.GetGrantStmt() != nil:
		return fmt.Errorf("GRANT statements are not allowed")
	case stmt.GetGrantRoleStmt() != nil:
		return fmt.Errorf("GRANT ROLE statements are not allowed")

	// Reject COPY, EXECUTE, CALL.
	case stmt.GetCopyStmt() != nil:
		return fmt.Errorf("COPY statements are not allowed")
	case stmt.GetExecuteStmt() != nil:
		return fmt.Errorf("EXECUTE statements are not allowed")
	case stmt.GetCallStmt() != nil:
		return fmt.Errorf("CALL statements are not allowed")

	// Reject transaction control.
	case stmt.GetTransactionStmt() != nil:
		return fmt.Errorf("transaction control statements are not allowed")

	// Reject SET/RESET.
	case stmt.GetVariableSetStmt() != nil:
		return fmt.Errorf("SET statements are not allowed")

	// Reject EXPLAIN (could be used to probe schema).
	case stmt.GetExplainStmt() != nil:
		return fmt.Errorf("EXPLAIN statements are not allowed")

	default:
		// Reject anything we don't explicitly allow.
		return fmt.Errorf("statement type not allowed: only SELECT is permitted")
	}
}

// validateSelectStmt checks that a SELECT does not contain CTEs with mutations,
// SELECT INTO clauses, or locking clauses.
func validateSelectStmt(sel *pg_query.SelectStmt) error {
	// Check CTEs for mutations (WITH ... INSERT/UPDATE/DELETE ... RETURNING).
	if sel.WithClause != nil {
		for _, cte := range sel.WithClause.Ctes {
			commonTE := cte.GetCommonTableExpr()
			if commonTE != nil && commonTE.Ctequery != nil {
				if err := validateStatement(commonTE.Ctequery); err != nil {
					return fmt.Errorf("CTE contains forbidden statement: %w", err)
				}
			}
		}
	}

	// Check for SELECT ... INTO (creates a table).
	if sel.IntoClause != nil {
		return fmt.Errorf("SELECT INTO is not allowed")
	}

	// Check for locking clauses (FOR UPDATE/SHARE).
	if len(sel.LockingClause) > 0 {
		return fmt.Errorf("locking clauses (FOR UPDATE/SHARE) are not allowed")
	}

	// Recursively validate set operations (UNION, INTERSECT, EXCEPT).
	if sel.Larg != nil {
		if err := validateSelectStmt(sel.Larg); err != nil {
			return err
		}
	}
	if sel.Rarg != nil {
		if err := validateSelectStmt(sel.Rarg); err != nil {
			return err
		}
	}

	return nil
}

// EnforceLimitAndTimeout wraps validated SQL with safety bounds.
// It returns two statements:
//   - setStmt: a SET LOCAL statement_timeout command (must be executed within a transaction)
//   - queryStmt: the original SQL, with LIMIT added if not already present
//
// The caller MUST execute these within an explicit database transaction
// (e.g., pgx.Pool.Begin) so that SET LOCAL takes effect and is scoped
// to that transaction only.
//
// Usage:
//
//	tx, _ := pool.Begin(ctx)
//	defer tx.Rollback(ctx)
//	setStmt, queryStmt := query.EnforceLimitAndTimeout(validatedSQL, 1000, 5*time.Second)
//	tx.Exec(ctx, setStmt)
//	rows, _ := tx.Query(ctx, queryStmt)
//	tx.Commit(ctx)
func EnforceLimitAndTimeout(sql string, maxRows int, timeout time.Duration) (setStmt, queryStmt string) {
	result, err := pg_query.Parse(sql)
	if err == nil && len(result.Stmts) > 0 {
		sel := result.Stmts[0].GetStmt().GetSelectStmt()
		if sel != nil && sel.LimitCount == nil {
			sql += fmt.Sprintf(" LIMIT %d", maxRows)
		}
	}

	setStmt = fmt.Sprintf("SET LOCAL statement_timeout = '%dms'", timeout.Milliseconds())
	queryStmt = sql
	return setStmt, queryStmt
}
