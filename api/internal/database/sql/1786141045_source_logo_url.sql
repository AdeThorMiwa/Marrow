-- +migrate Up
-- A source's own avatar/profile-picture/publication-logo, when the adapter
-- could cheaply get one at Resolve time — always optional, the client
-- falls back to initials when empty rather than treating this as required.
ALTER TABLE sources ADD COLUMN logo_url TEXT NOT NULL DEFAULT '';

-- +migrate Down
ALTER TABLE sources DROP COLUMN logo_url;
