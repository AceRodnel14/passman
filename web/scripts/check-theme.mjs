// Keeps app.css in sync with DEFAULT_HUE (no test runner needed: node:test ships with Node).
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { test } from 'node:test';
import { contrastBetween, contrastWithWhite, DEFAULT_HUE, themeVars } from '../src/lib/themeVars.js';

const css = readFileSync(new URL('../src/app.css', import.meta.url), 'utf8');
const lightEnd = css.indexOf('@media');
const darkStart = css.indexOf(':root[data-theme="dark"]');
const blocks = {
  light: css.slice(0, lightEnd),
  auto: css.slice(lightEnd, darkStart),
  dark: css.slice(darkStart, css.indexOf('* { box-sizing')),
};

test('app.css tokens are the DEFAULT_HUE values from themeVars', () => {
  const light = themeVars(DEFAULT_HUE, false);
  for (const [name, isDark] of [['light', false], ['auto', true], ['dark', true]]) {
    for (const [k, v] of Object.entries(themeVars(DEFAULT_HUE, isDark))) {
      if (isDark && v === light[k]) continue; // inherited from :root
      assert.ok(blocks[name].includes(`${k}: ${v};`), `${name}: ${k}: ${v}`);
    }
  }
});

test('hue tints backgrounds in both palettes', () => {
  assert.match(themeVars(40, false)['--bg'], /^hsl\(40 /);
  assert.match(themeVars(40, true)['--surface'], /^hsl\(40 /);
});

test('white text stays at AA on dark topbar and active-item fills for every hue, light and dark', () => {
  for (let h = 0; h < 360; h += 3) for (const dark of [false, true]) {
    const v = themeVars(h, dark);
    for (const k of (dark ? ['--bar-a', '--bar-b', '--accent-fill', '--accent-fill-hover', '--accent-glow', '--accent-hi'] : ['--bar-a', '--bar-b', '--accent-fill', '--accent-fill-hover', '--accent-hi'])) {
      const m = /^hsl\(([\d.]+) ([\d.]+)% ([\d.]+)%\)$/.exec(v[k]);
      assert.ok(m, `${k} h=${h} dark=${dark}`);
      assert.ok(contrastWithWhite(+m[1], +m[2], +m[3]) >= 4.5, `${k} h=${h} dark=${dark}`);
    }
  }
});

test('--accent is AA (4.5:1) on every background it sits on, for every hue, light and dark', () => {
  const parse = (str) => {
    const m = /^hsl\(([\d.]+) ([\d.]+)% ([\d.]+)%\)$/.exec(str);
    assert.ok(m, str);
    return [+m[1], +m[2], +m[3]];
  };
  for (let h = 0; h < 360; h++) for (const dark of [false, true]) {
    const v = themeVars(h, dark);
    for (const bg of ['--bg', '--surface', '--surface-2', '--accent-soft']) {
      const c = contrastBetween(parse(v['--accent']), parse(v[bg]));
      assert.ok(c >= 4.5, `--accent on ${bg} h=${h} dark=${dark}: ${c.toFixed(2)}`);
    }
  }
});

test('save scheduler: debounce, exit flush once, cancel', async () => {
  const { createSaveScheduler } = await import('../src/lib/saveScheduler.js');
  const calls = [];
  const mk = () => createSaveScheduler({ delay: 20, save: () => calls.push('save'), saveOnExit: () => calls.push('exit') });
  let s = mk();
  s.schedule(); s.schedule();
  s.flushOnExit(); s.flushOnExit(); // pagehide + visibilitychange: sent once
  await new Promise((r) => setTimeout(r, 40));
  assert.deepEqual(calls, ['exit']); // timer did not fire afterwards
  calls.length = 0;
  s = mk();
  s.schedule();
  await new Promise((r) => setTimeout(r, 40));
  s.flushOnExit(); // already saved: no double send
  assert.deepEqual(calls, ['save']);
  calls.length = 0;
  s = mk();
  s.schedule(); s.cancel(); s.flushOnExit(); // logout: nothing sent
  await new Promise((r) => setTimeout(r, 40));
  assert.deepEqual(calls, []);
});
