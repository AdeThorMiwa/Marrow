-- +migrate Up
CREATE EXTENSION IF NOT EXISTS vector;

-- +migrate Up
CREATE TABLE IF NOT EXISTS enriched_content
(
    content_id       TEXT PRIMARY KEY REFERENCES contents (id),
    text             TEXT      NOT NULL,
    embedding        VECTOR(768) NOT NULL,
    embedding_model  TEXT      NOT NULL,
    transcript_model TEXT,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- +migrate Up
CREATE INDEX IF NOT EXISTS idx_enriched_content_embedding
    ON enriched_content USING hnsw (embedding vector_cosine_ops);

-- +migrate Down
DROP TABLE IF EXISTS enriched_content;
DROP EXTENSION IF EXISTS vector;
