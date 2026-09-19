package cctv

import (
	"context"
	"math"

	"github.com/Alevsk/respondent/internal/domain"
)

const (
	// minAltitude is the minimum altitude (in meters) allowed for projection.
	// Values at or below this threshold cause division-by-zero in the pinhole camera model.
	minAltitude = 0.001
)

// CCTVService implements the CCTVService gRPC service.
// It depends on domain.CameraFeedRepository and domain.CalibrationRepository interfaces
// rather than a concrete implementation, following the Dependency Inversion Principle.
type CCTVService struct {
	cameraFeedRepo  domain.CameraFeedRepository
	calibrationRepo domain.CalibrationRepository
}

// NewCCTVService creates a new CCTVService with interface-based dependencies.
func NewCCTVService(cameraFeedRepo domain.CameraFeedRepository, calibrationRepo domain.CalibrationRepository) *CCTVService {
	return &CCTVService{
		cameraFeedRepo:  cameraFeedRepo,
		calibrationRepo: calibrationRepo,
	}
}

// GetCameraFeeds returns all camera feeds (optionally filtered by city)
func (s *CCTVService) GetCameraFeeds(ctx context.Context, city string) ([]*domain.CameraFeed, error) {
	return s.cameraFeedRepo.List(ctx, city)
}

// GetNearestCameraFeed returns the nearest camera feed to a given position
func (s *CCTVService) GetNearestCameraFeed(ctx context.Context, lat, lon float64, city string) (*domain.CameraFeed, error) {
	position := domain.GeoPoint{Lat: lat, Lon: lon}
	return s.cameraFeedRepo.GetNearest(ctx, position, city)
}

// ProjectPoint projects a 3D world coordinate to 2D image coordinates.
// The BFF layer is a thin orchestrator: it fetches the calibration from the
// repo and delegates the pure math to domain.ProjectPoint.
func (s *CCTVService) ProjectPoint(ctx context.Context, cameraFeedID string, lat, lon, alt float64) (float64, float64, error) {
	// Validate input ranges
	if lat < -90 || lat > 90 {
		return 0, 0, domain.NewInvalidInputError("latitude must be between -90 and 90", nil)
	}
	if lon < -180 || lon > 180 {
		return 0, 0, domain.NewInvalidInputError("longitude must be between -180 and 180", nil)
	}
	if math.Abs(alt) < minAltitude {
		return 0, 0, domain.NewInvalidInputError("altitude must be non-zero for projection", nil)
	}

	cal, err := s.calibrationRepo.GetByCameraFeedID(ctx, cameraFeedID)
	if err != nil {
		return 0, 0, err
	}

	camCal := domain.CalibrationToProjection(cal)
	x, y, err := domain.ProjectPoint(camCal, lat, lon, alt)
	return x, y, err
}
