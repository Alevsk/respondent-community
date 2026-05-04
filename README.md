# Respondent

> Real-time geospatial intelligence on a 3D globe.

Respondent ingests live data from 50+ public sources — flights, ships, earthquakes, fires, lightning, satellites, conflict events, weather alerts, internet outages and more — and renders them on an interactive 3D globe. Optional AI-powered analyses fuse data across layers to surface composite signals (e.g. military activity near conflict zones, ships near submarine cables, fires near air-quality anomalies).

📚 **Full documentation: [respondent-docs.alevsk.dev](https://respondent-docs.alevsk.dev)** — schema reference, source / analysis authoring guides, and configuration details.

This repository distributes the Community Edition: a single Docker container, a SQLite database, and a directory of YAML source / analysis definitions you can edit, extend, and contribute back.

<p align="center" width="100%">
  <video src="https://github.com/user-attachments/assets/22111890-6e4a-4d7b-9ca4-9d4cad79ccd5" width="80%" controls></video>
</p>

---

## Quick Start

You need [Docker](https://docs.docker.com/get-docker/) (v20.10+) and [Docker Compose](https://docs.docker.com/compose/install/) (v2.0+).

```bash
# 1. Clone this repo
git clone https://github.com/alevsk/respondent-community.git
cd respondent-community

# 2. (Optional) copy the env template and fill in API keys you have
cp .env.example .env
$EDITOR .env

# 3. Start Respondent
docker compose up -d

# 4. Open the globe
open http://localhost:8090   # macOS — or visit the URL in any browser
```

That's it. The container pulls `docker.io/alevsk/respondent-community:latest`, mounts the local `respondent.yaml`, `sources.d/`, and `analysis.d/` directories read-only, and persists the SQLite database in a Docker volume named `respondent_data`.

To watch ingestion happen:

```bash
docker compose logs -f
```

To stop:

```bash
docker compose down            # stop, keep data
docker compose down -v         # stop, drop the data volume (destructive)
```

### Alternative: `docker run`

Docker Compose is the **recommended** path. If you can't use it, the equivalent `docker run` command is:

```bash
docker volume create respondent_data

docker run -d \
  --name respondent-community \
  --restart unless-stopped \
  --env-file ./.env \
  -e RESPONDENT_DATABASE_PATH=/data/respondent.db \
  -p 8090:8090 \
  -v "$(pwd)/respondent.yaml:/etc/respondent/respondent.community.yaml:ro" \
  -v "$(pwd)/sources.d:/etc/respondent/sources.d:ro" \
  -v "$(pwd)/analysis.d:/etc/respondent/analysis.d:ro" \
  -v respondent_data:/data \
  docker.io/alevsk/respondent-community:latest
```

Then open the globe at **http://localhost:8090**.

---

## What's Included

- **Single-container deploy** — one image, one process, SQLite for storage. No Postgres, Redis, or Kafka required.
- **3D globe UI** — React + Cesium frontend, served from the same port as the API and WebSocket.
- **30+ public data sources** out of the box, including flights (ADS-B, OpenSky), ships (AIS), earthquakes (USGS, EMSC), fires (NASA FIRMS, NIFC), lightning (Blitzortung), satellites (CelesTrak, TLE API), volcanoes, weather alerts, air quality, conflict events, internet infrastructure, financial market indicators, the ISS, and more. See [`sources.d/README.md`](./sources.d/README.md) for the full catalog.
- **AI analyses** — declarative YAML pipelines that run on a schedule, query one or more layers, prompt an LLM with structured data, and store insights in the database. Examples include lightning-fire prediction, maritime cable threat detection, and a geopolitical hotspot index. See [`analysis.d/README.md`](./analysis.d/README.md) for the full catalog.
- **No code required** to add a source or analysis — everything is YAML + CEL expressions.

---

## Configuration

Two files control runtime behaviour:

| File | Purpose |
|---|---|
| `respondent.yaml` | Runtime config: server port, database path, AI toggle, LLM provider, geocoder. Mounted read-only into the container at `/etc/respondent/respondent.community.yaml`. |
| `.env` | Secrets and per-source API keys. Loaded automatically by Docker Compose if present. Gitignored. |

The defaults in `respondent.yaml` work out of the box — sources that need credentials silently no-op when their key is missing, so you can start with zero env vars and add keys as you go.

For the full schema reference (every field, every source transport type, every parser format), see the developer documentation — either browse it online at <https://respondent-docs.alevsk.dev> or run it locally:

```bash
docker compose -f developer-documentation/compose.yaml up -d
# Then open http://localhost:8080
```

The same content is also in [`developer-documentation/content/`](./developer-documentation/content/) as plain Markdown.

---

## API Keys (Optional)

Most sources work with no credentials. The following sources require a free API key or token to ingest data — without one, they will start, fail to authenticate, and stop until you provide a key. None of them are required for a working installation.

| Source | Where to get it | Env var(s) |
|---|---|---|
| NASA FIRMS active fires | <https://firms.modaps.eosdis.nasa.gov/api/area/> | `RESPONDENT_NASA_FIRMS_MAP_KEY` |
| OpenAQ air quality | <https://docs.openaq.org/> | `RESPONDENT_OPENAQ_API_KEY` |
| AISStream maritime AIS | <https://aisstream.io/> | `RESPONDENT_AISSTREAM_APY_KEY` |
| PurpleAir community air quality | <https://develop.purpleair.com/> | `RESPONDENT_PURPLEAIR_API_KEY` |
| ACLED armed conflict events | <https://acleddata.com/> | `RESPONDENT_ACLED_EMAIL`, `RESPONDENT_ACLED_PASSWORD` |
| APRS.fi amateur radio | <https://aprs.fi/> | `RESPONDENT_APRS_FI_API_KEY` |
| Cloudflare Radar internet outages | <https://radar.cloudflare.com/> | `RESPONDENT_CLOUDFLARE_RADAR_TOKEN` |
| Meshtastic LoRa mesh | <https://meshtastic.org/> (public credentials work) | `RESPONDENT_MESHTASTIC_USER`, `RESPONDENT_MESHTASTIC_PASS` |
| Ukraine air raid alerts | <https://alerts.in.ua/> | `RESPONDENT_UKRAINE_ALARM_TOKEN` |

Add the variables you have to `.env` (copy `.env.example` for the full annotated list, including LLM provider keys).

If you also want satellite imagery on the globe (Bing Maps Aerial via Cesium Ion), get a free token at <https://ion.cesium.com/signup> and set:

```bash
RESPONDENT_FRONTEND_CESIUM_ION_TOKEN=your-token-here
```

Without it, the globe falls back to Stadia Maps dark tiles.

---

## Enabling AI Analyses

AI is off by default. To enable it, edit `respondent.yaml`:

```yaml
ai:
  enabled: true

llm:
  provider: "openai"     # or anthropic, xai, gemini, zai, ollama, lmstudio
  openai:
    model: "gpt-4o"
    max_tokens: 2048
```

…and set the matching API key in `.env`:

```bash
RESPONDENT_LLM_OPENAI_API_KEY=sk-...
```

Then restart:

```bash
docker compose restart
```

Analyses defined in `analysis.d/` will be picked up automatically and start running on their configured schedules.

---

## Updating

```bash
docker compose pull
docker compose up -d
```

Your data volume (`respondent_data`) and config files are preserved.

---

## Documentation

- **End-user / operator docs**: this README and the [Getting Started](./developer-documentation/content/getting-started/) guides.
- **Schema reference** (every field of every YAML): [`developer-documentation/`](./developer-documentation/) — Hugo static site, also published at <https://respondent-docs.alevsk.dev>.
- **Source catalog**: [`sources.d/README.md`](./sources.d/README.md)
- **Analysis catalog**: [`analysis.d/README.md`](./analysis.d/README.md)
- **Templates**: [`sources.d/TEMPLATE.yaml`](./sources.d/TEMPLATE.yaml), [`analysis.d/TEMPLATE.yaml`](./analysis.d/TEMPLATE.yaml)

---

## Contributing

Pull requests for new sources, new analyses, and documentation improvements are welcome. See [DEVELOPMENT.md](./DEVELOPMENT.md) for the full contributor guide — local setup, the YAML schemas, how to test, and PR conventions.

---

## License

TBD. A `LICENSE` file will be added before the first tagged release. Until then, treat this repository as "all rights reserved" by the original authors; you may run it locally and submit contributions, but redistribution rights are not yet granted.

---

## Getting Help

- File an issue: <https://github.com/alevsk/respondent-community/issues>
- Documentation: <https://respondent-docs.alevsk.dev>
