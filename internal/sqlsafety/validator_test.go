package sqlsafety

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateReadOnlySQL_AllowedStatements(t *testing.T) {
	tests := []struct {
		name string
		sql  string
	}{
		{
			name: "simple SELECT",
			sql:  "SELECT 1",
		},
		{
			name: "SELECT from table",
			sql:  "SELECT id, name FROM entities",
		},
		{
			name: "SELECT with WHERE",
			sql:  "SELECT * FROM entities WHERE layer_type = 'flights'",
		},
		{
			name: "SELECT with JOIN",
			sql:  "SELECT e.id, o.ts FROM entities e JOIN observations o ON o.entity_id = e.id",
		},
		{
			name: "SELECT with multiple JOINs",
			sql:  "SELECT e.id, o.ts, m.value FROM entities e JOIN observations o ON o.entity_id = e.id LEFT JOIN metadata m ON m.entity_id = e.id",
		},
		{
			name: "SELECT with subquery",
			sql:  "SELECT * FROM entities WHERE id IN (SELECT entity_id FROM observations WHERE ts > now() - interval '1 hour')",
		},
		{
			name: "SELECT with correlated subquery",
			sql:  "SELECT e.* FROM entities e WHERE EXISTS (SELECT 1 FROM observations o WHERE o.entity_id = e.id)",
		},
		{
			name: "SELECT with read-only CTE",
			sql:  "WITH recent AS (SELECT * FROM observations WHERE ts > now() - interval '1 hour') SELECT * FROM recent",
		},
		{
			name: "SELECT with multiple CTEs",
			sql:  "WITH a AS (SELECT 1 AS x), b AS (SELECT 2 AS y) SELECT * FROM a, b",
		},
		{
			name: "SELECT with recursive CTE",
			sql:  "WITH RECURSIVE cte AS (SELECT 1 AS n UNION ALL SELECT n + 1 FROM cte WHERE n < 10) SELECT * FROM cte",
		},
		{
			name: "SELECT with LIMIT",
			sql:  "SELECT * FROM entities LIMIT 100",
		},
		{
			name: "SELECT with LIMIT and OFFSET",
			sql:  "SELECT * FROM entities LIMIT 100 OFFSET 50",
		},
		{
			name: "SELECT with ORDER BY",
			sql:  "SELECT * FROM entities ORDER BY created_at DESC",
		},
		{
			name: "SELECT with GROUP BY and HAVING",
			sql:  "SELECT layer_type, count(*) FROM entities GROUP BY layer_type HAVING count(*) > 10",
		},
		{
			name: "SELECT with aggregate functions",
			sql:  "SELECT count(*), avg(altitude), max(speed) FROM observations",
		},
		{
			name: "SELECT with DISTINCT",
			sql:  "SELECT DISTINCT layer_type FROM entities",
		},
		{
			name: "SELECT with CASE expression",
			sql:  "SELECT CASE WHEN altitude > 10000 THEN 'high' ELSE 'low' END FROM observations",
		},
		{
			name: "SELECT with window function",
			sql:  "SELECT id, row_number() OVER (PARTITION BY layer_type ORDER BY created_at) FROM entities",
		},
		{
			name: "SELECT with UNION",
			sql:  "SELECT id FROM entities UNION SELECT entity_id FROM observations",
		},
		{
			name: "SELECT with UNION ALL",
			sql:  "SELECT id FROM entities UNION ALL SELECT entity_id FROM observations",
		},
		{
			name: "SELECT with INTERSECT",
			sql:  "SELECT id FROM entities INTERSECT SELECT entity_id FROM observations",
		},
		{
			name: "SELECT with EXCEPT",
			sql:  "SELECT id FROM entities EXCEPT SELECT entity_id FROM observations",
		},
		{
			name: "SELECT with COALESCE and NULLIF",
			sql:  "SELECT COALESCE(name, 'unknown'), NULLIF(altitude, 0) FROM entities",
		},
		{
			name: "SELECT with cast",
			sql:  "SELECT id::text, altitude::integer FROM entities",
		},
		{
			name: "SELECT with lateral join",
			sql:  "SELECT * FROM entities e, LATERAL (SELECT * FROM observations o WHERE o.entity_id = e.id LIMIT 5) sub",
		},
		{
			name: "SELECT with JSON operators",
			sql:  "SELECT metadata->>'flight' FROM entities WHERE metadata->>'squawk' = '7700'",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateReadOnlySQL(tc.sql)
			assert.NoError(t, err)
		})
	}
}

