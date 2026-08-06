import type { BlockSummary } from './types';

// Audio/video blocks store `media_ref` as a self-describing
// "resolver://actual-url" envelope (see api/internal/model/media_ref.go) —
// split on the FIRST "://" only, since the URL itself contains "://".
// Image blocks store the direct URL with no envelope (images don't need a
// server-side MediaResolver), so they pass through unchanged.
export function getPlayableUrl(block: BlockSummary): string | undefined {
  if (!block.media_ref) return undefined;
  if (block.kind === 'image') return block.media_ref;

  const idx = block.media_ref.indexOf('://');
  if (idx === -1) return undefined;
  return block.media_ref.slice(idx + 3);
}
