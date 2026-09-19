package realtime

import (
	"testing"

	"github.com/Alevsk/respondent/internal/ai/notify"
)

func TestMatchesNotificationFilter_NilFilter_AcceptsAll(t *testing.T) {
	meta := notify.NotificationMeta{InsightType: "radiation_anomaly", Attention: "info", LayerType: "radiation"}
	if !matchesNotificationFilter(nil, meta) {
		t.Fatal("nil filter should accept all notifications")
	}
}

func TestMatchesNotificationFilter_MinAttention(t *testing.T) {
	filter := &NotificationFilter{MinAttention: "medium"}

	tests := []struct {
		attention string
		want      bool
	}{
		{"info", false},
		{"low", false},
		{"medium", true},
		{"high", true},
		{"critical", true},
		{"", true}, // no attention = accept (don't block unclassified)
	}
	for _, tt := range tests {
		meta := notify.NotificationMeta{InsightType: "test", Attention: tt.attention}
		got := matchesNotificationFilter(filter, meta)
		if got != tt.want {
			t.Errorf("MinAttention=medium, Attention=%q: got %v, want %v", tt.attention, got, tt.want)
		}
	}
}

func TestMatchesNotificationFilter_InsightTypes(t *testing.T) {
	filter := &NotificationFilter{
		InsightTypes: map[string]bool{"geopolitical_intel": true, "radiation_anomaly": true},
	}

	if !matchesNotificationFilter(filter, notify.NotificationMeta{InsightType: "geopolitical_intel"}) {
		t.Fatal("whitelisted type should pass")
	}
	if matchesNotificationFilter(filter, notify.NotificationMeta{InsightType: "maritime_cable_threat"}) {
		t.Fatal("non-whitelisted type should be rejected")
	}
}

func TestMatchesNotificationFilter_EmptyInsightTypes_AcceptsAll(t *testing.T) {
	filter := &NotificationFilter{InsightTypes: map[string]bool{}}
	if !matchesNotificationFilter(filter, notify.NotificationMeta{InsightType: "anything"}) {
		t.Fatal("empty whitelist should accept all types")
	}
}

func TestMatchesNotificationFilter_CombinedFilters(t *testing.T) {
	filter := &NotificationFilter{
		MinAttention: "high",
		InsightTypes: map[string]bool{"radiation_anomaly": true},
	}

	// Passes both: correct type AND high attention
	if !matchesNotificationFilter(filter, notify.NotificationMeta{InsightType: "radiation_anomaly", Attention: "critical"}) {
		t.Fatal("should pass both filters")
	}
	// Fails attention check
	if matchesNotificationFilter(filter, notify.NotificationMeta{InsightType: "radiation_anomaly", Attention: "low"}) {
		t.Fatal("should fail attention check")
	}
	// Fails type check
	if matchesNotificationFilter(filter, notify.NotificationMeta{InsightType: "geopolitical_intel", Attention: "high"}) {
		t.Fatal("should fail type check")
	}
}
