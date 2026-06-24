---
title: "Installation"
description: "Get the Respondent community edition running in under 5 minutes"
weight: 1
---

## Prerequisites

- [Docker](https://docs.docker.com/get-docker/) (v20.10+)
- [Docker Compose](https://docs.docker.com/compose/install/) (v2.0+)

Verify both are installed:

```bash
docker --version
docker compose version
```

## Create the project directory

Create a directory for your Respondent deployment with the required file structure:

```bash
mkdir -p respondent/{sources.d,analysis.d}
cd respondent
```

You will create two files: `compose.yaml` (tells Docker how to run Respondent) and `respondent.yaml` (configures the application itself).

## Create `compose.yaml`

Create `compose.yaml` in the project root:

```yaml
services:
  respondent:
    image: "docker.io/alevsk/respondent-community:latest"
    command: ["serve", "--config", "/etc/respondent/respondent.community.yaml"]
    restart: unless-stopped
    env_file:
      - path: .env
        required: false
    ports:
      - "${CADDY_HTTP_PORT:-8090}:8090"
    volumes:
      - ./respondent.yaml:/etc/respondent/respondent.community.yaml:ro
      - ./sources.d:/etc/respondent/sources.d:ro
      - ./analysis.d:/etc/respondent/analysis.d:ro
      - respondent_data:/data
    environment:
      - RESPONDENT_DATABASE_PATH=/data/respondent.db

volumes:
  respondent_data:
```

{{< callout type="info" title="What this does" >}}
- Pulls the community edition image from the Respondent registry
- Mounts your local config files as read-only volumes
- Persists the SQLite database in a Docker volume (`respondent_data`) so data survives container restarts
- Exposes the UI, API, and WebSocket on port **8090** (override with `CADDY_HTTP_PORT` env var)
{{< /callout >}}

## Create `respondent.yaml`

Create `respondent.yaml` in the project root with a minimal configuration:

```yaml
database:
  path: "./respondent.db"

server:
  port: 8090

ingest:
  sources_dir: "./sources.d"

ai:
  enabled: false

logging:
  level: "info"
```

This is the smallest valid configuration. It uses SQLite for storage, serves the UI on port 8090, and disables AI features. You will expand this file in the [Configuration]({{< relref "configuration" >}}) guide.

## Start Respondent

Docker Compose is the **recommended** way to run Respondent — it stitches together the volumes, env file, named data volume, and port mapping for you, and survives reboots via `restart: unless-stopped`.

```bash
docker compose up -d
```

### Alternative: `docker run`

If you cannot use Docker Compose, the same deployment can be launched directly with `docker run`. Pass the volumes, env file, and port mapping explicitly:

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
  docker.io/alevsk/respondent-community:latest \
  serve --config /etc/respondent/respondent.community.yaml
```

The container exposes the UI, API, and WebSocket on **port 8090** — open `http://localhost:8090` in your browser once the container is healthy.

## Verify the installation

Check that the container is running and healthy:

```bash
docker compose logs -f
```

Look for a log line containing **"server listening"** -- this confirms Respondent has started successfully. Press `Ctrl+C` to stop following logs.

Open your browser and navigate to:

```
http://localhost:8090
```

You should see the Respondent globe interface. The globe will be empty since you have not configured any data sources yet.

{{< callout type="tip" title="Tip" >}}
If port 8090 is already in use, set a different port before starting:

```bash
CADDY_HTTP_PORT=9090 docker compose up -d
```

Then access the UI at `http://localhost:9090`.
{{< /callout >}}

## Stopping and restarting

```bash
# Stop the container
docker compose down

# Start again (data is preserved in the Docker volume)
docker compose up -d

# Restart after a config change
docker compose restart
```

## Next steps

- [Configure database, AI, and LLM providers]({{< relref "configuration" >}})
- [Create your first data source]({{< relref "your-first-source" >}})
