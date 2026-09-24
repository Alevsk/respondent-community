package sqlite_test

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Alevsk/respondent/internal/domain"
	sqlitedb "github.com/Alevsk/respondent/internal/infra/sqlite"
)

// seedLayer writes n entities, each with one observation, into layerType.
func seedLayer(t *testing.T, db *sqlitedb.DB, layerType string, n int) {
	t.Helper()
	eRepo := sqlitedb.NewEntityRepository(db.SqlDB(), testLogger())
	oRepo := sqlitedb.NewObservationRepository(db.SqlDB(), testLogger())
	ctx := context.Background()

	entities := make([]*domain.Entity, 0, n)
	for i := range n {
		entities = append(entities, &domain.Entity{
			ID:         fmt.Sprintf("%s-e%05d", layerType, i),
			ExternalID: fmt.Sprintf("EXT%05d", i),
			LayerType:  layerType,
			Name:       fmt.Sprintf("entity %d", i),
		})
	}
	require.NoError(t, eRepo.CreateBatch(ctx, entities))

	// One shared timestamp: this is what the real store looks like for a
	// bulk-ingested catalog, and it is exactly the case where "ORDER BY ts DESC
	// LIMIT n" returns an arbitrary subset.
	ts := time.Date(2026, 9, 24, 12, 0, 0, 0, time.UTC)
	obs := make([]*domain.Observation, 0, n)
	for i, e := range entities {
		obs = append(obs, &domain.Observation{
			ID:        fmt.Sprintf("%s-o%05d", layerType, i),
			EntityID:  e.ID,
			Timestamp: ts,
		})
	}
	require.NoError(t, oRepo.CreateBatch(ctx, obs))
}

func externalIDs(snaps []*domain.EntitySnapshot) []string {
	out := make([]string, 0, len(snaps))
	for _, s := range snaps {
		out = append(out, s.Entity.ExternalID)
	}
	return out
}

// T2: the cache this replaces ranged a Go map and broke at limit, so two
// identical requests returned two arbitrary and sometimes disjoint subsets.
// The durable store must answer the same question the same way every time.
func TestGetLatestForLayerPageIsDeterministic(t *testing.T) {
	db := newTestDB(t)
	seedLayer(t, db, "flights", 500)
	repo := sqlitedb.NewObservationRepository(db.SqlDB(), testLogger())

	first, err := repo.GetLatestForLayerPage(context.Background(), "flights", 200, 0)
	require.NoError(t, err)
	require.Len(t, first, 200)

	for range 2 {
		again, err := repo.GetLatestForLayerPage(context.Background(), "flights", 200, 0)
		require.NoError(t, err)
		assert.Equal(t, externalIDs(first), externalIDs(again),
			"identical requests must return identical rows in identical order")
	}
}

// T3: paging must cover the layer exactly once — no gaps, no duplicates. The
// map-order implementation this replaces could not do either.
func TestGetLatestForLayerPageCoversTheLayerExactlyOnce(t *testing.T) {
	const total = 3500
	db := newTestDB(t)
	seedLayer(t, db, "power_plants", total)
	repo := sqlitedb.NewObservationRepository(db.SqlDB(), testLogger())

	seen := make(map[string]int)
	for offset := 0; offset < 100_000; offset += 1000 {
		page, err := repo.GetLatestForLayerPage(context.Background(), "power_plants", 1000, offset)
		require.NoError(t, err)
		if len(page) == 0 {
			break
		}
		for _, s := range page {
			seen[s.Entity.ExternalID]++
		}
		if len(page) < 1000 {
			break
		}
	}

	assert.Len(t, seen, total, "every entity must appear")
	for id, n := range seen {
		assert.Equal(t, 1, n, "entity %s returned %d times", id, n)
	}
}

// T4: the ordering contract. Locks in ascending external_id so nobody
// "restores" ORDER BY ts DESC, which reintroduces the temp B-tree.
func TestGetLatestForLayerPageOrdersByExternalID(t *testing.T) {
	db := newTestDB(t)
	seedLayer(t, db, "cctv", 50)
	repo := sqlitedb.NewObservationRepository(db.SqlDB(), testLogger())

	page, err := repo.GetLatestForLayerPage(context.Background(), "cctv", 50, 0)
	require.NoError(t, err)
	ids := externalIDs(page)
	require.Len(t, ids, 50)
	for i := 1; i < len(ids); i++ {
		assert.Less(t, ids[i-1], ids[i], "results must ascend by external_id")
	}
}

// T5: the query must be served from the (layer_type, external_id) index with no
// temp B-tree. A sort would make LIMIT 100 cost the same as the whole layer —
// measured at 167ms for power_plants before this change.
func TestGetLatestForLayerPageUsesNoTempBTree(t *testing.T) {
	db := newTestDB(t)
	seedLayer(t, db, "flights", 10)

	rows, err := db.SqlDB().Query(
		`EXPLAIN QUERY PLAN `+sqlitedb.LatestForLayerPageSQL, "flights", 10, 0)
	require.NoError(t, err)
	defer func() { _ = rows.Close() }()

	var plan strings.Builder
	for rows.Next() {
		var id, parent, notused int
		var detail string
		require.NoError(t, rows.Scan(&id, &parent, &notused, &detail))
		plan.WriteString(detail)
		plan.WriteString("\n")
	}
	require.NoError(t, rows.Err())

	assert.Contains(t, plan.String(), "idx_entities_layer_external")
	assert.NotContains(t, strings.ToUpper(plan.String()), "TEMP B-TREE")
}
