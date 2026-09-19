package sqlite_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Alevsk/respondent/internal/domain"
	sqlitedb "github.com/Alevsk/respondent/internal/infra/sqlite"
)

func makeInsight(insightType, sourceName, operationName string) *domain.AIInsight {
	attention := "high"
	layerType := "flights"
	dedupKey := uuid.New().String()
	return &domain.AIInsight{
		InsightType:   insightType,
		SourceName:    sourceName,
		OperationName: operationName,
		LayerType:     &layerType,
		Attention:     &attention,
		DedupKey:      &dedupKey,
		Result:        map[string]any{"summary": "test insight"},
		CreatedAt:     time.Now().UTC(),
	}
}

func TestAIInsightRepository_CreateAndGetByID(t *testing.T) {
	db := newTestDB(t)
	repo := sqlitedb.NewAIInsightRepository(db.SqlDB(), testLogger())

	ins := makeInsight("anomaly", "analyzer", "detect_anomaly")
	id, err := repo.Create(context.Background(), ins)
	require.NoError(t, err)
	assert.NotEmpty(t, id)

	got, err := repo.GetByID(context.Background(), id)
	require.NoError(t, err)
	assert.Equal(t, id, got.ID)
	assert.Equal(t, "anomaly", got.InsightType)
	assert.Equal(t, "high", *got.Attention)
	require.NotNil(t, got.AttentionRank)
	assert.Equal(t, 3, *got.AttentionRank)
	assert.Equal(t, "test insight", got.Result["summary"])
}

func TestAIInsightRepository_GetByID_NotFound(t *testing.T) {
	db := newTestDB(t)
	repo := sqlitedb.NewAIInsightRepository(db.SqlDB(), testLogger())

	_, err := repo.GetByID(context.Background(), uuid.New().String())
	assert.True(t, domain.IsNotFound(err))
}

func TestAIInsightRepository_CreateRef_PopulatesEntities(t *testing.T) {
	db := newTestDB(t)
	insightRepo := sqlitedb.NewAIInsightRepository(db.SqlDB(), testLogger())
	entityRepo := sqlitedb.NewEntityRepository(db.SqlDB(), testLogger())

	entity := makeEntity("flights", "F1", "Flight 1")
	require.NoError(t, entityRepo.Create(context.Background(), entity))

	ins := makeInsight("summary", "analyzer", "summarize")
	id, err := insightRepo.Create(context.Background(), ins)
	require.NoError(t, err)

	require.NoError(t, insightRepo.CreateRef(context.Background(), id, &entity.ID, nil))

	got, err := insightRepo.GetByID(context.Background(), id)
	require.NoError(t, err)
	assert.Equal(t, []string{entity.ID}, got.EntityIDs)
	require.Len(t, got.Entities, 1)
	assert.Equal(t, "F1", got.Entities[0].ExternalID)
	assert.Equal(t, "flights", got.Entities[0].LayerType)
}

func TestAIInsightRepository_List_FilterByInsightType(t *testing.T) {
	db := newTestDB(t)
	repo := sqlitedb.NewAIInsightRepository(db.SqlDB(), testLogger())

	_, err := repo.Create(context.Background(), makeInsight("anomaly", "src", "op"))
	require.NoError(t, err)
	_, err = repo.Create(context.Background(), makeInsight("summary", "src", "op"))
	require.NoError(t, err)

	results, total, err := repo.List(context.Background(), domain.InsightFilter{InsightType: "anomaly", Limit: 10})
	require.NoError(t, err)
	assert.Equal(t, 1, total)
	assert.Len(t, results, 1)
	assert.Equal(t, "anomaly", results[0].InsightType)
}

func TestAIInsightRepository_List_FilterByMinAttention(t *testing.T) {
	db := newTestDB(t)
	repo := sqlitedb.NewAIInsightRepository(db.SqlDB(), testLogger())

	low := "low"
	critical := "critical"

	ins1 := makeInsight("a", "src", "op")
	ins1.Attention = &low
	_, err := repo.Create(context.Background(), ins1)
	require.NoError(t, err)

	ins2 := makeInsight("b", "src", "op")
	ins2.Attention = &critical
	_, err = repo.Create(context.Background(), ins2)
	require.NoError(t, err)

	results, total, err := repo.List(context.Background(), domain.InsightFilter{MinAttention: "high", Limit: 10})
	require.NoError(t, err)
	assert.Equal(t, 1, total)
	assert.Len(t, results, 1)
	assert.Equal(t, "critical", *results[0].Attention)
}

