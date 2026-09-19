package cctv

import (
	"context"
	"errors"
	"math"
	"sync"
	"testing"
	"time"

	"github.com/Alevsk/respondent/internal/domain"
)

var (
	_ domain.CameraFeedRepository  = (*stubCameraFeedRepo)(nil)
	_ domain.CalibrationRepository = (*stubCalibrationRepo)(nil)
)

type stubCameraFeedRepo struct {
	mu         sync.RWMutex
	feeds      map[string]*domain.CameraFeed
	getErr     error
	listErr    error
	createErr  error
	nearestErr error
}

func newStubCameraFeedRepo() *stubCameraFeedRepo {
	return &stubCameraFeedRepo{feeds: make(map[string]*domain.CameraFeed)}
}

func (s *stubCameraFeedRepo) Create(_ context.Context, feed *domain.CameraFeed) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.createErr != nil {
		return s.createErr
	}
	s.feeds[feed.ID] = feed
	return nil
}

func (s *stubCameraFeedRepo) GetByID(_ context.Context, id string) (*domain.CameraFeed, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.getErr != nil {
		return nil, s.getErr
	}
	feed, ok := s.feeds[id]
	if !ok {
		return nil, domain.NewNotFoundError("camera feed not found", nil)
	}
	return feed, nil
}

func (s *stubCameraFeedRepo) List(_ context.Context, city string) ([]*domain.CameraFeed, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.listErr != nil {
		return nil, s.listErr
	}
	result := make([]*domain.CameraFeed, 0, len(s.feeds))
	for _, feed := range s.feeds {
		if city == "" || feed.Metadata["city"] == city {
			result = append(result, feed)
		}
	}
	return result, nil
}

func (s *stubCameraFeedRepo) GetNearest(_ context.Context, position domain.GeoPoint, city string) (*domain.CameraFeed, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.nearestErr != nil {
		return nil, s.nearestErr
	}
	var nearest *domain.CameraFeed
	minDist := math.MaxFloat64
	for _, feed := range s.feeds {
		if city != "" && feed.Metadata["city"] != city {
			continue
		}
		dlat := feed.Location.Lat - position.Lat
		dlon := feed.Location.Lon - position.Lon
		dist := dlat*dlat + dlon*dlon
		if dist < minDist {
			minDist = dist
			nearest = feed
		}
	}
	if nearest == nil {
		return nil, domain.NewNotFoundError("no camera feed found", nil)
	}
	return nearest, nil
}

type stubCalibrationRepo struct {
	mu           sync.RWMutex
	calibrations map[string]*domain.Calibration
	getErr       error
	updateErr    error
	deleteErr    error
}

func newStubCalibrationRepo() *stubCalibrationRepo {
	return &stubCalibrationRepo{calibrations: make(map[string]*domain.Calibration)}
}

func (s *stubCalibrationRepo) GetByCameraFeedID(_ context.Context, cameraFeedID string) (*domain.Calibration, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.getErr != nil {
		return nil, s.getErr
	}
	cal, ok := s.calibrations[cameraFeedID]
	if !ok {
		return nil, domain.NewNotFoundError("calibration not found", nil)
	}
	return cal, nil
}

func (s *stubCalibrationRepo) Update(_ context.Context, cal *domain.Calibration) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.updateErr != nil {
		return s.updateErr
	}
	s.calibrations[cal.CameraFeedID] = cal
	return nil
}

func (s *stubCalibrationRepo) Delete(_ context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.deleteErr != nil {
		return s.deleteErr
	}
	delete(s.calibrations, id)
	return nil
}

func TestNewCCTVService(t *testing.T) {
	cameraRepo := newStubCameraFeedRepo()
	calibrationRepo := newStubCalibrationRepo()
	svc := NewCCTVService(cameraRepo, calibrationRepo)
	if svc == nil {
		t.Fatal("expected non-nil CCTVService")
	}
	if svc.cameraFeedRepo != cameraRepo {
		t.Error("cameraFeedRepo not wired correctly")
	}
	if svc.calibrationRepo != calibrationRepo {
		t.Error("calibrationRepo not wired correctly")
	}
}

