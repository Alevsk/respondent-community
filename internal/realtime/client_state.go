package realtime

import (
	"context"
	"encoding/json"
	"time"

	"github.com/coder/websocket"
	"github.com/rs/zerolog"

	"github.com/Alevsk/respondent/internal/domain"
)

// NewClient creates a new Client with properly initialized internal fields
func NewClient(id string, conn *websocket.Conn, send chan []byte, logger zerolog.Logger) *Client {
	return &Client{
		ID:              id,
		Conn:            conn,
		Send:            send,
		Logger:          logger,
		subscriptions:   make(map[string]bool),
		viewports:       make(map[string]*domain.BBox),
		timeFrom:        make(map[string]time.Time),
		timeTo:          make(map[string]time.Time),
		timeFrozen:      make(map[string]bool),
		lastBackfill:    make(map[string]time.Time),
		lastOnDemand:    make(map[string]time.Time),
		lastTimeRange:   make(map[string]time.Time),
		onDemandTickers: make(map[string]context.CancelFunc),
		globalLayers:    make(map[string]bool),
	}
}

// SetViewport stores the client's viewport for a given layer.
func (c *Client) SetViewport(layerID string, bbox *domain.BBox) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.viewports == nil {
		c.viewports = make(map[string]*domain.BBox)
	}
	c.viewports[layerID] = bbox
}

// GetViewport returns the client's viewport for a given layer, or nil if not set.
func (c *Client) GetViewport(layerID string) *domain.BBox {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.viewports == nil {
		return nil
	}
	return c.viewports[layerID]
}

// IsGlobalMode returns true if the client has a global-mode subscription for the layer.
func (c *Client) IsGlobalMode(layerID string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.globalLayers[layerID]
}

// SetGlobalMode marks or clears global mode for a layer subscription.
func (c *Client) SetGlobalMode(layerID string, global bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.globalLayers == nil {
		c.globalLayers = make(map[string]bool)
	}
	if global {
		c.globalLayers[layerID] = true
	} else {
		delete(c.globalLayers, layerID)
	}
}

// IsSubscribed checks if client is subscribed to a layer
func (c *Client) IsSubscribed(layerID string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.subscriptions[layerID]
}

// IsFrozen checks if the client has a frozen time range for a layer (suppresses live updates).
func (c *Client) IsFrozen(layerID string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.timeFrozen[layerID]
}

// Subscribe adds a layer to the client's subscriptions and resets time range state.
func (c *Client) Subscribe(layerID string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.subscriptions[layerID] = true
	// Re-subscribing resets to live mode for this layer
	delete(c.timeFrom, layerID)
	delete(c.timeTo, layerID)
	delete(c.timeFrozen, layerID)
}

// EnableSubscription marks the client as subscribed to a layer's Pub/Sub
// updates without resetting the time range state. This allows live-mode
// time_range clients to receive real-time updates alongside their initial
// snapshot, unlike Subscribe() which clears time range state.
func (c *Client) EnableSubscription(layerID string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.subscriptions[layerID] = true
}

// Unsubscribe removes a layer from the client's subscriptions
func (c *Client) Unsubscribe(layerID string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.subscriptions, layerID)
	delete(c.timeFrom, layerID)
	delete(c.timeTo, layerID)
	delete(c.timeFrozen, layerID)
	delete(c.globalLayers, layerID)
}

// SetTimeRange sets the client's time range for a layer.
func (c *Client) SetTimeRange(layerID string, from, to time.Time, frozen bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.timeFrom == nil {
		c.timeFrom = make(map[string]time.Time)
		c.timeTo = make(map[string]time.Time)
		c.timeFrozen = make(map[string]bool)
	}
	c.timeFrom[layerID] = from
	c.timeTo[layerID] = to
	c.timeFrozen[layerID] = frozen
}

// IsInTimeRange checks if the client has an active time range for a layer.
func (c *Client) IsInTimeRange(layerID string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	_, ok := c.timeFrom[layerID]
	return ok
}

// GetTimeRange returns the active time range for a layer.
// Returns zero times and false if no time range is active.
func (c *Client) GetTimeRange(layerID string) (from, to time.Time, ok bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	from, ok = c.timeFrom[layerID]
	if !ok {
		return time.Time{}, time.Time{}, false
	}
	to = c.timeTo[layerID]
	return from, to, true
}

// canBackfill checks cooldown and returns true if a backfill query is allowed.
func (c *Client) canBackfill(layerID string, cooldown time.Duration) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.lastBackfill == nil {
		c.lastBackfill = make(map[string]time.Time)
	}
	if last, ok := c.lastBackfill[layerID]; ok && time.Since(last) < cooldown {
		return false
	}
	c.lastBackfill[layerID] = time.Now()
	return true
}

