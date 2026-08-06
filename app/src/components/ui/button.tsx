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

  // Static styling is computed here, not inside Pressable's style-callback —
  // on at least one real-device/Expo-Go combination, a Pressable's
  // function-valued `style` prop was observed silently failing to apply
  // (button stayed hit-testable but fully invisible: no fill, no border).
  // Only the genuinely pressed-dependent bit (opacity) stays in the callback.
  const baseStyle = {
    minHeight: size === 'md' ? MIN_TOUCH_TARGET : 36,
    minWidth: size === 'md' ? MIN_TOUCH_TARGET : undefined,
    paddingHorizontal: size === 'md' ? theme.spacing.lg : theme.spacing.md,
    justifyContent: 'center' as const,
    alignItems: 'center' as const,
    backgroundColor,
    borderWidth: variant === 'ghost' ? 0 : theme.borderWidth,
    borderColor,
    borderRadius: theme.radius,
    ...(variant === "outline" && { borderStyle: "solid" as "solid"})
  };

  return (
    <Pressable
      accessibilityRole="button"
      accessibilityState={{ disabled: !!disabled }}
      disabled={disabled}
      onPress={onPress}
      hitSlop={size === 'sm' ? 8 : undefined}
      style={({ pressed }) => ({ ...baseStyle, opacity: disabled ? 0.4 : pressed ? 0.6 : 1 })}
      {...props}>
      <Text variant="label" className='text-center' style={{ color: textColor }}>
        {children}
      </Text>
    </Pressable>
  );
}
