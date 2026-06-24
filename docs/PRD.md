# Marrow — Product Requirements Document

---

## 1. Product Vision

Marrow is a personal cross-platform application (deskop and mobile) for consuming and retaining the content that matters to you. It is built on a single conviction: most content consumption leaves nothing behind. You read, watch, and listen — and most of it evaporates. Marrow exists to change that ratio.

The product has two complementary modes that together form a complete system. **Garden mode** is ambient — a unified feed of content from sources you've chosen, consumed at whatever depth you choose. **Rabbithole mode** is intentional — a structured system that wraps content in mechanisms for comprehension, retention, and synthesis so that what you engage with deeply actually becomes knowledge.

The two modes are not separate apps. They are two postures in the same space, and you move between them fluidly.

---

## 2. Core Principles

**You only get what you want.** Marrow surfaces no content you didn't choose. No discovery, no recommendations, no algorithmic curation. Your sources are your seeds. Nothing enters the garden you didn't plant.

**Nothing is lost.** Every capture, every mid-consumption artifact, every card, every action item — all of it accumulates. There is no ephemeral state in Marrow.

**Consumption without obligation.** Not everything needs to become a rabbithole. Most of what flows through the garden stays in the garden — consumed and released. The system never pressures you to go deeper than you want to.

**No content generated on your behalf.** The AI asks questions, surfaces gaps, nudges connections. It never writes your synthesis, never summarises content to replace reading it, never tells you what to think. The thinking is yours.

**No external redirects.** All content is consumed inside Marrow.

**Long-form favoured.** Rabbithole mode is designed around depth. It favours long-form, substantive content over short-form. Short-form content can exist in the garden but is not the primary unit of deep engagement.

---

## 3. The Two Modes

### Garden Mode

The default state. Ambient, low-pressure, receptive. You are browsing what came in, following threads casually, tending what's already growing. Most sessions begin and end here.

### Rabbithole Mode

The intentional state. Activated from within a Deep Read. You have committed to going deeper on something — either completing a structured engagement with a single piece of content, or opening a sustained topic of inquiry. The posture shifts from receptive to active.

---

## 4. Garden Mode

### The Feed

A chronological stream of content from your sources. No algorithm, no ranking, no filtering. Time order only.

The feed is a river, not a queue. There is no read/unread state. There is no backlog to clear. No sense of falling behind. Items flow through — you catch what you catch.

### Sources

You add sources explicitly and intentionally. A source is a specific creator, channel, publication, or feed — not a platform. You add a YouTube channel you love, not YouTube. You add the specific podcast, not podcasts.

The primary gesture for adding a source is a URL. You paste what you have. Marrow resolves it to a feed. The resolution mechanism is an internal concern — the user should not need to know what RSS is or find a feed URL manually. If Marrow cannot resolve a URL it says so and asks for more information.

Source management is a maintenance surface, not a primary one. It lives out of the way. The feed is the only surface that matters.

**Source health** is surfaced passively. If a source goes stale or breaks, it surfaces as an informational card in the feed — the way a gardener notices a bad crop while tending. Not a notification, not an alert. Something you encounter naturally and can act on or ignore.

### The Empty Garden

A new user's garden is empty. This is intentional, not an error state. The framing is: you are about to plant something. The first gesture is adding a source.

### Tending

Even in garden mode, open topics exist in the periphery. Spaced repetition cards come due. Topic queues accumulate new content.

Tending is **surfaced, not pushed.** The garden shows you quietly: cards due, topic queue status. You act on it or ignore it. It does not nag.

### Active Topic Lens

If you have an active topic, the garden feed has two views:

- **Everything** — the full chronological feed from all sources
- **Topic lens** — the feed filtered toward your active topic, surfacing relevant items

You can switch between these with a single gesture. Items in the full feed that are relevant to your active topic carry a subtle visual signal.

### Engagement Detection

While you browse, Marrow passively tracks engagement across the session. It is looking for clusters — meaningful thematic overlap across items you've engaged with deeply in a short window.

Engagement is measured by a combination of signals:
- Time spent relative to estimated consumption time
- Scroll depth or playback progress
- Any highlight or flag made
- Return visits to the same item

