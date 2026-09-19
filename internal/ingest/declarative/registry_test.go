package declarative

import (
	"sync"
	"testing"
)

func TestRegistry_AddGetRemove(t *testing.T) {
	reg := NewRegistry()

	cs := &CompiledSource{
		definition: &SourceDefinition{Name: "test_source"},
	}

	// Get on empty registry
	_, ok := reg.Get("test_source")
	if ok {
		t.Error("expected not found on empty registry")
	}

	// Add
	reg.Add("test_source", cs)

	// Get
	got, ok := reg.Get("test_source")
	if !ok {
		t.Fatal("expected to find test_source after Add")
	}
	if got.Definition().Name != "test_source" {
		t.Errorf("expected name 'test_source', got %q", got.Definition().Name)
	}

	// List
	list := reg.List()
	if len(list) != 1 {
		t.Errorf("expected 1 source in list, got %d", len(list))
	}

	// Remove
	reg.Remove("test_source")
	_, ok = reg.Get("test_source")
	if ok {
		t.Error("expected not found after Remove")
	}

	// Remove non-existent (should not panic)
	reg.Remove("nonexistent")

	// List after remove
	list = reg.List()
	if len(list) != 0 {
		t.Errorf("expected 0 sources after remove, got %d", len(list))
	}
}

func TestRegistry_Overwrite(t *testing.T) {
	reg := NewRegistry()

	cs1 := &CompiledSource{
		definition: &SourceDefinition{Name: "source", DisplayName: "Version 1"},
	}
	cs2 := &CompiledSource{
		definition: &SourceDefinition{Name: "source", DisplayName: "Version 2"},
	}

	reg.Add("source", cs1)
	reg.Add("source", cs2)

	got, ok := reg.Get("source")
	if !ok {
		t.Fatal("expected to find source")
	}
	if got.Definition().DisplayName != "Version 2" {
		t.Errorf("expected 'Version 2', got %q", got.Definition().DisplayName)
	}

	list := reg.List()
	if len(list) != 1 {
		t.Errorf("expected 1 source after overwrite, got %d", len(list))
	}
}

func TestRegistry_ConcurrentAccess(t *testing.T) {
	reg := NewRegistry()

	var wg sync.WaitGroup
	const numGoroutines = 100

	// Concurrent writes
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			name := "source"
			cs := &CompiledSource{
				definition: &SourceDefinition{Name: name},
			}
			reg.Add(name, cs)
		}(i)
	}

	// Concurrent reads
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			reg.Get("source")
			reg.List()
		}()
	}

	wg.Wait()

	// Should still have the source
	_, ok := reg.Get("source")
	if !ok {
		t.Error("expected source to exist after concurrent access")
	}
}
