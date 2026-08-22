# Source Curation Notes

Running notes from walking through the Content Sources Master List —
gaps that need follow-up work later, not anything to act on now.

## Needs a newsletter adapter (not yet built)

- **Semafor Flagship** (World News) — only distributed via email newsletter,
  no RSS/web feed found. Needs a "subscribe via email, parse inbound
  newsletter" adapter — a genuinely new source type, not a variant of an
  existing one.
- **Semafor Africa** (Africa News) — same email-only situation as Semafor
  Flagship.
- **Poetry Foundation** (Poetry) — the RSS feed URL returns `403`
  (bot-blocking WAF); user confirms they only distribute via email
  newsletter now. Same newsletter-adapter gap as Semafor.
- **Asymptote Journal** (Poetry) — feed URL returns `520` (Cloudflare
  origin error); also newsletter-only per user.
- **ICv2** (Comics & Graphic Novels) — newsletter-subscription only per
  user, no RSS/web feed.

## Skipped

- **The Economist Espresso** (World News) — app-only, no public RSS.

## No identifier yet (revisit later)

- **Stears** (Nigeria News, RSS) — no identifier given.
- **Stears Business** (Nigeria News, podcast) — no identifier given.
- **The Vergecast** (Tech) — no identifier given.
- **Healthline Nutrition** (Nutrition & Recipes, Text) — no identifier given.
- **Dr. Emily Prpa — Science and The City** (Nutrition & Recipes, Substack)
  — no identifier given.
- **Rachael Hartley — The Joy of Eating** (Nutrition & Recipes, Substack)
  — no identifier given.
- **Nigerian History: Analyze This** (History, Podcast) — no identifier given.
- **Africa History YouTubers** (e.g. Kumbaya Africa, History, YouTube) —
  no identifier given.
- **Royal Historical Society Blog** (History, Text) — no identifier given.
- **Epoch Echoes** (History, Substack) — no identifier given.
- **Making Sense with Sam Harris** (Philosophy, Podcast) — no identifier given.

## No feed found (revisit later) — Philosophy

- **Peter Singer** (petersinger.info) — no feed link discoverable on the
  site.

## Skipped (bot-blocked)

- **Dani Rodrik's weblog** (Economics, rodrik.typepad.com) — returns
  `403` even after following redirects; bot-blocking, not a dead site.

## Needs a newsletter/podcast-only adapter or platform we don't support

- **The Republic** (Wale Lawal, History) — only found on Spotify
  (`spotify.com/show/5BO7ZSrSnDVnGC9Rm0OYp9`), no public RSS feed
  discoverable on their site.

## Skipped (user request)

- **Nicolas Titeux's blog** (Sound Design & Music Theory, Text) — skipped
  per user.

## Skipped (doesn't exist)

- **Layne Norton** (Nutrition & Recipes, Twitter) — `@BiolayneNorton`
  genuinely doesn't exist (confirmed via `twscrape` directly).
- **Nature News** (Research Papers, Twitter) — `@NatureNews` genuinely
  doesn't exist (confirmed via `twscrape` directly).

## Not applicable (tools, not content sources)

- **Google Scholar**, **Semantic Scholar**, **OpenAlex**, **Consensus**
  (Research Papers) — search/discovery tools, not feeds — nothing to
  ingest here regardless of adapter type. Not a gap to revisit, just
  genuinely out of scope for Marrow's source model.

## Skipped (account doesn't exist / inactive)

- **Glen Keane** (Animation & Directing, Instagram) — `glenkeanestudio`
  genuinely doesn't exist (confirmed 404 via Instagram's own API). Likely
  not active on socials anymore — added `@AnimationMentor` (Twitter)
  instead per user.

## Cosmetic (not worth fixing now)

