import assert from 'node:assert/strict';
import { existsSync, readFileSync } from 'node:fs';
import test from 'node:test';

const brandMark = readFileSync(new URL('../public/brand-mark.svg', import.meta.url), 'utf8');
const index = readFileSync(new URL('../index.html', import.meta.url), 'utf8');
const shell = readFileSync(
  new URL('../src/components/prototype/FunctionalAdminShell.svelte', import.meta.url),
  'utf8',
);

test('brand mark replaces the default favicon with the Phase 41 two-color identity', () => {
  assert.match(index, /<link rel="icon" type="image\/svg\+xml" href="\/brand-mark\.svg" \/>/);
  assert.match(brandMark, /fill="#71e2d1"/);
  assert.match(brandMark, /(?:fill|stroke)="#020b13"/);
  assert.doesNotMatch(brandMark, /<(?:linearGradient|radialGradient|filter|text)\b/);
  assert.doesNotMatch(brandMark, /#863bff|#7e14ff|#47bfff/i);
  assert.equal(existsSync(new URL('../public/favicon.svg', import.meta.url)), false);
});

test('the shared admin shell reuses the browser brand asset instead of the wa placeholder', () => {
  assert.match(
    shell,
    /<img class="brand-mark" src="\/brand-mark\.svg" alt="" aria-hidden="true" \/>/,
  );
  assert.doesNotMatch(shell, /<div class="brand-mark"[^>]*>wa<\/div>/i);
  assert.match(shell, /\.brand-mark\s*\{[\s\S]*?width:\s*40px;[\s\S]*?height:\s*40px;/);
  assert.match(shell, /\.rail-collapsed \.brand-block\s*\{[\s\S]*?justify-content:\s*center;/);
});
