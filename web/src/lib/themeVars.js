// Pure theme maths (no Svelte, no DOM) so a plain Node script can check it against app.css.

/** The one brand constant: app.css derives its default accent, backgrounds and borders from this hue (scripts/check-theme.mjs keeps them in sync). */
export const DEFAULT_HUE = 201;

/** Pull yellow/green hues darker so white text on the accent stays readable. */
const bump = (h) => Math.max(0, 1 - Math.abs(h - 100) / 75);
const hsl = (h, s, l) => `hsl(${(h + 360) % 360} ${s}% ${l}%)`;

function hslToRgb(h, s, l) {
  const sa = s / 100;
  const li = l / 100;
  const k = (n) => (n + h / 30) % 12;
  const a = sa * Math.min(li, 1 - li);
  const f = (n) => li - a * Math.max(-1, Math.min(k(n) - 3, Math.min(9 - k(n), 1)));
  return [f(0), f(8), f(4)];
}

/** WCAG contrast of white text on hsl(h s l). */
export function contrastWithWhite(h, s, l) {
  const lin = (c) => (c <= 0.04045 ? c / 12.92 : ((c + 0.055) / 1.055) ** 2.4);
  const [r, g, b] = hslToRgb(((h % 360) + 360) % 360, s, l).map(lin);
  return 1.05 / (0.2126 * r + 0.7152 * g + 0.0722 * b + 0.05);
}

const luminance = (h, s, l) => {
  const lin = (c) => (c <= 0.04045 ? c / 12.92 : ((c + 0.055) / 1.055) ** 2.4);
  const [r, g, b] = hslToRgb(((h % 360) + 360) % 360, s, l).map(lin);
  return 0.2126 * r + 0.7152 * g + 0.0722 * b;
};

/** WCAG contrast between two [h, s, l] colours. */
export function contrastBetween(a, b) {
  const [x, y] = [luminance(...a), luminance(...b)].sort((p, q) => q - p);
  return (x + 0.05) / (y + 0.05);
}

/** Like fitWhite, but for text/links on known backgrounds: moves L by step (-1 darkens, +1 lightens) until every [h, s, l] bg in bgs is at AA. */
function fitOn(h, s, l, bgs, step = -1, min = 4.6) {
  let li = l;
  while (li > 10 && li < 95 && bgs.some((bg) => contrastBetween([h, s, li], bg) < min)) li += step;
  return hsl(h, s, li);
}

/** hsl() that keeps white text at AA (4.5:1, with a little margin for 8-bit rounding) by darkening only as far as needed. */
function fitWhite(h, s, l, min = 4.6) {
  let li = l;
  while (li > 10 && contrastWithWhite(h, s, li) < min) li -= 1;
  return hsl(h, s, li);
}

/**
 * Every custom property the hue controls, for the light or dark palette. */
export function themeVars(h, dark) {
  const b = bump(h);
  return dark
    ? {
        // Dark palette: fills that carry white text (topbar, active items) are fitted to AA.
        '--accent-glow': fitWhite(h - 5, 75, 44 - 10 * b),
        '--accent-hi': fitWhite(h - 7, 60, 30 - 6 * b),
        '--accent-lo': hsl(h + 3, 80, 12),
        '--bar-a': fitWhite(h, 70, 38 - 8 * b),
        '--bar-b': fitWhite(h - 8, 75, 44 - 10 * b),
        '--accent-fill': fitWhite(h, 74, 52 - 17 * b),
        '--accent-fill-hover': fitWhite(h, 74, 36 - 12 * b),
        '--accent': fitOn(h, 74, 52 - 17 * b, [[h, 28, 8], [h, 24, 11], [h, 24, 14], [h + 3, 55, 15]], 1),
        '--accent-strong': hsl(h, 80, 61 - 12 * b),
        '--accent-soft': hsl(h + 3, 55, 15),
        '--bg': hsl(h, 28, 8),
        '--surface': hsl(h, 24, 11),
        '--surface-2': hsl(h, 24, 14),
        '--sidebar': hsl(h, 26, 10),
        '--hover': hsl(h, 26, 17),
        '--border': hsl(h, 22, 20),
        '--border-strong': hsl(h, 20, 29),
      }
    : {
        '--accent-glow': hsl(h - 5, 80, 52 - 14 * b),
        // only used as a glow behind the white login brand text, so fitted to AA like the fills
        '--accent-hi': fitWhite(h - 7, 87, 59 - 16 * b),
        '--accent-lo': hsl(h + 3, 100, 20 - 4 * b),
        // Fills that carry white text: the accent, darkened only as far as AA needs.
        '--bar-a': fitWhite(h, 100, 39 - 12 * b),
        '--bar-b': fitWhite(h - 3, 96, 41 - 13 * b),
        '--accent-fill': fitWhite(h, 100, 39 - 12 * b),
        '--accent-fill-hover': fitWhite(h, 100, 32 - 9 * b),
        // Links/icons sit on --bg, --surface, --surface-2 and --accent-soft: fit against all of them.
        '--accent': fitOn(h, 100, 39 - 12 * b, [[h, 42, 96], [h, 40, 99], [h, 38, 97], [h, 69, 95]]),
        '--accent-strong': hsl(h, 100, 32 - 9 * b),
        '--accent-soft': hsl(h, 69, 95),
        '--bg': hsl(h, 42, 96),
        '--surface': hsl(h, 40, 99),
        '--surface-2': hsl(h, 38, 97),
        '--sidebar': hsl(h, 50, 95),
        '--hover': hsl(h, 45, 93),
        '--border': hsl(h, 26, 88),
        '--border-strong': hsl(h, 22, 78),
      };
}

export const VAR_NAMES = Object.keys(themeVars(0, false));
