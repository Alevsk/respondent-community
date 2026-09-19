package cctvtest

import (
	"context"
	"sync"

	"github.com/Alevsk/respondent/internal/domain"
)

// FakeCameraFeedRepo implements domain.CameraFeedRepository for testing.
type FakeCameraFeedRepo struct {
	mu    sync.RWMutex
	Feeds map[string]*domain.CameraFeed
}

func NewFakeCameraFeedRepo() *FakeCameraFeedRepo {
	return &FakeCameraFeedRepo{Feeds: make(map[string]*domain.CameraFeed)}
}

func (f *FakeCameraFeedRepo) Create(_ context.Context, feed *domain.CameraFeed) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.Feeds[feed.ID] = feed
	return nil
}

func (f *FakeCameraFeedRepo) GetByID(_ context.Context, id string) (*domain.CameraFeed, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()
	feed, ok := f.Feeds[id]
	if !ok {
		return nil, domain.NewNotFoundError("camera feed not found", nil)
	}
	return feed, nil
}

func (f *FakeCameraFeedRepo) List(_ context.Context, _ string) ([]*domain.CameraFeed, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()
	var result []*domain.CameraFeed
	for _, feed := range f.Feeds {
		result = append(result, feed)
	}
	return result, nil
}

func (f *FakeCameraFeedRepo) GetNearest(_ context.Context, _ domain.GeoPoint, _ string) (*domain.CameraFeed, error) {
	return nil, domain.NewNotFoundError("no nearby camera", nil)
}

// FakeCalibrationRepo implements domain.CalibrationRepository for testing.
type FakeCalibrationRepo struct {
	mu           sync.RWMutex
	Calibrations map[string]*domain.Calibration // keyed by CameraFeedID
}

func NewFakeCalibrationRepo() *FakeCalibrationRepo {
	return &FakeCalibrationRepo{Calibrations: make(map[string]*domain.Calibration)}
}

func (f *FakeCalibrationRepo) GetByCameraFeedID(_ context.Context, id string) (*domain.Calibration, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()
	cal, ok := f.Calibrations[id]
	if !ok {
		return nil, domain.NewNotFoundError("calibration not found", nil)
	}
	return cal, nil
}

func (f *FakeCalibrationRepo) Update(_ context.Context, cal *domain.Calibration) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.Calibrations[cal.CameraFeedID] = cal
	return nil
}

func (f *FakeCalibrationRepo) Delete(_ context.Context, id string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.Calibrations, id)
	return nil
}

var (
	_ domain.CameraFeedRepository  = (*FakeCameraFeedRepo)(nil)
	_ domain.CalibrationRepository = (*FakeCalibrationRepo)(nil)
)
