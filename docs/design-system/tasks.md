# Design System — Implementation Tasks

> Implements `docs/design-system/design.md`. Complete top-to-bottom — later tasks depend on earlier ones. After each component task, run the app on web (fastest iteration loop) to visually verify before moving on, per `CLAUDE.md`'s "start the dev server and use the feature" guidance.

- [ ] 1. Remove bootstrap scaffolding
  - Delete `app/src/constants/theme.ts`, `themed-text.tsx`, `themed-view.tsx`, `hooks/use-theme.ts`, `hooks/use-color-scheme.ts`, `hooks/use-color-scheme.web.ts`, and the existing `components/ui/*` files
  - Remove any imports of the above from `app/src/app/*` (will be temporarily broken until task 8 rewires them — acceptable mid-task state)
  - _Design §1_

- [ ] 2. Install dependencies and wire NativeWind
  - `npm install nativewind tailwindcss class-variance-authority tailwind-merge` in `app/`
  - Configure `tailwind.config.js`, `babel.config.js`/`metro.config.js` per NativeWind v4 setup for Expo Router
  - Verify `app/global.css` Tailwind directives are actually applied (a utility class renders visibly on all three targets: iOS simulator, Android emulator, web)
  - _Design §1_

- [ ] 3. Token layer: color, typography, spacing, radii, border
  - `theme/tokens/colors.ts` — `light`/`dark` grayscale palettes (Design §2.1)
  - `theme/tokens/typography.ts` — `fontFamily` (voice/sans), `typeScale` including `itemTitle` (Design §2.2)
  - `theme/tokens/spacing.ts` — `spacing`, `maxContentWidth` (Design §2.3)
  - `theme/tokens/radii.ts` — `radius = 0` (Design §2.3)
  - `theme/tokens/border.ts` — `borderWidth = 1` (Design §2.4)
  - `theme/tokens/index.ts` — re-export all of the above
  - _Requirements 1, 2, 3, 4 — Design §2_

- [ ] 4. Register tokens in Tailwind + `useTheme()` hook
  - `tailwind.config.js` imports `theme/tokens/colors.ts`, registers `light`/`dark` as CSS-variable-backed theme colors with `dark:` variant support
  - `theme/theme-provider.tsx` — `ThemeProvider` + `useTheme()` reading `useColorScheme()`, returning `{ colors, typeScale, fontFamily, spacing, radius, borderWidth }`
  - Wrap app root (`app/src/app/_layout.tsx`) in `ThemeProvider`
  - Verify: toggling OS appearance (simulator/browser) flips the app between light/dark with no restart
  - _Requirement 5 — Design §3_

- [ ] 5. `Text` component
  - `components/ui/text.tsx` — `variant` (incl. `itemTitle` → serif), `tone` (primary/secondary/tertiary), `voice` (user/ai → italic), `allowFontScaling` on
  - _Requirement 6.1, 2.4 — Design §4.1_

- [ ] 6. `Button` component
  - `components/ui/button.tsx` — `solid`/`outline`/`ghost` variants via `cva`, `sm`/`md` sizes, `accessibilityRole`/`accessibilityState` set unconditionally, `radius: 0`
  - _Requirement 6.2, 7.2, 7.3 — Design §4.2_

- [ ] 7. `Surface` component
  - `components/ui/surface.tsx` — `bordered` (default true, 1px ink border), `padding`, `radius: 0`, no elevation prop
  - _Requirement 6.3 — Design §4.3_

- [ ] 8. `TextInput` component
  - `components/ui/text-input.tsx` — `label` (default accessibilityLabel), `error` (helper text + border weight 1px→2px, never color), `placeholder`
  - _Requirement 6.4, 7.2 — Design §4.4_

- [ ] 9. `Badge` component
  - `components/ui/badge.tsx` — `solid`/`outline` variants, text-only status (no `tone`/color prop)
  - _Requirement 6.5 — Design §4.5_

- [ ] 10. `Avatar` component
  - `components/ui/avatar.tsx` — square (not circular), `sm`/`md`/`lg` sizes, initials fallback (ink-on-background, 1px border) when `imageUrl` absent/fails
  - _Requirement 6.6 — Design §4.6_

- [ ] 11. `components/ui/index.ts` barrel export
  - Re-export all six primitives
  - _Design §5_

- [ ] 12. Rewire existing screens to the new primitives
  - Update `app/src/app/index.tsx`, `explore.tsx`, `_layout.tsx`, and any component still importing the deleted bootstrap files (task 1) to use the new `components/ui/*` instead
  - App must build and run on iOS, Android, and Web with no references to deleted files
  - _Design §1_

- [ ] 13. Cross-platform + accessibility verification pass
  - Run on iOS simulator, Android emulator, and web (`npm run ios` / `android` / `web`); confirm every component in tasks 5–10 renders identically (modulo platform font rendering) with no rounded corners, no shadows, no color anywhere but pure black/white
  - Confirm `inkTertiary` text usage sites are all ≥18pt/14pt-bold (per the accepted AA-Large-only contrast tradeoff, Design §2.1)
  - Confirm touch targets on `Button`/interactive `Badge`/`Avatar` meet 44×44pt (iOS) / 48×48dp (Android)
  - Confirm Dynamic Type / Android font scale actually resizes rendered text
  - _Requirements 6.7, 7 — Design §6_
