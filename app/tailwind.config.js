/** @type {import('tailwindcss').Config} */
module.exports = {
  darkMode: 'media', // system-appearance-driven, no manual toggle (Req 5.2)
  content: ['./src/**/*.{js,jsx,ts,tsx}'],
  presets: [require('nativewind/preset')],
  theme: {
    extend: {
      // Color is NOT registered here. It's applied via `useTheme()` + inline
      // `style` in every primitive instead of `dark:`-prefixed className —
      // this sidesteps NativeWind's CSS-variable dark-mode wiring (still an
      // open question per docs/design-system/design.md §7) and keeps color
      // resolution in one place (theme/tokens/colors.js) with no risk of a
      // className/style mismatch at runtime.
      borderRadius: {
        DEFAULT: '0px',
        none: '0px',
      },
      spacing: {
        xs: '4px',
        sm: '8px',
        md: '16px',
        lg: '24px',
        xl: '32px',
        xxl: '48px',
      },
    },
  },
  plugins: [],
};
