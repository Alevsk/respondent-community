# Declarative Source Definitions

YAML source definitions for the feeder's declarative ingestion mode. Each file defines a complete data pipeline — transport, parsing, CEL mapping, and display — without any Go code.

## Source Catalog

### Active Sources

| Source File | Layer | Data | Transport | Interval |
|---|---|---|---|---|
| `adsb_lol_flights.yaml` | `flights_commercial` | ADS-B Exchange commercial flights | HTTP poll (spatial hex_grid) | 30s |
| `adsb_military.yaml` | `flights_military` | ADS-B military aircraft | HTTP poll | 30s |
| `aisstream_ships.yaml` | `ships` | AIS global vessel positions | WebSocket streaming | 5s batch |
| `aviationweather_sigmets.yaml` | `aviation_weather` | Aviation hazardous weather SIGMETs/AIRMETs | HTTP poll | 300s |
| `bellingcat_ukraine.yaml` | `conflict_events` | Verified civilian harm incidents in Ukraine | HTTP poll (static CDN JSON) | 3600s |
| `blitzortung_lightning.yaml` | `lightning` | Real-time lightning strikes worldwide | WebSocket (LZW-compressed) | streaming |
| `cbp_border_wait.yaml` | `border_crossings` | US border crossing wait times | HTTP poll + lookup table | 300s |
| `celestrak_satellites.yaml` | `satellites` | CelesTrak NORAD satellite TLEs | HTTP poll (paginated) | 300s |
| `copernicus_ems.yaml` | `ems_activations` | EU emergency management activations | HTTP poll (GeoJSON) | 1800s |
| `emsc_earthquakes.yaml` | `earthquakes` | European seismology events | HTTP poll (GeoJSON) | 60s |
| `epa_radnet.yaml` | `radiation_us` | EPA RadNet US radiation air monitoring | HTTP poll + lookup table | 3600s |
| `gdacs_disasters.yaml` | `disaster_alerts` | Global disaster alerts | HTTP poll (GeoJSON) | 900s |
| `iss_position.yaml` | `iss` | International Space Station | HTTP poll | 10s |
| `nasa_firms_fires.yaml` | `fires_active` | NASA FIRMS active fire detections | HTTP poll (CSV) | 10800s |
| `nifc_wildfires.yaml` | `wildfires` | NIFC US wildfire perimeters | HTTP poll (GeoJSON) | 900s |
| `noaa_buoys.yaml` | `ocean_buoys` | NOAA NDBC live ocean buoy observations (wind, waves, temp, pressure) | HTTP poll (whitespace-delimited text) | 600s |
| `noaa_space_weather.yaml` | `space_weather` | NOAA space weather indicators (R/S/G scales) | HTTP poll (global indicator) | 60s |
| `yahoo_finance_markets.yaml` | `markets` | Financial market indicators (VIX, S&P 500, DXY, WTI, Gold, BTC, 10Y) | HTTP poll (global indicator) | 300s |
| `noaa_weather_alerts.yaml` | `weather_alerts` | NOAA weather alerts | HTTP poll (GeoJSON) | 60s |
| `open_sky_flights.yaml` | `flights_commercial` | OpenSky Network flights | HTTP poll (array_columns) | 60s |
| `openaq_air_quality.yaml` | `air_quality` | OpenAQ air quality stations worldwide | HTTP poll (paginated) | 900s |
| `peeringdb_facilities.yaml` | `internet_infrastructure` | Internet exchange & data center facilities | HTTP poll (paginated) | 86400s |
| `purpleair_air_quality.yaml` | `air_quality` | PurpleAir community PM2.5 sensors (30k+) | HTTP poll (array_columns) | 900s |
| `safecast_radiation.yaml` | `radiation` | Safecast radiation sensors | HTTP poll | 900s |
| `smithsonian_volcanoes.yaml` | `volcanoes` | Smithsonian GVP active volcanoes | HTTP poll | 3600s |
| `sondehub_radiosondes.yaml` | `radiosondes` | Weather balloon telemetry | HTTP poll (object_to_records) | 60s |
| `spacedevs_launches.yaml` | `rocket_launches` | Upcoming rocket launches worldwide | HTTP poll | 600s |
| `submarine_cable_landings.yaml` | `subsea_cables` | Global submarine cable landing points | HTTP poll (GeoJSON) | 86400s |
| `tle_api_satellites.yaml` | `satellites` | TLE API satellite catalog | HTTP poll (paginated) | 300s |
| `usgs_earthquakes.yaml` | `earthquakes` | USGS earthquake hazards | HTTP poll (GeoJSON) | 60s |

