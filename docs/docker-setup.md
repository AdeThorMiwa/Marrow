# Running Marrow with Docker

Brings up the whole backend stack — Postgres, Ollama, and the Go API
(with twscrape/instaloader/yt-dlp bundled in) — with one command.

Compose files live in `api/` (`api/docker-compose.yml`,
`api/docker-compose.override.yml`, `api/docker-compose.whisper.yml`) —
`scripts/bootstrap.sh` and `scripts/stop.sh` `cd` there for you, so you
never need to reference those paths directly unless running `docker
compose` by hand.

## Prerequisites

- Docker Desktop
- `api/.env` populated with `APP_TWITTER_USERNAME`, `APP_TWITTER_COOKIES`,
  `APP_INSTAGRAM_USERNAME`, `APP_INSTAGRAM_COOKIES`
- Optional but recommended: a native `whisper-server` build
  ([whisper.cpp](https://github.com/ggml-org/whisper.cpp)) with a model at
  `~/whisper-models/ggml-medium.bin`. If present, Marrow uses it directly
  from the host (Metal-accelerated, fast). If absent, it falls back to a
  Dockerized CPU-only build automatically — slower, but zero setup.

## Quick start

```bash
./scripts/bootstrap.sh
```

This checks your `api/.env`, detects whether a native `whisper-server` is
running, brings up `docker compose` with the right profile, waits for the
API to report healthy, pulls the embedding model into Ollama, and runs a
few smoke checks. Re-running it is safe — everything it does is
idempotent.

Once it's done, the API is at **http://localhost:8091**.

```bash
./scripts/stop.sh            # stop everything, keep data
./scripts/stop.sh --wipe     # also delete postgres/ollama/twscrape volumes
```

`stop.sh` tears down the full compose stack (including the Dockerized
whisper fallback, if it was active) and kills a native `whisper-server`
process if one is running.

## What's running

| Service | Image | Purpose |
|---|---|---|
| `api` | built from `api/Dockerfile` | Go backend — ingest, enrichment, feed |
| `postgres` | `pgvector/pgvector:pg16` | primary datastore (pgvector required — enrichment embeddings) |
| `ollama` | `ollama/ollama` | embeddings (`nomic-embed-text`) |
| `whisper` | built from `whisper/Dockerfile` | *only if no native `whisper-server` was found* — audio transcription fallback |

Data persists across restarts in named Docker volumes
(`postgres_data`, `ollama_data`, `twscrape_data`) — `./scripts/stop.sh` is
safe; `./scripts/stop.sh --wipe` deletes them.

## Hot reload for local development

`docker compose up` (no extra flags) picks up `docker-compose.override.yml`
automatically, which builds the `api` image's `dev` target and bind-mounts
the whole `api/` directory into the container. Edit any Go file on the
host and [`air`](https://github.com/air-verse/air) rebuilds and restarts
the server inside the container — same as running `air` natively.
`air` is configured to poll for changes (`.air.toml`) rather than rely on
filesystem events, since Docker Desktop's macOS bind-mount doesn't
reliably forward those into the container.

To run the real production image instead (no bind mount, static binary):

```bash
cd api && docker compose up
```

## Manually using the Dockerized whisper fallback

`./scripts/bootstrap.sh` does this automatically when no native
`whisper-server` is found. To force it by hand:

```bash
cd api
WHISPER_MODEL_DIR=~/whisper-models docker compose \
  -f docker-compose.yml -f docker-compose.override.yml \
  -f docker-compose.whisper.yml up -d --build
```

## Reaching it remotely

The stack publishes the API on `localhost:8091`, matching the existing
Cloudflare tunnel's ingress config
(`~/.cloudflared/config.yml` → `http://localhost:8091`). Keep
`cloudflared` running on the host as usual — nothing about the tunnel
setup changes; `https://marrow.adethormiwa.me` continues to work once the
container is up.

## Useful commands

Run from `api/` (or prefix each with `-f api/docker-compose.yml -f
api/docker-compose.override.yml` from the repo root):

```bash
docker compose ps                 # status + health of every service
docker compose logs -f api        # follow api logs
docker compose exec api sh        # shell into the running container
```
