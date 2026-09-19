package entity

import (
	"context"
	"fmt"
	"time"

	"github.com/rs/zerolog"

	"github.com/Alevsk/respondent/internal/domain"
)

// EntityService handles entity detail lookups from the database.
// It depends on repo interfaces, not concrete implementations.
type EntityService struct {
	entities     domain.EntityRepository
	observations domain.ObservationRepository
	logger       zerolog.Logger
}

// NewEntityService creates a new EntityService. The optional logger allows
// callers to inject a configured zerolog.Logger; if omitted, logging is
// discarded (Nop).
func NewEntityService(
	entities domain.EntityRepository,
	observations domain.ObservationRepository,
	logger ...zerolog.Logger,
) *EntityService {
	l := zerolog.Nop()
	if len(logger) > 0 {
		l = logger[0]
	}
	return &EntityService{
		entities:     entities,
		observations: observations,
		logger:       l,
	}
}

// GetEntityDetail returns a single entity with its latest observation.
// entityID can be either a composite key "layerType:externalID" (e.g. "satellites:3230")
// or a raw UUID (e.g. from AI insight entity refs). Composite keys are parsed and
// looked up by (layer_type, external_id); UUIDs are looked up directly by primary key.
func (s *EntityService) GetEntityDetail(ctx context.Context, entityID string) (*domain.EntityDetail, error) {
	var entity *domain.Entity
	var err error

	layerType, externalID := domain.ParseEntityID(entityID)
	if layerType != "" && externalID != "" {
		entity, err = s.entities.GetByExternalID(ctx, string(layerType), externalID)
	} else {
		// Assume raw UUID — used by AI insight entity refs.
		entity, err = s.entities.GetByID(ctx, entityID)
	}
	if err != nil {
		return nil, err
	}

	// Observations are keyed by the DB UUID, not the composite ID.
	// Observation is supplementary — log errors but continue without it.
	latest, err := s.observations.GetLatest(ctx, entity.ID)
	if err != nil {
		s.logger.Warn().Err(err).
			Str("entity_id", entity.ID).
			Msg("failed to fetch latest observation for entity detail")
	}

	return &domain.EntityDetail{
		Entity:            entity,
		LatestObservation: latest,
	}, nil
}

// GetObservationHistory returns paginated observations for an entity, newest-first.
// entityID is a composite key "layerType:externalID". We resolve to the DB UUID
// before querying observations. The two-query approach is intentional: passing
// a literal UUID lets the planner use the (entity_id, ts DESC) index optimally,
// whereas a JOIN/subquery causes catastrophic mis-estimation (~50K row scan).
// If before is zero, all observations up to now are returned.
// Returns (observations, hasMore, error).
func (s *EntityService) GetObservationHistory(
	ctx context.Context,
	entityID string,
	limit int,
	before time.Time,
) ([]*domain.Observation, bool, error) {
	layerType, externalID := domain.ParseEntityID(entityID)
	if layerType == "" || externalID == "" {
		return nil, false, fmt.Errorf("invalid entity ID format: %s", entityID)
	}

	entity, err := s.entities.GetByExternalID(ctx, string(layerType), externalID)
	if err != nil {
		return nil, false, err
	}

	if limit <= 0 {
		limit = 50
	}

	if before.IsZero() {
		before = time.Now()
	}

	// Fetch one extra to determine hasMore.
	observations, err := s.observations.GetByEntityID(ctx, entity.ID, limit+1, before)
	if err != nil {
		return nil, false, err
	}

	hasMore := len(observations) > limit
	if hasMore {
		observations = observations[:limit]
	}

	return observations, hasMore, nil
}

// SearchEntities searches entities by query string with optional layer type filter.
// Returns matching results, total count, and any error.
func (s *EntityService) SearchEntities(ctx context.Context, query string, layerType string, limit int) ([]*domain.EntitySearchResult, int, error) {
	if len(query) < 2 {
		return nil, 0, fmt.Errorf("query must be at least 2 characters")
	}

	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}

	return s.entities.SearchEntities(ctx, query, layerType, limit)
}

// maxBatchSize is the maximum number of entity IDs allowed in a batch request.
const maxBatchSize = 50

// GetEntitiesBatch returns details for multiple entities by their composite IDs.
// Each entityID is "layerType:externalID". Entities that are not found are
// silently skipped (partial success).
func (s *EntityService) GetEntitiesBatch(ctx context.Context, entityIDs []string) ([]*domain.EntityDetail, error) {
	if len(entityIDs) == 0 {
		return nil, nil
	}
	if len(entityIDs) > maxBatchSize {
		return nil, fmt.Errorf("batch size %d exceeds maximum of %d", len(entityIDs), maxBatchSize)
	}

	// Group external IDs by layer type for efficient batch lookups.
	grouped := make(map[string][]string) // layerType -> []externalID
	for _, id := range entityIDs {
		lt, eid := domain.ParseEntityID(id)
		if lt == "" || eid == "" {
			continue // skip malformed IDs
		}
		grouped[string(lt)] = append(grouped[string(lt)], eid)
	}

	// Fetch entities grouped by layer type.
	// Map: DB UUID -> entity, and externalKey -> entity for lookup.
	var allEntities []*domain.Entity
	for layerType, externalIDs := range grouped {
		entities, err := s.entities.GetByExternalIDs(ctx, layerType, externalIDs)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch entities for layer %s: %w", layerType, err)
		}
		allEntities = append(allEntities, entities...)
	}

	if len(allEntities) == 0 {
		return nil, nil
	}

	// Collect DB UUIDs for observation lookup.
	dbIDs := make([]string, len(allEntities))
	for i, e := range allEntities {
		dbIDs[i] = e.ID
	}

	// Batch fetch latest observations keyed by entity DB UUID.
	obsMap, err := s.observations.GetLatestForEntityIDs(ctx, dbIDs)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch observations: %w", err)
	}

	// Build results.
	results := make([]*domain.EntityDetail, 0, len(allEntities))
	for _, entity := range allEntities {
		detail := &domain.EntityDetail{
			Entity:            entity,
			LatestObservation: obsMap[entity.ID],
		}
		results = append(results, detail)
	}

	return results, nil
}
