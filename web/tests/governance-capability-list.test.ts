import assert from 'node:assert/strict';
import test from 'node:test';
import {
  resolveGovernanceCapabilityHierarchy,
  filterGovernanceCapabilities,
  countIncludedGovernanceComponents,
  type GovernanceCapabilityRecord
} from '../src/lib/governance-capability-list.ts';

const snapshot = {
  key: 'gitlab.snapshot', kind: 'plugin', name: 'GitLab 快照插件',
  description: '提取代码变更与历史', tools: ['repository.read']
};
const knowledge = {
  key: 'knowledge.search', kind: 'context_provider', name: '知识检索',
  description: '检索领域资料', tools: ['knowledge.query']
};
const fixtures = (): GovernanceCapabilityRecord[] => [
  { capability_key: 'code_review', kind: 'skill', status: 'disabled', owner: 'review-team', included_components: [snapshot, knowledge] },
  { capability_key: 'gitlab.snapshot', kind: 'plugin', status: 'active', parent_skill_key: 'code_review' },
  { capability_key: 'knowledge.search', kind: 'context_provider', status: 'active', parent_skill_key: 'code_review' },
  { capability_key: 'standalone.git', kind: 'plugin', status: 'active', description: '独立 Git 能力' }
];
const keys = (items: GovernanceCapabilityRecord[]) => items.map((item) => item.capability_key);
const filters = (search = '', kind = 'all', status = 'all') => ({ search, kind, status });

test('installed scope requires verified server artifacts and excludes builtins, plugins and stale receipts', () => {
  const items: GovernanceCapabilityRecord[] = [
    {capability_key:'well-ambient',kind:'skill',status:'active',origin:'system_builtin',install_state:'builtin'},
    {capability_key:'system.headroom',kind:'plugin',status:'active',origin:'system_builtin',install_state:'builtin'},
    {capability_key:'installed.skill',kind:'skill',status:'disabled',origin:'installed',install_state:'installed'},
    {capability_key:'missing.skill',kind:'skill',status:'active',origin:'installed',install_state:'missing'},
    {capability_key:'registered.skill',kind:'skill',status:'active',origin:'registry_only',install_state:'unverified'},
    {capability_key:'installed.plugin',kind:'plugin',status:'active',origin:'installed',install_state:'installed'},
  ];
  assert.deepEqual(keys(filterGovernanceCapabilities(items,{...filters(),scope:'installed'})),['installed.skill']);
  assert.deepEqual(keys(filterGovernanceCapabilities(items,{...filters(),scope:'system'})),['well-ambient','system.headroom']);
  assert.deepEqual(keys(filterGovernanceCapabilities(items,{...filters(),scope:'registry'})),['installed.skill','missing.skill','registered.skill','installed.plugin']);
});

test('owned components belong to skill summaries and never become independently manageable rows', () => {
  const { records, orphans } = resolveGovernanceCapabilityHierarchy(fixtures());
  assert.deepEqual(keys(records), ['code_review', 'standalone.git']);
  assert.deepEqual(records[0].included_components?.map((item) => item.key), ['gitlab.snapshot', 'knowledge.search']);
  assert.equal(countIncludedGovernanceComponents(records), 2);
  assert.deepEqual(orphans, []);
});

test('searching a component key, description, name or tool retains its owning skill', () => {
  const { records } = resolveGovernanceCapabilityHierarchy(fixtures());
  for (const search of ['gitlab.snapshot', ' KNOWLEDGE.SEARCH ', '知识检索', '领域资料', 'repository.read', 'review-team']) {
    assert.deepEqual(keys(filterGovernanceCapabilities(records, filters(search))), ['code_review'], search);
  }
});