func TestValidateReadOnlySQL_RejectedMutations(t *testing.T) {
	tests := []struct {
		name        string
		sql         string
		errContains string
	}{
		{
			name:        "INSERT",
			sql:         "INSERT INTO entities (id, name) VALUES ('abc', 'test')",
			errContains: "INSERT statements are not allowed",
		},
		{
			name:        "UPDATE",
			sql:         "UPDATE entities SET name = 'changed' WHERE id = 'abc'",
			errContains: "UPDATE statements are not allowed",
		},
		{
			name:        "DELETE",
			sql:         "DELETE FROM entities WHERE id = 'abc'",
			errContains: "DELETE statements are not allowed",
		},
		{
			name:        "INSERT with RETURNING",
			sql:         "INSERT INTO entities (id, name) VALUES ('abc', 'test') RETURNING id",
			errContains: "INSERT statements are not allowed",
		},
		{
			name:        "UPDATE with RETURNING",
			sql:         "UPDATE entities SET name = 'changed' WHERE id = 'abc' RETURNING *",
			errContains: "UPDATE statements are not allowed",
		},
		{
			name:        "DELETE with RETURNING",
			sql:         "DELETE FROM entities WHERE id = 'abc' RETURNING id",
			errContains: "DELETE statements are not allowed",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateReadOnlySQL(tc.sql)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.errContains)
		})
	}
}

func TestValidateReadOnlySQL_RejectedDDL(t *testing.T) {
	tests := []struct {
		name        string
		sql         string
		errContains string
	}{
		{
			name:        "CREATE TABLE",
			sql:         "CREATE TABLE evil (id serial PRIMARY KEY)",
			errContains: "CREATE statements are not allowed",
		},
		{
			name:        "CREATE INDEX",
			sql:         "CREATE INDEX idx_test ON entities (name)",
			errContains: "not allowed",
		},
		{
			name:        "DROP TABLE",
			sql:         "DROP TABLE entities",
			errContains: "DROP statements are not allowed",
		},
		{
			name:        "DROP INDEX",
			sql:         "DROP INDEX idx_test",
			errContains: "DROP statements are not allowed",
		},
		{
			name:        "ALTER TABLE",
			sql:         "ALTER TABLE entities ADD COLUMN evil text",
			errContains: "ALTER statements are not allowed",
		},
		{
			name:        "TRUNCATE",
			sql:         "TRUNCATE entities",
			errContains: "TRUNCATE statements are not allowed",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateReadOnlySQL(tc.sql)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.errContains)
		})
	}
}

func TestValidateReadOnlySQL_RejectedPrivilegeCommands(t *testing.T) {
	tests := []struct {
		name        string
		sql         string
		errContains string
	}{
		{
			name:        "GRANT SELECT",
			sql:         "GRANT SELECT ON entities TO evil_role",
			errContains: "GRANT statements are not allowed",
		},
		{
			name:        "GRANT ALL",
			sql:         "GRANT ALL ON entities TO evil_role",
			errContains: "GRANT statements are not allowed",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateReadOnlySQL(tc.sql)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.errContains)
		})
	}
}

func TestValidateReadOnlySQL_RejectedUtilityStatements(t *testing.T) {
	tests := []struct {
		name        string
		sql         string
		errContains string
	}{
		{
			name:        "COPY TO",
			sql:         "COPY entities TO '/tmp/evil.csv'",
			errContains: "COPY statements are not allowed",
		},
		{
			name:        "COPY FROM",
			sql:         "COPY entities FROM '/tmp/evil.csv'",
			errContains: "COPY statements are not allowed",
		},
		{
			name:        "EXECUTE prepared statement",
			sql:         "EXECUTE my_plan",
			errContains: "EXECUTE statements are not allowed",
		},
		{
			name:        "CALL procedure",
			sql:         "CALL my_procedure()",
			errContains: "CALL statements are not allowed",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateReadOnlySQL(tc.sql)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.errContains)
		})
	}
}

func TestValidateReadOnlySQL_RejectedTransactionControl(t *testing.T) {
	tests := []struct {
		name        string
		sql         string
		errContains string
	}{
		{
			name:        "BEGIN",
			sql:         "BEGIN",
			errContains: "transaction control statements are not allowed",
		},
		{
			name:        "COMMIT",
			sql:         "COMMIT",
			errContains: "transaction control statements are not allowed",
		},
		{
			name:        "ROLLBACK",
			sql:         "ROLLBACK",
			errContains: "transaction control statements are not allowed",
		},
		{
			name:        "SAVEPOINT",
			sql:         "SAVEPOINT my_savepoint",
			errContains: "transaction control statements are not allowed",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateReadOnlySQL(tc.sql)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.errContains)
		})
	}
}

