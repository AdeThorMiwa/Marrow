import RNMarkdown from 'react-native-markdown-display';

import { useTheme } from '@/theme/theme-provider';

export type MarkdownProps = {
  children: string;
  tone?: 'primary' | 'secondary';
  size?: 'body' | 'small';
};

const toneKey = { primary: 'ink', secondary: 'inkSecondary' } as const;

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
        // Full-width block, not an inline thumbnail — including when the
        // image is itself wrapped in a link (a bare `![alt](url)` render
        // the same as a linked one, react-native-markdown-display treats
        // both as an `image` node either way).
        image: {
          width: '100%',
          height: 220,
          borderRadius: theme.radius,
          marginVertical: theme.spacing.sm,
        },
      }}>
      {children}
    </RNMarkdown>
  );
}
