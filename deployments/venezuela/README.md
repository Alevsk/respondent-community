# Venezuela Earthquake Monitoring

## Layers (sources.d/)

| File | Layer | Source / endpoint | Notes |
|------|-------|-------------------|-------|
| `venezuela_damage_reports.yaml` | `venezuela_reports` | `/api/reports/geojson` | Crowd damage reports. The `source` field = platform (**YouTube / X / Instagram / Web**); markers are color-coded by platform. `damage_level` 1–5. |
| `venezuela_relief_centers.yaml` | `venezuela_relief_centers` | `/api/relief-centers` | **Acopios** — aid collection centers (what they accept, address, state). |
| `venezuela_building_damage.yaml` | `venezuela_building_damage` | `/api/building-damage` | **Buildings** — damaged/collapsed structures, color-coded by severity (`partial` / `severe` / `total`). |
| `venezuela_missing_persons.yaml` | `venezuela_missing_persons` | `/api/missing-persons` | Missing / located persons. **Coordinate-filtered** — upstream currently lacks coords, so it shows nothing until they geocode (no 0,0 spam). |
| `venezuela_aid_requests.yaml` | `venezuela_aid_requests` | `/api/needs` | **Solicitudes** — aid requests. **DISABLED**: the endpoint is currently empty and exposes no schema; the mapping is a documented best-effort to verify + enable when data appears. |
| `venezuela_earthquakes.yaml` | `venezuela_earthquakes` | USGS FDSN (Venezuela bbox) | **Added** — authoritative epicenters + magnitudes (M ≥ 2.5), so the map shows the actual seismicity, not just social reports. Includes an AI risk-assessment op. |

Translation reference: *acopios* → relief/collection centers, *solicitudes* →
aid requests, *daños* → damage, *parcial/severo/total* → partial/severe/total,
*sin-contacto* → no contact, *localizado* → located.

## Analyses (analysis.d/)

| File | What it does |
|------|--------------|
| `venezuela_relief_coverage_gap.yaml` | Flags severe/total collapses with **no relief center within ~10 km** (underserved areas) via `haversine_km`. |
| `venezuela_severe_damage_digest.yaml` | Ranks the most-corroborated severe/total collapses (last 72h) into a concise AI triage digest. |

Both require AI enabled (an LLM provider/key) to produce insights.

## Run locally

From the repo root:

```bash
# build the frontend into the binary once
make build
./bin/community serve --config deployments/venezuela/respondent.venezuela.yaml
# → http://localhost:8095   (API + WebSocket + Earth UI on one origin)
```

or without a build (dev):

```bash
go run ./cmd/community serve --config deployments/venezuela/respondent.venezuela.yaml
```

Enable the data layers from the Earth UI's **Data Layers** panel.

### AI (optional)

The analyses + earthquake risk op need an LLM. Set the key and keep
`ai.enabled: true`:

```bash
export RESPONDENT_LLM_ZAI_API_KEY=...   # never commit this
```

To run without AI, set `ai.enabled: false` in the config — the data layers still
load and render.

## Container / production notes

The image has no WORKDIR, so use **absolute** paths in the config when running in
a container (mirrors the haru deployment convention):

```yaml
database:   { path: /data/respondent-venezuela.db }
ingest:     { sources_dir: /etc/respondent/sources.d }
ai:         { analysis_dir: /etc/respondent/analysis-venezuela.d }
```

Mount `sources.d/` → `/etc/respondent/sources.d`, `analysis.d/` →
`/etc/respondent/analysis-venezuela.d`, and a writable `data/` → `/data`. Supply
`command: ["serve","--config","/etc/respondent/respondent.venezuela.yaml"]`.

## Source provenance — why we use the `/api/*` endpoints

Verified against the upstream repo
([nochinxx/venezuela-earthquake-map](https://github.com/nochinxx/venezuela-earthquake-map)):
the humanitarian/crowd data is **not directly accessible from its origin**. The
flow is `credentialed scrapers → private Supabase DB → Next.js /api/* routes`:

- The `/api/*` routes query Supabase with a **secret `SUPABASE_SERVICE_KEY`**
  (RLS-bypassing, server-only). There is **no public/anon key** and the browser
  never touches Supabase directly — so the DB itself is not unauthenticated.
- The true upstreams are scrapers requiring credentials we don't have and
  shouldn't replicate: X (`api.twitter.com` bearer token), YouTube
  (`youtube/v3` API key), Instagram (burner login), localizadosvenezuela.com.

So the unauthenticated `/api/*` endpoints are the **only** accessible interface
for reports / acopios / buildings / missing-persons / needs — they are the
curated aggregation layer, and that's what we use.

**Exception — earthquakes:** we bypass the site entirely and pull the
**authoritative primary source (USGS FDSN)** directly, which is more reliable
than the proxy.

## Data caveats

- Reports are **crowd-sourced** (social media scrapes) — treat `credibility` /
  `verified` accordingly; the USGS earthquake layer is the authoritative anchor.
- Missing-persons and aid-requests upstream data is currently sparse/empty; those
  layers are scaffolded to light up when the upstream populates them. A heavy
  geocoded missing-persons GeoJSON endpoint exists
  (`/api/missing-persons/external`, ~24 MB / ~50k jittered points) — documented
  in `venezuela_missing_persons.yaml` but not enabled (too heavy/imprecise).
