import { Pressable, type PressableProps } from 'react-native';

import { useTheme } from '@/theme/theme-provider';

import { Text } from './text';

export type ButtonProps = {
  variant?: 'solid' | 'outline' | 'ghost';
  size?: 'sm' | 'md';
  disabled?: boolean;
  onPress: () => void;
  children: string;
} & Omit<PressableProps, 'onPress' | 'children' | 'style' | 'disabled'>;

const MIN_TOUCH_TARGET = 44; // iOS 44pt / Android 48dp minimum — md clears both directly

export function Button({ variant = 'solid', size = 'md', disabled, onPress, children, ...props }: ButtonProps) {
  const theme = useTheme();

  const backgroundColor = variant === 'solid' ? theme.colors.ink : 'transparent';
  const borderColor = variant === 'ghost' ? 'transparent' : theme.colors.ink;
  const textColor = variant === 'solid' ? theme.colors.background : theme.colors.ink;

  return (
    <Pressable
      accessibilityRole="button"
      accessibilityState={{ disabled: !!disabled }}
      disabled={disabled}
      onPress={onPress}
      hitSlop={size === 'sm' ? 8 : undefined}
      style={({ pressed }) => ({
        minHeight: size === 'md' ? MIN_TOUCH_TARGET : 36,
        minWidth: size === 'md' ? MIN_TOUCH_TARGET : undefined,
        paddingHorizontal: size === 'md' ? theme.spacing.lg : theme.spacing.md,
        justifyContent: 'center',
        alignItems: 'center',
        backgroundColor,
        borderWidth: variant === 'ghost' ? 0 : theme.borderWidth,
        borderColor,
        borderRadius: theme.radius,
        opacity: disabled ? 0.4 : pressed ? 0.6 : 1,
      })}
      {...props}>
      <Text variant="label" style={{ color: textColor }}>
        {children}
      </Text>
    </Pressable>
  );
}
