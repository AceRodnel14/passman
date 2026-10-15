export async function copyText(text) {
  if (navigator.clipboard && window.isSecureContext) {
    try {
      await navigator.clipboard.writeText(text);
      return true;
    } catch {
      /* fall through */
    }
  }
  // Fallback for plain-HTTP deployments where the Clipboard API is unavailable.
  const ta = document.createElement('textarea');
  ta.value = text;
  ta.setAttribute('readonly', '');
  ta.style.position = 'fixed';
  ta.style.opacity = '0';
  document.body.appendChild(ta);
  ta.select();
  let ok = false;
  try {
    ok = document.execCommand('copy');
  } catch {
    ok = false;
  }
  ta.remove();
  return ok;
}

export function relativeTime(date, now = Date.now()) {
  const s = Math.max(0, (now - new Date(date).getTime()) / 1000);
  const m = s / 60, h = m / 60, d = h / 24;
  if (s < 45) return 'a few seconds ago';
  if (s < 90) return 'a minute ago';
  if (m < 45) return `${Math.round(m)} minutes ago`;
  if (m < 90) return 'an hour ago';
  if (h < 22) return `${Math.round(h)} hours ago`;
  if (h < 36) return 'a day ago';
  if (d < 26) return `${Math.round(d)} days ago`;
  if (d < 45) return 'a month ago';
  if (d < 320) return `${Math.round(d / 30.44)} months ago`;
  if (d < 548) return 'a year ago';
  return `${Math.round(d / 365.25)} years ago`;
}

export function plural(n, one, many) {
  return `${n} ${n === 1 ? one : many}`;
}

export const healthText = {
  red: 'Very weak password',
  orange: 'Username can be guessed from the password',
  yellow: 'Duplicate password',
  green: 'OK — unique password',
};

export function storageGet(key, fallback) {
  try {
    const v = localStorage.getItem(key);
    return v == null ? fallback : JSON.parse(v);
  } catch {
    return fallback;
  }
}

export function storageSet(key, value) {
  try {
    localStorage.setItem(key, JSON.stringify(value));
  } catch {
    /* ignore */
  }
}
