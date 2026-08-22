#!/bin/sh
# Stops everything scripts/bootstrap.sh brings up: the Docker stack (api,
# postgres, ollama, and whisper if the Dockerized fallback was active) plus
# any native whisper-server process on the host.
#
#   ./scripts/stop.sh            # stop everything, keep data
#   ./scripts/stop.sh --wipe     # also delete postgres/ollama/twscrape volumes
#
set -e

cd "$(dirname "$0")/../api"

WIPE=false
case "$1" in
  --wipe|--wipe-volumes|-v)
    WIPE=true
    ;;
  "") ;;
  *)
    echo "usage: $0 [--wipe]" >&2
    exit 1
    ;;
esac

COMPOSE_FILES="-f docker-compose.yml -f docker-compose.override.yml -f docker-compose.whisper.yml"

echo "==> stopping docker compose stack"
if [ "$WIPE" = true ]; then
  # shellcheck disable=SC2086
  docker compose $COMPOSE_FILES down -v
  echo "    volumes wiped (postgres, ollama, twscrape)"
else
  # shellcheck disable=SC2086
  docker compose $COMPOSE_FILES down
fi

echo "==> stopping native whisper-server (if running)"
if pkill -f "whisper-server -m" 2>/dev/null; then
  echo "    stopped."
else
  echo "    not running."
fi

echo ""
echo "Marrow is stopped."
