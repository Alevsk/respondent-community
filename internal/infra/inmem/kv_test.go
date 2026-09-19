package inmem_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Alevsk/respondent/internal/domain"
	"github.com/Alevsk/respondent/internal/infra/inmem"
)

func TestKVCache_SetAndGet(t *testing.T) {
	c := inmem.NewKVCache()

	require.NoError(t, c.Set(context.Background(), "key1", "value1", 0))

	val, err := c.Get(context.Background(), "key1")
	require.NoError(t, err)
	assert.Equal(t, "value1", val)
}

func TestKVCache_Get_NotFound(t *testing.T) {
	c := inmem.NewKVCache()

	_, err := c.Get(context.Background(), "missing")
	assert.True(t, domain.IsNotFound(err))
}

func TestKVCache_Get_Expired(t *testing.T) {
	c := inmem.NewKVCache()

	require.NoError(t, c.Set(context.Background(), "tmp", "data", 50*time.Millisecond))

	val, err := c.Get(context.Background(), "tmp")
	require.NoError(t, err)
	assert.Equal(t, "data", val)

	time.Sleep(100 * time.Millisecond)

	_, err = c.Get(context.Background(), "tmp")
	assert.True(t, domain.IsNotFound(err))
}

func TestKVCache_Set_Overwrite(t *testing.T) {
	c := inmem.NewKVCache()

	require.NoError(t, c.Set(context.Background(), "key", "v1", 0))
	require.NoError(t, c.Set(context.Background(), "key", "v2", 0))

	val, err := c.Get(context.Background(), "key")
	require.NoError(t, err)
	assert.Equal(t, "v2", val)
}

func TestKVCache_NoExpiry(t *testing.T) {
	c := inmem.NewKVCache()

	require.NoError(t, c.Set(context.Background(), "permanent", "here", 0))
	time.Sleep(50 * time.Millisecond)

	val, err := c.Get(context.Background(), "permanent")
	require.NoError(t, err)
	assert.Equal(t, "here", val)
}
