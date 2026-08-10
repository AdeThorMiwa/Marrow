package api

import (
	"context"

	model "marrow/internal/model"
)

// MediaResolver turns a MediaRef into raw media bytes. Implemented by
// whichever adapters actually produce audio/video content (not
// text-only adapters like Substack) — the same concrete adapter struct
// that implements SourceAdapter for its source type.
type MediaResolver interface {
	Resolve(ctx context.Context, ref model.MediaRef) (model.Media, error)
}

// PlaybackURLResolver turns a MediaRef into a fresh, currently-playable
// URL — a distinct capability from MediaResolver (which buffers full
// bytes for transcription, the wrong shape for "give me a URL"), for
// adapters whose CDN URLs are short-lived signed links rather than stable
// direct file URLs. The URL stored in ContentBlock.MediaRef at ingest time
// can go stale before a client ever plays it; this re-resolves on demand,
// right before playback, via the same registry-lookup pattern every other
// optional capability (MediaResolver, CommentsProvider) already uses —
// looked up by ref.Resolver, never hardcoded to one adapter.
type PlaybackURLResolver interface {
	ResolvePlaybackURL(ctx context.Context, ref model.MediaRef) (string, error)
}