### Active Sources (Auth Required)

These sources are implemented but disabled by default — they require API keys or tokens:

| Source File | Layer | Data | Transport | Interval | Auth |
|---|---|---|---|---|---|
| `acled_conflicts.yaml` | `conflict_events` | ACLED armed conflict events worldwide | HTTP poll | 3600s | Free registration at acleddata.com |
| `aprs_fi_stations.yaml` | `radio_aprs` | APRS amateur radio station positions | HTTP poll | 300s | Free API key from aprs.fi |
| `cloudflare_radar_outages.yaml` | `internet_outages` | Cloudflare Radar internet outage annotations | HTTP poll + lookup table | 900s | Cloudflare API token |
| `meshtastic_nodes.yaml` | `meshtastic` | Meshtastic LoRa mesh node positions | MQTT streaming | 10s batch | Public (meshdev/large4cats) |
| `ukraine_air_raids.yaml` | `air_raid_alerts` | Ukraine air raid alerts by oblast | HTTP poll + lookup table | 60s | Free token from ukrainealarm.com |

### Disabled Sources

| Source File | Blocker | Action Needed |
|---|---|---|
| `reliefweb_disasters.yaml` | ReliefWeb API now requires approved appname (403) | Register an appname, update `appname` query param |
| `who_disease_outbreaks.yaml` | API returns no geographic coordinates | Needs geocoding of country names from free text |

### Skipped Sources (commented out, kept as documentation)

| Source File | Reason |
|---|---|
| `gdelt_events.yaml` | GEO 2.0 API returns 404 since March 2026 |

### Not Yet Implemented (Tier 3)

Sources that need engine enhancements:

| Source | Blocker |
|---|---|
| Starlink constellation | Starlink satellite positions via CelesTrak group URL (already supported via TLE parser) |
| Airport/seaport locations | File-based static source loader |
| ACARS messages | TCP transport + custom parser |

## Adding a New Source

1. Copy `TEMPLATE.yaml` or an existing source as a starting point.
2. Update all fields: `name`, `source_type`, `layer_type` must be unique.
3. Choose your entity type:
   - **Spatial (default):** omit `entity_type`. Requires `observation.latitude` and `observation.longitude`. Each record becomes a positioned point on the globe.
   - **Global indicator:** set `entity_type: global_indicator`. Omit `observation.latitude` and `observation.longitude`. Requires an `indicator` spec with at least one entry in `indicator.values`. The source renders as a HUD overlay panel instead of globe markers.
4. CEL expressions in `filter`, `entity`, and `observation` must be valid.
5. For auth, use `transport.auth` with `env_var` (resolved as `RESPONDENT_<VAR>`). URLs support `${VAR}` substitution with the same prefix.
6. Validate by enabling the source in `respondent.yaml`, restarting the container (`docker compose restart`), and watching the logs for ingestion activity:

   ```bash
   docker compose logs -f respondent
   ```

   Schema and CEL errors are surfaced at startup; runtime fetch/parse errors appear once the first poll fires.

## Reference

- `TEMPLATE.yaml` — annotated starter template covering every supported field
- CEL language: <https://github.com/google/cel-spec>
- JSON Schema: <https://json-schema.org/>
