package domain

import "testing"

func TestDynamicRegistry_FilteringModes(t *testing.T) {
	reg := &DynamicSourceRegistry{
		sourceToLayer:      make(map[SourceType]LayerType),
		layerToSources:     make(map[LayerType][]SourceType),
		displayConfigs:     make(map[LayerType]*LayerDisplayConfig),
		filteringModes:     make(map[LayerType]string),
		backfillThresholds: make(map[LayerType]int),
		onDemandURLs:       make(map[LayerType]string),
	}

	lt := LayerType("test_flights")

	// Not set yet
	if _, ok := reg.LookupFilteringMode(lt); ok {
		t.Error("expected LookupFilteringMode to return false before set")
	}

	// Set and lookup
	reg.SetFilteringMode(lt, "viewport")
	mode, ok := reg.LookupFilteringMode(lt)
	if !ok {
		t.Fatal("expected LookupFilteringMode to return true after set")
	}
	if mode != "viewport" {
		t.Errorf("LookupFilteringMode = %q, want %q", mode, "viewport")
	}

	// AllFilteringModes
	reg.SetFilteringMode(LayerType("test_quakes"), "all")
	all := reg.AllFilteringModes()
	if len(all) != 2 {
		t.Errorf("AllFilteringModes len = %d, want 2", len(all))
	}
	if all[LayerType("test_quakes")] != "all" {
		t.Errorf("AllFilteringModes[test_quakes] = %q, want %q", all[LayerType("test_quakes")], "all")
	}

	// Overwrite
	reg.SetFilteringMode(lt, "all")
	mode, _ = reg.LookupFilteringMode(lt)
	if mode != "all" {
		t.Errorf("overwritten mode = %q, want %q", mode, "all")
	}
}

func TestDynamicRegistry_BackfillThresholds(t *testing.T) {
	reg := &DynamicSourceRegistry{
		sourceToLayer:      make(map[SourceType]LayerType),
		layerToSources:     make(map[LayerType][]SourceType),
		displayConfigs:     make(map[LayerType]*LayerDisplayConfig),
		filteringModes:     make(map[LayerType]string),
		backfillThresholds: make(map[LayerType]int),
		onDemandURLs:       make(map[LayerType]string),
	}

	lt := LayerType("earthquakes")

	// Not set yet
	if _, ok := reg.LookupBackfillThreshold(lt); ok {
		t.Error("expected LookupBackfillThreshold to return false before set")
	}

	// Set and lookup
	reg.SetBackfillThreshold(lt, 5)
	threshold, ok := reg.LookupBackfillThreshold(lt)
	if !ok {
		t.Fatal("expected LookupBackfillThreshold to return true after set")
	}
	if threshold != 5 {
		t.Errorf("LookupBackfillThreshold = %d, want %d", threshold, 5)
	}

	// AllBackfillThresholds
	reg.SetBackfillThreshold(LayerType("satellites"), 10)
	all := reg.AllBackfillThresholds()
	if len(all) != 2 {
		t.Errorf("AllBackfillThresholds len = %d, want 2", len(all))
	}
}

func TestDynamicRegistry_OnDemandURLs(t *testing.T) {
	reg := &DynamicSourceRegistry{
		sourceToLayer:      make(map[SourceType]LayerType),
		layerToSources:     make(map[LayerType][]SourceType),
		displayConfigs:     make(map[LayerType]*LayerDisplayConfig),
		filteringModes:     make(map[LayerType]string),
		backfillThresholds: make(map[LayerType]int),
		onDemandURLs:       make(map[LayerType]string),
	}

	lt := LayerType("flights_commercial")

	// Not set yet
	if _, ok := reg.LookupOnDemandURL(lt); ok {
		t.Error("expected LookupOnDemandURL to return false before set")
	}

	// Set and lookup
	url := "https://api.adsb.lol/v2/point/{lat}/{lon}/250"
	reg.SetOnDemandURL(lt, url)
	got, ok := reg.LookupOnDemandURL(lt)
	if !ok {
		t.Fatal("expected LookupOnDemandURL to return true after set")
	}
	if got != url {
		t.Errorf("LookupOnDemandURL = %q, want %q", got, url)
	}

	// AllOnDemandURLs
	all := reg.AllOnDemandURLs()
	if len(all) != 1 {
		t.Errorf("AllOnDemandURLs len = %d, want 1", len(all))
	}
}

func TestDynamicRegistry_V2SnapshotIsolation(t *testing.T) {
	// Verify that All* methods return snapshots, not live references.
	reg := &DynamicSourceRegistry{
		sourceToLayer:      make(map[SourceType]LayerType),
		layerToSources:     make(map[LayerType][]SourceType),
		displayConfigs:     make(map[LayerType]*LayerDisplayConfig),
		filteringModes:     make(map[LayerType]string),
		backfillThresholds: make(map[LayerType]int),
		onDemandURLs:       make(map[LayerType]string),
	}

	lt := LayerType("snap_test")
	reg.SetFilteringMode(lt, "viewport")

	snap := reg.AllFilteringModes()
	// Mutate the snapshot — should not affect the registry.
	snap[LayerType("injected")] = "hacked"

	if _, ok := reg.LookupFilteringMode(LayerType("injected")); ok {
		t.Error("snapshot mutation leaked into registry")
	}
}
