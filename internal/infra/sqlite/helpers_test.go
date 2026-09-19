package sqlite

import (
	"database/sql"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMarshalJSON(t *testing.T) {
	assert.Equal(t, `{"a":"b"}`, marshalJSON(map[string]string{"a": "b"}))
	assert.Equal(t, "{}", marshalJSON(nil))
}

func TestUnmarshalStringMap(t *testing.T) {
	m := unmarshalStringMap(`{"foo":"bar"}`)
	assert.Equal(t, "bar", m["foo"])

	empty := unmarshalStringMap("")
	assert.NotNil(t, empty)
	assert.Len(t, empty, 0)
}

func TestUnmarshalAnyMap(t *testing.T) {
	m := unmarshalAnyMap(`{"count":42}`)
	assert.Equal(t, float64(42), m["count"])
}

func TestUnmarshalFloat64Map(t *testing.T) {
	m := unmarshalFloat64Map(`{"speed":123.5}`)
	assert.InDelta(t, 123.5, m["speed"], 0.001)
}

func TestFormatParseTime(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Nanosecond)
	s := formatTime(now)
	parsed, err := parseTime(s)
	require.NoError(t, err)
	assert.True(t, now.Equal(parsed))
}

func TestNullString(t *testing.T) {
	s := "hello"
	ns := nullString(&s)
	assert.True(t, ns.Valid)
	assert.Equal(t, "hello", ns.String)

	ns2 := nullString(nil)
	assert.False(t, ns2.Valid)
}

func TestStringPtr(t *testing.T) {
	p := stringPtr(sql.NullString{String: "x", Valid: true})
	require.NotNil(t, p)
	assert.Equal(t, "x", *p)

	p2 := stringPtr(sql.NullString{})
	assert.Nil(t, p2)
}

func TestMergeJSONMaps(t *testing.T) {
	result := mergeJSONMaps(`{"a":"1"}`, map[string]any{"b": "2"})
	m := unmarshalAnyMap(result)
	assert.Equal(t, "1", m["a"])
	assert.Equal(t, "2", m["b"])
}
