import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import test from 'node:test';

function webFile(relativePath: string): string {
  return readFileSync(new URL(`../${relativePath}`, import.meta.url), 'utf8');
}

function repoFile(relativePath: string): string {
  return readFileSync(new URL(`../../${relativePath}`, import.meta.url), 'utf8');
}

const app = webFile('src/App.svelte');
const settings = webFile('src/components/SettingsPanel.svelte');
const maintenance = webFile('src/components/config/WellOSMaintenanceConfig.svelte');
const sections = webFile('src/lib/settings-sections.ts');
const switchSource = webFile('src/components/shared/Switch.svelte');
const modal = webFile('src/components/shared/Modal.svelte');
const configExample = repoFile('config.example.yaml');
const productionConfigExample = repoFile('deploy/config.production.example.yaml');
const compose = repoFile('compose.yaml');

test('WellOS maintenance is a first-class security settings section', () => {
  assert.match(sections, /\|\s*'wellos_auth'/);
  assert.match(
    sections,
    /id: 'wellos_auth'[\s\S]*?group: '安全与授权'[\s\S]*?label: '登录维护模式'[\s\S]*?domain: '认证连续性'/,
  );
  assert.ok(sections.indexOf("id: 'wellos_auth'") < sections.indexOf("id: 'users'"));
  assert.match(settings, /import WellOSMaintenanceConfig/);
  assert.match(settings, /activeSection === 'wellos_auth'/);
  assert.match(settings, /currentUserRole === 'super_admin'[\s\S]*?config:write/);
  assert.match(app, /currentUserRole=\{currentUserRole\}/);
});

test('maintenance control uses atomic status and update endpoints', () => {
  assert.match(maintenance, /fetch\('\/api\/config\/wellos-maintenance'\)/);
  assert.match(maintenance, /method: 'PUT'/);
  assert.match(maintenance, /expected_version: status\.version/);
  assert.match(maintenance, /configured/);
  assert.match(maintenance, /effective/);
  assert.match(maintenance, /override_locked/);
  assert.match(maintenance, /override_name/);
  assert.match(maintenance, /optimistic=\{false\}/);
  assert.match(maintenance, /expandedHitArea/);
  assert.match(maintenance, /layer="critical"/);
  assert.match(maintenance, /closeDisabled=\{saving\}/);
  assert.match(maintenance, /最多仍可使用 2 小时/);
});

test('shared switch and modal support confirmed server-authoritative changes', () => {
  assert.match(switchSource, /export let optimistic = true/);
  assert.match(switchSource, /if \(optimistic\) checked = next/);
  assert.match(modal, /export let closeDisabled = false/);
  assert.match(modal, /if \(closeDisabled\) return/);
  assert.match(modal, /disabled=\{closeDisabled\}/);
});

test('static YAML examples contain bootstrap settings only', () => {
  for (const source of [configExample, productionConfigExample]) {
    assert.match(source, /^database:/m);
    assert.match(source, /^server:/m);
    assert.match(source, /^\s+attachment_dir:/m);
    assert.doesNotMatch(source, /^gitlab:/m);
    assert.doesNotMatch(source, /^jira:/m);
    assert.doesNotMatch(source, /^ai:/m);
    assert.doesNotMatch(source, /^smtp:/m);
    assert.doesNotMatch(source, /^\s+maintenance_mode:/m);
    assert.doesNotMatch(source, /^\s+public_url:/m);
  }
  assert.match(compose, /WELL_AMBIENT_MAINTENANCE_MODE/);
});
