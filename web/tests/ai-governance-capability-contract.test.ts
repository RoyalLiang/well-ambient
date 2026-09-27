import test from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';
import {
  AI_GOVERNANCE_SECTION_DEFINITIONS,
  canAccessAIGovernanceSection,
  canManageAIGovernanceSkills,
  canReadSolutionPublicURL,
  canWriteAIGovernanceRules,
  canWriteSolutionPublicURL,
  firstAccessibleAIGovernanceSection,
  resolveAIGovernanceSection,
} from '../src/lib/ai-governance-sections.ts';

const rootDir = path.resolve(import.meta.dirname, '..');
const appPath = path.resolve(rootDir, 'src/App.svelte');
const shellPath = path.resolve(rootDir, 'src/components/prototype/FunctionalAdminShell.svelte');
const aiGovPath = path.resolve(rootDir, 'src/components/AIGovernanceCenter.svelte');
const promptConfigPath = path.resolve(rootDir, 'src/components/config/SolutionPromptConfig.svelte');
const settingsPath = path.resolve(rootDir, 'src/lib/settings-sections.ts');
const settingsPanelPath = path.resolve(rootDir, 'src/components/SettingsPanel.svelte');
const settingsPreviewPath = path.resolve(rootDir, 'src/components/prototype/SettingsConfigPreview.svelte');
const aiConfigPath = path.resolve(rootDir, 'src/components/config/AIConfig.svelte');
const corpusSourceLibraryPath = path.resolve(rootDir, 'src/components/config/CorpusSourceLibrary.svelte');
const corpusCandidateReviewPath = path.resolve(rootDir, 'src/components/config/CorpusCandidateReview.svelte');
const serverPath = path.resolve(rootDir, '../internal/server/server.go');

const source = (filePath: string) => fs.readFileSync(filePath, 'utf8');

test('AI governance permission personas use the shared section contract without admin bypass', () => {
  assert.deepEqual(AI_GOVERNANCE_SECTION_DEFINITIONS.map(section => section.id), ['skills', 'prompts', 'rules', 'context']);

  const personas = [
    {
      name: 'context reader',
      role: 'member',
      permissions: ['ai_context:read'],
      accessible: ['skills', 'context'],
    },
    {
      name: 'rules reader',
      role: 'member',
      permissions: ['dashboard:read'],
      accessible: ['rules'],
    },
    {
      name: 'prompt manager without config read',
      role: 'super_admin',
      permissions: ['solution_prompt:manage'],
      accessible: ['prompts'],
    },
    {
      name: 'admin with every named permission',
      role: 'admin',
      permissions: ['ai_context:read', 'solution_prompt:manage', 'dashboard:read', 'config:read', 'config:write'],
      accessible: ['skills', 'rules', 'context'],
    },
    {
      name: 'full super admin',
      role: 'super_admin',
      permissions: ['*'],
      accessible: ['skills', 'prompts', 'rules', 'context'],
    },
    {
      name: 'no governance access',
      role: 'super_admin',
      permissions: [],
      accessible: [],
    },
  ];

  for (const persona of personas) {
    const accessible = AI_GOVERNANCE_SECTION_DEFINITIONS
      .filter(section => canAccessAIGovernanceSection(section.id, persona.permissions, persona.role))
      .map(section => section.id);
    assert.deepEqual(accessible, persona.accessible, persona.name);
  }

  assert.equal(canManageAIGovernanceSkills(['solution_prompt:manage'], 'super_admin'), true);
  assert.equal(canManageAIGovernanceSkills(['solution_prompt:manage'], 'admin'), false);
  assert.equal(canWriteAIGovernanceRules(['config:write']), true);
  assert.equal(canReadSolutionPublicURL(['solution_prompt:manage', 'config:read'], 'super_admin'), true);
  assert.equal(canWriteSolutionPublicURL(['solution_prompt:manage', 'config:read'], 'super_admin'), false);
  assert.equal(canWriteSolutionPublicURL(['solution_prompt:manage', 'config:read', 'config:write'], 'super_admin'), true);
});

test('AI governance fallback resolves defaults, invalid sections, and forbidden sections to first accessible', () => {
  assert.equal(firstAccessibleAIGovernanceSection(['dashboard:read'], 'member'), 'rules');
  assert.equal(resolveAIGovernanceSection('prompts', ['dashboard:read'], 'member'), 'rules');
  assert.equal(resolveAIGovernanceSection('unknown', ['ai_context:read'], 'member'), 'skills');
  assert.equal(resolveAIGovernanceSection(undefined, ['solution_prompt:manage'], 'super_admin'), 'prompts');
  assert.equal(resolveAIGovernanceSection('skills', [], 'member'), null);
});

