package models

// MediaKind is narrower than ContentBlockKind — Media is only ever resolved
// audio or video bytes, never text, so it gets its own type rather than
// reusing ContentBlockKind (which also admits BlockText).
type MediaKind string

const (
	MediaAudio   MediaKind = "audio"
	MediaVideo   MediaKind = "video"
	MediaCaption MediaKind = "caption" // resolved plain-text captions, not raw audio/video bytes — see YouTubeCaptionResolver
)

// Media is raw resolved media bytes, ready for transcription. It carries no
// source/adapter knowledge — that's MediaResolver's job (adapter/api/media.go).
type Media struct {
	Buffer []byte
	Kind   MediaKind
}
