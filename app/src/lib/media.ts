import { getAuthHeaders, resolveBaseUrl } from "./api";
import type { BlockSummary } from "./types";

// Resolvers whose stored ref isn't a directly-playable URL at all — it has
// to be re-resolved server-side, right before playback, via
// /media/playback-url (see api/internal/adapter/api/media.go's
// PlaybackURLResolver doc comment for why: Instagram's CDN URLs expire
// within hours, so the one captured at ingest time is often already stale
// by the time a user watches).
const PLAYBACK_URL_RESOLVERS = new Set(["instagram"]);

// "proxy" is a distinct resolver, not a real adapter — rss_media.go tags a
// block this way when its enclosure is plain http:// with no https
// alternative anywhere (some older podcast feeds, e.g. BBC's redirector
// chain). Both iOS (App Transport Security) and Android (cleartext-traffic
// policy) block that for a mobile client by default, silently — so instead
// of unwrapping to the raw URL, this routes through the backend's actual
// byte-streaming proxy (not a redirect — redirecting to the same http://
// URL would hit the identical block).
const PROXY_RESOLVER = "proxy";

type MediaBlock = Pick<BlockSummary, "kind" | "media_ref">;

// Audio/video blocks store `media_ref` as a self-describing
// "resolver://actual-url" envelope (see api/internal/model/media_ref.go) —
// split on the FIRST "://" only, since the URL itself contains "://".
// Image blocks store the direct URL with no envelope (images don't need a
// server-side MediaResolver), so they pass through unchanged.
function splitMediaRef(
	mediaRef: string,
): { resolver: string; ref: string } | null {
	const idx = mediaRef.indexOf("://");
	if (idx === -1) return null;
	return { resolver: mediaRef.slice(0, idx), ref: mediaRef.slice(idx + 3) };
}

// Media endpoints are authed (router.go's `authed` group), so players must
// send the Bearer token — expo-image, expo-video and expo-audio all accept
// per-request `headers`. Empty when logged out; callers pass the result
// straight through either way.
export function getMediaHeaders(): Record<string, string> {
	return getAuthHeaders();
}

// The backend route is a wildcard (/media/playback-url/*ref,
// /media/proxy/*ref) but the serialized ref routinely contains `?`, `&`,
// `#` or spaces from the underlying URL — appended raw, everything from
// the first `?` would be parsed as the request's own query string and the
// ref would arrive truncated. Encoding the whole ref keeps it intact; gin
// unescapes path params before Deserialize, so the round-trip is exact.
function buildMediaUrl(
	endpoint: "playback-url" | "proxy",
	mediaRef: string,
): string {
	return `${resolveBaseUrl()}/media/${endpoint}/${encodeURIComponent(mediaRef)}`;
}

export function getPlayableUrl(block: MediaBlock): string | undefined {
	if (!block.media_ref) return undefined;
	if (block.kind === "image") return block.media_ref;

	const split = splitMediaRef(block.media_ref);
	if (!split) return undefined;

	if (PLAYBACK_URL_RESOLVERS.has(split.resolver)) {
		return buildMediaUrl("playback-url", block.media_ref);
	}
	if (split.resolver === PROXY_RESOLVER) {
		return buildMediaUrl("proxy", block.media_ref);
	}
	return split.ref;
}

// A video block's media_ref envelope is "youtube://{videoID}" when it came
// from the YouTube adapter (see api/internal/adapter/impl/youtube.go) — not
// a raw file URL like RSS-media's, since YouTube doesn't hand those out.
// getPlayableUrl's generic unwrap would still "work" (it'd return the video
// ID as if it were a URL), so this needs its own check rather than reusing
// that function for video blocks.
export function getYoutubeVideoId(block: MediaBlock): string | undefined {
	if (!block.media_ref) return undefined;
	const split = splitMediaRef(block.media_ref);
	if (!split || split.resolver !== "youtube") return undefined;
	return split.ref;
}
