#!/bin/sh
# Runs migrations then starts the server — one script instead of a
# separate init container, since migrations are embedded (//go:embed) and
# idempotent (sql-migrate tracks what's already applied).
set -e

if [ "$MARROW_MODE" = "dev" ]; then
  # No static binary in the dev image — air compiles/reruns the actual
  # binary itself, but migrations need to run once up front.
  go run ./cmd/marrow migrate up
  exec air
else
  marrow migrate up
  exec marrow serve
fi