func TestValidateReadOnlySQL_RejectedSETAndRESET(t *testing.T) {
	tests := []struct {
		name        string
		sql         string
		errContains string
	}{
		{
			name:        "SET variable",
			sql:         "SET statement_timeout = '0'",
			errContains: "SET statements are not allowed",
		},
		{
			name:        "SET LOCAL",
			sql:         "SET LOCAL statement_timeout = '0'",
			errContains: "SET statements are not allowed",
		},
		{
			name:        "RESET variable",
			sql:         "RESET statement_timeout",
			errContains: "SET statements are not allowed",
		},
		{
			name:        "RESET ALL",
			sql:         "RESET ALL",
			errContains: "SET statements are not allowed",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateReadOnlySQL(tc.sql)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.errContains)
		})
	}
}

func TestValidateReadOnlySQL_RejectedEXPLAIN(t *testing.T) {
	tests := []struct {
		name        string
		sql         string
		errContains string
	}{
		{
			name:        "EXPLAIN SELECT",
			sql:         "EXPLAIN SELECT * FROM entities",
			errContains: "EXPLAIN statements are not allowed",
		},
		{
			name:        "EXPLAIN ANALYZE SELECT",
			sql:         "EXPLAIN ANALYZE SELECT * FROM entities",
			errContains: "EXPLAIN statements are not allowed",
		},
		{
			name:        "EXPLAIN with options",
			sql:         "EXPLAIN (FORMAT JSON, ANALYZE) SELECT * FROM entities",
			errContains: "EXPLAIN statements are not allowed",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateReadOnlySQL(tc.sql)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.errContains)
		})
	}
}

func TestValidateReadOnlySQL_RejectedCTEWithMutation(t *testing.T) {
	tests := []struct {
		name        string
		sql         string
		errContains string
	}{
		{
			name:        "CTE with INSERT RETURNING",
			sql:         "WITH inserted AS (INSERT INTO entities (id, name) VALUES ('abc', 'test') RETURNING id) SELECT * FROM inserted",
			errContains: "CTE contains forbidden statement",
		},
		{
			name:        "CTE with UPDATE RETURNING",
			sql:         "WITH updated AS (UPDATE entities SET name = 'changed' WHERE id = 'abc' RETURNING *) SELECT * FROM updated",
			errContains: "CTE contains forbidden statement",
		},
		{
			name:        "CTE with DELETE RETURNING",
			sql:         "WITH deleted AS (DELETE FROM entities WHERE id = 'abc' RETURNING id) SELECT * FROM deleted",
			errContains: "CTE contains forbidden statement",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateReadOnlySQL(tc.sql)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.errContains)
		})
	}
}

func TestValidateReadOnlySQL_RejectedSELECTINTO(t *testing.T) {
	err := ValidateReadOnlySQL("SELECT * INTO new_table FROM entities")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "SELECT INTO is not allowed")
}

func TestValidateReadOnlySQL_RejectedLockingClauses(t *testing.T) {
	tests := []struct {
		name        string
		sql         string
		errContains string
	}{
		{
			name:        "FOR UPDATE",
			sql:         "SELECT * FROM entities FOR UPDATE",
			errContains: "locking clauses (FOR UPDATE/SHARE) are not allowed",
		},
		{
			name:        "FOR SHARE",
			sql:         "SELECT * FROM entities FOR SHARE",
			errContains: "locking clauses (FOR UPDATE/SHARE) are not allowed",
		},
		{
			name:        "FOR NO KEY UPDATE",
			sql:         "SELECT * FROM entities FOR NO KEY UPDATE",
			errContains: "locking clauses (FOR UPDATE/SHARE) are not allowed",
		},
		{
			name:        "FOR KEY SHARE",
			sql:         "SELECT * FROM entities FOR KEY SHARE",
			errContains: "locking clauses (FOR UPDATE/SHARE) are not allowed",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateReadOnlySQL(tc.sql)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.errContains)
		})
	}
}

