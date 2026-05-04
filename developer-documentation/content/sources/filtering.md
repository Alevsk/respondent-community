---
title: "Filtering"
description: "Filter records using CEL expressions before they become entities"
weight: 4
---

The `filter` field is an optional CEL expression that controls which parsed records are processed into entities and observations. Only records where the filter evaluates to `true` are kept -- everything else is discarded before entity mapping runs.

```yaml
filter: >
  has(record.lat) && has(record.lon)
```

The `record` variable is bound to each parsed record (a map). If no filter is set, all parsed records are processed.

## Field existence

Check that required fields exist before accessing them. This prevents runtime errors from missing data.

```yaml
filter: >
  has(record.lat) && has(record.lon)
```

{{< cel-example expression="has(record.lat) && has(record.lon)" input="record = {\"lat\": 39.7, \"lon\": -104.9}" output="true" >}}
Both fields exist, so the record passes the filter.
{{< /cel-example >}}

{{< cel-example expression="has(record.lat) && has(record.lon)" input="record = {\"lat\": 39.7}" output="false" >}}
The `lon` field is missing, so the record is discarded.
{{< /cel-example >}}

## Numeric comparisons

Filter records by numeric thresholds. Use `double()` to convert string values to numbers.

```yaml
filter: >
  double(record.magnitude) >= 2.0
```

{{< cel-example expression="double(record.magnitude) >= 2.0" input="record.magnitude = \"4.5\"" output="true" >}}
Magnitude 4.5 passes the threshold of 2.0.
{{< /cel-example >}}

{{< cel-example expression="double(record.magnitude) >= 2.0" input="record.magnitude = \"0.8\"" output="false" >}}
Magnitude 0.8 is below the threshold and is discarded.
{{< /cel-example >}}

## String matching

Filter by exact string values or exclude specific strings.

```yaml
filter: >
  record.status == "active"
```

```yaml
filter: >
  record.type != "test"
```

{{< cel-example expression="record.status == \"active\"" input="record.status = \"active\"" output="true" >}}
Exact match -- only records with status "active" are kept.
{{< /cel-example >}}

## Excluding invalid data

Some APIs use sentinel values (like `"MM"`) for missing data instead of omitting the field. Filter these out explicitly.

```yaml
filter: >
  record.LAT != "MM" && record.LON != "MM"
```

{{< cel-example expression="record.LAT != \"MM\" && record.LON != \"MM\"" input="record.LAT = \"39.7\", record.LON = \"-104.9\"" output="true" >}}
Valid coordinates pass through.
{{< /cel-example >}}

{{< cel-example expression="record.LAT != \"MM\" && record.LON != \"MM\"" input="record.LAT = \"MM\", record.LON = \"MM\"" output="false" >}}
Sentinel values are excluded.
{{< /cel-example >}}

## Null checks

Some APIs include the field but set its value to `null`. Use a null check in addition to `has()`.

```yaml
filter: >
  has(record.lat) && record.lat != null
```

## Compound filters

Combine multiple conditions to precisely control which records are processed.

```yaml
filter: >
  has(record.lat) && has(record.lon) &&
  record.LAT != "MM" && record.LON != "MM" &&
  has(record.mag) && double(record.mag) >= 2.0
```

This filter keeps only records that:
1. Have both coordinate fields present
2. Do not contain sentinel values for coordinates
3. Have a magnitude field with a value of 2.0 or greater

## TLE / SGP4 filters

For satellite TLE data, use SGP4 propagation functions to filter out entries with invalid or decayed orbits.

```yaml
filter: >
  has(record.line1) && has(record.line2) &&
  sgp4_lat(record.line1, record.line2) != 0.0
```

{{< cel-example expression="sgp4_lat(record.line1, record.line2) != 0.0" input="record.line1 = \"1 25544U ...\", record.line2 = \"2 25544 ...\"" output="true" >}}
SGP4 propagation succeeded and returned a non-zero latitude -- the TLE is valid.
{{< /cel-example >}}

## Available CEL functions

The following built-in functions are available in filter expressions:

| Function | Returns | Description |
|----------|---------|-------------|
| `has(field)` | bool | Check if a field exists in the record |
| `size(list)` | int | Return the length of a list |
| `string(val)` | string | Convert a value to string |
| `int(val)` | int | Convert a value to integer |
| `double(val)` | double | Convert a value to double |
| `coerce_double(val, default)` | double | Safe double conversion with fallback |
| `sgp4_lat(line1, line2)` | double | SGP4-propagated latitude |
| `sgp4_lon(line1, line2)` | double | SGP4-propagated longitude |
| `sgp4_alt_m(line1, line2)` | double | SGP4-propagated altitude (meters) |
| `sgp4_vel_mps(line1, line2)` | double | SGP4-propagated velocity (m/s) |
| `lookup("table", key, "field")` | string | Query a lookup table |

{{< callout type="warning" title="Always guard optional fields" >}}
Without `has()`, accessing a missing field causes a runtime error that silently drops the record. Always wrap optional field access with `has()` guards, especially in compound filters where earlier conditions might not prevent evaluation of later ones.
{{< /callout >}}
