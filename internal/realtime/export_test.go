package realtime

import (
	"context"
	"encoding/json"
	"time"

	"github.com/rs/zerolog"

	"github.com/Alevsk/respondent/internal/domain"
)

// MsgType* constants expose the WebSocket message type strings used by the
// server so that external test packages can assert against them without
// duplicating the string literals.
const (
	MsgTypeLayerUpdate        = "layer.update"
	MsgTypeLayerSnapshot      = "layer.snapshot"
	MsgTypeIndicatorUpdate    = "indicator.update"
	MsgTypeCCTVFrame          = "cctv.frame"
	MsgTypeLayerBatchUpdate   = "layer.batch_update"
	MsgTypeSystemBackpressure = "system.backpressure"
)

// FetchSparseRegion exports fetchSparseRegion for external tests.
func (s *Server) FetchSparseRegion(
	ctx context.Context,
	c *Client,
	layerID, layerType string,
	bbox *domain.BBox,
	cacheEntities []*domain.Entity,
) ([]*domain.Entity, []*domain.Observation) {
	return s.fetchSparseRegion(ctx, c, layerID, layerType, bbox, cacheEntities)
}

// BBoxCenter exports bboxCenter for external tests.
func BBoxCenter(bbox *domain.BBox) (lat, lon float64) {
	return bboxCenter(bbox)
}

// GetOnDemandPollInterval exports getOnDemandPollInterval for external tests.
func (s *Server) GetOnDemandPollInterval(layerType string) time.Duration {
	return s.getOnDemandPollInterval(layerType)
}

// DoOnDemandFetchDirect exports doOnDemandFetchDirect for external tests.
func (s *Server) DoOnDemandFetchDirect(
	ctx context.Context,
	layerID, layerType string,
	bbox *domain.BBox,
) ([]*domain.Entity, []*domain.Observation) {
	return s.doOnDemandFetchDirect(ctx, layerID, layerType, bbox)
}

// StartOnDemandTicker exports startOnDemandTicker for external tests.
func (c *Client) StartOnDemandTicker(layerID, layerType string, bbox *domain.BBox) {
	c.startOnDemandTicker(layerID, layerType, bbox)
}

// StopOnDemandTicker exports stopOnDemandTicker for external tests.
func (c *Client) StopOnDemandTicker(layerID string) {
	c.stopOnDemandTicker(layerID)
}

// StopAllOnDemandTickers exports stopAllOnDemandTickers for external tests.
func (c *Client) StopAllOnDemandTickers() {
	c.stopAllOnDemandTickers()
}

// OnDemandTickerCount returns the number of active on-demand tickers for testing.
func (c *Client) OnDemandTickerCount() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.onDemandTickers)
}

// HasOnDemandTicker reports whether a ticker is registered for the given layer.
func (c *Client) HasOnDemandTicker(layerID string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	_, ok := c.onDemandTickers[layerID]
	return ok
}

// SetServer wires a server into a client for testing without a real WebSocket
// connection (mirrors what the production readPump does implicitly via the
// server.HandleWS path).
func (c *Client) SetServer(s *Server) {
	c.server = s
}

// HandleTimeRange exports handleTimeRange for external tests.
func (c *Client) HandleTimeRange(msg WSMessage) {
	c.handleTimeRange(msg)
}

// CanBackfill exports canBackfill for external tests.
func (c *Client) CanBackfill(layerID string, cooldown time.Duration) bool {
	return c.canBackfill(layerID, cooldown)
}

// CanTimeRange exports canTimeRange for external tests.
func (c *Client) CanTimeRange(layerID string, cooldown time.Duration) bool {
	return c.canTimeRange(layerID, cooldown)
}

// CanOnDemand exports canOnDemand for external tests.
func (c *Client) CanOnDemand(layerID string, cooldown time.Duration) bool {
	return c.canOnDemand(layerID, cooldown)
}

// IsSpatialLayer exports isSpatialLayer for external tests.
func (s *Server) IsSpatialLayer(layerType string) bool {
	return s.isSpatialLayer(layerType)
}

