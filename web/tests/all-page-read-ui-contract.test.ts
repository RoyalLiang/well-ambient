import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import test from 'node:test';

const deliveryPlan = readFileSync(new URL('../src/components/DeliveryPlan.svelte', import.meta.url), 'utf8');
const aiConfig = readFileSync(new URL('../src/components/config/AIConfig.svelte', import.meta.url), 'utf8');
const corpusCandidates = readFileSync(new URL('../src/components/config/CorpusCandidateReview.svelte', import.meta.url), 'utf8');
const corpusSources = readFileSync(new URL('../src/components/config/CorpusSourceLibrary.svelte', import.meta.url), 'utf8');

test('first-wave high-cardinality page consumers use the shared bounded resource', () => {
  for (const [name, source] of [
    ['DeliveryPlan', deliveryPlan],
    ['AIConfig', aiConfig],
    ['CorpusCandidateReview', corpusCandidates],
    ['CorpusSourceLibrary', corpusSources]
  ] as const) {
    assert.match(source, /new PagedResource</, `${name} bypasses the shared bounded resource`);
    assert.match(source, /\.dispose\(\)/, `${name} does not release its request owner`);
  }
});

test('release filters are server scoped and both Jira lists have continuation', () => {
  assert.match(deliveryPlan, /new URLSearchParams\(\{ view: releaseListView \}\)/);
  assert.match(deliveryPlan, /params\.set\('project_key'/);
  assert.doesNotMatch(deliveryPlan, /params\.set\('status'/);
  assert.match(deliveryPlan, /params\.set\('q'/);
  assert.doesNotMatch(deliveryPlan, /visibleItems\s*=\s*releaseState\.items\.filter/);
  assert.match(deliveryPlan, /candidateJiraResource\.loadMore\(\)/);
  assert.match(deliveryPlan, /linkedJiraResource\.loadMore\(\)/);
  assert.match(deliveryPlan, />\s*加载更多候选\s*</);
  assert.match(deliveryPlan, />\s*加载更多已关联事项\s*</);
});
