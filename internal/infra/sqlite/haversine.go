package sqlite

import (
	"database/sql/driver"
	"math"

	sqlite3 "modernc.org/sqlite"
)

// init registers custom scalar SQL functions on the modernc driver. Registration
// is process-global and applies to every connection opened afterwards, so doing
// it at package init (before any Open) makes the functions available everywhere.
func init() {
	// Deterministic: same inputs always yield the same output, which lets SQLite
	// cache/optimize calls within a query.
	if err := sqlite3.RegisterDeterministicScalarFunction("haversine_km", 4, haversineKm); err != nil {
		panic("sqlite: register haversine_km: " + err.Error())
	}
}

// haversineKm(lat1, lon1, lat2, lon2) returns the great-circle distance in
// kilometres (Earth radius 6371 km). It replaces PostGIS ST_Distance/ST_DWithin,
// which modernc.org/sqlite (pure Go, no loadable extensions) cannot provide.
// NULL or non-numeric arguments yield a NULL result, so observations without a
// position are naturally excluded by `haversine_km(...) <= radius` predicates.
func haversineKm(_ *sqlite3.FunctionContext, args []driver.Value) (driver.Value, error) {
	if len(args) != 4 {
		return nil, nil
	}
	lat1, ok1 := toFloat(args[0])
	lon1, ok2 := toFloat(args[1])
	lat2, ok3 := toFloat(args[2])
	lon2, ok4 := toFloat(args[3])
	if !ok1 || !ok2 || !ok3 || !ok4 {
		return nil, nil
	}
	const deg = math.Pi / 180
	dLat := (lat2 - lat1) * deg
	dLon := (lon2 - lon1) * deg
	a := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Cos(lat1*deg)*math.Cos(lat2*deg)*math.Sin(dLon/2)*math.Sin(dLon/2)
	return 6371.0 * 2 * math.Asin(math.Sqrt(a)), nil
}

// toFloat coerces a driver.Value (REAL or INTEGER from SQLite) to float64.
func toFloat(v driver.Value) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case int64:
		return float64(n), true
	default:
		return 0, false
	}
}
