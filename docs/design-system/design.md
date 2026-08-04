# Design System — Design

> Implements `docs/design-system/requirements.md`. Target: Expo / React Native (iOS, Android, Web — equal priority).
>
> **Design intent:** boxy, boring, black and white. The surface reads as neutral infrastructure, not something competing for attention — old-newspaper in spirit. No gradients, no shadows, no rounded corners, no color as decoration or status signal. The only color-like thing anywhere on this surface is pure black on pure white (and the strict inversion in dark mode). This is a direct expression of "no progress mechanics designed to be gamed" / "the only pressure is the one you bring."

## Status legend

| | |
|---|---|
| ✅ Refined | Design decision made, ready to implement |
| 🔄 Open | Needs a decision before/during implementation |

---

## 1. Styling Engine ✅

**Decision: NativeWind v4** (Tailwind CSS for React Native), with **`class-variance-authority` (cva)** for variant composition and **`tailwind-merge`** for safe className overrides.

**Why:**
- Tailwind's utility model maps directly onto "semantic token roles, no raw values in component code" (Req 1.3, 2.1, 3.1) — a `tailwind.config.js` theme extension *is* the token layer's public surface.
- Works identically across iOS, Android, and Web from the same `className` API (Req 6.7, 5.3) — no per-platform styling code in consumers.
- `app/global.css` already exists in the bootstrap importing Tailwind directives, and `react-native-web` is already a dependency — the plumbing is half-present.
- A monochrome, border-only, zero-radius system is *less* work in Tailwind than a full color/shadow/radius system would have been — mostly `border`, `border-black`, spacing, and text utilities.

