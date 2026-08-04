package models

import (
	"fmt"
	"strings"
)

// MediaRef is a self-describing reference to a piece of media: which
// resolver knows how to fetch it, and the resolver-specific identifier
// (a URL, a video ID, whatever that resolver expects). Adapters build this
// at Discover time and serialize it into a RawContentBlock/ContentBlock's
// MediaRef — resolving it back never needs to consult the owning Content's
// Source, which may have been deleted by the time enrichment runs.
type MediaRef struct {
	Resolver string
	Ref      string
}

func (m MediaRef) Serialize() string {
	return m.Resolver + "://" + m.Ref
}

// Deserialize splits on the first "://" only, so Ref may itself contain
// "://" (e.g. a real URL) without breaking the round-trip.
func Deserialize(s string) (MediaRef, error) {
	resolver, ref, ok := strings.Cut(s, "://")
	if !ok {
		return MediaRef{}, fmt.Errorf("malformed media ref: %q", s)
	}
	return MediaRef{Resolver: resolver, Ref: ref}, nil
}
