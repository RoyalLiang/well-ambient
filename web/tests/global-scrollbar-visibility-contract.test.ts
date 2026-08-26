import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import test from 'node:test';

const tokens = readFileSync(
  new URL('../src/styles/modern-admin-tokens.css', import.meta.url),
  'utf8'
);

const entrypoints = [
  '../src/main.ts',
  '../src/prototype-entry.ts',
  '../src/settings-preview-entry.ts'
].map((path) => readFileSync(new URL(path, import.meta.url), 'utf8'));

test('every web entrypoint loads the shared admin token layer', () => {
  for (const entrypoint of entrypoints) {
    assert.match(entrypoint, /import ['"]\.\/styles\/modern-admin-tokens\.css['"]/);
  }
});

test('shared scrollbar chrome is hidden without disabling overflow scrolling', () => {
  const contractStart = tokens.indexOf('/* Global hidden-scrollbar contract:');
  const contractEnd = tokens.indexOf('.wa-icon {', contractStart);
  const contract = tokens.slice(contractStart, contractEnd);

  assert.match(
    contract,
    /\* Global hidden-scrollbar contract:[\s\S]*?\*\s*\{[\s\S]*?scrollbar-width:\s*none\s*!important;[\s\S]*?-ms-overflow-style:\s*none;/
  );
  assert.match(
    contract,
    /\*::-webkit-scrollbar\s*\{[\s\S]*?display:\s*none\s*!important;[\s\S]*?width:\s*0\s*!important;[\s\S]*?height:\s*0\s*!important;/
  );
  assert.doesNotMatch(
    contract,
    /overflow:\s*hidden/
  );
});