When a cluster is detected, Marrow proposes a Deep Read at a natural pause point. The proposal is non-blocking, appears once per cluster per session, and disappears if dismissed. Dismissed means dismissed for that session.

---

## 5. The Deep Read

Deep Read is the transitional state between casual garden browsing and full rabbithole mode. It is the moment the retention loop activates on a piece of content.

### Two Entry Points

**System-proposed:** Marrow detects an engagement cluster and surfaces a proposal. User accepts or dismisses.

**User-initiated:** At any point while reading an item, the user can manually trigger a Deep Read. No engagement detection required.

### What Activates

When a Deep Read begins:
- Passive capture becomes active — highlights and flags are now being recorded
- Active retention mechanisms become available on demand
- The loop begins listening from the start of the content, not the moment of activation — prior engagement in this item is backfilled where possible

### Passive Capture

Always on during a Deep Read. Never intrusive.

- **Highlights** — marked passages, moments, or segments. Each carries a reaction type.
- **Flags** — lighter marks. "I want to return to this" without committing to why yet. Resolved in the full loop at completion.

**Reaction types:**
- Insight — this reframes something I thought I understood
- Surprising — I didn't expect this
- Disagree — I have friction with this claim
- Question — I want to understand this more deeply
- Actionable — this changes something I will do

The reaction type is not cosmetic. It shapes what happens downstream — what gets surfaced in gaps analysis, what kind of Socratic mode is most relevant, what kind of card gets generated.

### Active Retention Mechanisms

Available on demand throughout consumption. Never surfaced automatically. You invoke them, they don't appear on their own.

**Spot comprehension** — a quick pulse check on what you've consumed so far. Two or three questions on recent content. Useful after a dense section before continuing.

**Inline application prompt** — invokable when you highlight something actionable or mark a strong insight. A single application question on just that moment. Captures the thought while it's hot.

**Socratic nudge** — invokable when you flag something as disagree or question. Opens a focused Socratic exchange on that specific claim — devil's advocate or contradiction surfacing on one point, right there, without leaving the content.

### Mid-Consumption Artifacts

Everything produced during a Deep Read before completion — highlights, flags, spot comprehension responses, inline application responses, Socratic exchange transcripts — is saved. These artifacts are first-class citizens. They accumulate across sittings for long-form content. They inform the full retention loop at completion. Nothing is ephemeral.

### Declaring Completion

For short-form content, completion may be implicit — you reach the end. For long-form content (books, papers), completion must be explicit — the user declares they are done. This declaration is the trigger for the full retention loop.

---

## 6. The Retention Loop

Runs at completion of a Deep Read. Informed by everything accumulated mid-consumption. The same loop applies regardless of whether the Deep Read completes as a standalone engagement or transitions into a topic rabbithole.

### Phase 1 — Gaps Analysis

Marrow compares what you captured against the full content. It surfaces important ideas the content contained that your capture didn't reflect — weighted toward core arguments, not peripheral detail.

Three categories of gaps:
- **Uncaptured claims** — ideas the content made that you didn't mark
- **Unresolved flags** — marks you made during consumption that didn't get annotated
- **Assumption zones** — places where the content made an assumption you likely didn't notice because it matched your existing beliefs

The output is a set of gap cards — specific ideas or claims that your capture missed. These feed directly into the comprehension check.

### Phase 2 — Comprehension Check

A short set of questions — five to seven maximum. Not a test, a probe.

Questions are generated from:
- Gap cards from Phase 1
- Question-flagged highlights from consumption
- The content's core claims regardless of whether you captured them

Questions are not generated from your insight, surprising, or actionable highlights. You already engaged with those. The check targets what you didn't engage with.

**Question types:**
- Recall — can you reconstruct this idea in your own words?
- Application — given this idea, what would follow in a situation you know?
- Tension — this claim seems to conflict with something. How do you resolve that?
- Completion — you captured part of an argument. What's the rest?

Responses are not graded. They calibrate the initial difficulty of spaced repetition cards generated from this content.

### Phase 3 — Application

A short conversation calibrated to the content's applicability type.

