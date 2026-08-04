import { Platform } from 'react-native';

// Two families only — serif reserved exclusively for item titles, sans for
// everything else. See docs/design-system/design.md §2.2.
export const fontFamily = {
  voice: Platform.select({
    ios: 'ui-serif',
    android: 'serif',
    web: 'Georgia, "Times New Roman", serif',
    default: 'serif',
  }) as string, // item titles only
  sans: Platform.select({
    ios: 'system-ui',
    android: 'sans-serif',
    web: 'system-ui, sans-serif',
    default: 'sans-serif',
  }) as string, // everything else
};

export const typeScale = {
  caption: { fontSize: 12, lineHeight: 16, fontWeight: '400', family: 'sans' },
  label: { fontSize: 14, lineHeight: 20, fontWeight: '500', family: 'sans' },
  body: { fontSize: 16, lineHeight: 26, fontWeight: '400', family: 'sans' },
  bodyLarge: { fontSize: 18, lineHeight: 29, fontWeight: '400', family: 'sans' },
  heading3: { fontSize: 20, lineHeight: 26, fontWeight: '600', family: 'sans' },
  heading2: { fontSize: 24, lineHeight: 30, fontWeight: '600', family: 'sans' },
  heading1: { fontSize: 32, lineHeight: 38, fontWeight: '700', family: 'sans' },
  // The one serif use-case — FeedItem/ContentItem titles only.
  itemTitle: { fontSize: 20, lineHeight: 27, fontWeight: '400', family: 'voice' },
} as const;

export type TypeVariant = keyof typeof typeScale;
