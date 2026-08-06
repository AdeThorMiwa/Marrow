-- +migrate Up
-- Tracked separately from consecutive_failures — a reachable source that
-- returns zero new items N times in a row is a different failure mode
-- ("stale": nothing new to say) from one that can't be reached at all
-- ("broken": N consecutive unreachable attempts). Each gets its own
-- exponential backoff on next_poll_at.
ALTER TABLE sources ADD COLUMN consecutive_empty_polls INT NOT NULL DEFAULT 0;

-- +migrate Up
-- The stale-path backoff cap is per-source, not global — a weekly YouTube
-- channel going quiet for a week is normal, not stale; a daily feed going
-- quiet for a week is. Resolved once at add-source time (see each
-- adapter's Resolve — typically the gap between the two most recent
-- items), defaulting to 7 days when there isn't enough feed history to
-- tell. Broken (unreachable) still uses the fixed global cap — network
-- failures don't get more patient just because a source posts rarely.
ALTER TABLE sources ADD COLUMN stale_after_seconds INT NOT NULL DEFAULT 604800;

-- +migrate Down
ALTER TABLE sources DROP COLUMN stale_after_seconds;
ALTER TABLE sources DROP COLUMN consecutive_empty_polls;
