package sqlite

import (
	"database/sql"
	"encoding/json"
	"time"

	"github.com/rs/zerolog/log"
)

func marshalJSON(v any) string {
	if v == nil {
		return "{}"
	}
	b, err := json.Marshal(v)
	if err != nil {
		log.Warn().Err(err).Msg("failed to marshal JSON for database write")
		return "{}"
	}
	return string(b)
}

func unmarshalStringMap(s string) map[string]string {
	if s == "" || s == "{}" {
		return make(map[string]string)
	}
	var m map[string]string
	if err := json.Unmarshal([]byte(s), &m); err != nil {
		log.Warn().Err(err).Msg("failed to unmarshal string map")
		return make(map[string]string)
	}
	return m
}

func unmarshalAnyMap(s string) map[string]any {
	if s == "" || s == "{}" {
		return make(map[string]any)
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(s), &m); err != nil {
		log.Warn().Err(err).Msg("failed to unmarshal any map")
		return make(map[string]any)
	}
	if m == nil {
		// JSON "null" unmarshals to a nil map without error. Callers assign into
		// the result (e.g. mergeJSONMaps for ai_metadata patches), so it must
		// never be nil.
		return make(map[string]any)
	}
	return m
}

func unmarshalFloat64Map(s string) map[string]float64 {
	if s == "" || s == "{}" {
		return make(map[string]float64)
	}
	var m map[string]float64
	if err := json.Unmarshal([]byte(s), &m); err != nil {
		log.Warn().Err(err).Msg("failed to unmarshal float64 map")
		return make(map[string]float64)
	}
	return m
}

func formatTime(t time.Time) string {
	return t.UTC().Format(time.RFC3339Nano)
}

func parseTime(s string) (time.Time, error) {
	return time.Parse(time.RFC3339Nano, s)
}

func nullString(s *string) sql.NullString {
	if s == nil {
		return sql.NullString{}
	}
	return sql.NullString{String: *s, Valid: true}
}

func nullTime(t *time.Time) sql.NullString {
	if t == nil {
		return sql.NullString{}
	}
	return sql.NullString{String: formatTime(*t), Valid: true}
}

func stringPtr(ns sql.NullString) *string {
	if !ns.Valid {
		return nil
	}
	return &ns.String
}

func timePtr(ns sql.NullString) *time.Time {
	if !ns.Valid {
		return nil
	}
	t, err := parseTime(ns.String)
	if err != nil {
		return nil
	}
	return &t
}

func intPtr(ni sql.NullInt64) *int {
	if !ni.Valid {
		return nil
	}
	v := int(ni.Int64)
	return &v
}

func mergeJSONMaps(existing string, patch map[string]any) string {
	base := unmarshalAnyMap(existing)
	for k, v := range patch {
		base[k] = v
	}
	return marshalJSON(base)
}