func TestCCTVService_GetCameraFeeds(t *testing.T) {
	ctx := context.Background()
	now := time.Now()

	t.Run("returns empty list when no feeds", func(t *testing.T) {
		cameraRepo := newStubCameraFeedRepo()
		calibrationRepo := newStubCalibrationRepo()
		svc := NewCCTVService(cameraRepo, calibrationRepo)

		feeds, err := svc.GetCameraFeeds(ctx, "")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(feeds) != 0 {
			t.Errorf("expected 0 feeds, got %d", len(feeds))
		}
	})

	t.Run("returns all feeds when city is empty", func(t *testing.T) {
		cameraRepo := newStubCameraFeedRepo()
		feed1 := &domain.CameraFeed{
			ID:        "feed-1",
			Name:      "Camera 1",
			Provider:  "Provider A",
			Location:  domain.GeoPoint{Lat: 37.77, Lon: -122.42},
			StreamURL: "rtsp://camera1.example.com",
			CreatedAt: now,
		}
		feed2 := &domain.CameraFeed{
			ID:        "feed-2",
			Name:      "Camera 2",
			Provider:  "Provider B",
			Location:  domain.GeoPoint{Lat: 51.5, Lon: -0.1},
			StreamURL: "rtsp://camera2.example.com",
			CreatedAt: now,
		}
		_ = cameraRepo.Create(ctx, feed1)
		_ = cameraRepo.Create(ctx, feed2)

		svc := NewCCTVService(cameraRepo, newStubCalibrationRepo())
		feeds, err := svc.GetCameraFeeds(ctx, "")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(feeds) != 2 {
			t.Errorf("expected 2 feeds, got %d", len(feeds))
		}
	})

	t.Run("filters feeds by city", func(t *testing.T) {
		cameraRepo := newStubCameraFeedRepo()
		feed1 := &domain.CameraFeed{
			ID:        "feed-1",
			Name:      "Camera 1",
			Location:  domain.GeoPoint{Lat: 37.77, Lon: -122.42},
			Metadata:  map[string]string{"city": "san-francisco"},
			CreatedAt: now,
		}
		feed2 := &domain.CameraFeed{
			ID:        "feed-2",
			Name:      "Camera 2",
			Location:  domain.GeoPoint{Lat: 51.5, Lon: -0.1},
			Metadata:  map[string]string{"city": "london"},
			CreatedAt: now,
		}
		_ = cameraRepo.Create(ctx, feed1)
		_ = cameraRepo.Create(ctx, feed2)

		svc := NewCCTVService(cameraRepo, newStubCalibrationRepo())
		feeds, err := svc.GetCameraFeeds(ctx, "san-francisco")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(feeds) != 1 {
			t.Errorf("expected 1 feed, got %d", len(feeds))
		}
		if feeds[0].ID != "feed-1" {
			t.Errorf("expected feed-1, got %s", feeds[0].ID)
		}
	})

	t.Run("propagates repository error", func(t *testing.T) {
		cameraRepo := newStubCameraFeedRepo()
		cameraRepo.listErr = errors.New("database error")
		svc := NewCCTVService(cameraRepo, newStubCalibrationRepo())

		_, err := svc.GetCameraFeeds(ctx, "")
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if err.Error() != "database error" {
			t.Errorf("expected 'database error', got %q", err.Error())
		}
	})
}

