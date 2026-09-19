package domain

import "testing"

func TestObservationRecordMode_Valid(t *testing.T) {
	tests := []struct {
		mode ObservationRecordMode
		want bool
	}{
		{RecordAppend, true},
		{RecordUpsert, true},
		{RecordDedupe, true},
		{"unknown", false},
		{"", false},
		{"APPEND", false}, // case-sensitive
	}

	for _, tt := range tests {
		t.Run(string(tt.mode), func(t *testing.T) {
			got := tt.mode.Valid()
			if got != tt.want {
				t.Errorf("ObservationRecordMode(%q).Valid() = %v, want %v", tt.mode, got, tt.want)
			}
		})
	}
}

func TestFilteringMode_Constants(t *testing.T) {
	// Verify string values are correct.
	if FilteringViewport != "viewport" {
		t.Errorf("FilteringViewport = %q, want %q", FilteringViewport, "viewport")
	}
	if FilteringAll != "all" {
		t.Errorf("FilteringAll = %q, want %q", FilteringAll, "all")
	}
}

func TestObservationRecordMode_Constants(t *testing.T) {
	// Verify string values are correct.
	if RecordAppend != "append" {
		t.Errorf("RecordAppend = %q, want %q", RecordAppend, "append")
	}
	if RecordUpsert != "upsert" {
		t.Errorf("RecordUpsert = %q, want %q", RecordUpsert, "upsert")
	}
	if RecordDedupe != "dedupe" {
		t.Errorf("RecordDedupe = %q, want %q", RecordDedupe, "dedupe")
	}
}