test('App and shell share definitions, hydrate permissions safely, and preserve legacy links', () => {
  const app = source(appPath);
  const shell = source(shellPath);

  assert.match(app, /ai_governance:\s*\[\]/);
  assert.match(app, /if \(tab === 'ai_governance'\) return firstAccessibleAIGovernanceSection\(\) !== null/);
  assert.match(app, /let permissionsHydrated = !jwtToken/);
  assert.match(app, /async function refreshCurrentUserProfile\(\)[\s\S]*?if \(!res\.ok\)[\s\S]*?throw new Error[\s\S]*?finally \{[\s\S]*?permissionsHydrated = true;[\s\S]*?autoRedirectTab\(\);[\s\S]*?applyLocationIntent\(\);/);
  assert.match(app, /\{:else if !permissionsHydrated\}[\s\S]*?正在加载访问权限/);
  assert.match(app, /settingsSection === 'solution_prompts'[\s\S]*?selectAIGovernanceSection\('prompts'/);
  assert.match(app, /settingsSection === 'ai_context'[\s\S]*?selectAIGovernanceSection\('context'/);
  assert.match(app, /AI_GOVERNANCE_SECTION_MAP\[activeAIGovernanceSection\]\.label/);
  assert.match(app, /<AIGovernanceCenter[\s\S]*?\{currentUserRole\}[\s\S]*?onSectionChange=\{handleAIGovernanceNavigate\}/);

  assert.match(shell, /AI_GOVERNANCE_SECTION_DEFINITIONS\.map/);
  assert.match(shell, /item\.group === 'AI 治理'[\s\S]*?canAccessAIGovernanceSection\(item\.section, currentUserPermissions, currentUserRole\)/);
  assert.match(shell, /activeRoute === 'ai_governance' \? activeAIGovernanceSection/);
});

test('AIGovernanceCenter loads only the active authorized section through narrow endpoints', () => {
  const aiGov = source(aiGovPath);

  assert.match(aiGov, /accessibleSections = AI_GOVERNANCE_SECTION_DEFINITIONS\.filter/);
  assert.match(aiGov, /\{#each accessibleSections as section, index\}/);
  assert.match(aiGov, /resolvedActiveSection = canAccessAIGovernanceSection/);
  assert.match(aiGov, /if \(section === 'skills'\) loaded = await loadCapabilities\(\)/);
  assert.match(aiGov, /if \(section === 'prompts'\) loaded = await loadSolutionPublicURL\(\)/);
  assert.match(aiGov, /if \(section === 'rules'\) loaded = await loadRepoPolicies\(\)/);
  assert.match(aiGov, /if \(section === 'context'\) loaded = await loadContextReadiness\(\)/);
  assert.match(aiGov, /if \(loaded\) loadedSections = new Set/);
  assert.match(aiGov, /retrySection\(section: AIGovernanceSection\)/);
  assert.match(aiGov, /\/api\/solution-prompts\/public-url/);
  assert.match(aiGov, /\/api\/ai\/context-readiness/);
  assert.doesNotMatch(aiGov, /['"]\/api\/config['"]/);
  assert.doesNotMatch(aiGov, /GovernanceConfig|loadGovernanceConfig|canReadConfig|canWriteConfig|governanceConfig/);

  const onMountBody = aiGov.match(/onMount\(\(\) => \{([\s\S]*?)\n  \}\);/)?.[1] || '';
  assert.match(onMountBody, /mounted = true/);
  assert.doesNotMatch(onMountBody, /loadCapabilities|loadRepoPolicies|loadSolutionPublicURL|loadContextReadiness/);
});

test('AI governance mutation controls use the authoritative write permissions', () => {
  const aiGov = source(aiGovPath);
  const aiConfig = source(aiConfigPath);
  const corpusSourceLibrary = source(corpusSourceLibraryPath);
  const corpusCandidateReview = source(corpusCandidateReviewPath);

  assert.match(aiGov, /canManageSkills = canManageAIGovernanceSkills/);
  assert.match(aiGov, /\{#if canManageSkills\}[\s\S]*?导入 \/ 远程安装技能/);
  assert.match(aiGov, /disabled=\{!canManageSkills \|\| togglingStatusKey === cap\.capability_key\}/);
  assert.match(aiGov, /\{#if canManageSkills\}[\s\S]*?openUpgradeModal/);
  assert.match(aiGov, /canWriteRules = canWriteAIGovernanceRules/);
  assert.match(aiGov, /disabled=\{!canWriteRules \|\| savingPolicy\}/);
  assert.match(aiGov, /readonly=\{!canWriteRules\}/);

  assert.match(aiConfig, /canWriteContext = hasGovernancePermission\('ai_context:write'\)/);
  assert.match(aiConfig, /canPreviewContext = hasGovernancePermission\('ai_context:preview'\)/);
  assert.match(aiConfig, /readonly=\{!canWriteContext\}/);
  assert.match(aiConfig, /disabled=\{!canPreviewContext\}/);
  assert.match(corpusSourceLibrary, /currentUserPermissions\.includes\('\*'\)/);
  assert.match(corpusCandidateReview, /currentUserPermissions\.includes\('\*'\)/);
});

test('public URL uses optimistic expected version and reloads after a stale conflict', () => {
  const aiGov = source(aiGovPath);
  const promptConfig = source(promptConfigPath);
  const server = source(serverPath);

  assert.match(promptConfig, /export let publicURLVersion = 0/);
  assert.match(promptConfig, /export let publicURLLoading = false/);
  assert.match(promptConfig, /export let canReadPublicURL = true/);
  assert.match(promptConfig, /export let canWritePublicURL = false/);
  assert.match(promptConfig, /onSavePublicURL: \(value: string, expectedVersion: number\)/);
  assert.match(promptConfig, /await onSavePublicURL\(normalized, publicURLVersion\)/);
  assert.match(promptConfig, /publicURLSaveError = requestError\.message/);
  assert.match(promptConfig, /readonly=\{!canWritePublicURL\}/);
  assert.match(promptConfig, /\{#if canWritePublicURL\}[\s\S]*?保存地址/);

  assert.match(aiGov, /expected_version: expectedVersion/);
  assert.match(aiGov, /err instanceof GovernanceRequestError && err\.status === 409/);
  assert.match(aiGov, /const reloaded = await loadSolutionPublicURL\(\)/);
  assert.match(aiGov, /你的输入仍保留，请确认后再次保存/);
  assert.match(aiGov, /publicURL=\{solutionPublicURL\}/);
  assert.match(aiGov, /publicURLVersion=\{solutionPublicURLVersion\}/);
  assert.match(aiGov, /publicURLLoading=\{solutionPublicURLLoading\}/);

  assert.match(server, /GET \/api\/solution-prompts\/public-url[\s\S]*?config:read[\s\S]*?solution_prompt:manage[\s\S]*?withGlobalSuperAdmin/);
  assert.match(server, /PUT \/api\/solution-prompts\/public-url[\s\S]*?config:write[\s\S]*?config:read[\s\S]*?solution_prompt:manage[\s\S]*?withGlobalSuperAdmin/);
  assert.match(server, /GET \/api\/ai\/context-readiness[\s\S]*?ai_context:read/);
});

test('AIGovernanceCenter supports run bindings count and cold archive workflow', () => {
  const content = source(aiGovPath);

  assert.match(content, /export let onSectionChange:\s*\(section:\s*string\)\s*=>\s*void/);
  assert.match(content, /function switchSection\(sec:\s*AIGovernanceSection\)/);
  assert.match(content, /bindings_count\?:\s*number;/);
  assert.match(content, /is_archived\?:\s*boolean;/);
  assert.match(content, /status:\s*'active'\s*\|\s*'disabled'\s*\|\s*'retired'\s*\|\s*'uninstalled'\s*\|\s*'archived'/);
  assert.match(content, /<th[^>]*>关联运行<\/th>/);
  assert.match(content, /cap\.bindings_count/);
  assert.match(content, /已冷归档 \(保留审计\)/);
  assert.match(content, /deleteMode\s*===\s*'cold_archive'/);
  assert.match(content, /deleteMode\s*===\s*'purge'/);
  assert.match(content, /运行审计与重放关联提醒/);
  assert.match(content, /推荐模式/);
});

test('AIGovernanceCenter keeps styled selects, responsive table scrolling, and reduced motion', () => {
  const content = source(aiGovPath);

  assert.match(content, /class="custom-select-wrap"/);
  assert.match(content, /class="modern-select"/);
  assert.match(content, /class="select-chevron"/);
  assert.match(content, /appearance:\s*none;/);
  assert.match(content, /box-shadow:\s*0 0 0 3px rgba\(0,\s*143,\s*150,\s*0\.15\);/);
  assert.match(content, /\.action-btn-group\s*\{[\s\S]*?white-space:\s*nowrap\s*!important;/);
  assert.match(content, /\.btn\s*\{[\s\S]*?word-break:\s*keep-all\s*!important;/);
  assert.match(content, /\.gov-table-container\s*\{[\s\S]*?overflow-x:\s*auto/);
  assert.match(content, /@media \(max-width: 900px\)[\s\S]*?\.rules-workbench[\s\S]*?grid-template-columns:\s*1fr/);
  assert.match(content, /@media\s*\(prefers-reduced-motion:\s*reduce\)[\s\S]*?animation:\s*none/);
});

test('AIGovernanceCenter preserves skill hierarchy and canonical SKILL.md rendering', () => {
  const content = source(aiGovPath);

  assert.match(content, /interface IncludedComponent/);
  assert.match(content, /is_top_level_skill\?:/);
  assert.match(content, /parent_skill_key\?:/);
  assert.match(content, /included_components\?:/);
  assert.match(content, /badge-top-skill/);
  assert.match(content, /cap-included-block/);
  assert.match(content, /cap-parent-ref-block/);
  assert.match(content, /drawerActiveView:\s*'skill_doc'\s*\|\s*'slices'/);
  assert.match(content, /drawer-view-tabs/);
  assert.match(content, /skill-doc-container/);
  assert.match(content, /skill-doc-prose/);
  assert.match(content, /copySkillDoc/);
  assert.match(content, /detailRequestID/);
  assert.match(content, /selectedCapability\?\.capability_key !== cap\.capability_key/);
});

test('Settings no longer owns AI governance sections after migration', () => {
  const app = source(appPath);
  const aiGov = source(aiGovPath);
  const settings = source(settingsPath);
  const settingsPanel = source(settingsPanelPath);
  const settingsPreview = source(settingsPreviewPath);
  const aiConfig = source(aiConfigPath);
  const corpusSourceLibrary = source(corpusSourceLibraryPath);

  assert.doesNotMatch(settings, /\|\s*'solution_prompts'/);
  assert.doesNotMatch(settings, /\|\s*'ai_context'/);
  assert.doesNotMatch(settings, /id:\s*'solution_prompts'/);
  assert.doesNotMatch(settings, /id:\s*'ai_context'/);
  assert.match(settings, /id:\s*'ai'[\s\S]*?label:\s*'AI 引擎配置'/);
  assert.match(settings, /SETTINGS_ROUTE_PERMISSIONS\s*=\s*\['config:read', 'users:read'\]/);
  assert.match(settings, /SETTINGS_FALLBACK_ORDER[^=]*=\s*\['gitlab', 'users'\]/);

  assert.doesNotMatch(settingsPanel, /import SolutionPromptConfig/);
  assert.doesNotMatch(settingsPanel, /activeSection === 'solution_prompts'/);
  assert.doesNotMatch(settingsPanel, /activeSection === 'ai_context'/);
  assert.doesNotMatch(settingsPreview, /\{\s*id:\s*'ai_context'/);
  assert.doesNotMatch(settingsPreview, /['"](?:ai_context|corpus_candidate):[^'"]+['"]/);
  assert.doesNotMatch(aiGov, /\bloadPrompts\b/);
  assert.doesNotMatch(aiGov, /activate_on_save/);
  assert.match(aiGov, /import SolutionPromptConfig from '.\/config\/SolutionPromptConfig\.svelte'/);
  assert.match(aiGov, /import AIConfig from '.\/config\/AIConfig\.svelte'/);
  assert.match(aiGov, /<SolutionPromptConfig/);
  assert.match(aiGov, /<AIConfig[\s\S]*?view="context"/);

  assert.match(corpusSourceLibrary, /if\s*\(\s*aiReady\s*===\s*false\s*\)/);
  assert.match(corpusSourceLibrary, /disabled=\{!canWrite \|\| aiReady === false\}/);
  assert.match(aiConfig, /onMount\(\(\) => \{[\s\S]*?if \(showContextPanel\)[\s\S]*?fetchContextFacts\(\)[\s\S]*?contextFactResource\.dispose/);
  assert.match(app, /settingsSection === 'solution_prompts'[\s\S]*?activeAIGovernanceSection = section/);
  assert.match(app, /settingsSection === 'ai_context'[\s\S]*?activeAIGovernanceSection = section/);
});