// ViewportEntityLimit exports viewportEntityLimit for external tests.
func ViewportEntityLimit(bbox *domain.BBox, baseLimit int) int {
	return viewportEntityLimit(bbox, baseLimit)
}

// GenerateClientID exports generateClientID for external tests.
func GenerateClientID() string {
	return generateClientID()
}

// HandleMessage exports handleMessage for external tests.
func (c *Client) HandleMessage(message []byte) {
	c.handleMessage(message)
}

// GetBackfillThreshold exports getBackfillThreshold for external tests.
func (s *Server) GetBackfillThreshold(layerType string) int {
	return s.getBackfillThreshold(layerType)
}

// DoBackfill exports doBackfill for external tests.
func (s *Server) DoBackfill(
	ctx context.Context,
	layerID, layerType string,
	bbox *domain.BBox,
	existingEntities []*domain.Entity,
) ([]*domain.Entity, []*domain.Observation) {
	return s.doBackfill(ctx, layerID, layerType, bbox, existingEntities)
}

// FlushBatchedUpdates exports flushBatchedUpdates for external tests.
func (s *Server) FlushBatchedUpdates() {
	s.flushBatchedUpdates()
}

// AddPendingUpdate injects a raw JSON update into the server's pending updates
// map so that FlushBatchedUpdates can be tested without a real Redis connection.
func (s *Server) AddPendingUpdate(layerID string, data []byte, lat, lon float64, hasSpatial bool) {
	s.pendingMu.Lock()
	defer s.pendingMu.Unlock()
	s.pendingUpdates[layerID] = append(s.pendingUpdates[layerID], pendingUpdate{
		data:       json.RawMessage(data),
		lat:        lat,
		lon:        lon,
		hasSpatial: hasSpatial,
	})
}

// HandleViewportUpdate exports handleViewportUpdate for external tests.
func (c *Client) HandleViewportUpdate(msg WSMessage) {
	c.handleViewportUpdate(msg)
}

// HandleSubscribe exports handleSubscribe for external tests.
func (c *Client) HandleSubscribe(msg WSMessage) {
	c.handleSubscribe(msg)
}

// HandleUnsubscribe exports handleUnsubscribe for external tests.
func (c *Client) HandleUnsubscribe(msg WSMessage) {
	c.handleUnsubscribe(msg)
}

// HandlePage exports handlePage for external tests.
func (c *Client) HandlePage(msg WSMessage) {
	c.handlePage(msg)
}

// EncodeCursor exports encodeCursor for external tests.
func EncodeCursor(offset int) string {
	return encodeCursor(offset)
}

// DecodeCursor exports decodeCursor for external tests.
func DecodeCursor(cursor string) (int, error) {
	pc, err := decodeCursor(cursor)
	if err != nil {
		return 0, err
	}
	return pc.Offset, nil
}

// BuildIndicatorUpdateExport exports buildIndicatorUpdate for external tests.
func BuildIndicatorUpdateExport(dynReg *domain.DynamicSourceRegistry, layerType string, obs *domain.Observation) *IndicatorUpdateData {
	return buildIndicatorUpdate(dynReg, layerType, obs)
}

// NewClientWithNilMaps creates a Client without initializing the internal maps,
// allowing tests to exercise the nil-init branches in SetViewport / SetTimeRange
// / canBackfill / canTimeRange / canOnDemand.
// This simulates a client created via a code path that skips NewClient().
func NewClientWithNilMaps(id string, send chan []byte, logger zerolog.Logger) *Client {
	return &Client{
		ID:     id,
		Send:   send,
		Logger: logger,
		// All optional maps intentionally left nil to exercise nil-init branches.
		subscriptions:   make(map[string]bool),
		onDemandTickers: make(map[string]context.CancelFunc),
		// viewports, timeFrom, timeTo, timeFrozen, lastBackfill, lastOnDemand,
		// lastTimeRange are all nil.
	}
}

// AddClientDirect inserts a client directly into the server's clients map,
// bypassing the register channel. This allows tests to set up client state
// without needing a running Run() loop (e.g., to test the unregister channel
// full default branch in BroadcastLayerUpdate).
func (s *Server) AddClientDirect(c *Client) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.clients[c] = true
}
