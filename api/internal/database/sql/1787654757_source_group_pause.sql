-- +migrate Up
-- See docs/pause-source-group/design.md §1.
ALTER TABLE sources ADD COLUMN paused BOOLEAN NOT NULL DEFAULT false;

-- +migrate Up
ALTER TABLE groups ADD COLUMN paused BOOLEAN NOT NULL DEFAULT false;

-- +migrate Down
ALTER TABLE sources DROP COLUMN paused;
ALTER TABLE groups DROP COLUMN paused;