func TestCCTVService_GetNearestCameraFeed(t *testing.T) {
	ctx := context.Background()
	now := time.Now()

	t.Run("returns nearest feed", func(t *testing.T) {
		cameraRepo := newStubCameraFeedRepo()
		feed1 := &domain.CameraFeed{
			ID:        "feed-1",
			Name:      "Distant Camera",
			Location:  domain.GeoPoint{Lat: 40.0, Lon: -75.0},
			CreatedAt: now,
		}
		feed2 := &domain.CameraFeed{
			ID:        "feed-2",
			Name:      "Nearby Camera",
			Location:  domain.GeoPoint{Lat: 37.78, Lon: -122.43},
			CreatedAt: now,
		}
		_ = cameraRepo.Create(ctx, feed1)
		_ = cameraRepo.Create(ctx, feed2)

		svc := NewCCTVService(cameraRepo, newStubCalibrationRepo())
		result, err := svc.GetNearestCameraFeed(ctx, 37.77, -122.42, "")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.ID != "feed-2" {
			t.Errorf("expected nearest feed 'feed-2', got %s", result.ID)
		}
	})

	t.Run("filters by city when finding nearest", func(t *testing.T) {
		cameraRepo := newStubCameraFeedRepo()
		feed1 := &domain.CameraFeed{
			ID:        "feed-1",
			Name:      "SF Camera",
			Location:  domain.GeoPoint{Lat: 37.77, Lon: -122.42},
			Metadata:  map[string]string{"city": "san-francisco"},
			CreatedAt: now,
		}
		feed2 := &domain.CameraFeed{
			ID:        "feed-2",
			Name:      "London Camera",
			Location:  domain.GeoPoint{Lat: 51.5, Lon: -0.1},
			Metadata:  map[string]string{"city": "london"},
			CreatedAt: now,
		}
		_ = cameraRepo.Create(ctx, feed1)
		_ = cameraRepo.Create(ctx, feed2)

		svc := NewCCTVService(cameraRepo, newStubCalibrationRepo())
		result, err := svc.GetNearestCameraFeed(ctx, 51.5, -0.1, "london")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.ID != "feed-2" {
			t.Errorf("expected feed-2, got %s", result.ID)
		}
	})

	t.Run("returns error when no feeds available", func(t *testing.T) {
		cameraRepo := newStubCameraFeedRepo()
		svc := NewCCTVService(cameraRepo, newStubCalibrationRepo())

		_, err := svc.GetNearestCameraFeed(ctx, 37.77, -122.42, "")
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})

	t.Run("returns error when no feeds in city", func(t *testing.T) {
		cameraRepo := newStubCameraFeedRepo()
		feed := &domain.CameraFeed{
			ID:        "feed-1",
			Location:  domain.GeoPoint{Lat: 37.77, Lon: -122.42},
			Metadata:  map[string]string{"city": "san-francisco"},
			CreatedAt: now,
		}
		_ = cameraRepo.Create(ctx, feed)

		svc := NewCCTVService(cameraRepo, newStubCalibrationRepo())
		_, err := svc.GetNearestCameraFeed(ctx, 51.5, -0.1, "london")
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})

	t.Run("propagates repository error", func(t *testing.T) {
		cameraRepo := newStubCameraFeedRepo()
		cameraRepo.nearestErr = errors.New("spatial query failed")
		svc := NewCCTVService(cameraRepo, newStubCalibrationRepo())

		_, err := svc.GetNearestCameraFeed(ctx, 37.77, -122.42, "")
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if err.Error() != "spatial query failed" {
			t.Errorf("expected 'spatial query failed', got %q", err.Error())
		}
	})
}