// canTimeRange checks cooldown and returns true if a time_range query is allowed.
func (c *Client) canTimeRange(layerID string, cooldown time.Duration) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.lastTimeRange == nil {
		c.lastTimeRange = make(map[string]time.Time)
	}
	if last, ok := c.lastTimeRange[layerID]; ok && time.Since(last) < cooldown {
		return false
	}
	c.lastTimeRange[layerID] = time.Now()
	return true
}

// canOnDemand checks cooldown and returns true if an on-demand fetch is allowed.
func (c *Client) canOnDemand(layerID string, cooldown time.Duration) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.lastOnDemand == nil {
		c.lastOnDemand = make(map[string]time.Time)
	}
	if last, ok := c.lastOnDemand[layerID]; ok && time.Since(last) < cooldown {
		return false
	}
	c.lastOnDemand[layerID] = time.Now()
	return true
}

// GetSubscriptions returns a copy of the client's subscriptions
func (c *Client) GetSubscriptions() []string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	subs := make([]string, 0, len(c.subscriptions))
	for layerID := range c.subscriptions {
		subs = append(subs, layerID)
	}
	return subs
}

// readPump reads messages from the WebSocket connection.
// It also starts a background goroutine that sends periodic pings to detect
// dead clients (coder/websocket does not auto-ping on the accept side).
func (c *Client) readPump(unregister chan *Client) {
	ctx, cancel := context.WithCancel(context.Background())
	defer func() {
		cancel()
		unregister <- c
		_ = c.Conn.CloseNow()
	}()

	// Periodic pings — detect dead clients even when no messages are flowing.
	go func() {
		ticker := time.NewTicker(54 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				pingCtx, pingCancel := context.WithTimeout(ctx, 10*time.Second)
				if err := c.Conn.Ping(pingCtx); err != nil {
					pingCancel()
					cancel() // peer is unresponsive — abort read loop
					return
				}
				pingCancel()
			}
		}
	}()

	for {
		_, message, err := c.Conn.Read(ctx)
		if err != nil {
			// Don't log expected closures or deliberate cancellations.
			if ctx.Err() == nil {
				status := websocket.CloseStatus(err)
				if status != websocket.StatusNormalClosure && status != websocket.StatusGoingAway {
					c.Logger.Error().Err(err).Msg("read error")
				}
			}
			break
		}

		// Handle client messages (subscriptions, etc.)
		c.handleMessage(message)
	}
}

// writePump drains the Send channel and writes messages to the WebSocket.
// Ping/pong is handled by the readPump's background goroutine, so no
// ticker is needed here — this is purely a write loop.
func (c *Client) writePump() {
	for message := range c.Send {
		// Before forwarding the queued message, notify the client if any
		// messages were silently dropped since the last write.  The
		// backpressure frame is sent inline here — not via the Send channel
		// — so it is never itself subject to buffer-full drops.
		if dropped := c.droppedCount.Swap(0); dropped > 0 {
			bp := struct {
				Type string `json:"type"`
				Data struct {
					Dropped int64  `json:"dropped"`
					Message string `json:"message"`
				} `json:"data"`
			}{
				Type: "backpressure",
			}
			bp.Data.Dropped = dropped
			bp.Data.Message = "messages were dropped, consider resyncing"
			if bpData, err := json.Marshal(bp); err == nil {
				bpCtx, bpCancel := context.WithTimeout(context.Background(), 10*time.Second)
				writeErr := c.Conn.Write(bpCtx, websocket.MessageText, bpData)
				bpCancel()
				if writeErr != nil {
					c.Logger.Error().Err(writeErr).Msg("write error (backpressure notification)")
					_ = c.Conn.CloseNow()
					return
				}
			}
		}

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		err := c.Conn.Write(ctx, websocket.MessageText, message)
		cancel()
		if err != nil {
			c.Logger.Error().Err(err).Msg("write error")
			// Close the underlying connection so readPump's conn.Read
			// unblocks immediately instead of waiting for the next ping
			// timeout (up to 64s).
			_ = c.Conn.CloseNow()
			return
		}
	}
	// Send channel closed — graceful close.
	_ = c.Conn.Close(websocket.StatusNormalClosure, "")
}

// handleMessage handles incoming client messages
func (c *Client) handleMessage(message []byte) {
	var msg WSMessage
	if err := json.Unmarshal(message, &msg); err != nil {
		c.Logger.Error().Err(err).Msg("failed to parse message")
		return
	}

	switch msg.Type {
	case "subscribe":
		c.handleSubscribe(msg)

	case "unsubscribe":
		c.handleUnsubscribe(msg)

	case "viewport_update":
		c.handleViewportUpdate(msg)

	case "time_range":
		c.handleTimeRange(msg)

	case "notification_filter":
		c.handleNotificationFilter(msg)

	case "page":
		c.handlePage(msg)

	case "ping": // reply pong to keep application-level heartbeat alive
		if b, err := json.Marshal(WSMessage{Type: "pong"}); err == nil {
			select {
			case c.Send <- b:
			default:
			}
		}

	default:
		c.Logger.Warn().Str("type", msg.Type).Msg("unknown message type")
	}
}
