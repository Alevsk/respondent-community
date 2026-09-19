package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Alevsk/respondent/internal/domain"
)

// errTestDB opens an in-memory SQLite database with migrations applied.
func errTestDB(t *testing.T) *DB {
	t.Helper()
	db, err := Open(":memory:", zerolog.Nop())
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	require.NoError(t, db.RunMigrations())
	return db
}

func TestTranslateError_Nil(t *testing.T) {
	assert.Nil(t, translateError("entity", nil))
}

func TestTranslateError_NoRows(t *testing.T) {
	err := translateError("entity", sql.ErrNoRows)
	assert.True(t, domain.IsNotFound(err))
}

func TestTranslateError_DeadlineExceeded(t *testing.T) {
	err := translateError("entity", context.DeadlineExceeded)
	assert.True(t, domain.IsTimeout(err))
}

func TestTranslateError_Canceled(t *testing.T) {
	err := translateError("entity", context.Canceled)
	assert.True(t, domain.IsTimeout(err))
}

func TestTranslateError_UnknownError(t *testing.T) {
	err := translateError("entity", errors.New("something unexpected"))
	assert.True(t, domain.IsInternal(err))
}

func TestTranslateError_UniqueConstraint_Integration(t *testing.T) {
	db := errTestDB(t)

	_, execErr := db.SqlDB().Exec(
		`INSERT INTO entities (id, external_id, layer_type, name) VALUES (?, ?, ?, ?)`,
		"e1", "ext1", "flights", "Entity 1",
	)
	require.NoError(t, execErr)

	_, execErr = db.SqlDB().Exec(
		`INSERT INTO entities (id, external_id, layer_type, name) VALUES (?, ?, ?, ?)`,
		"e2", "ext1", "flights", "Entity 2",
	)
	require.Error(t, execErr)

	err := translateError("entity", execErr)
	assert.True(t, domain.IsConflict(err), "expected CONFLICT, got: %v", err)
}

func TestTranslateError_ForeignKeyConstraint_Integration(t *testing.T) {
	db := errTestDB(t)

	_, execErr := db.SqlDB().Exec(
		`INSERT INTO observations (id, entity_id, ts) VALUES (?, ?, ?)`,
		"o1", "nonexistent", "2024-01-01T00:00:00Z",
	)
	require.Error(t, execErr)

	err := translateError("observation", execErr)
	assert.True(t, domain.IsInvalidInput(err), "expected INVALID_INPUT, got: %v", err)
}

func TestTranslateError_NotNullConstraint_Integration(t *testing.T) {
	db := errTestDB(t)

	_, execErr := db.SqlDB().Exec(
		`INSERT INTO entities (id, external_id, layer_type) VALUES (?, ?, NULL)`,
		"e1", "ext1",
	)
	require.Error(t, execErr)

	err := translateError("entity", execErr)
	assert.True(t, domain.IsInvalidInput(err), "expected INVALID_INPUT, got: %v", err)
}