func TestCCTVService_ProjectPoint(t *testing.T) {
	ctx := context.Background()

	t.Run("projects point successfully", func(t *testing.T) {
		cameraRepo := newStubCameraFeedRepo()
		calibrationRepo := newStubCalibrationRepo()
		calibrationRepo.calibrations["feed-1"] = &domain.Calibration{
			ID:           "cal-1",
			CameraFeedID: "feed-1",
			Heading:      0,
			Pitch:        -30,
			Roll:         0,
			FOV:          90,
			RangeM:       1000,
			HeightM:      10,
			OffsetNorthM: 0,
			OffsetEastM:  0,
			UpdatedAt:    time.Now(),
		}

		svc := NewCCTVService(cameraRepo, calibrationRepo)
		imageX, imageY, err := svc.ProjectPoint(ctx, "feed-1", 37.77, -122.42, 100)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if math.IsNaN(imageX) || math.IsNaN(imageY) {
			t.Error("expected valid projection, got NaN")
		}
		if math.IsInf(imageX, 0) || math.IsInf(imageY, 0) {
			t.Error("expected finite projection, got Inf")
		}
	})

	t.Run("returns error when calibration not found", func(t *testing.T) {
		cameraRepo := newStubCameraFeedRepo()
		calibrationRepo := newStubCalibrationRepo()

		svc := NewCCTVService(cameraRepo, calibrationRepo)
		_, _, err := svc.ProjectPoint(ctx, "nonexistent", 37.77, -122.42, 100)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})

	t.Run("propagates repository error", func(t *testing.T) {
		cameraRepo := newStubCameraFeedRepo()
		calibrationRepo := newStubCalibrationRepo()
		calibrationRepo.getErr = errors.New("connection lost")

		svc := NewCCTVService(cameraRepo, calibrationRepo)
		_, _, err := svc.ProjectPoint(ctx, "feed-1", 37.77, -122.42, 100)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if err.Error() != "connection lost" {
			t.Errorf("expected 'connection lost', got %q", err.Error())
		}
	})

	t.Run("projection with different FOV values", func(t *testing.T) {
		cameraRepo := newStubCameraFeedRepo()
		calibrationRepo := newStubCalibrationRepo()

		fovValues := []float64{30, 60, 90, 120, 180}
		for _, fov := range fovValues {
			calibrationRepo.calibrations["feed-1"] = &domain.Calibration{
				ID:           "cal-1",
				CameraFeedID: "feed-1",
				FOV:          fov,
				OffsetNorthM: 0,
				OffsetEastM:  0,
				UpdatedAt:    time.Now(),
			}

			svc := NewCCTVService(cameraRepo, calibrationRepo)
			imageX, imageY, err := svc.ProjectPoint(ctx, "feed-1", 37.77, -122.42, 100)
			if err != nil {
				t.Errorf("FOV %f: unexpected error: %v", fov, err)
			}
			if math.IsNaN(imageX) || math.IsNaN(imageY) {
				t.Errorf("FOV %f: got NaN", fov)
			}
		}
	})

	t.Run("projection with offsets", func(t *testing.T) {
		cameraRepo := newStubCameraFeedRepo()
		calibrationRepo := newStubCalibrationRepo()
		calibrationRepo.calibrations["feed-1"] = &domain.Calibration{
			ID:           "cal-1",
			CameraFeedID: "feed-1",
			FOV:          90,
			OffsetNorthM: 100,
			OffsetEastM:  50,
			UpdatedAt:    time.Now(),
		}

		svc := NewCCTVService(cameraRepo, calibrationRepo)
		imageX, imageY, err := svc.ProjectPoint(ctx, "feed-1", 37.77, -122.42, 100)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if math.IsNaN(imageX) || math.IsNaN(imageY) {
			t.Error("expected valid projection, got NaN")
		}
		if math.IsInf(imageX, 0) || math.IsInf(imageY, 0) {
			t.Error("expected finite projection, got Inf")
		}
	})
}

// TestLatLonAltToWorld tests the domain projection helper via the exported function.
func TestLatLonAltToWorld(t *testing.T) {
	tests := []struct {
		name          string
		lat, lon, alt float64
		wantValid     bool
	}{
		{"equator prime meridian", 0, 0, 0, true},
		{"SF coordinates", 37.77, -122.42, 100, true},
		{"london coordinates", 51.5, -0.1, 50, true},
		{"negative altitude", 0, 0, -100, true},
		{"high altitude", 0, 0, 10000, true},
		{"extreme longitude", 0, 180, 0, true},
		{"extreme latitude", 90, 0, 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			x, y, z := domain.LatLonAltToWorld(tt.lat, tt.lon, tt.alt)
			if math.IsNaN(x) || math.IsNaN(y) || math.IsNaN(z) {
				t.Errorf("got NaN: x=%f, y=%f, z=%f", x, y, z)
			}
			if math.IsInf(x, 0) || math.IsInf(y, 0) || math.IsInf(z, 0) {
				t.Errorf("got Inf: x=%f, y=%f, z=%f", x, y, z)
			}
			if z != tt.alt {
				t.Errorf("expected z=%f, got %f", tt.alt, z)
			}
		})
	}
}

func TestLatLonAltToWorld_Normalization(t *testing.T) {
	x1, y1, _ := domain.LatLonAltToWorld(0, 100, 0)
	x2, _, _ := domain.LatLonAltToWorld(0, 260, 0)
	if x1 == x2 {
		t.Error("expected different x values for different longitudes")
	}
	if math.Abs(x1) > 10000 || math.Abs(y1) > 10000 {
		t.Errorf("values should be normalized to < 10000: x=%f, y=%f", x1, y1)
	}
}
