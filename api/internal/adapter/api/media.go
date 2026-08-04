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
