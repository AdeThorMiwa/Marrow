#!/bin/sh
# First-run (and every-run) startup for the Dockerized Marrow stack.
# Safe to re-run: everything it does is idempotent.
#
#   ./scripts/bootstrap.sh
#
set -e

cd "$(dirname "$0")/../api"

if [ ! -f .env ]; then
  echo "error: api/.env is missing." >&2
  echo "  Create it with APP_TWITTER_USERNAME / APP_TWITTER_COOKIES /" >&2
  echo "  APP_INSTAGRAM_USERNAME / APP_INSTAGRAM_COOKIES before continuing." >&2
  exit 1
fi

COMPOSE_FILES="-f docker-compose.yml -f docker-compose.override.yml"

if command -v whisper-server >/dev/null 2>&1; then
  echo "==> native whisper-server found on PATH"
  if curl -sf http://localhost:8090/ >/dev/null 2>&1; then
    echo "    and it's already running — good."
  else
    echo "    WARNING: it's not currently running. Start it before" >&2
    echo "    continuing, e.g.:" >&2
    echo "      whisper-server -m ~/whisper-models/ggml-medium.bin --port 8090 --convert" >&2
    read -p "    Press enter once it's running (or Ctrl-C to abort)... " _
  fi
else
  echo "==> no native whisper-server found — using the Dockerized fallback"
  echo "    (CPU-only, slower than a native Metal-accelerated build)"
  WHISPER_MODEL_DIR="${WHISPER_MODEL_DIR:-$HOME/whisper-models}"
  if [ ! -d "$WHISPER_MODEL_DIR" ]; then
    echo "error: whisper model directory not found at $WHISPER_MODEL_DIR" >&2
    echo "  Set WHISPER_MODEL_DIR to the directory containing ggml-medium.bin" >&2
    exit 1
  fi
  export WHISPER_MODEL_DIR
  COMPOSE_FILES="$COMPOSE_FILES -f docker-compose.whisper.yml"
fi

echo "==> docker compose up -d"
# shellcheck disable=SC2086
docker compose $COMPOSE_FILES up -d --build

echo "==> waiting for api to report healthy"
tries=0
until [ "$(docker compose ps -q api | xargs docker inspect -f '{{.State.Health.Status}}' 2>/dev/null)" = "healthy" ]; do
  tries=$((tries + 1))
  if [ "$tries" -gt 60 ]; then
    echo "error: api didn't become healthy in time — check: docker compose logs api" >&2
    exit 1
  fi
  sleep 2
done
echo "    api is healthy."

echo "==> pulling the embedding model into ollama (no-ops if already present)"
docker compose exec -T ollama ollama pull nomic-embed-text

echo "==> smoke checks"
if curl -sf http://localhost:8091/feed >/dev/null; then
  echo "    [ok] api /feed responds"
else
  echo "    [FAIL] api /feed did not respond" >&2
fi

if docker compose exec -T ollama ollama list | grep -q nomic-embed-text; then
  echo "    [ok] ollama has the embedding model"
else
  echo "    [FAIL] ollama is missing nomic-embed-text" >&2
fi

echo ""
echo "Marrow is up: http://localhost:8091"
