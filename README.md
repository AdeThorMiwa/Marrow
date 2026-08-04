# Marrow

A personal cross-platform app for consuming and retaining content that matters. Most content consumption leaves nothing behind — Marrow changes that ratio.

Marrow has two complementary modes:

- **Garden** — an ambient, chronological feed of content from sources you've chosen. No algorithm, no ranking, no discovery.
- **Rabbithole** — structured deep engagement on a piece of content or an ongoing topic of inquiry, wrapping consumption in comprehension checks, spaced repetition, and Socratic prompting.

The two are not separate apps. You move between them fluidly.

---

## Stack

| Layer | Technology |
|-------|-----------|
| API | Go + Gin |
| Database | PostgreSQL + pgvector |
| App | React Native (Expo) |
| Platforms | iOS, Android, Web |

---

## Repo structure

```
marrow/
├── api/          # Go backend
│   ├── cmd/marrow/     # CLI entrypoint (serve, migrate up/down)
│   ├── configs/        # YAML config files (base.yaml, dev.yaml, etc.)
│   └── internal/       # Config, database, adapters, queue, scheduler, services, handlers
├── app/          # Expo (React Native) application
└── docs/
    ├── PRD.md          # Full product requirements
    ├── DESIGN.md       # System design
    └── ingest/         # Ingest context spec (requirements, design, tasks)
```

---

## Getting started

### Prerequisites

- Go 1.26+
- Node.js + npm
- PostgreSQL with the pgvector extension

### API

```bash
cd api
marrow migrate up   # apply database migrations (once, or after schema changes)
air                 # run the dev server with live reload
```

Config is loaded from `configs/base.yaml` and merged with an environment-specific file (e.g. `configs/dev.yaml`). The environment is read from the `APP_ENV` environment variable and defaults to `dev`.

To override config values via environment variables, prefix with `APP_` and use underscores for nesting:

```bash
APP_SERVER_PORT=9000 go run ./cmd/marrow serve
```

### App

```bash
cd app
npm start        # Expo dev server — choose a platform from the menu
npm run ios
npm run android
npm run web
```

---

## Configuration

`configs/base.yaml` holds defaults:


Create `configs/dev.yaml` or `configs/prod.yaml` to override per environment. Values in the environment-specific file are merged on top of base.

---

## Core concepts

**Dive** — the bridge between casual browsing and a full Rabbithole. Activates the retention loop: highlights, flags, spot comprehension checks, and Socratic nudges become available. Everything captured mid-consumption is a first-class artifact.

**Retention loop** — runs at the end of every Dive. Five phases: gaps analysis, comprehension check, application prompt, card generation, Socratic exit nudge.

**Rabbithole** — an ongoing area of inquiry built across multiple Dives. Has a library (what you've processed), a frontier/graph (what to explore next), and a synthesis (what you're building — yours, not AI-generated).

**Spaced repetition** — cards are generated automatically from highlights and comprehension gaps. Scheduling is algorithm-driven from first review onward.

**Action items** — extracted during the application phase and exported immediately via webhook. Marrow does not track them after generation.

See [`docs/PRD.md`](docs/PRD.md) for the full product specification.
