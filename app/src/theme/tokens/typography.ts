// Single family across the whole app — Smooch Sans (Google Fonts), loaded
// via @expo-google-fonts/smooch-sans in the root layout before first render.
// Unlike Tulpen One/Unica One, this ships real weight files, so each
// typeScale entry below maps to its own font file rather than relying on
// synthetic/faux bold.
export const fontFamily = {
  '400': 'SmoochSans_400Regular',
  '500': 'SmoochSans_500Medium',
  '600': 'SmoochSans_600SemiBold',
  '700': 'SmoochSans_700Bold',
} as const;

export const typeScale = {
  caption: { fontSize: 15, lineHeight: 20, fontWeight: '400' },
  label: { fontSize: 17, lineHeight: 24, fontWeight: '500' },
  body: { fontSize: 24, lineHeight: 30, fontWeight: '400' },
  bodyLarge: { fontSize: 22, lineHeight: 34, fontWeight: '400' },
  heading3: { fontSize: 24, lineHeight: 31, fontWeight: '600' },
  heading2: { fontSize: 30, lineHeight: 37, fontWeight: '600' },
  heading1: { fontSize: 40, lineHeight: 46, fontWeight: '700' },
  // The one distinct use-case — FeedItem/ContentItem titles only.
  itemTitle: { fontSize: 28, lineHeight: 34, fontWeight: '400' },
} as const;

export type TypeVariant = keyof typeof typeScale;
