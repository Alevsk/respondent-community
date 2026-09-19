package sqlite

import (
	"context"
	"database/sql"
	"errors"

	"modernc.org/sqlite"
	sqlite3 "modernc.org/sqlite/lib"

	"github.com/Alevsk/respondent/internal/domain"
)

// translateError converts SQLite database errors into domain errors.
func translateError(resource string, err error) error {
	if err == nil {
		return nil
	}

	if errors.Is(err, sql.ErrNoRows) {
		return domain.NewNotFoundError(resource+" not found", err)
	}

	if errors.Is(err, context.DeadlineExceeded) {
		return domain.NewTimeoutError(resource+" query timed out", err)
	}

	if errors.Is(err, context.Canceled) {
		return domain.NewTimeoutError(resource+" query canceled", err)
	}

	var sqliteErr *sqlite.Error
	if errors.As(err, &sqliteErr) {
		switch sqliteErr.Code() {
		case sqlite3.SQLITE_CONSTRAINT_UNIQUE, sqlite3.SQLITE_CONSTRAINT_PRIMARYKEY:
			return domain.NewConflictError(resource+" already exists", err)
		case sqlite3.SQLITE_CONSTRAINT_FOREIGNKEY:
			return domain.NewInvalidInputError(resource+" references invalid foreign key", err)
		case sqlite3.SQLITE_CONSTRAINT_NOTNULL:
			return domain.NewInvalidInputError(resource+" missing required field", err)
		case sqlite3.SQLITE_CONSTRAINT_CHECK:
			return domain.NewInvalidInputError(resource+" violates constraint", err)
		}
	}

	return domain.NewInternalError(resource+" database error", err)
}
