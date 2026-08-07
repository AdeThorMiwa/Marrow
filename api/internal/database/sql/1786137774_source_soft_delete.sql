-- +migrate Up
-- DELETE /sources/:id soft-deletes rather than removing the row: Content
-- keeps its original source_id pointing at the same row (so its real
-- name/adapter/identifier survive), the row just stops showing up in
-- listings and stops being polled by the scheduler. No FK/content changes
-- needed at all — that's the whole point of doing it this way instead of
-- reassigning content to a sentinel row or nulling source_id out.
ALTER TABLE sources ADD COLUMN deleted_at TIMESTAMPTZ;

-- +migrate Down
ALTER TABLE sources DROP COLUMN deleted_at;
