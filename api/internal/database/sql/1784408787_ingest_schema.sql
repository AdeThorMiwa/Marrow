-- +migrate Up
CREATE TABLE IF NOT EXISTS sources
(
    id                   TEXT PRIMARY KEY,
    adapter_id           TEXT        NOT NULL,
    identifier           TEXT        NOT NULL,
    name                 TEXT        NOT NULL,
    last_fetched_at      TIMESTAMPTZ,
    next_poll_at         TIMESTAMPTZ NOT NULL,
    health               TEXT        NOT NULL DEFAULT 'ok',
    consecutive_failures INT         NOT NULL DEFAULT 0,
    created_at           TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- +migrate Up
CREATE INDEX IF NOT EXISTS idx_sources_next_poll_at ON sources (next_poll_at);

-- +migrate Up
-- Content carries no kind/body/media_ref of its own — those live on its
-- ContentBlocks (see content_blocks below). A Content is a title, metadata,
-- and an ordered sequence of blocks.
CREATE TABLE IF NOT EXISTS contents
(
    id           TEXT PRIMARY KEY,
    source_id    TEXT        NOT NULL REFERENCES sources (id),
    url          TEXT        NOT NULL UNIQUE,
    title        TEXT        NOT NULL,
    description  TEXT,
    published_at TIMESTAMPTZ NOT NULL,
    metadata     JSONB       NOT NULL DEFAULT '{}',
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- +migrate Up
CREATE TABLE IF NOT EXISTS content_blocks
(
    id            TEXT PRIMARY KEY,
    content_id    TEXT NOT NULL REFERENCES contents (id),
    position      INT  NOT NULL,
    kind          TEXT NOT NULL,
    markdown      TEXT,
    media_ref     TEXT,
    caption       TEXT,
    thumbnail_url TEXT,
    UNIQUE (content_id, position)
);

-- +migrate Up
CREATE TABLE IF NOT EXISTS authors
(
    id   TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    url  TEXT
);

-- +migrate Up
CREATE UNIQUE INDEX IF NOT EXISTS idx_authors_url ON authors (url) WHERE url IS NOT NULL;

-- +migrate Up
CREATE INDEX IF NOT EXISTS idx_authors_name ON authors (name);

-- +migrate Up
CREATE TABLE IF NOT EXISTS content_authors
(
    content_id TEXT NOT NULL REFERENCES contents (id),
    author_id  TEXT NOT NULL REFERENCES authors (id),
    role       TEXT,
    PRIMARY KEY (content_id, author_id)
);

-- +migrate Down
DROP TABLE IF EXISTS content_authors;
DROP TABLE IF EXISTS authors;
DROP TABLE IF EXISTS content_blocks;
DROP TABLE IF EXISTS contents;
DROP TABLE IF EXISTS sources;
