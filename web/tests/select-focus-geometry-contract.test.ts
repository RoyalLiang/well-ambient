import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import test from 'node:test';

function source(relativePath: string) {
  return readFileSync(new URL(`../${relativePath}`, import.meta.url), 'utf8');
}

const select = source('src/components/shared/Select.svelte');
const multiSelect = source('src/components/shared/MultiSelect.svelte');
const decision = source('src/components/DecisionDashboard.svelte');

test('shared searchable selects delegate focus indication to the stable field trigger', () => {
  assert.match(
    select,
    /\.select-inline-input\s*\{[\s\S]*?border-radius:\s*var\(--wa-radius-sm,\s*8px\)/
  );
  assert.match(
    select,
    /\.select-inline-input:focus,[\s\S]*?\.select-inline-input:focus-visible\s*\{[\s\S]*?outline:\s*none\s*!important/
  );
  assert.match(
    select,
    /\.select-input-shell:has\(\.select-inline-input:focus-visible\)\s*\{[\s\S]*?outline:\s*2px solid var\(--wa-border-focus/
  );

  assert.match(
    multiSelect,
    /\.multi-select-trigger input\s*\{[^}]*border-radius:\s*var\(--wa-radius-sm,\s*8px\)/
  );
  assert.match(
    multiSelect,
    /\.multi-select-trigger input:focus,[\s\S]*?\.multi-select-trigger input:focus-visible\s*\{[\s\S]*?outline:\s*none\s*!important/
  );
  assert.match(
    multiSelect,
    /\.multi-select-trigger:has\(input:focus-visible\)\s*\{[\s\S]*?outline:\s*2px solid var\(--wa-border-focus/
  );
});

test('every decision-list dropdown stays on the shared Select and MultiSelect focus contract', () => {
  const filterControls = decision.match(
    /<AdminListFilterBar[\s\S]*?className="decision-filter-bar"[\s\S]*?<\/AdminListFilterBar>/
  )?.[0] ?? '';
  const tabletRules = decision.match(
    /@media \(max-width: 800px\) \{([\s\S]*?)\n  \}\n\n  @media \(max-width: 640px\)/
  )?.[1] ?? '';

  assert.equal((filterControls.match(/<Select\b/g) ?? []).length, 2);
  assert.equal((filterControls.match(/<MultiSelect\b/g) ?? []).length, 1);
  assert.doesNotMatch(
    decision,
    /\.toolbar-select\s+:global\([^)]*(?:select-inline-input|multi-select-trigger input):focus/
  );
  assert.match(
    tabletRules,
    /\.column-select :global\(\.multi-select-group\.summary-mode \.multi-select-trigger\)\s*\{[\s\S]*?height:\s*var\(--wa-touch-h,\s*44px\)/
  );
});
