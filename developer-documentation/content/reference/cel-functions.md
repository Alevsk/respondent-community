---
title: "CEL Functions"
description: "Complete reference for all CEL functions available in source definitions"
weight: 1
---

Respondent uses the [Common Expression Language (CEL)](https://github.com/google/cel-go) to map API response fields to domain entities and observations. Every CEL expression receives a `record` variable -- a map representing one parsed record from the API response.

All expressions are compiled at config load time and evaluated at runtime. Expressions are limited to 4096 bytes and 10,000 cost units to prevent abuse.

## Time Functions

Functions for parsing timestamps from various API formats. All return a CEL `timestamp`.

{{< cel-example expression="now()" input="" output="2026-04-29T12:00:00Z" >}}
Returns the current time. Use as a fallback only -- prefer parsing the record's own timestamp.
{{< /cel-example >}}

{{< cel-example expression="unix_ms(record.time)" input="record.time = 1714392000000" output="2024-04-29T12:00:00Z" >}}
Converts epoch milliseconds to a timestamp. Accepts `int64` or `float64` input.
{{< /cel-example >}}

{{< cel-example expression="unix_s(record.timestamp)" input="record.timestamp = 1714392000" output="2024-04-29T12:00:00Z" >}}
Converts epoch seconds to a timestamp. Accepts `int64` or `float64` input.
{{< /cel-example >}}

{{< cel-example expression="parse_rfc3339(record.date)" input="record.date = \"2024-01-01T00:00:00Z\"" output="2024-01-01T00:00:00Z" >}}
Parses an RFC 3339 formatted string. The string must include a timezone suffix (e.g., `Z` or `+05:00`).
{{< /cel-example >}}

{{< cel-example expression="parse_iso8601(record.date)" input="record.date = \"2024-01-01T00:00:00\"" output="2024-01-01T00:00:00Z" >}}
Parses an ISO 8601 string, automatically appending `Z` (UTC) if no timezone info is present. Useful for APIs like GDACS that omit timezone suffixes.
{{< /cel-example >}}

{{< cel-example expression="parse_rfc2822(record.pubDate)" input="record.pubDate = \"Mon, 02 Jan 2006 15:04:05 -0700\"" output="2006-01-02T22:04:05Z" >}}
Parses RFC 2822 date strings commonly found in RSS feeds. Tries RFC 1123Z (numeric offset) first, then RFC 1123 (named timezone).
{{< /cel-example >}}

{{< cel-example expression="parse_datetime(record.date, \"2006-01-02 15:04:05\")" input="record.date = \"2024-04-29 12:30:00\"" output="2024-04-29T12:30:00Z" >}}
Parses a date string using a Go time layout. The layout uses Go's reference time (`2006-01-02T15:04:05`) as a format specification.
{{< /cel-example >}}

## Type Coercion

{{< cel-example expression="coerce_double(record.altitude, 0.0)" input="record.altitude = \"ground\"" output="0.0" >}}
Safely converts mixed-type fields to `double`. Handles `float64`, `int64`, `string`, and `nil` inputs. Returns the default value when conversion fails. Useful for APIs that return non-numeric strings for certain states (e.g., `"ground"` for altitude of grounded aircraft).
{{< /cel-example >}}

## Lookup Table Functions

Lookup tables are static key-value datasets defined inline or loaded from files. They are indexed at config time and queried in CEL expressions at runtime.

{{< callout type="info" title="Availability" >}}
Lookup functions are only available when the source defines a `lookup_tables` section. They are injected into an extended CEL environment at load time.
{{< /callout >}}

{{< cel-example expression="lookup(\"airlines\", record.iata, \"name\")" input="record.iata = \"AA\"" output="\"American Airlines\"" >}}
Looks up a value from a named table by key and field. Returns an error if the table, key, or field is not found.
{{< /cel-example >}}

{{< cel-example expression="has_lookup(\"airlines\", record.iata)" input="record.iata = \"XX\"" output="false" >}}
Checks if a key exists in a named lookup table. Returns `false` if the table does not exist or the key is not found.
{{< /cel-example >}}

{{< cel-example expression="lookup_or(\"airlines\", record.iata, \"name\", \"Unknown\")" input="record.iata = \"XX\"" output="\"Unknown\"" >}}
Looks up a value with a default fallback. Returns the default if the table, key, or field is not found -- never errors.
{{< /cel-example >}}

## Orbital (TLE/SGP4)

Functions for computing real-time satellite positions from Two-Line Element (TLE) sets using the SGP4 propagation model. Results are cached per `(line1, line2, second)` tuple, so calling all four functions for the same satellite incurs only a single propagation.

All SGP4 functions return `0.0` on error (malformed TLE, decayed orbit, etc.) rather than raising a runtime error.

{{< cel-example expression="sgp4_lat(record.line1, record.line2)" input="TLE lines for ISS" output="34.567" >}}
Propagates a TLE to the current time and returns latitude in degrees.
{{< /cel-example >}}

{{< cel-example expression="sgp4_lon(record.line1, record.line2)" input="TLE lines for ISS" output="-118.234" >}}
Propagates a TLE to the current time and returns longitude in degrees.
{{< /cel-example >}}

{{< cel-example expression="sgp4_alt_m(record.line1, record.line2)" input="TLE lines for ISS" output="408000.0" >}}
Propagates a TLE to the current time and returns altitude in meters.
{{< /cel-example >}}

{{< cel-example expression="sgp4_vel_mps(record.line1, record.line2)" input="TLE lines for ISS" output="7660.0" >}}
Propagates a TLE to the current time and returns velocity in meters per second.
{{< /cel-example >}}

## Standard Built-ins

These are built-in CEL functions available in every expression.

{{< cel-example expression="has(record.lat)" input="record = {\"lat\": 40.7}" output="true" >}}
Tests whether a field exists on the record. Always use `has()` to guard optional fields -- accessing a missing field without it causes a runtime error that silently skips the record.
{{< /cel-example >}}

{{< cel-example expression="size(record.items)" input="record.items = [1, 2, 3]" output="3" >}}
Returns the length of a list, map, or string.
{{< /cel-example >}}

{{< cel-example expression="string(record.id)" input="record.id = 42" output="\"42\"" >}}
Converts a value to string. Also available: `int(val)`, `double(val)` for numeric conversion, and `timestamp(val)` for timestamp conversion.
{{< /cel-example >}}

## String Extensions

Provided by the [cel-go string extension library](https://pkg.go.dev/github.com/google/cel-go/ext#Strings). Called as methods on string values.

| Function | Example | Result |
|----------|---------|--------|
| `split` | `"a,b,c".split(",")` | `["a", "b", "c"]` |
| `contains` | `"hello world".contains("world")` | `true` |
| `startsWith` | `"hello".startsWith("hel")` | `true` |
| `endsWith` | `"hello".endsWith("llo")` | `true` |
| `trim` | `" hello ".trim()` | `"hello"` |
| `lowerAscii` | `"HELLO".lowerAscii()` | `"hello"` |
| `upperAscii` | `"hello".upperAscii()` | `"HELLO"` |

## Math Extensions

Provided by the [cel-go math extension library](https://pkg.go.dev/github.com/google/cel-go/ext#Math). Functions are available in the `math` namespace.

| Function | Example | Result |
|----------|---------|--------|
| `math.abs` | `math.abs(-5)` | `5` |
| `math.ceil` | `math.ceil(2.3)` | `3` |
| `math.floor` | `math.floor(2.7)` | `2` |
| `math.round` | `math.round(2.5)` | `3` |
| `math.min` | `math.min(3, 7)` | `3` |
| `math.max` | `math.max(3, 7)` | `7` |
| `math.sqrt` | `math.sqrt(16.0)` | `4.0` |
| `math.pow` | `math.pow(2.0, 3.0)` | `8.0` |

{{< callout type="warning" title="Security" >}}
The CEL environment does not include Encoders (base64, etc.) to prevent covert data exfiltration. There is no access to environment variables, filesystem, or network from within CEL expressions.
{{< /callout >}}