func TestValidateReadOnlySQL_CaseInsensitivity(t *testing.T) {
	// The PostgreSQL parser handles case insensitivity natively.
	// These tests verify that mixed-case SQL keywords are correctly parsed
	// and rejected at the AST level, not via string matching.
	tests := []struct {
		name        string
		sql         string
		errContains string
	}{
		{
			name:        "mixed case INSERT",
			sql:         "InSeRt INTO entities (id) VALUES ('x')",
			errContains: "INSERT statements are not allowed",
		},
		{
			name:        "mixed case UPDATE",
			sql:         "uPdAtE entities SET name = 'x'",
			errContains: "UPDATE statements are not allowed",
		},
		{
			name:        "mixed case DELETE",
			sql:         "DeLeTe FROM entities",
			errContains: "DELETE statements are not allowed",
		},
		{
			name:        "mixed case DROP",
			sql:         "dRoP TABLE entities",
			errContains: "DROP statements are not allowed",
		},
		{
			name:        "mixed case CREATE",
			sql:         "cReAtE TABLE evil (id int)",
			errContains: "CREATE statements are not allowed",
		},
		{
			name:        "mixed case TRUNCATE",
			sql:         "tRuNcAtE entities",
			errContains: "TRUNCATE statements are not allowed",
		},
		{
			name:        "mixed case SELECT passes",
			sql:         "sElEcT 1",
			errContains: "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateReadOnlySQL(tc.sql)
			if tc.errContains == "" {
				assert.NoError(t, err)
			} else {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.errContains)
			}
		})
	}
}

func TestValidateReadOnlySQL_CommentBypassAttempts(t *testing.T) {
	// The PostgreSQL AST parser strips comments and parses the underlying
	// SQL. Comment-based bypass attempts are handled correctly because
	// we validate the parsed AST, not the raw string.
	tests := []struct {
		name        string
		sql         string
		errContains string
	}{
		{
			name:        "line comment before INSERT",
			sql:         "-- this is a comment\nINSERT INTO entities (id) VALUES ('x')",
			errContains: "INSERT statements are not allowed",
		},
		{
			name:        "block comment around INSERT",
			sql:         "/*hidden*/ INSERT INTO entities (id) VALUES ('x')",
			errContains: "INSERT statements are not allowed",
		},
		{
			name:        "comment inside SELECT is fine",
			sql:         "SELECT /* just a comment */ 1",
			errContains: "",
		},
		{
			name:        "block comment before DROP",
			sql:         "/* SELECT */ DROP TABLE entities",
			errContains: "DROP statements are not allowed",
		},
		{
			name:        "nested comments before DELETE",
			sql:         "/* outer /* inner */ */ DELETE FROM entities",
			errContains: "DELETE statements are not allowed",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateReadOnlySQL(tc.sql)
			if tc.errContains == "" {
				assert.NoError(t, err)
			} else {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.errContains)
			}
		})
	}
}

func TestValidateReadOnlySQL_MultipleStatements(t *testing.T) {
	tests := []struct {
		name        string
		sql         string
		errContains string
	}{
		{
			name:        "SELECT then DROP",
			sql:         "SELECT 1; DROP TABLE entities",
			errContains: "multiple statements are not allowed",
		},
		{
			name:        "two SELECTs",
			sql:         "SELECT 1; SELECT 2",
			errContains: "multiple statements are not allowed",
		},
		{
			name:        "SELECT then INSERT",
			sql:         "SELECT 1; INSERT INTO entities (id) VALUES ('x')",
			errContains: "multiple statements are not allowed",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateReadOnlySQL(tc.sql)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.errContains)
		})
	}
}

func TestValidateReadOnlySQL_EdgeCases(t *testing.T) {
	tests := []struct {
		name        string
		sql         string
		errContains string
	}{
		{
			name:        "empty string",
			sql:         "",
			errContains: "empty SQL input",
		},
		{
			name:        "whitespace only",
			sql:         "   \t\n  ",
			errContains: "", // pg_query may parse this differently
		},
		{
			name:        "invalid SQL",
			sql:         "NOT VALID SQL AT ALL ???",
			errContains: "SQL parse error",
		},
		{
			name:        "incomplete SQL",
			sql:         "SELECT FROM WHERE",
			errContains: "SQL parse error",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateReadOnlySQL(tc.sql)
			if tc.errContains == "" {
				// Some edge cases may or may not error - just check the parser doesn't panic
				_ = err
			} else {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.errContains)
			}
		})
	}
}

func TestValidateReadOnlySQL_SetOperationWithLocking(t *testing.T) {
	// PostgreSQL's grammar does not allow FOR UPDATE combined with UNION,
	// so the parser itself rejects it as a syntax error. This is the desired
	// outcome: the SQL cannot be executed regardless.
	err := ValidateReadOnlySQL("SELECT * FROM entities FOR UPDATE UNION SELECT * FROM observations")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "SQL parse error")
}

func TestEnforceLimitAndTimeout_AddsLimitWhenMissing(t *testing.T) {
	setStmt, queryStmt := EnforceLimitAndTimeout("SELECT * FROM entities", 1000, 5*time.Second)

	assert.Equal(t, "SET LOCAL statement_timeout = '5000ms'", setStmt)
	assert.Equal(t, "SELECT * FROM entities LIMIT 1000", queryStmt)
}