**Applicability type is user-set explicitly.** Marrow does not infer it. You declare it before the application phase begins. The act of choosing forces critical thinking about what kind of content this actually is.

Three types:
- **Immediately applicable** — changes something you do this week. *"What's one thing you could act on before your next session?"*
- **Slow-burn** — shifts how you think over time. *"What belief or assumption does this put pressure on?"*
- **Foundational** — changes what's possible to think. *"What does understanding this unlock that you couldn't reason about before?"*

In a topic rabbithole, the application conversation has richer context — it can reference your library and synthesis. Prior conclusions you've reached in the topic can be surfaced and challenged.

The output is zero or more **action items** — things you articulate during the conversation that the AI recognises and extracts. Action items are pure output artifacts. They leave Marrow immediately via webhook. Marrow manages no lifecycle for them.

### Phase 4 — Card Generation

Spaced repetition cards are generated automatically from:
- Highlights (especially insight and disagree types — high retention value)
- Gap cards confirmed weak in the comprehension check
- Questions you struggled with in Phase 2
- Key claims from the content's core argument

Card generation is automatic and not exhaustive. Quality over quantity. Each card has an initial difficulty calibrated by comprehension check performance.

When a card surfaces for the first time, the user rates it: **keep** or **not useful.** Not useful doesn't discard the card permanently — it signals to the system that this type of card isn't working and adjusts future card generation accordingly. Over time the generation model calibrates to what you actually find retainable.

Scheduling is handled by spaced repetition algorithm from that point forward.

### Phase 5 — Socratic Exit Nudge

At the natural exit point after card generation, Marrow surfaces a single question — generated from the content, genuinely specific to what you just read. Not generic.

If you engage with it, the full Socratic session opens. If you dismiss it, you're done.

**Five Socratic modes, all fully on demand:**
- **Devil's advocate** — AI steelmans the position the content argues against
- **Teach it back** — you explain the concept; AI probes where your explanation is thin
- **Contradiction surfacing** — tensions between this content and your library
- **Synthesis** — connections between this content and other things you've processed
- **Open questions** — what this content doesn't answer; what you'd need to read next

### After the Loop

Two paths:

**Complete** — artifacts and cards go to library and review pool. You return to the garden.

**Go Deeper** — topic rabbithole initiates. The content becomes the first item in a new or existing topic.

---

## 7. Rabbithole Mode — Topic

A topic is an ongoing area of inquiry. You're not just processing a single piece of content — you're building an accumulated understanding across many pieces over time. The artifact isn't any single item. It's a synthesis you're working toward.

### What a Topic Has

**Library** — everything you've processed that belongs to this topic. Your accumulated understanding.

**Graph / Frontier** — the navigable space of what to explore next. The frontier and the graph are the same thing viewed differently: as navigation it's a graph, as a queue it's the frontier.

**Synthesis** — the living document of what you're building. Starts empty. Grows across sessions.

### Initiating a Topic

A topic can only be initiated from within a completed Deep Read — not declared upfront from the garden.

If you try to create a topic that is meaningfully similar to an existing one, Marrow surfaces the existing topic and asks if you want to add to it instead. No duplicate topics, no fragmented knowledge.

### Topic Lifecycle

**Active** — your current focus. One at a time. The garden's topic lens points here. Sessions have full structure.

**Watching** — open but not your current focus. Cards still come due. Frontier still accumulates. Up to five watching topics simultaneously.

**Paused** — explicitly set aside. Cards pause. Frontier freezes.

**Closed** — two sub-types:
- *Completed* — you wrote a synthesis, declared sufficiency, closed deliberately. Library and synthesis preserved. Cards continue in general review pool.
- *Abandoned* — closed without synthesis. Work preserved, topic archived.

A topic is closeable when you can articulate what you now understand that you didn't before — when you can write a coherent synthesis. Completion is triggered by a felt sense of sufficiency, not by finishing a queue.

### Topic Sessions

Each engagement with an active topic has a loose shape:

**Orient** — brief re-entry. Where did you leave off? What's due for review? What's next in the graph? A moment of reorientation, not a dashboard.

