import { resolveBaseUrl } from './api';
import type { BlockSummary } from './types';

// Resolvers whose stored ref isn't a directly-playable URL at all — it has
// to be re-resolved server-side, right before playback, via
// /media/playback-url (see api/internal/adapter/api/media.go's
// PlaybackURLResolver doc comment for why: Instagram's CDN URLs expire
// within hours, so the one captured at ingest time is often already stale
// by the time a user watches).
const PLAYBACK_URL_RESOLVERS = new Set(['instagram']);

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

  const resolver = block.media_ref.slice(0, idx);
  if (PLAYBACK_URL_RESOLVERS.has(resolver)) {
    return `${resolveBaseUrl()}/media/playback-url/${block.media_ref}`;
  }
  return block.media_ref.slice(idx + 3);
}

// A video block's media_ref envelope is "youtube://{videoID}" when it came
// from the YouTube adapter (see api/internal/adapter/impl/youtube.go) — not
// a raw file URL like RSS-media's, since YouTube doesn't hand those out.
// getPlayableUrl's generic unwrap would still "work" (it'd return the video
// ID as if it were a URL), so this needs its own check rather than reusing
// that function for video blocks.
export function getYoutubeVideoId(block: BlockSummary): string | undefined {
  if (!block.media_ref) return undefined;
  const idx = block.media_ref.indexOf('://');
  if (idx === -1) return undefined;
  if (block.media_ref.slice(0, idx) !== 'youtube') return undefined;
  return block.media_ref.slice(idx + 3);
}
