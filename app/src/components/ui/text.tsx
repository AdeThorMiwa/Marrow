import { Text as RNText, type TextProps as RNTextProps, type TextStyle } from 'react-native';

import { useTheme } from '@/theme/theme-provider';
import type { TypeVariant } from '@/theme/tokens';

export type TextProps = {
  variant?: TypeVariant;
  tone?: 'primary' | 'secondary' | 'tertiary';
  voice?: 'user' | 'ai'; // 'ai' renders italic (Req 1.5) — never color, color is unavailable
} & RNTextProps;

const toneKey = {
  primary: 'ink',
  secondary: 'inkSecondary',
  tertiary: 'inkTertiary',
} as const;

export function Text({ variant = 'body', tone = 'primary', voice = 'user', style, ...props }: TextProps) {
  const theme = useTheme();
  const scale = theme.typeScale[variant];
  const family = theme.fontFamily[scale.family as 'sans' | 'voice'];

  const resolvedStyle: TextStyle = {
    fontSize: scale.fontSize,
    lineHeight: scale.lineHeight,
    fontWeight: scale.fontWeight as TextStyle['fontWeight'],
    fontFamily: family,
    fontStyle: voice === 'ai' ? 'italic' : 'normal',
    color: theme.colors[toneKey[tone]],
  };

  return <RNText allowFontScaling style={[resolvedStyle, style]} {...props} />;
}
