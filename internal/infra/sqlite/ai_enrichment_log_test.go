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

func makeEnrichmentLog(entityID, sourceName, operationName string) *domain.AIEnrichmentLog {
	return &domain.AIEnrichmentLog{
		EntityID:      entityID,
		SourceName:    sourceName,
		OperationName: operationName,
		Status:        "pending",
		Provider:      "",
		Model:         "",
		CreatedAt:     time.Now().UTC(),
	}
}

func TestAIEnrichmentLog_CreateAndGetByEntityAndOperation(t *testing.T) {
	db := newTestDB(t)
	repo := sqlitedb.NewAIEnrichmentLogRepository(db.SqlDB(), testLogger())

	entityID := uuid.New().String()
	log := makeEnrichmentLog(entityID, "analyzer", "enrich_entity")
	require.NoError(t, repo.Create(context.Background(), log))
	assert.NotEmpty(t, log.ID)

	got, err := repo.GetByEntityAndOperation(context.Background(), entityID, "enrich_entity")
	require.NoError(t, err)
	assert.Equal(t, log.ID, got.ID)
	assert.Equal(t, entityID, got.EntityID)
	assert.Equal(t, "pending", got.Status)
}

func TestAIEnrichmentLog_GetByEntityAndOperation_NotFound(t *testing.T) {
	db := newTestDB(t)
	repo := sqlitedb.NewAIEnrichmentLogRepository(db.SqlDB(), testLogger())

	_, err := repo.GetByEntityAndOperation(context.Background(), uuid.New().String(), "op")
	assert.True(t, domain.IsNotFound(err))
}

func TestAIEnrichmentLog_UpdateStatus_Success(t *testing.T) {
	db := newTestDB(t)
	repo := sqlitedb.NewAIEnrichmentLogRepository(db.SqlDB(), testLogger())

	entityID := uuid.New().String()
	log := makeEnrichmentLog(entityID, "analyzer", "enrich_entity")
	require.NoError(t, repo.Create(context.Background(), log))

	usage := &domain.AIUsage{
		Provider:         "openai",
		Model:            "gpt-4o",
		PromptTokens:     100,
		CompletionTokens: 50,
		LatencyMS:        320,
	}
	result := map[string]any{"label": "benign"}
	require.NoError(t, repo.UpdateStatus(context.Background(), log.ID, "completed", result, usage, ""))

	got, err := repo.GetByEntityAndOperation(context.Background(), entityID, "enrich_entity")
	require.NoError(t, err)
	assert.Equal(t, "completed", got.Status)
	assert.Equal(t, "openai", got.Provider)
	assert.Equal(t, "gpt-4o", got.Model)
	assert.Equal(t, 100, got.PromptTokens)
	assert.Equal(t, 50, got.CompletionTokens)
	assert.Equal(t, 320, got.LatencyMS)
	assert.Equal(t, "benign", got.Result["label"])
	assert.NotNil(t, got.CompletedAt)
	assert.Empty(t, got.ErrorMessage)
}

func TestAIEnrichmentLog_UpdateStatus_WithError(t *testing.T) {
	db := newTestDB(t)
	repo := sqlitedb.NewAIEnrichmentLogRepository(db.SqlDB(), testLogger())

	entityID := uuid.New().String()
	log := makeEnrichmentLog(entityID, "analyzer", "enrich_entity")
	require.NoError(t, repo.Create(context.Background(), log))

	require.NoError(t, repo.UpdateStatus(context.Background(), log.ID, "failed", nil, nil, "model timeout"))

	got, err := repo.GetByEntityAndOperation(context.Background(), entityID, "enrich_entity")
	require.NoError(t, err)
	assert.Equal(t, "failed", got.Status)
	assert.Equal(t, "model timeout", got.ErrorMessage)
}

func TestAIEnrichmentLog_UpdateStatus_NotFound(t *testing.T) {
	db := newTestDB(t)
	repo := sqlitedb.NewAIEnrichmentLogRepository(db.SqlDB(), testLogger())

	err := repo.UpdateStatus(context.Background(), uuid.New().String(), "completed", nil, nil, "")
	assert.True(t, domain.IsNotFound(err))
}

func TestAIEnrichmentLog_DeleteStrandedLogs(t *testing.T) {
	db := newTestDB(t)
	repo := sqlitedb.NewAIEnrichmentLogRepository(db.SqlDB(), testLogger())
	ctx := context.Background()

	old := &domain.AIEnrichmentLog{EntityID: "e1", SourceName: "s", OperationName: "o", Status: domain.AIStatusPending, CreatedAt: time.Now().UTC().Add(-time.Hour)}
	require.NoError(t, repo.Create(ctx, old))
	fresh := &domain.AIEnrichmentLog{EntityID: "e2", SourceName: "s", OperationName: "o", Status: domain.AIStatusPending, CreatedAt: time.Now().UTC()}
	require.NoError(t, repo.Create(ctx, fresh))

	n, err := repo.DeleteStrandedLogs(ctx, 30*time.Minute, []string{domain.AIStatusPending, domain.AIStatusProcessing})
	require.NoError(t, err)
	assert.Equal(t, int64(1), n) // only the old pending row

	got, err := repo.GetByEntityAndOperation(ctx, "e1", "o")
	require.NoError(t, err)
	assert.Equal(t, domain.AIStatusFailed, got.Status)
	assert.Equal(t, "orphaned: restart/timeout", got.ErrorMessage)
	assert.NotNil(t, got.CompletedAt)

	// Fresh row must be untouched.
	freshGot, err := repo.GetByEntityAndOperation(ctx, "e2", "o")
	require.NoError(t, err)
	assert.Equal(t, domain.AIStatusPending, freshGot.Status)

	// Empty status slice is a no-op.
	n, err = repo.DeleteStrandedLogs(ctx, 30*time.Minute, nil)
	require.NoError(t, err)
	assert.Equal(t, int64(0), n)
}

func TestAIEnrichmentLog_Create_WithObservationID(t *testing.T) {
	db := newTestDB(t)
	repo := sqlitedb.NewAIEnrichmentLogRepository(db.SqlDB(), testLogger())

	entityID := uuid.New().String()
	obsID := uuid.New().String()
	promptHash := "sha256:abc"
	log := makeEnrichmentLog(entityID, "analyzer", "enrich_obs")
	log.ObservationID = &obsID
	log.PromptHash = &promptHash

	require.NoError(t, repo.Create(context.Background(), log))

	got, err := repo.GetByEntityAndOperation(context.Background(), entityID, "enrich_obs")
	require.NoError(t, err)
	require.NotNil(t, got.ObservationID)
	assert.Equal(t, obsID, *got.ObservationID)
	require.NotNil(t, got.PromptHash)
	assert.Equal(t, promptHash, *got.PromptHash)
}
