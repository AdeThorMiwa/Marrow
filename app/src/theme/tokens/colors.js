// Plain JS (not TS): required directly by tailwind.config.js via CommonJS
// `require()`, which cannot process TypeScript. This is the single source
// of truth for color tokens — theme-provider.tsx imports this same file.
//
// Grayscale only, strict light/dark inversion. See docs/design-system/design.md §2.1.
const light = {
  background: '#FFFFFF',
  ink: '#000000', // text, borders, buttons, icons
  inkSecondary: '#666666', // byline, meta, timestamps
  inkTertiary: '#777777', // source-health notices and similar low-emphasis text (AA-Large only — keep to >=18pt/14pt-bold)
};

const dark = {
  background: '#000000',
  ink: '#FFFFFF',
  inkSecondary: '#999999',
  inkTertiary: '#888888',
};

module.exports = { light, dark };
