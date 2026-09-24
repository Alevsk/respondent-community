package analysis

import (
	"testing"

	"github.com/Alevsk/respondent/internal/domain"
)

type stubLayers struct{ declared []domain.LayerType }

func (s stubLayers) AllLayerTypes() []domain.LayerType { return s.declared }

// An analysis definition that names no layers queries "all" of them. Sourcing
// that list from the database means rows left behind by a source that has since
// changed its layer_type keep being analysed: the engine pulls their
// observations into memory and feeds stale entities to the LLM, for a layer the
// operator can no longer see or turn off.
func TestDeclaredLayersOnly(t *testing.T) {
	declared := stubLayers{declared: []domain.LayerType{"cctv", "radio_stations"}}
	inDatabase := []string{"cctv", "cctv_austin", "cctv_calgary", "radio_stations"}

	got := declaredLayersOnly(declared, inDatabase)

	want := []string{"cctv", "radio_stations"}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("got %v, want %v", got, want)
		}
	}
}

// Order follows the database list so callers keep a stable iteration order.
func TestDeclaredLayersOnlyPreservesDatabaseOrder(t *testing.T) {
	declared := stubLayers{declared: []domain.LayerType{"b", "a"}}
	got := declaredLayersOnly(declared, []string{"a", "b"})
	if len(got) != 2 || got[0] != "a" || got[1] != "b" {
		t.Errorf("got %v, want [a b]", got)
	}
}
