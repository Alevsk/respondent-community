package realtime

import (
	"encoding/json"

	"github.com/Alevsk/respondent/internal/ai/notify"
	"github.com/Alevsk/respondent/internal/domain"
)

// NotificationFilter defines per-client notification preferences.
// A nil filter means "accept all notifications".
type NotificationFilter struct {
	// MinAttention is the minimum attention level to receive (inclusive).
	// Empty string means no minimum — accept all levels.
	MinAttention string

	// InsightTypes is a whitelist of insight types to receive.
	// Empty map means no restriction — accept all types.
	InsightTypes map[string]bool
}

// notificationBroadcastMsg bundles the filter metadata with the pre-serialized
// JSON payload for the filtered broadcast channel.
type notificationBroadcastMsg struct {
	Meta notify.NotificationMeta
	Data []byte
}

// matchesNotificationFilter checks whether a notification passes the client's filter.
// Returns true if the notification should be delivered.
func matchesNotificationFilter(f *NotificationFilter, meta notify.NotificationMeta) bool {
	if f == nil {
		return true
	}

	// Check minimum attention level. Empty meta.Attention passes through
	// to avoid blocking unclassified notifications.
	if f.MinAttention != "" && meta.Attention != "" {
		if domain.AttentionRank(meta.Attention) < domain.AttentionRank(f.MinAttention) {
			return false
		}
	}

	// Check insight type whitelist.
	if len(f.InsightTypes) > 0 {
		if !f.InsightTypes[meta.InsightType] {
			return false
		}
	}

	return true
}

// handleNotificationFilter processes a notification_filter WebSocket message
// and stores the filter preferences on the client.
func (c *Client) handleNotificationFilter(msg WSMessage) {
	if msg.Data == nil {
		c.mu.Lock()
		c.notificationFilter = nil
		c.mu.Unlock()
		c.Logger.Info().Msg("notification filter cleared")
		return
	}

	dataBytes, err := json.Marshal(msg.Data)
	if err != nil {
		c.Logger.Error().Err(err).Msg("failed to marshal notification_filter data")
		return
	}

	var filterData struct {
		MinAttention string   `json:"min_attention"`
		InsightTypes []string `json:"insight_types"`
	}
	if err := json.Unmarshal(dataBytes, &filterData); err != nil {
		c.Logger.Error().Err(err).Msg("failed to parse notification_filter data")
		return
	}

	filter := &NotificationFilter{
		MinAttention: filterData.MinAttention,
	}
	if len(filterData.InsightTypes) > 0 {
		filter.InsightTypes = make(map[string]bool, len(filterData.InsightTypes))
		for _, t := range filterData.InsightTypes {
			filter.InsightTypes[t] = true
		}
	}

	c.mu.Lock()
	c.notificationFilter = filter
	c.mu.Unlock()

	c.Logger.Info().
		Str("min_attention", filterData.MinAttention).
		Int("insight_types", len(filterData.InsightTypes)).
		Msg("notification filter updated")
}
