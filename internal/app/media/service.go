// Package media implements the playback-notification use case.
//
// Some catalogs ask clients to report when a user starts playing an entry.
// Respondent honours that without ever becoming an open relay: the caller
// names a stored entity and one of the media slots its layer declares, and
// every other input — origin, method, path expression — comes from the source
// definition. The response carries a boolean and nothing else.
package media

import (
	"context"
	"regexp"
	"strings"

	"github.com/rs/zerolog"

	"github.com/Alevsk/respondent/internal/domain"
)

// SourceRegistry is the read-only view of declarative layer metadata the media
// service depends on. *domain.DynamicSourceRegistry satisfies it; depending on
// the interface keeps the service decoupled from the concrete registry (DIP).
type SourceRegistry interface {
	LookupDisplayConfig(lt domain.LayerType) (*domain.LayerDisplayConfig, bool)
}

// mediaIDRE matches the source-name-style identifiers media slots use. It also
// keeps a URL, a path, or an expression from ever reaching the resolver.
var mediaIDRE = regexp.MustCompile(`^[a-z][a-z0-9_]{0,63}$`)

// maxEntityIDLen bounds the identifier a caller may submit.
const maxEntityIDLen = 1024

// Service reports user-initiated playback starts to the owning source.
// It depends on repository and registry interfaces rather than on other
// services, so the only thing above it in the dependency graph is the
// composition root.
type Service struct {
	entities     domain.EntityRepository
	observations domain.ObservationRepository
	registry     SourceRegistry
	resolver     domain.MediaActionResolver
	executor     domain.MediaActionExecutor
	logger       zerolog.Logger
}

// NewService creates a Service from its ports. The optional logger variadic
// matches the other app services.
func NewService(
	entities domain.EntityRepository,
	observations domain.ObservationRepository,
	registry SourceRegistry,
	resolver domain.MediaActionResolver,
	executor domain.MediaActionExecutor,
	logger ...zerolog.Logger,
) *Service {
	l := zerolog.Nop()
	if len(logger) > 0 {
		l = logger[0]
	}
	return &Service{
		entities:     entities,
		observations: observations,
		registry:     registry,
		resolver:     resolver,
		executor:     executor,
		logger:       l,
	}
}

// ReportPlayback notifies the source that a user started playing the given
// media slot of the given entity.
//
// It returns whether the notification was delivered. A notification that fails
// upstream is reported as false rather than as an error: the listener's audio
// is already playing, and turning a courtesy call into an API failure would
// invite the client to retry something that must happen at most once.
func (s *Service) ReportPlayback(ctx context.Context, entityID, mediaID string) (bool, error) {
	entityID = strings.TrimSpace(entityID)
	if entityID == "" || len(entityID) > maxEntityIDLen || strings.ContainsAny(entityID, "/\\?#") || strings.ContainsAny(entityID, " \t\r\n") {
		return false, domain.NewInvalidInputError("invalid entity id", nil)
	}
	if !mediaIDRE.MatchString(mediaID) {
		return false, domain.NewInvalidInputError("invalid media id", nil)
	}

	entity, err := domain.ResolveEntity(ctx, s.entities, entityID)
	if err != nil {
		return false, err
	}
	if entity == nil {
		return false, domain.NewNotFoundError("entity not found", nil)
	}

	layerType := domain.LayerType(entity.LayerType)
	display, ok := s.registry.LookupDisplayConfig(layerType)
	if !ok || display == nil {
		return false, domain.NewNotFoundError("layer declares no media", nil)
	}

	var media *domain.MediaConfig
	for i := range display.Media {
		if display.Media[i].ID == mediaID {
			media = &display.Media[i]
			break
		}
	}
	if media == nil {
		return false, domain.NewNotFoundError("media not declared for this layer", nil)
	}
	if media.PlaybackAction == "" {
		// Nothing to report for this slot. Not an error — most media has no
		// notification contract at all.
		return false, nil
	}

	// The observation is supplementary: a station with no observation yet can
	// still be reported from its entity metadata alone.
	latest, err := s.observations.GetLatest(ctx, entity.ID)
	if err != nil {
		s.logger.Warn().Err(err).
			Str("entity_id", entity.ID).
			Msg("failed to fetch latest observation for playback notification")
	}

	action, err := s.resolver.ResolveMediaAction(layerType, media.PlaybackAction, mergedMetadata(entity, latest))
	if err != nil {
		s.logger.Warn().Err(err).
			Str("layer_type", string(layerType)).
			Str("media_id", mediaID).
			Msg("could not resolve declared playback action")
		return false, nil
	}

	if err := s.executor.ExecuteMediaAction(ctx, action); err != nil {
		s.logger.Warn().Err(err).
			Str("layer_type", string(layerType)).
			Str("media_id", mediaID).
			Msg("playback notification failed")
		return false, nil
	}

	return true, nil
}

// mergedMetadata combines entity and latest-observation metadata with the same
// precedence the entity overview uses: the latest observation wins.
func mergedMetadata(entity *domain.Entity, latest *domain.Observation) map[string]string {
	merged := make(map[string]string, len(entity.Metadata))
	for k, v := range entity.Metadata {
		merged[k] = v
	}
	if latest != nil {
		for k, v := range latest.Metadata {
			merged[k] = v
		}
	}
	return merged
}