func TestEnforceLimitAndTimeout_PreservesExistingLimit(t *testing.T) {
	setStmt, queryStmt := EnforceLimitAndTimeout("SELECT * FROM entities LIMIT 50", 1000, 5*time.Second)

	assert.Equal(t, "SET LOCAL statement_timeout = '5000ms'", setStmt)
	assert.Equal(t, "SELECT * FROM entities LIMIT 50", queryStmt)
}

func TestEnforceLimitAndTimeout_VariousTimeouts(t *testing.T) {
	tests := []struct {
		name            string
		timeout         time.Duration
		expectedSetStmt string
	}{
		{
			name:            "1 second",
			timeout:         1 * time.Second,
			expectedSetStmt: "SET LOCAL statement_timeout = '1000ms'",
		},
		{
			name:            "500 milliseconds",
			timeout:         500 * time.Millisecond,
			expectedSetStmt: "SET LOCAL statement_timeout = '500ms'",
		},
		{
			name:            "10 seconds",
			timeout:         10 * time.Second,
			expectedSetStmt: "SET LOCAL statement_timeout = '10000ms'",
		},
		{
			name:            "30 seconds",
			timeout:         30 * time.Second,
			expectedSetStmt: "SET LOCAL statement_timeout = '30000ms'",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			setStmt, _ := EnforceLimitAndTimeout("SELECT 1", 100, tc.timeout)
			assert.Equal(t, tc.expectedSetStmt, setStmt)
		})
	}
}

func TestEnforceLimitAndTimeout_VariousMaxRows(t *testing.T) {
	tests := []struct {
		name     string
		maxRows  int
		expected string
	}{
		{
			name:     "100 rows",
			maxRows:  100,
			expected: "SELECT * FROM entities LIMIT 100",
		},
		{
			name:     "500 rows",
			maxRows:  500,
			expected: "SELECT * FROM entities LIMIT 500",
		},
		{
			name:     "1 row",
			maxRows:  1,
			expected: "SELECT * FROM entities LIMIT 1",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, queryStmt := EnforceLimitAndTimeout("SELECT * FROM entities", tc.maxRows, 5*time.Second)
			assert.Equal(t, tc.expected, queryStmt)
		})
	}
}

func TestEnforceLimitAndTimeout_ComplexQueries(t *testing.T) {
	tests := []struct {
		name          string
		sql           string
		maxRows       int
		expectedQuery string
	}{
		{
			name:          "query with WHERE adds LIMIT",
			sql:           "SELECT * FROM entities WHERE layer_type = 'flights'",
			maxRows:       1000,
			expectedQuery: "SELECT * FROM entities WHERE layer_type = 'flights' LIMIT 1000",
		},
		{
			name:          "query with ORDER BY adds LIMIT",
			sql:           "SELECT * FROM entities ORDER BY created_at DESC",
			maxRows:       1000,
			expectedQuery: "SELECT * FROM entities ORDER BY created_at DESC LIMIT 1000",
		},
		{
			name:          "query with existing LIMIT preserved",
			sql:           "SELECT * FROM entities ORDER BY created_at DESC LIMIT 10",
			maxRows:       1000,
			expectedQuery: "SELECT * FROM entities ORDER BY created_at DESC LIMIT 10",
		},
		{
			name:          "query with JOIN adds LIMIT",
			sql:           "SELECT e.id, o.ts FROM entities e JOIN observations o ON o.entity_id = e.id",
			maxRows:       500,
			expectedQuery: "SELECT e.id, o.ts FROM entities e JOIN observations o ON o.entity_id = e.id LIMIT 500",
		},
		{
			name:          "query with CTE adds LIMIT",
			sql:           "WITH recent AS (SELECT * FROM observations) SELECT * FROM recent",
			maxRows:       1000,
			expectedQuery: "WITH recent AS (SELECT * FROM observations) SELECT * FROM recent LIMIT 1000",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, queryStmt := EnforceLimitAndTimeout(tc.sql, tc.maxRows, 5*time.Second)
			assert.Equal(t, tc.expectedQuery, queryStmt)
		})
	}
}

func TestEnforceLimitAndTimeout_InvalidSQL(t *testing.T) {
	// If SQL is unparseable, EnforceLimitAndTimeout should still return
	// the original SQL unchanged (the caller should have validated first).
	setStmt, queryStmt := EnforceLimitAndTimeout("NOT VALID SQL", 1000, 5*time.Second)

	assert.Equal(t, "SET LOCAL statement_timeout = '5000ms'", setStmt)
	assert.Equal(t, "NOT VALID SQL", queryStmt)
}
