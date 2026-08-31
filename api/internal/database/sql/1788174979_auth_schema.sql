-- +migrate Up
CREATE TABLE IF NOT EXISTS users (
    id TEXT PRIMARY KEY,
    email TEXT NOT NULL,
    display_name TEXT NOT NULL,
    password_hash TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- +migrate Up
CREATE UNIQUE INDEX IF NOT EXISTS idx_users_email ON users (email);

-- +migrate Up
CREATE INDEX IF NOT EXISTS idx_users_email_lower ON users (lower(email));

-- +migrate Up
CREATE TABLE IF NOT EXISTS refresh_tokens (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    token_hash TEXT NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    revoked_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- +migrate Up
CREATE UNIQUE INDEX IF NOT EXISTS idx_refresh_tokens_hash ON refresh_tokens (token_hash);

-- +migrate Up
CREATE INDEX IF NOT EXISTS idx_refresh_tokens_user_id ON refresh_tokens (user_id);

-- +migrate Up
CREATE TABLE IF NOT EXISTS user_sources (
    user_id TEXT NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    source_id TEXT NOT NULL REFERENCES sources (id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, source_id)
);

-- +migrate Up
CREATE INDEX IF NOT EXISTS idx_user_sources_source_id ON user_sources (source_id);

-- +migrate Up
CREATE TABLE IF NOT EXISTS user_groups (
    user_id TEXT NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    group_id TEXT NOT NULL REFERENCES groups (id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, group_id)
);

-- +migrate Up
CREATE INDEX IF NOT EXISTS idx_user_groups_group_id ON user_groups (group_id);

-- +migrate Down
DROP TABLE IF EXISTS user_groups;

DROP TABLE IF EXISTS user_sources;

DROP TABLE IF EXISTS refresh_tokens;

DROP TABLE IF EXISTS users;