test('category and status filters apply to the governable record rather than its components', () => {
  const { records } = resolveGovernanceCapabilityHierarchy(fixtures());
  assert.deepEqual(keys(filterGovernanceCapabilities(records, filters('', 'plugin'))), ['standalone.git']);
  assert.deepEqual(keys(filterGovernanceCapabilities(records, filters('', 'skills_only'))), ['code_review']);
  assert.deepEqual(keys(filterGovernanceCapabilities(records, filters('gitlab.snapshot', 'all', 'active'))), []);
  assert.deepEqual(keys(filterGovernanceCapabilities(records, filters('gitlab.snapshot', 'skill', 'disabled'))), ['code_review']);
  assert.equal(countIncludedGovernanceComponents(filterGovernanceCapabilities(records, filters('', 'plugin'))), 0);
});

test('ownership is recovered from either relation direction without hiding truly independent capabilities', () => {
  const items = fixtures();
  items[1].parent_skill_key = undefined;
  items[2].parent_skill_key = undefined;
  const inferred = resolveGovernanceCapabilityHierarchy(items);
  assert.deepEqual(keys(inferred.records), ['code_review', 'standalone.git']);

  items[0].included_components = undefined;
  items[1].parent_skill_key = 'code_review';
  items[1].description = '快照数据读取';
  const explicit = resolveGovernanceCapabilityHierarchy(items);
  assert.deepEqual(keys(explicit.records), ['code_review', 'knowledge.search', 'standalone.git']);
  assert.deepEqual(keys(filterGovernanceCapabilities(explicit.records, filters('快照数据'))), ['code_review']);
});

test('missing owners are reported without granting the orphan an independent lifecycle', () => {
  const items: GovernanceCapabilityRecord[] = [
    { capability_key: 'orphan.plugin', kind: 'plugin', status: 'active', parent_skill_key: 'missing.skill' },
    { capability_key: 'standalone.context', kind: 'context_provider', status: 'active' }
  ];
  const result = resolveGovernanceCapabilityHierarchy(items);
  assert.deepEqual(keys(result.records), ['standalone.context']);
  assert.deepEqual(result.orphans, [{ key: 'orphan.plugin', parentKeys: ['missing.skill'] }]);
});

test('nested ownership resolves to the root skill, while unrooted cycles remain relationship errors', () => {
  const items: GovernanceCapabilityRecord[] = [
    { capability_key: 'root.skill', kind: 'skill', status: 'active' },
    { capability_key: 'nested.plugin', kind: 'plugin', status: 'active', parent_skill_key: 'root.skill' },
    { capability_key: 'deep.provider', kind: 'context_provider', status: 'active', parent_skill_key: 'nested.plugin' },
    { capability_key: 'cycle.a', kind: 'plugin', status: 'active', parent_skill_key: 'cycle.b' },
    { capability_key: 'cycle.b', kind: 'plugin', status: 'active', parent_skill_key: 'cycle.a' }
  ];
  const result = resolveGovernanceCapabilityHierarchy(items);
  assert.deepEqual(keys(result.records), ['root.skill']);
  assert.deepEqual(result.records[0].included_components?.map((item) => item.key), ['nested.plugin', 'deep.provider']);
  assert.deepEqual(result.orphans.map((item) => item.key), ['cycle.a', 'cycle.b']);
  assert.deepEqual(keys(filterGovernanceCapabilities(result.records, filters('deep.provider'))), ['root.skill']);
});

test('duplicate registry rows and shared components do not inflate counts or mutate the API response', () => {
  const items = fixtures();
  items.push({ ...items[0] }, { capability_key: 'another.skill', kind: 'skill', status: 'active', included_components: [knowledge] });
  const before = structuredClone(items);
  const { records } = resolveGovernanceCapabilityHierarchy(items);
  assert.equal(records.length, 3);
  assert.equal(countIncludedGovernanceComponents(records), 2);
  records[0].included_components?.[0].tools?.push('new.tool');
  assert.deepEqual(items, before);
});

test('declared embedded components remain visible even without a separately registered row', () => {
  const result = resolveGovernanceCapabilityHierarchy([fixtures()[0]]);
  assert.equal(result.records.length, 1);
  assert.equal(countIncludedGovernanceComponents(result.records), 2);
  assert.deepEqual(result.orphans, []);
  assert.deepEqual(keys(filterGovernanceCapabilities(result.records, filters('knowledge.query'))), ['code_review']);
});