**Process** — take an item from the frontier through the full Deep Read and retention loop. In topic context, the application conversation draws on your library and synthesis.

**Review** — spaced repetition cards due from this topic surface here. Topic-specific, not your full card pool.

**Surface** — at the end, Marrow surfaces one synthesis prompt. An invitation, not a demand. You write into it or dismiss it.

### The Synthesis

The synthesis is not a summary. A summary is what the content said. A synthesis is what you now think — shaped by the content, but yours.

It grows incrementally throughout the topic's life. Each session's synthesis prompt is an invitation to add a thread, challenge a claim you wrote earlier, or connect two things that didn't connect before.

The AI's role is as a thinking partner. It asks questions, surfaces contradictions in what you've written, points to things in your library your synthesis hasn't accounted for. It does not write the synthesis for you.

### System-Sourced Content

In topic rabbithole mode only, Marrow can surface content on your behalf beyond what's in your garden — wikis, research papers, books, long-form reference material. Deeper, slower, more authoritative content than your curated feed of favourite creators typically produces.

The garden has a natural ceiling — it's bounded by who you follow. A topic rabbithole by definition wants to go deeper than that ceiling. System-sourced content removes it.

System-sourced content is clearly distinguished from user-sourced content. The user always knows which lane an item came from — the epistemic situation is different and that matters.

System-sourced content does not appear in the garden. It exists exclusively in the topic frontier.

---

## 8. The Graph

The graph is the navigable structure of a topic's frontier. It is not a knowledge base Marrow maintains. It is a trace of your inquiry — it only exists where you have navigated.

### How It Works

The graph is lazy. Connections are not pre-built or inferred in advance. When you land on a node, Marrow fetches its connections at that moment. Until you navigate toward something, it doesn't exist in the system.

**Node** — a concept. Could be a broad area or a specific idea.

**Connections** — related concepts adjacent to the current node. Fetched on demand when you arrive.

**Traversal** — you click a connection, it becomes the new focus node, its connections are fetched, the previous node becomes one of its visible connections. You can always navigate back.

**Content** — each node has associated content items sourced from your garden feeds and system sources. Processing a content item belonging to a node adds it to your topic library.

### The Local Window

The interface at any moment shows:
- The current node in focus
- Its direct connections
- The previous node as one of those connections — always present, giving you a way back

The window is always small and legible. You are never staring at the full graph.

### The Full Map

The full graph — everywhere you've been in this topic — is consultable as a map. Not a primary navigation surface. A reference. A way to see the shape of your inquiry so far.

### Sequencing

In v1 the graph is built correctly as an underlying structure. Topic frontier navigation is queue-based. In v2, graph navigation becomes the primary surface. The migration is a frontend concern, not an architectural one — the data is already there.

---

## 9. What the System Doesn't Do

**No discovery.** Marrow never suggests sources you didn't choose.

**No social layer.** Nothing is shared, compared, or visible to anyone else.

**No progress mechanics designed to be gamed.** No streaks, no completion percentages, no inbox zero pressure. The only pressure is the one you bring.

**No AI-generated thinking.** The AI asks, probes, connects, and nudges. It never generates your conclusions for you.

**No external redirects.** All content is consumed inside Marrow.

**No action item lifecycle.** Action items are output artifacts. They leave Marrow via webhook. Marrow does not track, manage, or display them after generation.

---

## 10. Open Questions

**Engagement cluster heuristics** — the specific weights and thresholds for detecting a cluster are directionally locked but need refinement at build time.

**Cross-topic connections** — concepts will inevitably overlap across topics. Currently cross-topic connections only surface in the Socratic synthesis mode. Whether a lighter ambient signal is warranted is unresolved.

**The unified library interface** — mid-consumption artifacts and loop-completion artifacts are all first-class and accumulate together. The interface that presents this unified library is directionally clear but not yet designed in detail.

**Graph node generation** — when a user arrives at a node, what generates its connections and associated content? What sources does Marrow draw on and what is the selection logic? Needs design at build time.

**Completion signal for audio and video** — declaring completion for text is straightforward. For podcasts and video, the completion signal and how mid-consumption retention mechanisms surface without interrupting playback needs specific design.
