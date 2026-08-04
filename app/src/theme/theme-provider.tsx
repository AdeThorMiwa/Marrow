import { createContext, useContext, useMemo, type ReactNode } from 'react';
import { useColorScheme } from 'react-native';

import {
  borderWidth,
  borderWidthError,
  colors,
  fontFamily,
  maxContentWidth,
  radius,
  spacing,
  typeScale,
} from './tokens';

type ColorScheme = 'light' | 'dark';

type Theme = {
  colorScheme: ColorScheme;
  colors: typeof colors.light;
  typeScale: typeof typeScale;
  fontFamily: typeof fontFamily;
  spacing: typeof spacing;
  maxContentWidth: number;
  radius: number;
  borderWidth: number;
  borderWidthError: number;
};

const ThemeContext = createContext<Theme | null>(null);

// System-appearance-only (Req 5.2) — no manual light/dark toggle exists yet.
// This provider is still a single override point should one be added later,
// and a stable place to memoize the derived theme value.
export function ThemeProvider({ children }: { children: ReactNode }) {
  const scheme = useColorScheme();
  const colorScheme: ColorScheme = scheme === 'dark' ? 'dark' : 'light';

  const value = useMemo<Theme>(
    () => ({
      colorScheme,
      colors: colors[colorScheme],
      typeScale,
      fontFamily,
      spacing,
      maxContentWidth,
      radius,
      borderWidth,
      borderWidthError,
    }),
    [colorScheme]
  );

  return <ThemeContext.Provider value={value}>{children}</ThemeContext.Provider>;
}

export function useTheme(): Theme {
  const ctx = useContext(ThemeContext);
  if (!ctx) {
    throw new Error('useTheme must be used within a ThemeProvider');
  }
  return ctx;
}
