-- +migrate Up
-- See docs/source-groups/design.md §1.
CREATE TABLE IF NOT EXISTS groups
(
    id         TEXT PRIMARY KEY,
    name       TEXT        NOT NULL,
    icon       TEXT        NOT NULL,
    is_default BOOLEAN     NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- +migrate Up
CREATE UNIQUE INDEX IF NOT EXISTS idx_groups_single_default ON groups (is_default) WHERE is_default = true;

-- +migrate Up
CREATE TABLE IF NOT EXISTS source_groups
(
    source_id  TEXT        NOT NULL REFERENCES sources (id),
    group_id   TEXT        NOT NULL REFERENCES groups (id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (source_id, group_id)
);

-- +migrate Up
CREATE INDEX IF NOT EXISTS idx_source_groups_group_id ON source_groups (group_id);

-- +migrate Up
INSERT INTO groups (id, name, icon, is_default)
VALUES ('default', 'All Sources', 'folder', true)
ON CONFLICT (id) DO NOTHING;

-- +migrate Down
DROP TABLE IF EXISTS source_groups;
DROP TABLE IF EXISTS groups;