- **Short of the Week** (Short Films, RSS) — created with the display
  name "Short of the WeekShort of the Week" — the feed's own `<title>`
  field is genuinely doubled at the source. `Verify()` always re-derives
  the name from the feed (by design, so it can't be client-spoofed), so a
  one-off rename doesn't stick — would need an actual dedup fix in the
  RSS parsing path to fix properly.

## No identifier yet (revisit later) — Animation & Directing

- **Let Me Explain Studios** (Rebecca Parham, YouTube)
- **The Amazing Digital Circus** (Glitch Productions, YouTube)
- **Casually Explained** (YouTube)
- **Animation Mentor blog** (Text)

## Skipped (squatted handle)

- **Aaron Blaise** (Animation & Directing, Twitter) — `@AaronBlaise` now
  resolves to "Aaron Sunchild" with a default profile picture, not the
  real animator. Already have him via Instagram + YouTube.
- **Lev Polyakov** (Animation & Directing, Twitter) — `@levpolyakov`
  resolves to "Lev" with a default profile picture — same squatted-handle
  pattern.

## Skipped (no longer applicable)

- **Rich Hickey** (Engineering, Twitter) — not currently on Twitter/X;
  `@RichHickey` is now a squatted, unrelated account (default profile
  picture, no real activity).
- **Graham Cluley** (Security, Twitter) — `@gcluley` is now squatted by an
  unrelated new account (created 2026-05, 41 followers, 0 tweets). Added
  his RSS blog (`grahamcluley.com/feed/`) instead.

## No feed found (revisit later) — Psychology

- **Psychology Today** (Psychology, RSS) — the given feed URL is `404`;
  no feed link discoverable anywhere on the site.
- **Hidden Brain** (Psychology, Podcast RSS) — the given simplecast feed
  ID is stale (`404 NoSuchKey`); no feed link discoverable on
  hiddenbrain.org either.

## Skipped (unfetchable)

- **High Scalability** (Engineering, RSS) — its feedburner URL now
  redirects to a dead Squarespace parking page; the blog looks defunct /
  domain lapsed.
- **Storyfix** (Writing & Storytelling Craft, RSS) — the whole site
  (`storyfix.com`) returns a persistent `503`, not just the feed URL.
  Real outage, not our issue — revisit later.
- **Uber Engineering** (Engineering, RSS) — their blog returns `406` even
  with a normal browser User-Agent; looks like bot-detection/WAF that a
  plain server-side fetch can't get past.
- **CockroachDB Blog** (Database, RSS) — the given feed URL is `404`;
  common alternate paths (`/rss.xml`, `/feed.xml`, `/feed/`, `/rss/`) are
  all also `404`. Needs the actual current feed URL.
- **iquilezles.org** (Inigo Quilez, Graphics Engineering) — no feed link
  found anywhere on the page; looks like a genuinely feed-less static site.
- **the-witness.net/news/** (Graphics Engineering) — same, no discoverable
  feed.
- **casual-effects.com** (Morgan McGuire, Graphics Engineering) — the only
  feed link on the page is for an unrelated book's page
  (`cgpp/about.xml`), not the blog itself.

## Skipped (protected/private account)

- **Benedict Evans** (Tech, Twitter) — his Twitter/X account is protected
  (`"protected": true`), so twscrape's authenticated session can't see his
  tweets at all — a real platform limitation, not a bug. Skipped rather
  than falling back to an Instagram account of the same name (unverified
  whether it's really him).

## Pending retry (Twitter rate-limited mid-session)

Confirmed as a real, hard rate limit, not just flakiness — twscrape's own
error surfaced it directly: `No account available for queue "UserTweets".
Next available at 23:55:28` (single configured account). All caused by the
volume of resolve calls across this whole walkthrough (each `Resolve` does
2 Twitter API calls — user lookup + a tweet sample for cadence estimation).

- **Ethan Mollick** (`@emollick`) and **Shawn Wang / swyx** (`@swyx`) —
  resolved correctly earlier in the session; `@emollick` later fell back
  to an unrelated Instagram account ("Elena Wolansky") once Twitter
  started failing. Skipped rather than risk adding a wrong source.
- **Dan Luu** (`@danluu`), **Julia Evans** (`@b0rk`), **Evan Klitzke**
  (`@eklitzke`) — all resolved individually, but `AddSources`'s
  server-side re-verify (which always re-runs the full `Resolve`, `user_tweets`
  included) hit the rate limit on creation. Dan Luu's RSS blog (a separate,
  unrelated source) was added fine — only the 3 Twitter accounts are
  pending.
- Retry all 5 once the rate limit window has passed.
- Worth noting as a real correctness gap in `ResolveUrl` (`internal/service/ingest.go`):
  it silently falls through to the next registered adapter on any
  `Resolve` error, with no distinction between "this identifier genuinely
  doesn't match this adapter" and "this adapter is rate-limited/degraded
  right now" — the latter can produce a wrong candidate from a completely
  different adapter rather than surfacing the real problem. Not fixing
  this now (out of scope for the curation pass), just flagging it.
