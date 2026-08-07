-- +migrate Up
-- Discover's Reachable=false today gives no detail on WHY — this captures
-- the underlying error text (e.g. an expired Twitter/Instagram auth
-- cookie) so a broken source's health card can say something more useful
-- than "hasn't updated recently." Cleared on any successful poll — a
-- historical reason is meaningless once the source is reachable again.
ALTER TABLE sources ADD COLUMN failure_reason TEXT;

-- +migrate Down
ALTER TABLE sources DROP COLUMN failure_reason;
