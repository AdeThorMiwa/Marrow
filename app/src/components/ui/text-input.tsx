import { useState } from 'react';
import { TextInput as RNTextInput, View, type TextInputProps as RNTextInputProps } from 'react-native';

import { useTheme } from '@/theme/theme-provider';

import { Text } from './text';

export type TextInputProps = {
  label: string;
  error?: string;
} & RNTextInputProps;

export function TextInput({ label, error, style, onFocus, onBlur, ...props }: TextInputProps) {
  const theme = useTheme();
  const [focused, setFocused] = useState(false);

  // Focus/error communicated by border weight, never color (Req 1.4).
  const emphasized = focused || !!error;

  return (
    <View style={{ gap: theme.spacing.xs }}>
      <Text variant="label" tone="secondary">
        {label}
      </Text>
      <RNTextInput
        accessibilityLabel={label}
        placeholderTextColor={theme.colors.inkSecondary}
        onFocus={(e) => {
          setFocused(true);
          onFocus?.(e);
        }}
        onBlur={(e) => {
          setFocused(false);
          onBlur?.(e);
        }}
        style={[
          {
            minHeight: 44,
            borderWidth: emphasized ? theme.borderWidthError : theme.borderWidth,
            borderColor: theme.colors.ink,
            borderRadius: theme.radius,
            paddingHorizontal: theme.spacing.md,
            paddingVertical: theme.spacing.sm,
            fontSize: theme.typeScale.body.fontSize,
            lineHeight: theme.typeScale.body.lineHeight,
            fontFamily: theme.fontFamily['400'],
            color: theme.colors.ink,
          },
          style,
        ]}
        {...props}
      />
      {error ? (
        <Text variant="caption" tone="secondary">
          {error}
        </Text>
      ) : null}
    </View>
  );
}
