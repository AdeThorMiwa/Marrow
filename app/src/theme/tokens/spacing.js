// Plain JS — see colors.js for why. Aligned to Tailwind's default 4px step
// scale so utility classes (p-4) and these semantic aliases always agree.
const spacing = { xs: 4, sm: 8, md: 16, lg: 24, xl: 32, xxl: 48 };

const maxContentWidth = 800; // Req 3.2 — readable measure on web/tablet

module.exports = { spacing, maxContentWidth };
