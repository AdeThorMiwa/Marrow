// colors/spacing/radii/border are plain .js (required directly by
// tailwind.config.js's CommonJS loader) — re-exported here so app code has
// one import path for the whole token layer regardless of file extension.
import { light, dark } from './colors';
import { spacing, maxContentWidth } from './spacing';
import { radius } from './radii';
import { borderWidth, borderWidthError } from './border';

export { fontFamily, typeScale } from './typography';
export type { TypeVariant } from './typography';

export const colors = { light, dark };
export { spacing, maxContentWidth, radius, borderWidth, borderWidthError };

export type ColorToken = keyof typeof light & keyof typeof dark;