func TestAIInsightRepository_List_Pagination(t *testing.T) {
	db := newTestDB(t)
	repo := sqlitedb.NewAIInsightRepository(db.SqlDB(), testLogger())

	for i := 0; i < 5; i++ {
		_, err := repo.Create(context.Background(), makeInsight("anomaly", "src", "op"))
		require.NoError(t, err)
	}

	page1, total, err := repo.List(context.Background(), domain.InsightFilter{Limit: 2, Offset: 0})
	require.NoError(t, err)
	assert.Equal(t, 5, total)
	assert.Len(t, page1, 2)

	page2, _, err := repo.List(context.Background(), domain.InsightFilter{Limit: 2, Offset: 2})
	require.NoError(t, err)
	assert.Len(t, page2, 2)

	// Ensure no overlap.
	assert.NotEqual(t, page1[0].ID, page2[0].ID)
}

func TestAIInsightRepository_DeleteExpired(t *testing.T) {
	db := newTestDB(t)
	repo := sqlitedb.NewAIInsightRepository(db.SqlDB(), testLogger())

	past := time.Now().UTC().Add(-time.Hour)
	future := time.Now().UTC().Add(time.Hour)

	ins1 := makeInsight("a", "src", "op")
	ins1.ExpiresAt = &past
	_, err := repo.Create(context.Background(), ins1)
	require.NoError(t, err)

	ins2 := makeInsight("b", "src", "op")
	ins2.ExpiresAt = &future
	_, err = repo.Create(context.Background(), ins2)
	require.NoError(t, err)

	ins3 := makeInsight("c", "src", "op")
	// No expiry.
	_, err = repo.Create(context.Background(), ins3)
	require.NoError(t, err)

	deleted, err := repo.DeleteExpired(context.Background())
	require.NoError(t, err)
	assert.Equal(t, int64(1), deleted)

	_, total, err := repo.List(context.Background(), domain.InsightFilter{Limit: 10})
	require.NoError(t, err)
	assert.Equal(t, 2, total)
}

func TestAIInsight_Create_DuplicateDedupKeyIgnored(t *testing.T) {
	db := newTestDB(t)
	repo := sqlitedb.NewAIInsightRepository(db.SqlDB(), testLogger())
	ctx := context.Background()
	mk := func() *domain.AIInsight {
		dk := "dk-1"
		return &domain.AIInsight{InsightType: "t", SourceName: "src", OperationName: "op", DedupKey: &dk, Result: map[string]any{"x": 1}, CreatedAt: time.Now().UTC()}
	}
	_, err := repo.Create(ctx, mk())
	require.NoError(t, err)
	_, err = repo.Create(ctx, mk())
	require.NoError(t, err, "duplicate dedup key must be ignored, not error")

	got, _, err := repo.List(ctx, domain.InsightFilter{})
	require.NoError(t, err)
	assert.Len(t, got, 1)
}

func TestAIInsightRepository_GetRecentDedupKeys(t *testing.T) {
	db := newTestDB(t)
	repo := sqlitedb.NewAIInsightRepository(db.SqlDB(), testLogger())

	key := "dedup-abc"
	ins := makeInsight("anomaly", "analyzer", "detect")
	ins.DedupKey = &key
	_, err := repo.Create(context.Background(), ins)
	require.NoError(t, err)

	keys, err := repo.GetRecentDedupKeys(context.Background(), "analyzer", "detect", time.Now().UTC().Add(-time.Minute))
	require.NoError(t, err)
	assert.True(t, keys["dedup-abc"])

	// Different operation — should return empty.
	keys2, err := repo.GetRecentDedupKeys(context.Background(), "analyzer", "other_op", time.Now().UTC().Add(-time.Minute))
	require.NoError(t, err)
	assert.Len(t, keys2, 0)
}
