# Design System — Requirements

## Introduction

This document scopes a cross-platform design system for Marrow's app (Expo / React Native, targeting iOS, Android, and Web equally). It covers two layers: a **token layer** (color, typography, spacing, radii, elevation) and a first set of **foundational primitive components** built on those tokens.

**Visual direction (settled):** black and white only, old-newspaper in spirit — neutral infrastructure, not something competing for attention. No gradients, no shadows, no rounded corners, no color used as decoration or status signal anywhere. This is a direct expression of the PRD's "no progress mechanics designed to be gamed" / "the only pressure is the one you bring" principles — a loud, colorful surface would work against them.

Anything currently in `app/` (theme.ts, themed-* components, etc.) is unmodified bootstrap scaffolding and carries no weight here — this is a green-field spec.

---

## Requirements

### Requirement 1 — Color Tokens

**User Story:** As a developer building any screen, I want a consistent, themed color palette, so screens are visually coherent without hand-picking colors.

#### Acceptance Criteria

1. WHEN a color is used anywhere in a component THE SYSTEM SHALL resolve it from a single defined palette — never a hardcoded literal in component code.
2. THE SYSTEM SHALL define exactly two palettes — light (white background, black ink) and dark (black background, white ink) — as a strict inversion of one another, switching automatically based on system appearance.
3. THE SYSTEM SHALL define semantic roles (e.g. `background`, `ink-primary`, `ink-secondary`, `ink-tertiary`, `border`) that resolve only to black, white, or grayscale values — no hue anywhere in either palette.
4. THE SYSTEM SHALL NOT use color to communicate status (error, warning, success, health, etc.) anywhere in the design system — status SHALL be communicated through text, shape, or iconography only.
5. THE SYSTEM SHALL define a distinct non-color visual treatment (e.g. a typographic style) to distinguish AI-authored text from user-authored text wherever both appear together, since color is unavailable for this purpose (PRD: "No content generated on your behalf").

---

### Requirement 2 — Typography Tokens

**User Story:** As a developer, I want a type scale and font tokens, so text hierarchy is consistent and long-form reading is comfortable.

#### Acceptance Criteria

1. THE SYSTEM SHALL define a constrained type scale (a fixed set of sizes and weights) — component code SHALL NOT set arbitrary font sizes.
2. THE SYSTEM SHALL define at least one reading-optimized body style (comfortable line-height, constrained measure), reflecting the PRD's "long-form favoured" principle.
3. THE SYSTEM SHALL use exactly two type families: a serif ("editorial") family used only for item titles, and a sans-serif ("UI") family used for all other text — bylines, meta, body preview text, labels, and every other piece of chrome. This is the only typographic warmth the system permits.
4. THE SYSTEM SHALL scale rendered text with OS-level accessibility text-size settings where the platform supports it (Dynamic Type on iOS, font scale on Android).

---

### Requirement 3 — Spacing & Layout Tokens

#### Acceptance Criteria

1. THE SYSTEM SHALL define a spacing scale used for all padding, margin, and gap values — component code SHALL NOT use arbitrary pixel values.
2. THE SYSTEM SHALL define a maximum content width for readable measure on wide viewports (web/tablet), consistent with long-form reading comfort.
3. THE SYSTEM SHALL use a corner radius of zero everywhere — no rounded corners in any component, with no exceptions.

---

### Requirement 4 — Surface Separation

#### Acceptance Criteria

1. THE SYSTEM SHALL separate surfaces (cards, dividers, input boundaries) using a 1px solid border in the ink-primary color — never a shadow, glow, or fill-based elevation cue.
2. THE SYSTEM SHALL NOT implement a shadow or elevation system of any kind — flatness applies uniformly across iOS, Android, and Web.

---

### Requirement 5 — Theming Mechanism

#### Acceptance Criteria

1. THE SYSTEM SHALL expose all tokens to components through a single theming mechanism (e.g. a theme provider/hook) — not duplicated per-component constants.
2. WHEN system appearance changes (light/dark) at runtime THE SYSTEM SHALL update themed components without an app restart.
3. THE SYSTEM SHALL make the theming mechanism equally usable on iOS, Android, and Web build targets.

---

### Requirement 6 — Core Primitive Components

**User Story:** As a developer building screens, I want a small set of foundational components built on the token layer, so I don't reimplement basic UI patterns per screen.

#### Acceptance Criteria

1. THE SYSTEM SHALL provide a `Text` component applying typography tokens via a defined set of semantic variants (e.g. heading, body, caption, item title) rather than raw style props.
2. THE SYSTEM SHALL provide a `Button` component supporting at least a solid (filled), outline, and ghost/text-only variant, distinguished only by fill, border, and typography — never by hue.
3. THE SYSTEM SHALL provide a `Surface`/`Card` component applying background and border tokens for grouping content — no radius or elevation variance.
4. THE SYSTEM SHALL provide a `TextInput` component styled from tokens, supporting label, placeholder, and error states, where the error state is communicated through text and border weight, not color.
5. THE SYSTEM SHALL provide a `Badge`/`Tag` component for compact status or label display (e.g. source health, highlight reaction type), where status is communicated through the label text and border/fill treatment only — never through color.
6. THE SYSTEM SHALL provide an `Avatar` component for author/creator representation, with a graceful fallback (initials or icon) when no image is available.
7. EVERY component in this requirement SHALL render correctly on iOS, Android, and Web without platform-specific consumer code.

---

### Requirement 7 — Accessibility Baseline

#### Acceptance Criteria

1. THE SYSTEM SHALL ensure text/background combinations in the default palettes meet WCAG AA contrast for body text — trivially satisfied for pure black-on-white/white-on-black `ink-primary` text, but SHALL be explicitly verified for `ink-secondary`/`ink-tertiary` grayscale tones, which are not maximum-contrast by design.
2. THE SYSTEM SHALL expose accessible labels/roles on interactive primitives (`Button`, `TextInput`) by default, not as an opt-in.
3. THE SYSTEM SHALL give interactive components a touch target of at least the platform-recommended minimum size (44×44pt iOS / 48×48dp Android).

---

## Out of Scope

- **Screen-level composition** (Feed, Dive reading view, Rabbithole graph, Review card UI) — these consume the design system but are specced separately per surface.
- **Iconography set** — components accept an icon slot; the icon library itself is not chosen here.
- **Motion/animation language** — deferred to a later pass once static tokens and components are settled. Directional intent already given for that future pass: fluid, high-contrast, sparse motion (referenced against the black-and-white *Samurai Jack* episode) — noted here so it isn't lost, not specced now.
- **Final visual identity** (exact palette values, chosen typefaces) — palette is settled (pure black/white/grayscale, no hue, per Requirement 1); exact grayscale hex values and the specific serif/sans typefaces are a `design.md` decision.
- **Styling library/implementation choice** (NativeWind, Tamagui, plain StyleSheet, unistyles, etc.) — explicitly deferred to `design.md`.
