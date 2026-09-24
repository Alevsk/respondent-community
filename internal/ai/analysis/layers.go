package analysis

import "github.com/Alevsk/respondent/internal/domain"

// LayerRegistry reports which layer types a loaded source declares. The engine
// depends on the interface rather than the concrete registry (DIP).
type LayerRegistry interface {
	AllLayerTypes() []domain.LayerType
}

// declaredLayersOnly narrows layer types discovered in the database to those a
// source still declares.
//
// An analysis that names no layers means "all layers". Answering that from the
// database alone includes rows stranded by a source that has since changed its
// layer_type or been removed: the engine would keep pulling their observations
// into memory and feeding stale entities to the LLM, for a layer the operator
// can no longer see in the panel or switch off. Declarations decide which
// layers exist; the database only says which of them hold data.
func declaredLayersOnly(registry LayerRegistry, inDatabase []string) []string {
	declared := make(map[string]struct{})
	for _, lt := range registry.AllLayerTypes() {
		declared[string(lt)] = struct{}{}
	}
	kept := make([]string, 0, len(inDatabase))
	for _, lt := range inDatabase {
		if _, ok := declared[lt]; ok {
			kept = append(kept, lt)
		}
	}
	return kept
}
