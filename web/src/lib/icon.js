// Tile colours. `auto` picks one from the name so every entry gets a stable colour.
export const palette = {
  blue: 200,
  green: 140,
  purple: 260,
  orange: 20,
  pink: 330,
  yellow: 45,
};
const autoHues = [200, 160, 260, 20, 330, 120, 40, 290];

export function iconHue(label, icon) {
  if (icon?.color && palette[icon.color] !== undefined) return palette[icon.color];
  let h = 0;
  for (const c of label) h = (h * 31 + c.charCodeAt(0)) >>> 0;
  return autoHues[h % autoHues.length];
}

const segmenter = typeof Intl !== 'undefined' && Intl.Segmenter ? new Intl.Segmenter(undefined, { granularity: 'grapheme' }) : null;

/** Split into user-visible characters (so an emoji counts as one). */
export function graphemes(s) {
  return segmenter ? Array.from(segmenter.segment(s), (x) => x.segment) : Array.from(s);
}

export function firstLetter(label) {
  return (graphemes(label.trim())[0] ?? '?').toUpperCase();
}

export const ICON_SIZE = 64;

/**
 * Shrinks an uploaded image (PNG, ICO, JPG, WEBP, SVG, GIF…) to fit 64×64,
 * centred on a transparent canvas, and returns a PNG data URL. Uses a data
 * URL rather than a blob URL so it works under the page's CSP.
 */
export async function shrinkImage(file) {
  if (file.size > 5 * 1024 * 1024) throw new Error('That image is too large (5 MB max).');
  const src = await new Promise((resolve, reject) => {
    const r = new FileReader();
    r.onload = () => resolve(r.result);
    r.onerror = () => reject(new Error('Could not read the file.'));
    r.readAsDataURL(file);
  });
  const img = new Image();
  img.src = src;
  try {
    await img.decode();
  } catch {
    throw new Error("That file isn't an image this browser can read.");
  }

  let w = img.naturalWidth || ICON_SIZE;
  let h = img.naturalHeight || ICON_SIZE;
  const scale = Math.min(1, ICON_SIZE / Math.max(w, h));
  const tw = Math.max(1, Math.round(w * scale));
  const th = Math.max(1, Math.round(h * scale));

  // Halve step by step for big images: much smoother than one big downscale.
  let source = img;
  while (w / 2 >= tw * 2 && h / 2 >= th * 2) {
    w = Math.round(w / 2);
    h = Math.round(h / 2);
    const step = document.createElement('canvas');
    step.width = w;
    step.height = h;
    const sctx = step.getContext('2d');
    sctx.imageSmoothingQuality = 'high';
    sctx.drawImage(source, 0, 0, w, h);
    source = step;
  }

  const canvas = document.createElement('canvas');
  canvas.width = ICON_SIZE;
  canvas.height = ICON_SIZE;
  const ctx = canvas.getContext('2d');
  ctx.imageSmoothingQuality = 'high';
  ctx.drawImage(source, Math.round((ICON_SIZE - tw) / 2), Math.round((ICON_SIZE - th) / 2), tw, th);
  return canvas.toDataURL('image/png');
}
