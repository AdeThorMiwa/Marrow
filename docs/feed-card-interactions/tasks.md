# Feed Card Interactions — Implementation Tasks

> Implements `docs/feed-card-interactions/design.md`.

- [x] 1. Instant scroll on new post reveal
  - `app/src/app/index.tsx`: `showNewItems` — `scrollToOffset({ offset: 0, animated: false })`
  - _Requirement 1 — Design §1_

- [x] 2. Swallow video taps before they reach the card's navigation `Pressable`
  - `app/src/app/index.tsx`: `ContentMedia`'s video branch — wrapped the `YouTubeEmbed` return in a no-op `Pressable`, placed in `ContentMedia` per Design §2.3
  - 🔬 Manual on-device verification still needed: tapping the video plays/pauses without navigating; tapping elsewhere on the card still navigates
  - _Requirement 2 — Design §2_
