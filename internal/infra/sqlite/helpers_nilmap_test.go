package sqlite

import "testing"

// JSON "null" unmarshals to a nil map; unmarshalAnyMap must still return a usable
// (non-nil) map, and mergeJSONMaps must not panic patching onto it.
func TestUnmarshalAnyMap_NullNeverNil(t *testing.T) {
	for _, in := range []string{"null", "", "{}", `{"a":1}`} {
		if m := unmarshalAnyMap(in); m == nil {
			t.Errorf("unmarshalAnyMap(%q) returned nil map", in)
		}
	}
}

func TestMergeJSONMaps_OntoNullDoesNotPanic(t *testing.T) {
	got := mergeJSONMaps("null", map[string]any{"k": "v"})
	if got != `{"k":"v"}` {
		t.Errorf("mergeJSONMaps onto null = %q, want {\"k\":\"v\"}", got)
	}
}
