package adapter

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	model "marrow/internal/model"
)

// YouTubeCaptionResolver is the MediaResolver half of the YouTube adapter —
// see RSSMediaSourceAdapter's doc comment for why this is a separate Go
// value sharing the same adapter ID ("youtube") rather than a second method
// on YouTubeSourceAdapter.
//
// It resolves a video ID into that video's own English caption track
// (manual if the creator wrote one, otherwise YouTube's auto-generated one)
// by shelling out to yt-dlp for subtitles only (--skip-download — no video
// or audio is ever fetched here). This isn't the original design: a direct,
// unauthenticated call to YouTube's timedtext endpoint was tried first and
// confirmed dead against a real video — YouTube now gates caption requests
// behind a PO token (proof-of-origin) anti-bot check introduced in
// 2024-2025, which broke every plain-HTTP scraping approach industry-wide.
// yt-dlp is actively maintained against exactly this kind of change, so
// this shells out to it rather than re-implementing that arms race.
type YouTubeCaptionResolver struct {
	id string
}

// NewYoutubeCaptionResolver asserts yt-dlp is actually on PATH before
// returning — Resolve shells out to it on every call, so a missing binary
// should fail loudly at startup (registry.go constructs every adapter
// eagerly), not as an opaque "exec: yt-dlp: not found" the first time a
// YouTube video is enriched.
func NewYoutubeCaptionResolver() *YouTubeCaptionResolver {
	if _, err := exec.LookPath("yt-dlp"); err != nil {
		panic("youtube adapter requires yt-dlp on PATH (used to fetch caption tracks): " + err.Error())
	}
	return &YouTubeCaptionResolver{id: "youtube"}
}

func (r *YouTubeCaptionResolver) Id() string { return r.id }

// Resolve returns the plain-text transcript for ref.Ref (a video ID) as
// Media{Kind: MediaCaption}. A video with no English captions at all
// (creator disabled auto-captions and never wrote their own — uncommon but
// real) is not an error: yt-dlp still exits cleanly, it just produces no
// subtitle file, and this returns an empty transcript so Enrichment still
// succeeds for that block, just with nothing to contribute. A failure to
// even run yt-dlp (video removed, network down, binary missing) is a real
// error.
func (r *YouTubeCaptionResolver) Resolve(ctx context.Context, ref model.MediaRef) (model.Media, error) {
	videoID := ref.Ref

	tmpDir, err := os.MkdirTemp("", "marrow-yt-captions-")
	if err != nil {
		return model.Media{}, fmt.Errorf("failed to create temp dir for captions: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	cmd := exec.CommandContext(ctx, "yt-dlp",
		"--write-subs", "--write-auto-subs", "--sub-langs", "en",
		"--skip-download", "--sub-format", "vtt",
		"-o", filepath.Join(tmpDir, "%(id)s"),
		"https://www.youtube.com/watch?v="+videoID,
	)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	runErr := cmd.Run()

	vttPath := filepath.Join(tmpDir, videoID+".en.vtt")
	data, readErr := os.ReadFile(vttPath)
	if readErr != nil {
		if runErr != nil {
			return model.Media{}, fmt.Errorf("yt-dlp failed to fetch captions for %s: %w (%s)", videoID, runErr, strings.TrimSpace(stderr.String()))
		}
		// yt-dlp ran fine; this video simply has no English captions.
		return model.Media{Buffer: nil, Kind: model.MediaCaption}, nil
	}

	return model.Media{Buffer: []byte(vttToText(string(data))), Kind: model.MediaCaption}, nil
}

var vttTagPattern = regexp.MustCompile(`<[^>]*>`)

// vttToText flattens a WebVTT caption file into plain text.
//
// Manually authored tracks are simple: one line of text per cue, in order —
// concatenating each cue's text already gives the transcript.
//
// Auto-generated tracks use a "rolling" display instead: each cue repeats
// the previous cue's line as its first line and appends newly-recognized
// words as its second line, so naively concatenating every line produces
// heavy duplication. But across both formats, a cue's LAST non-blank line
// is always its newest content — for a single-line manual cue that's the
// whole line; for a two-line rolling cue that's the appended part. Taking
// just that line per cue, skipping blanks, and skipping an exact repeat of
// the immediately preceding kept line (the rolling format's own "flush"
// cues, which repeat the prior line with nothing new) reconstructs the
// transcript correctly for both formats with the same logic.
func vttToText(raw string) string {
	blocks := strings.Split(strings.ReplaceAll(raw, "\r\n", "\n"), "\n\n")

	var out []string
	var prev string
	for _, block := range blocks {
		lines := strings.Split(block, "\n")

		timingIdx := -1
		for i, l := range lines {
			if strings.Contains(l, "-->") {
				timingIdx = i
				break
			}
		}
		if timingIdx == -1 || timingIdx+1 >= len(lines) {
			continue // WEBVTT header, or a cue with no text lines
		}

		var last string
		for _, l := range lines[timingIdx+1:] {
			if s := strings.TrimSpace(vttTagPattern.ReplaceAllString(l, "")); s != "" {
				last = s
			}
		}
		if last == "" || last == prev {
			continue
		}
		out = append(out, last)
		prev = last
	}

	return strings.Join(out, " ")
}
