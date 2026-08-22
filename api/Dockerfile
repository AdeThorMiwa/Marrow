# syntax=docker/dockerfile:1

# Builds only the static prod binary — dev never needs this stage, since
# `air` compiles the running binary itself inside the dev container.
FROM golang:1.26-bookworm AS builder
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /out/marrow ./cmd/marrow

# ---- prod: slim, no Go toolchain needed at runtime ----
FROM python:3.12-slim-bookworm AS prod
RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates curl \
    && rm -rf /var/lib/apt/lists/*
RUN pip install --no-cache-dir twscrape==0.19.2 instaloader==4.15.3 yt-dlp==2026.07.04
WORKDIR /app
COPY --from=builder /out/marrow /usr/local/bin/marrow
COPY configs/ ./configs/
COPY internal/adapter/impl/scripts/ ./internal/adapter/impl/scripts/
COPY docker-entrypoint.sh /usr/local/bin/docker-entrypoint.sh
RUN chmod +x /usr/local/bin/docker-entrypoint.sh
ENV MARROW_MODE=prod
EXPOSE 8081
ENTRYPOINT ["docker-entrypoint.sh"]

# ---- dev: needs the real Go toolchain since air rebuilds on every change ----
FROM golang:1.26-bookworm AS dev
RUN apt-get update && apt-get install -y --no-install-recommends \
      python3 python3-pip ca-certificates curl \
    && rm -rf /var/lib/apt/lists/*
# apt's python3 is PEP 668 externally-managed (same footgun as Homebrew's on
# the host) — --break-system-packages is safe here since this container has
# no other Python-based OS tooling to conflict with.
RUN pip install --no-cache-dir --break-system-packages \
      twscrape==0.19.2 instaloader==4.15.3 yt-dlp==2026.07.04
RUN go install github.com/air-verse/air@latest
WORKDIR /app
COPY docker-entrypoint.sh /usr/local/bin/docker-entrypoint.sh
RUN chmod +x /usr/local/bin/docker-entrypoint.sh
ENV MARROW_MODE=dev
EXPOSE 8081
ENTRYPOINT ["docker-entrypoint.sh"]
