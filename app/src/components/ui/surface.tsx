import { View, type ViewProps } from 'react-native';

import { useTheme } from '@/theme/theme-provider';

export type SurfaceProps = {
  bordered?: boolean;
  padding?: 'none' | 'xs' | 'sm' | 'md' | 'lg' | 'xl' | 'xxl';
} & ViewProps;

export function Surface({ bordered = true, padding = 'md', style, ...props }: SurfaceProps) {
  const theme = useTheme();
  const paddingValue = padding === 'none' ? 0 : theme.spacing[padding];

  return (
    <View
      style={[
        {
          backgroundColor: theme.colors.background,
          borderWidth: bordered ? theme.borderWidth : 0,
          borderColor: theme.colors.ink,
          borderRadius: theme.radius,
          padding: paddingValue,
        },
        style,
      ]}
      {...props}
    />
  );
}
