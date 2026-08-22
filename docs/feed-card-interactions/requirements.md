# Feed Card Interactions — Requirements

## Introduction

Two small, unrelated interaction bugs on the feed screen, grouped into one spec because they're both quick fixes to existing feed-card behavior rather than new features:

1. When new posts arrive and the user taps the "new posts" pill, the feed animates a smooth scroll back to the top. It should jump there instantly.
2. Tapping a YouTube video embedded in a feed card navigates to Content Detail instead of letting the video's own tap-to-play/pause interaction happen — the card's outer press handler is capturing taps that land on the video.

---

## Requirements

### Requirement 1 — Instant Scroll on New Post Reveal

**User Story:** As a user, I want tapping "new posts" to take me to the top immediately, not watch an animated scroll.

#### Acceptance Criteria

1. THE SYSTEM SHALL scroll the feed list to the top instantly (no animation) when the user taps the "new posts" pill.
2. THE SYSTEM SHALL NOT change any other behavior of `showNewItems` (prepending new items, clearing the pending-new-items state) — only the scroll's animated flag changes.

---

### Requirement 2 — YouTube Video Taps Don't Navigate to Content Detail

**User Story:** As a user, I want to tap directly on a video to interact with it (play/pause), and tap anywhere else on the card to open Content Detail — not have every tap on the card navigate away regardless of where I touched.

#### Acceptance Criteria

1. THE SYSTEM SHALL NOT navigate to Content Detail when the user's tap lands on the YouTube video embed area of a feed card.
2. THE SYSTEM SHALL continue to navigate to Content Detail when the user taps any other part of a `detailable` card (title, summary, source row, whitespace) — Requirement 2.1 scopes only the video's own tap area, not the whole card.
3. THE SYSTEM SHALL NOT change tap behavior for feed cards that don't contain a video block — this requirement only affects cards rendering `ContentMedia` with `type === 'video'`.
4. THE SYSTEM SHALL preserve the existing `detailable` gate (Content Detail Requirement 3.3) — a non-`detailable` card still gets no press affordance at all, regardless of this change.

---

## Out of Scope

- Any change to how the YouTube video itself plays, pauses, or errors (`YouTubeEmbed`'s existing play-state/error-fallback logic is untouched).
- The web platform variant (`youtube-embed.web.tsx`) — this spec is scoped to the native tap-bubbling bug reported on mobile; the web build's click handling is a separate DOM event model and not confirmed to have the same issue. Design phase should note whether it needs the same fix.
- Any other feed-card gesture (long-press, swipe actions) — none currently exist and none are being added here.
