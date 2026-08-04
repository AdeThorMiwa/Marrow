import { View } from 'react-native';

import { useTheme } from '@/theme/theme-provider';

import { Text } from './text';

export type BadgeProps = {
  variant?: 'solid' | 'outline';
  children: string; // the label carries the meaning — e.g. "STALE", "BROKEN", "INSIGHT"
};

export function Badge({ variant = 'outline', children }: BadgeProps) {
  const theme = useTheme();

  const backgroundColor = variant === 'solid' ? theme.colors.ink : 'transparent';
  const textColor = variant === 'solid' ? theme.colors.background : theme.colors.ink;

  return (
    <View
      style={{
        alignSelf: 'flex-start',
        backgroundColor,
        borderWidth: theme.borderWidth,
        borderColor: theme.colors.ink,
        borderRadius: theme.radius,
        paddingHorizontal: theme.spacing.sm,
        paddingVertical: 2,
      }}>
      <Text variant="caption" style={{ color: textColor, textTransform: 'uppercase' }}>
        {children}
      </Text>
    </View>
  );
}