**Rejected:** Tamagui (compiler-based, heavier setup/learning-curve commitment for what's currently a small app), plain `StyleSheet` (would require hand-rolling the entire variant/theming system Req 5 asks for), `react-native-unistyles` (smaller ecosystem, no strong advantage over NativeWind here).

**New dependencies to add:** `nativewind`, `tailwindcss` (peer), `class-variance-authority`, `tailwind-merge`.

**Revised during implementation:** `className`/NativeWind ended up unused by the six primitives — see §3's revision note. All styling (including layout) is inline `style` from `useTheme()`, since color already had to be there and splitting layout across both `style` and `className` added complexity for no benefit. `nativewind`/`tailwindcss` remain installed and wired (babel/metro config, `global.css`) so `className` is available the moment something actually needs it; `class-variance-authority`/`tailwind-merge` are installed but currently unused dead weight — fine to remove later if `className` stays unused, or put to work once real screens (Feed, etc.) get built.

**Bootstrap cleanup:** `app/src/constants/theme.ts`, `themed-text.tsx`, `themed-view.tsx`, `hooks/use-theme.ts`, `hooks/use-color-scheme*.ts`, and the existing `components/ui/*` are unmodified scaffolding per your note — they get replaced, not extended. Removal is a tasks.md item, not done here.

---

## 2. Token Layer ✅

All tokens live under `app/src/theme/tokens/`, plain TypeScript, framework-agnostic — the single source of truth. `tailwind.config.js` imports and maps them into `theme.extend`; anything needing JS-level values (icon fill colors, computed border styles) imports the same files directly via `useTheme()` (§4). No values are hand-typed a second time in either place.

### 2.1 Color (Req 1)

Two palettes, strict inversions of each other, grayscale only — no hue anywhere:

```ts
// theme/tokens/colors.ts
export const light = {
  background:    '#FFFFFF',
  ink:           '#000000', // text, borders, buttons, icons
  inkSecondary:  '#666666', // byline, meta, timestamps
  inkTertiary:   '#777777', // source-health notices and similar low-emphasis text
} as const;

export const dark = {
  background:    '#000000',
  ink:           '#FFFFFF',
  inkSecondary:  '#999999', // strict inversion of #666666
  inkTertiary:   '#888888', // strict inversion of #777777
} as const;

export type ColorToken = keyof typeof light & keyof typeof dark;
```

No `surface`, `accent`, `danger`/`warning`/`success` roles — there is nothing for them to mean here. A "card" is `background` + a 1px `ink` border (§2.4), not a different fill. Status (source health, form errors) is communicated by **text and border weight**, never by color (Req 1.4) — see `Badge` (§4.5) and `TextInput` (§4.4).

**AI-vs-user text (Req 1.5):** no color role is available, so the distinction is typographic — AI-authored text renders *italic*, user-authored text renders upright, both at `ink`/`inkSecondary`. Carried by `Text`'s `voice` prop (§4.1), not a color prop.

**Contrast check:** `#666666` on `#FFFFFF` → 5.74:1 (clears AA). `#777777` on `#FFFFFF` → 4.47:1 — just under AA's 4.5:1 for normal text, clears AA-Large (3:1). **Accepted as-is** — `inkTertiary` usage should stay to ≥18pt/14pt-bold text (source-health notices, meta) where AA-Large applies, rather than small body text. Dark-mode inversions (`#999999`/`#888888` on `#000000`) both land above their light-mode counterparts (7.37:1 and ~6:1), so no adjustment needed there.

### 2.2 Typography (Req 2)

Two families only — serif reserved *exclusively* for item titles (the one permitted warmth), sans for everything else, including chrome, meta, and body preview text:

```ts
// theme/tokens/typography.ts
export const fontFamily = {
  voice: Platform.select({
    ios: 'ui-serif', android: 'serif', web: 'Georgia, "Times New Roman", serif', default: 'serif',
  }), // item titles only
  sans: Platform.select({
    ios: 'system-ui', android: 'sans-serif', web: 'system-ui, sans-serif', default: 'sans-serif',
  }), // everything else
} as const;

export const typeScale = {
  caption:    { fontSize: 12, lineHeight: 16, fontWeight: '400', family: 'sans' },
  label:      { fontSize: 14, lineHeight: 20, fontWeight: '500', family: 'sans' },
  body:       { fontSize: 16, lineHeight: 26, fontWeight: '400', family: 'sans' },
  bodyLarge:  { fontSize: 18, lineHeight: 29, fontWeight: '400', family: 'sans' }, // reading-optimized chrome text (Req 2.2)
  heading3:   { fontSize: 20, lineHeight: 26, fontWeight: '600', family: 'sans' },
  heading2:   { fontSize: 24, lineHeight: 30, fontWeight: '600', family: 'sans' },
  heading1:   { fontSize: 32, lineHeight: 38, fontWeight: '700', family: 'sans' },
  itemTitle:  { fontSize: 20, lineHeight: 27, fontWeight: '400', family: 'voice' }, // the one serif use-case — FeedItem/ContentItem titles
} as const;

export type TypeVariant = keyof typeof typeScale;
```

`itemTitle` is deliberately separate from `heading1`–`3`: those are sans UI chrome (section headers, nav titles); `itemTitle` is the single place `family: 'voice'` (serif) is ever used, matching "used only for item titles" literally rather than mapping every heading to serif.

Dynamic Type / Android font scale (Req 2.4) comes from **not** disabling `allowFontScaling` on text render (RN default) — enforced in `Text` (§4.1), not a token.

### 2.3 Spacing & Layout (Req 3)

Unaffected by the monochrome direction — aligned to Tailwind's default 4px-step scale so utility classes and semantic aliases always agree:

```ts
// theme/tokens/spacing.ts
export const spacing = { xs: 4, sm: 8, md: 16, lg: 24, xl: 32, xxl: 48 } as const;
export const maxContentWidth = 800; // Req 3.2 — readable measure on web/tablet
```

```ts
// theme/tokens/radii.ts
export const radius = 0; // Req 3.3 — zero everywhere, no exceptions
```

**Consequence for `Avatar` (§4.6):** a conventional circular avatar is incompatible with "radius zero, no exceptions." **Confirmed: `Avatar` is square**, not circular — the literal reading of "boxy," no exceptions.

### 2.4 Surface Separation (Req 4)

No elevation system. Surfaces are separated by a single 1px `ink`-colored border — nothing else:

```ts
// theme/tokens/border.ts
export const borderWidth = 1; // Req 4.1 — the only separation mechanism; never a shadow
```

Renders identically on iOS/Android/Web since it's a plain `View` border, not a platform shadow primitive — trivially satisfies Req 4.2's cross-platform consistency requirement (there's no platform-specific shadow code to reconcile in the first place).

---

## 3. Theming Mechanism ✅ (Req 5) — revised during implementation

Originally planned as two halves (Tailwind `dark:` classes for color + a `useTheme()` escape hatch for JS-only cases). **Changed during implementation**: NativeWind v4's exact CSS-variable dark-mode wiring syntax couldn't be verified with confidence (this was already flagged as the open question below), so **all color is now applied via `useTheme()` + inline `style`**, never via `dark:`-prefixed className. Tailwind/NativeWind `className` is used only for structural, non-color-dependent utilities (nothing currently needs this, since padding/layout also go through `style` in practice — `className` is wired and available for future use, e.g. flex/gap utilities, but isn't load-bearing for anything theme-related). This is more verbose per-component but has zero risk of a className/style mismatch, and every component already needs `useTheme()` for typography anyway, so it's not an added dependency.

`tailwind.config.js` no longer imports the color tokens at all — `theme/tokens/colors.js` is consumed only by `theme-provider.tsx`.

`useTheme()` is the single source of themed values for every component:

```ts
// theme/theme-provider.tsx (actual signature, includes borderWidthError for TextInput)
export function useTheme() {
  const scheme = useColorScheme(); // React Native hook — live-updates on OS change
  const colorScheme = scheme === 'dark' ? 'dark' : 'light';
  return {
    colorScheme,
    colors: colors[colorScheme],
    typeScale, fontFamily, spacing, maxContentWidth, radius, borderWidth, borderWidthError,
  };
}
```

A thin `ThemeProvider` wraps the app root as a single override point in case a future in-app light/dark toggle is added (today: system-only, per Req 5.2's literal scope — no user override exists yet).

Satisfies Req 5.3 — `useColorScheme` and NativeWind both work identically on iOS/Android/Web.

---

## 4. Core Primitive Components ✅ (Req 6)

Location: `app/src/components/ui/`. Each component:
- Takes a closed set of `variant` props (no open-ended style overrides beyond an escape-hatch `style` prop for layout-only tweaks like margin).
- Resolves its variant → theme values (color, border) directly from `useTheme()`, applied via inline `style` (§3 — not `cva`/className, per the theming mechanism revision above).
- Sets accessibility defaults per Req 7.2/7.3 without opt-in.
- Never varies by hue — only by fill (`ink`/`background`), border presence/weight, and typography.

### 4.1 `Text` (Req 6.1)

```ts
type TextProps = {
  variant?: TypeVariant; // default 'body' — see §2.2, includes 'itemTitle' (serif)
  tone?: 'primary' | 'secondary' | 'tertiary'; // maps to ink/inkSecondary/inkTertiary — default 'primary'
  voice?: 'user' | 'ai'; // 'ai' renders italic (Req 1.5) — default 'user'
} & RNTextProps;
```
`allowFontScaling` left enabled (default) to satisfy Req 2.4.

### 4.2 `Button` (Req 6.2)

```ts
type ButtonProps = {
  variant?: 'solid' | 'outline' | 'ghost'; // default 'solid'
  size?: 'sm' | 'md'; // default 'md' — meets the 44/48pt touch target (Req 7.3)
  disabled?: boolean;
  onPress: () => void;
  children: string;
};
```
- `solid` — `background: ink`, label rendered in `background` color (i.e. inverted fill: black button/white text in light mode, white button/black text in dark mode). The one "loud" affordance in the system, reserved for the primary action.
- `outline` — transparent fill, 1px `ink` border, `ink`-colored label.
- `ghost` — no fill, no border, `ink`-colored label only.

`accessibilityRole="button"` and `accessibilityState={{ disabled }}` set unconditionally (Req 7.2). `radius: 0` always (Req 3.3) — a solid button is a black rectangle, not a pill.

### 4.3 `Surface` / `Card` (Req 6.3)

```ts
type SurfaceProps = {
  bordered?: boolean; // default true — 1px ink border on all four edges
  padding?: keyof typeof spacing; // default 'md'
} & ViewProps;
```
`background` + optional `borderWidth`-driven border, `radius: 0` always. No elevation prop — there's nothing to vary (Req 4).

### 4.4 `TextInput` (Req 6.4)

```ts
type TextInputProps = {
  label: string; // required — becomes the default accessibilityLabel (Req 7.2)
  error?: string; // presence renders helper text below AND doubles border width (1px → 2px) — never a color change
  placeholder?: string;
} & RNTextInputProps;
```
Focus state likewise communicated by border weight (thin → thick), not color — consistent with "status via text/shape, not hue" (Req 1.4).

### 4.5 `Badge` / `Tag` (Req 6.5)

```ts
type BadgeProps = {
  variant?: 'solid' | 'outline'; // default 'outline'
  children: string; // the label carries the meaning — e.g. "STALE", "BROKEN", "INSIGHT"
};
```
No `tone`/color prop exists — status is the text itself plus `solid` (filled, high-emphasis) vs `outline` (bordered, low-emphasis). Consumers (e.g. `SourceHealthRenderer`'s `stale`/`broken`, a Highlight's reaction type) choose the *word* and the emphasis level, never a hue (Req 6.5).

### 4.6 `Avatar` (Req 6.6)

```ts
type AvatarProps = {
  name: string; // used to derive initials fallback
  imageUrl?: string;
  size?: 'sm' | 'md' | 'lg'; // default 'md'
};
```
**Square**, not circular (see §2.3's radius note). Falls back to initials in `ink` on `background` with a 1px `ink` border when `imageUrl` is absent or fails to load.

---

## 5. Directory Layout ✅

```
app/src/
  theme/
    tokens/
      colors.ts
      typography.ts
      spacing.ts
      radii.ts
      border.ts
      index.ts            # re-exports everything
    theme-provider.tsx     # ThemeProvider + useTheme()
  components/
    ui/
      text.tsx
      button.tsx
      surface.tsx
      text-input.tsx
      badge.tsx
      avatar.tsx
      index.ts
tailwind.config.js          # imports theme/tokens/*, registers light/dark as CSS vars
global.css                  # existing Tailwind directives (currently unused — gets wired up)
```

---

## 6. Accessibility Baseline ✅ (Req 7)

- **Contrast (7.1):** `ink`/`background` pairs are pure black/white — 21:1, maximum possible contrast, trivially clears AA and AAA. `inkSecondary` clears AA. `inkTertiary` is AA-Large only (~4.47:1 in light mode) — accepted, restricted to ≥18pt/14pt-bold usage (§2.1).
- **Labels/roles (7.2):** built into `Button` and `TextInput` unconditionally, as specified in §4.2/§4.4 — not opt-in props.
- **Touch targets (7.3):** `Button`'s `md` size and any tappable `Badge`/`Avatar` wrapper are sized to clear 44pt/48dp via a fixed `minHeight`/`minWidth` in each component's base style, not left to call-site padding choices.

---

## 7. Open Questions 🔄

- **NativeWind v4 dark-mode wiring specifics** — CSS-variable-based theme registration in `tailwind.config.js` needs to be verified against whatever NativeWind version lands in `package.json` at implementation time; API surface for this has moved across versions.
- **Motion language** — out of scope per requirements, but directional intent is captured: fluid, high-contrast, sparse motion, referenced against the black-and-white *Samurai Jack* episode. Not designed here.
- **Icon library** — deferred per requirements; none of the components above define an icon slot API yet.
- **In-app light/dark override** — current design is system-appearance-only; whether Marrow ever needs a manual override toggle is unresolved.
