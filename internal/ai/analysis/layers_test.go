package analysis

import (
	"context"
	"testing"
	"time"

	"github.com/rs/zerolog"

	"github.com/Alevsk/respondent/internal/llm"

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

// A layer whose source no longer declares it must not be analysed, even though
// its rows are still in the database: the engine would pull stale observations
// into memory and feed them to the LLM for a layer the operator can no longer
// see in the panel or switch off. This exercises the narrowing through
// RunAnalysis rather than through the helper alone.
func TestRunAnalysisSkipsUndeclaredLayers(t *testing.T) {
	provider := &mockLLMProvider{
		response: &llm.CompletionResponse{Content: `{"summary":"ok"}`, Model: "test"},
	}
	engine, entityRepo, obsRepo, _ := newTestEngine(t, provider)

	// Both layers hold data; only one is still declared.
	entityRepo.distinctLayers = []string{"cctv", "cctv_austin"}
	engine.layers = stubLayers{declared: []domain.LayerType{"cctv"}}

	now := engine.clock.Now()
	for _, lt := range []string{"cctv", "cctv_austin"} {
		e := makeTestEntity("ent-"+lt, "EXT-"+lt, "cam "+lt, lt)
		entityRepo.AddEntity(e)
		obsRepo.AddLatestForLayer(lt, []*domain.Observation{
			makeTestObservation("obs-"+lt, e.ID, 40, -74, 0, now.Add(-time.Minute)),
		})
	}

	def := makeTestDefinition("undeclared_layer_test")
	def.Data.Layers = []string{} // empty = all layers

	records, err := engine.fetchData(context.Background(), def, zerolog.Nop())
	if err != nil {
		t.Fatalf("fetchData: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("got %d records, want 1 (only the declared layer)", len(records))
	}
	if records[0].LayerType != "cctv" {
		t.Errorf("analysed layer %q, want cctv; the undeclared layer's rows must be skipped", records[0].LayerType)
	}
}
