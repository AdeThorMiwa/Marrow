# Feed Card Interactions — Design

> Implements `docs/feed-card-interactions/requirements.md`. Both fixes live in `app/src/app/index.tsx`; Requirement 2 also touches `app/src/components/ui/youtube-embed.tsx` only if on-device testing shows the nested-Pressable fix alone isn't enough.

## Status legend

| | |
|---|---|
| ✅ Refined | Design decision made, ready to implement |
| 🔄 Open | Needs a decision, or on-device verification, before implementation |

---

## 1. Requirement 1 — Instant Scroll ✅

`showNewItems` (`app/src/app/index.tsx`) already does exactly the right thing except for one flag:

```tsx
const showNewItems = useCallback(() => {
  setItems((prev) => [...newItems, ...prev]);
  setNewItems([]);
  flatListRef.current?.scrollToOffset({ offset: 0, animated: true });  // → animated: false
}, [newItems]);
```

Change `animated: true` → `animated: false`. Nothing else in the function changes — the prepend and state clear are correct as-is.

---

## 2. Requirement 2 — Video Taps Shouldn't Navigate ✅

### 2.1 Root cause

`ContentRow` wraps its entire card body — including `ContentMedia`, which renders `YouTubeEmbed` for video cards — in one outer `Pressable`:

```tsx
const content = (
  <View ...>
    ...
    <ContentMedia type={type} blocks={payload.blocks} isVisible={isVisible} />
    ...
  </View>
);
if (!payload.detailable) return content;
return <Pressable onPress={() => router.push(`/content/${payload.content_id}`)}>{content}</Pressable>;
```

`YouTubeEmbed` renders `react-native-youtube-iframe`'s `YoutubePlayer`, which hosts the actual YouTube IFrame player inside a `react-native-webview` `WebView`. The `WebView` is a native view with its own touch/gesture handling for the embedded page (YouTube's own play/pause tap target) — but nothing today stops a tap that lands on it from also completing as a press on the outer card `Pressable`, so `router.push` fires and the video never gets the interaction.

### 2.2 Fix: swallow the tap at the video boundary

Wrap only the `YouTubeEmbed` render path in `ContentMedia` with a nested no-op `Pressable`. React Native's touch responder system resolves nested `Pressable`/`Touchable` components innermost-first — the deepest one that claims the responder wins, and a claimed touch does not continue on to an ancestor `Pressable`'s `onPress`. Adding an inner `Pressable` around just the video means:

- A tap starting on the video area is claimed by the inner `Pressable` (a no-op `onPress`), so the outer card `Pressable` never sees it → no navigation.
- The underlying `WebView`'s own native touch handling (YouTube's play/pause) is unaffected — the inner `Pressable` doesn't consume the native touch itself, it only wins the RN responder negotiation against the *outer* `Pressable`.

```tsx
// app/src/components/ui/youtube-embed.tsx — or inline in ContentMedia, see 2.3
import { Pressable } from 'react-native';

if (type === 'video') {
  const block = blocks.find((b) => b.kind === 'video');
  const youtubeVideoId = block ? getYoutubeVideoId(block) : undefined;
  if (youtubeVideoId) {
    return (
      <Pressable onPress={() => {}}>
        <YouTubeEmbed videoId={youtubeVideoId} isVisible={isVisible} />
      </Pressable>
    );
  }
  ...
}
```

Scoped to only this branch — `VideoPlayer` (non-YouTube video) and the audio branch are untouched, matching Requirement 2.3. The existing `YouTubeEmbed` error-state fallback (its own internal `Pressable` that opens YouTube externally) is unaffected either way, since it already sits inside whatever wraps `YouTubeEmbed`.

### 2.3 Placement: wrapper in `ContentMedia` vs. inside `YouTubeEmbed` 🔄

Two equally-correct places to add the wrapper:

- **In `ContentMedia`** (shown above) — keeps `YouTubeEmbed` itself unaware of the navigation-swallowing concern; the card-composition component (`ContentMedia`) owns it, which matches Requirement 2.3's framing of this as a per-branch card-composition rule, not a property of `YouTubeEmbed` as a reusable component.
- **Inside `YouTubeEmbed`**, wrapping its own returned `View` — makes `YouTubeEmbed` itself "safe to embed in a Pressable" everywhere it's used, which matters if it's ever reused somewhere else with the same bubbling risk (currently it's only used here).

Leaning toward **`ContentMedia`** (first option) — `YouTubeEmbed` shouldn't need to know it's inside a navigable card; that's a fact about where it's placed, not about the component itself. Confirm during implementation if `YouTubeEmbed` turns out to have other call sites.

### 2.4 Verification 🔄

No automated test infrastructure exists for this component tree (confirmed — no test files under `app/src/app/` or `app/src/components/ui/`). Verification is manual: run the app (`npm run ios` / `npm run android`), open the feed, find a card with a YouTube video, and confirm (a) tapping the video area plays/pauses it without navigating, (b) tapping the title/summary/source row still navigates to Content Detail.

---

## 3. Files touched

- `app/src/app/index.tsx` — `showNewItems`'s `animated` flag; `ContentMedia`'s video branch gets the wrapping `Pressable`.
