import test from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';

const rootDir = path.resolve(import.meta.dirname, '..');
const appPath = path.resolve(rootDir, 'src/App.svelte');
const shellPath = path.resolve(rootDir, 'src/components/prototype/FunctionalAdminShell.svelte');
const aiGovPath = path.resolve(rootDir, 'src/components/AIGovernanceCenter.svelte');

test('App.svelte ensures ai_governance has robust permission access and admin bypass', () => {
  const content = fs.readFileSync(appPath, 'utf8');

  // Verify tabPermissions.ai_governance contains fallback read permissions
  assert.match(content, /ai_governance:\s*\[[^\]]*'ai_governance:read'[^\]]*\]/, 'ai_governance should include ai_governance:read');
  
  // Verify canAccessTab allows super_admin and admin
  assert.match(content, /function canAccessTab[\s\S]*?currentUserRole === 'super_admin' \|\| currentUserRole === 'admin'/, 'canAccessTab must allow super_admin and admin bypass');

  // Verify hasPermission allows super_admin and admin
  assert.match(content, /function hasPermission[\s\S]*?currentUserRole === 'super_admin' \|\| currentUserRole === 'admin'/, 'hasPermission must allow super_admin and admin bypass');

  // Verify applyLocationIntent parses tab=ai_governance or section parameters
  assert.match(content, /params\.get\('tab'\) === 'ai_governance'/, 'applyLocationIntent must parse tab=ai_governance');

  // Verify AIGovernanceCenter component receives onSectionChange
  assert.match(content, /<AIGovernanceCenter[\s\S]*?onSectionChange=\{handleAIGovernanceNavigate\}/, 'AIGovernanceCenter must receive onSectionChange');
});

test('FunctionalAdminShell.svelte correctly exposes ai_governance navigation with subitems', () => {
  const content = fs.readFileSync(shellPath, 'utf8');

  // Verify aiGovernanceSubnav exists with 4 sections
  assert.match(content, /const aiGovernanceSubnav:\s*NavSubItem\[\]/, 'aiGovernanceSubnav must be defined');
  assert.match(content, /section:\s*'skills'/, 'must include skills subnav');
  assert.match(content, /section:\s*'prompts'/, 'must include prompts subnav');
  assert.match(content, /section:\s*'rules'/, 'must include rules subnav');
  assert.match(content, /section:\s*'context'/, 'must include context subnav');

  // Verify canAccessSubItem allows admin role bypass
  assert.match(content, /function canAccessSubItem[\s\S]*?currentUserRole === 'super_admin' \|\| currentUserRole === 'admin'/, 'canAccessSubItem must allow super_admin and admin');

  // Verify navItems has ai_governance with sparkles icon
  assert.match(content, /route:\s*'ai_governance'[\s\S]*?icon:\s*'sparkles'/, 'navItems must include ai_governance');
});

test('AIGovernanceCenter.svelte supports run bindings count and cold archive workflow', () => {
  const content = fs.readFileSync(aiGovPath, 'utf8');

  // Verify onSectionChange prop and switchSection helper
  assert.match(content, /export let onSectionChange:\s*\(section:\s*string\)\s*=>\s*void/, 'must export onSectionChange prop');
  assert.match(content, /function switchSection\(sec:\s*string\)/, 'must have switchSection helper to sync tabs');

  // Verify CapabilityItem model includes bindings_count and is_archived
  assert.match(content, /bindings_count\?:\s*number;/, 'CapabilityItem must declare bindings_count');
  assert.match(content, /is_archived\?:\s*boolean;/, 'CapabilityItem must declare is_archived');
  assert.match(content, /status:\s*'active'\s*\|\s*'disabled'\s*\|\s*'retired'\s*\|\s*'uninstalled'\s*\|\s*'archived'/, 'status must include archived');

  // Verify table displays Run Bindings and Cold Archive status
  assert.match(content, /<th[^>]*>关联运行<\/th>/, 'Table header must display Run Bindings column');
  assert.match(content, /cap\.bindings_count/, 'Table body must render cap.bindings_count');
  assert.match(content, /已冷归档 \(保留审计\)/, 'Table body must render archived tag');

  // Verify delete modal provides cold_archive and purge options
  assert.match(content, /deleteMode\s*===\s*'cold_archive'/, 'Modal must support cold_archive mode');
  assert.match(content, /deleteMode\s*===\s*'purge'/, 'Modal must support purge mode');
  assert.match(content, /运行审计与重放关联提醒/, 'Modal must display audit warning when historical bindings exist');
  assert.match(content, /推荐模式/, 'Modal must recommend cold archive for run-bound capabilities');
});

test('AIGovernanceCenter.svelte uses modern styled select controls and enforces strict button nowrap', () => {
  const content = fs.readFileSync(aiGovPath, 'utf8');

  // Verify modern select wrapper and custom chevron
  assert.match(content, /class="custom-select-wrap"/, 'Must wrap filter select in custom-select-wrap');
  assert.match(content, /class="modern-select"/, 'Must apply modern-select class to filter dropdowns');
  assert.match(content, /class="select-chevron"/, 'Must render custom SVG chevron for filter dropdowns');

  // Verify CSS eliminates native appearance and adds focus rings
  assert.match(content, /appearance:\s*none;/, 'Must disable native browser select appearance');
  assert.match(content, /box-shadow:\s*0 0 0 3px rgba\(0,\s*143,\s*150,\s*0\.15\);/, 'Must style focus ring for select');

  // Verify strict button nowrap
  assert.match(content, /\.action-btn-group\s*\{[\s\S]*?white-space:\s*nowrap\s*!important;/, 'action-btn-group must enforce nowrap');
  assert.match(content, /\.action-btn-group\s*\{[\s\S]*?flex-wrap:\s*nowrap\s*!important;/, 'action-btn-group must not wrap');
  assert.match(content, /\.btn\s*\{[\s\S]*?white-space:\s*nowrap\s*!important;/, 'btn must enforce nowrap');
  assert.match(content, /\.btn\s*\{[\s\S]*?word-break:\s*keep-all\s*!important;/, 'btn must keep words together');
});

test('AIGovernanceCenter.svelte supports skill hierarchy identification and canonical SKILL.md rendering', () => {
  const content = fs.readFileSync(aiGovPath, 'utf8');

  // Verify skill hierarchy data structures and badges
  assert.match(content, /interface IncludedComponent/, 'Must declare IncludedComponent interface');
  assert.match(content, /is_top_level_skill\?:/, 'CapabilityItem must declare is_top_level_skill');
  assert.match(content, /parent_skill_key\?:/, 'CapabilityItem must declare parent_skill_key');
  assert.match(content, /included_components\?:/, 'CapabilityItem must declare included_components');
  assert.match(content, /badge-top-skill/, 'Must render badge for top-level business skill');
  assert.match(content, /cap-included-block/, 'Must render included components list block for skill');
  assert.match(content, /cap-parent-ref-block/, 'Must render parent skill reference for sub-plugins');

  // Verify SKILL.md drawer rendering
  assert.match(content, /drawerActiveView:\s*'skill_doc'\s*\|\s*'slices'/, 'Drawer must support dual view switching');
  assert.match(content, /drawer-view-tabs/, 'Drawer must render view switcher tabs');
  assert.match(content, /skill-doc-container/, 'Drawer must contain SKILL.md document container');
  assert.match(content, /skill-doc-prose/, 'Drawer must contain markdown prose container');
  assert.match(content, /copySkillDoc/, 'Drawer must support copying raw SKILL.md');
});
