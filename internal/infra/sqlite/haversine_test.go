package sqlite

import (
	"database/sql"
	"math"
	"testing"
)

func TestHaversineKm_RegisteredAndAccurate(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()

	// 1 degree of longitude at the equator ≈ 111.19 km.
	var km float64
	if err := db.QueryRow("SELECT haversine_km(0,0,0,1)").Scan(&km); err != nil {
		t.Fatalf("query: %v", err)
	}
	if math.Abs(km-111.19) > 0.5 {
		t.Errorf("haversine_km(0,0,0,1) = %.2f, want ~111.19", km)
	}

	// Identical points → 0.
	if err := db.QueryRow("SELECT haversine_km(40.7,-74.0,40.7,-74.0)").Scan(&km); err != nil {
		t.Fatal(err)
	}
	if km != 0 {
		t.Errorf("identical points = %.4f, want 0", km)
	}

	// NULL argument → NULL result (so positionless rows fail proximity predicates).
	var n sql.NullFloat64
	if err := db.QueryRow("SELECT haversine_km(NULL,0,0,1)").Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n.Valid {
		t.Errorf("NULL arg should yield NULL, got %v", n.Float64)
	}
}
