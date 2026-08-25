import { useEffect, useState } from 'react';
import { Image } from 'react-native';

import RNMarkdown from 'react-native-markdown-display';

import { useTheme } from '@/theme/theme-provider';

export type MarkdownProps = {
  children: string;
  tone?: 'primary' | 'secondary';
  size?: 'body' | 'small';
};

const toneKey = { primary: 'ink', secondary: 'inkSecondary' } as const;

// react-native-markdown-display's own `image` rule spreads a props object
// that includes `key` into <FitImage {...props}>, which React now rejects
// ("keys must be passed directly to JSX, not via spread") — a real crash on
// every image.
//
// Also, FitImage itself (which we tried using directly, key passed
// correctly) turned out unreliable on web specifically: it computes height
// from an onLayout-measured width + a separately-fetched natural size, both
// arriving after the first paint, so the box has no real height reserved
// yet and surrounding text renders through/over it before the async
// callbacks land. RN's `aspectRatio` style property doesn't have this
// problem — Yoga (RN's layout engine, used on web too) resolves it in the
// same layout pass as everything else, so the box is correctly sized from
// the first paint. AspectImage below fetches the natural size once via
// Image.getSize and renders nothing until it's known, rather than briefly
// occupying the wrong size.
function AspectImage({ src, alt, style }: { src: string; alt?: string; style: any }) {
  const [ratio, setRatio] = useState<number | null>(null);

  useEffect(() => {
    let ignore = false;
    Image.getSize(
      src,
      (w, h) => {
        if (!ignore) setRatio(h > 0 ? w / h : null);
      },
      () => {
        if (!ignore) setRatio(null); // failed to fetch size — render nothing rather than a broken box
      },
    );
    return () => {
      ignore = true;
    };
  }, [src]);

  if (!ratio) return null;

  return (
    <Image
      source={{ uri: src }}
      style={[style, { width: '100%', aspectRatio: ratio }]}
      accessible={!!alt}
      accessibilityLabel={alt || undefined}
    />
  );
}

const markdownRules = {
  image: (node: any, _children: any, _parent: any, styles: any) => {
    const { src, alt } = node.attributes;
    return <AspectImage key={node.key} src={src} alt={alt} style={styles.image} />;
  },
};

// Thin themed wrapper over react-native-markdown-display — maps every rule
// back to the token layer so rendered Markdown never introduces a color,
// font, or weight outside the design system (see @/components/ui/text.tsx).
export function Markdown({ children, tone = 'primary', size = 'body' }: MarkdownProps) {
  const theme = useTheme();
  const color = theme.colors[toneKey[tone]];
  const body = theme.typeScale.body;
  // 'small' isn't a named type-scale step — a feed summary needs to read as
  // secondary to the title without dropping all the way to caption size.
  const fontSize = size === 'small' ? body.fontSize - 5 : body.fontSize;
  const lineHeight = size === 'small' ? body.lineHeight - 4 : body.lineHeight;

  const textBase = {
    color,
    fontFamily: theme.fontFamily['400'],
    fontSize,
    lineHeight,
  };
  const semiBold = { fontFamily: theme.fontFamily['600'], fontWeight: '600' as const };
  const bold = { fontFamily: theme.fontFamily['700'], fontWeight: '700' as const };

  return (
    <RNMarkdown
      rules={markdownRules}
      style={{
        body: { ...textBase, margin: 0 },
        paragraph: { marginTop: 0, marginBottom: theme.spacing.md },
        strong: bold,
        em: { fontStyle: 'italic' },
        // Links get underline, never a distinct color — this design system
        // communicates state through weight/decoration, not color (Req 4.1).
        link: { color, textDecorationLine: 'underline' },
        heading1: { ...textBase, ...semiBold, fontSize: theme.typeScale.heading2.fontSize, marginBottom: theme.spacing.sm },
        heading2: { ...textBase, ...semiBold, fontSize: theme.typeScale.heading3.fontSize, marginBottom: theme.spacing.sm },
        heading3: { ...textBase, ...semiBold, fontSize: theme.typeScale.heading3.fontSize, marginBottom: theme.spacing.sm },
        heading4: { ...textBase, ...semiBold, marginBottom: theme.spacing.sm },
        heading5: { ...textBase, ...semiBold, marginBottom: theme.spacing.sm },
        heading6: { ...textBase, ...semiBold, marginBottom: theme.spacing.sm },
        bullet_list: { marginBottom: theme.spacing.sm },
        ordered_list: { marginBottom: theme.spacing.sm },
        list_item: { ...textBase },
        bullet_list_icon: { color },
        blockquote: {
          // react-native-markdown-display's own style merge is shallow
          // per-property (index.js's getStyle) — any key we don't set here
          // falls through to the library's default, which is
          // backgroundColor: '#F5F5F5'. Same reason code_inline/code_block/
          // fence below already set backgroundColor explicitly.
          backgroundColor: 'transparent',
          borderLeftWidth: theme.hairlineWidth,
          borderLeftColor: color,
          paddingLeft: theme.spacing.sm,
          marginVertical: theme.spacing.sm,
        },
        code_inline: {
          fontFamily: 'monospace',
          backgroundColor: 'transparent',
          borderWidth: theme.hairlineWidth,
          borderColor: color,
          borderRadius: theme.radius,
          paddingHorizontal: theme.spacing.xs,
        },
        code_block: {
          fontFamily: 'monospace',
          backgroundColor: 'transparent',
          borderWidth: theme.hairlineWidth,
          borderColor: color,
          borderRadius: theme.radius,
          padding: theme.spacing.sm,
        },
        fence: {
          fontFamily: 'monospace',
          backgroundColor: 'transparent',
          borderWidth: theme.hairlineWidth,
          borderColor: color,
          borderRadius: theme.radius,
          padding: theme.spacing.sm,
        },
        hr: { backgroundColor: theme.colors.divider, height: theme.hairlineWidth },
        // A link whose only content is an image gets promoted to a
        // `blocklink` node (react-native-markdown-display's cleanupTokens.js)
        // instead of `image` — its default style has a hardcoded black
        // bottom border we'd otherwise inherit verbatim in dark mode too.
        blocklink: { borderBottomWidth: theme.hairlineWidth, borderColor: theme.colors.divider },
        table: { borderWidth: theme.hairlineWidth, borderColor: theme.colors.divider, borderRadius: theme.radius },
        tr: { borderBottomWidth: theme.hairlineWidth, borderColor: theme.colors.divider },
        // No width/height here — AspectImage (above) sets width: 100% and
        // a computed aspectRatio itself, so images render at their real
        // proportions instead of being cropped into a fixed box.
        image: {
          borderRadius: theme.radius,
          marginVertical: theme.spacing.sm,
        },
      }}>
      {children}
    </RNMarkdown>
  );
}
