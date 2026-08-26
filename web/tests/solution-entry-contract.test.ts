import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import test from 'node:test';

function component(relativePath: string) {
  return readFileSync(new URL(`../src/components/${relativePath}`, import.meta.url), 'utf8');
}

const demandKanban = component('DemandKanban.svelte');
const deliveryControl = component('DemandDeliveryControl.svelte');
const solutionWorkspace = component('SolutionWorkspace.svelte');

test('solution button updates the existing right inspector without adding a third tab', () => {
  assert.match(demandKanban, /import SolutionWorkspace from '\.\/SolutionWorkspace\.svelte'/);
  assert.match(demandKanban, /type ScheduleInspectorMode = 'schedule' \| 'telemetry' \| 'solution'/);
  assert.match(demandKanban, /actions=\{renderScheduleActions\}/);
  assert.match(demandKanban, /on:click=\{\(\) => selectScheduleItem\(row\.item, 'solution'\)\}/);
  assert.match(demandKanban, /<SolutionWorkspace\s+demand=\{solutionDemand\}/);
  assert.match(demandKanban, /selectedDemand\?\.task_id === selectedScheduleItem\.demand_id[\s\S]*?task_id: selectedScheduleItem\.demand_id/);
  assert.doesNotMatch(demandKanban, /id="schedule-inspector-tab-solution"/);
  assert.doesNotMatch(demandKanban, /class:has-solution=/);
  assert.doesNotMatch(deliveryControl, /SolutionWorkspace/);
  assert.doesNotMatch(demandKanban, /润色候选|Agent 候选/);
});

test('solution deep links update the right inspector while preserving the schedule table', () => {
  assert.match(demandKanban, /linkedSolutionRequested[\s\S]*?const linkedScheduleItem = scheduleItems\.find/);
  assert.match(demandKanban, /selectScheduleItem\(linkedScheduleItem, hasPermission\('solution:read'\) \? 'solution' : 'schedule'\)/);
  assert.doesNotMatch(demandKanban, /class:is-solution=/);
  assert.doesNotMatch(demandKanban, /\.schedule-main-grid\.is-solution/);
  assert.doesNotMatch(demandKanban, /\.schedule-main-grid\.is-solution[\s\S]*?\.schedule-table-stack[\s\S]*?display: none/);
});

test('missing Jira solution sources render one compact empty state without creation controls', () => {
  assert.match(solutionWorkspace, /hasSolutionContent = !!workspace\?\.working \|\| !!latestJob/);
  assert.match(solutionWorkspace, /暂无解决方案/);
  assert.match(solutionWorkspace, /Jira 尚未同步到标记为方案的评论/);
  assert.equal(solutionWorkspace.match(/Jira 尚未同步到标记为方案的评论/g)?.length, 1);
  assert.doesNotMatch(solutionWorkspace, />建立方案<\/button>/);
});

test('a loaded solution renders its preview outside the empty-state branch', () => {
  assert.match(
    solutionWorkspace,
    /\{:else if !workspace \|\| !hasSolutionContent\}[\s\S]*?Jira 尚未同步到标记为方案的评论[\s\S]*?\{:else\}[\s\S]*?<MarkdownWorkbench[\s\S]*?mode="preview"[\s\S]*?readonly/,
  );
});

test('solution preview fills the desktop inspector without implementation-detail pills', () => {
  assert.doesNotMatch(solutionWorkspace, /solution-facts|当前来源快照|Gzip 压缩保存|原文保存/);
  assert.doesNotMatch(solutionWorkspace, /currentSources/);
  assert.match(
    solutionWorkspace,
    /<div class="solution-preview-content">[\s\S]*?<button[^>]*on:click=\{openEditor\}[^>]*>编辑方案<\/button>[\s\S]*?<MarkdownWorkbench[\s\S]*?mode="preview"/,
  );
  const previewLayout = solutionWorkspace.match(/\.solution-preview-content\s*\{[^}]*\}/)?.[0] || '';
  assert.match(previewLayout, /flex:\s*1 1 auto/);
  assert.match(previewLayout, /min-height:\s*0/);
  assert.match(previewLayout, /grid-template-rows:\s*auto minmax\(0,1fr\)/);
  assert.match(
    solutionWorkspace,
    /\.solution-preview-content\s*>\s*:global\(\.markdown-workbench\)\s*\{[^}]*height:\s*100%[^}]*min-height:\s*0/s,
  );
  assert.match(
    demandKanban,
    /\.schedule-solution-inline\s*\{[^}]*display:\s*flex[^}]*flex-direction:\s*column/s,
  );
  assert.match(
    solutionWorkspace,
    /@media \(max-width: 1280px\)[\s\S]*?\.solution-preview-content\s*\{[^}]*flex:\s*0 0 auto[^}]*grid-template-rows:\s*auto auto[^}]*\}[\s\S]*?\.solution-preview-content\s*>\s*:global\(\.markdown-workbench\)\s*\{[^}]*height:\s*var\(--workbench-height\)/,
  );
});

test('solution editing uses one canonical live draft and exposes real generation states', () => {
  assert.doesNotMatch(solutionWorkspace, /MarkdownMode/);
  assert.doesNotMatch(solutionWorkspace, /availableModes=/);
  assert.match(solutionWorkspace, /mode="live"/);
  assert.doesNotMatch(solutionWorkspace, /保存 Markdown|复制本地 Markdown|Markdown 方案已保存/);
  assert.match(solutionWorkspace, /排队中/);
  assert.match(solutionWorkspace, /正在生成方案/);
  assert.match(solutionWorkspace, /等待重试/);
  assert.match(solutionWorkspace, /已到重试时间，正在等待后台队列/);
  assert.match(solutionWorkspace, /方案生成失败/);
  assert.match(solutionWorkspace, /重新生成/);
  assert.match(solutionWorkspace, /重新排队中…/);
  assert.match(solutionWorkspace, /\/api\/solutions\/jobs\/\$\{failedJob\.id\}\/retry/);
  assert.match(solutionWorkspace, /查看技术详情/);
  assert.match(solutionWorkspace, /上游服务等待超时/);
  assert.doesNotMatch(solutionWorkspace, /Agent 候选|重新润色|candidate-review|requestPolish|applyCandidate/);
  assert.doesNotMatch(solutionWorkspace, /候选 \{workspace\.candidates\.length\}/);
});

test('solution defaults to preview and keeps save and publish inside one edit dialog', () => {
  assert.match(solutionWorkspace, /import Modal from '\.\/shared\/Modal\.svelte'/);
  assert.match(
    solutionWorkspace,
    /<button[^>]*on:click=\{openEditor\}[^>]*>编辑方案<\/button>[\s\S]*?<MarkdownWorkbench[\s\S]*?mode="preview"[\s\S]*?readonly/,
  );
  assert.match(
    solutionWorkspace,
    /<Modal[\s\S]*?title="即时修改方案"[\s\S]*?<MarkdownWorkbench[\s\S]*?mode="live"[\s\S]*?on:save=\{saveDraft\}/,
  );
  assert.match(solutionWorkspace, /slot="footer"[\s\S]*?on:click=\{saveDraft\}[\s\S]*?'保存'/);
  assert.match(solutionWorkspace, /slot="footer"[\s\S]*?on:click=\{saveAndPublish\}[\s\S]*?'发布'/);
  assert.doesNotMatch(solutionWorkspace, />保存方案<\/button>|>发布方案<\/button>|>编辑新版本<\/button>/);
  assert.doesNotMatch(solutionWorkspace, /方案 v\{|历史版本 \{|远端新版本|加载远端版本/);
});

test('solution edit dialog presents the markdown content as the modal body instead of a nested card', () => {
  assert.match(
    solutionWorkspace,
    /<Modal[\s\S]*?title="即时修改方案"[\s\S]*?hideBodyScrollbar=\{true\}[\s\S]*?<MarkdownWorkbench[\s\S]*?mode="live"[\s\S]*?embedded=\{true\}/,
  );
  assert.match(solutionWorkspace, /\.solution-editor-body\s*\{[^}]*padding:\s*0/s);
  assert.match(
    solutionWorkspace,
    /@media \(max-width: 860px\)[\s\S]*?\.solution-editor-body :global\(\.markdown-workbench\)\s*\{[^}]*height:\s*clamp\(280px,calc\(100dvh - 288px\),560px\)/,
  );
  assert.doesNotMatch(solutionWorkspace, /showDocumentMeta=\{true\}/);
});

test('solution edit dialog protects dirty content and preserves backend revision concurrency', () => {
  assert.match(solutionWorkspace, /function requestCloseEditor\(\)[\s\S]*?editorState\.dirty[\s\S]*?discardPrompt = true/);
  assert.match(solutionWorkspace, /function discardEditorChanges\(\)[\s\S]*?workspace\?\.working\?\.markdown[\s\S]*?editorOpen = false/);
  assert.match(solutionWorkspace, /有未保存修改[\s\S]*?继续编辑[\s\S]*?放弃修改/);
  assert.match(solutionWorkspace, /expected_revision: workspace\.asset\.revision/);
  assert.match(solutionWorkspace, /base_revision_id: workspace\.working\.id/);
  assert.match(solutionWorkspace, /editorState\.remoteUpdateAvailable/);
});

test('solution draft save uses the global toast without shifting or remounting the editor body', () => {
  assert.match(solutionWorkspace, /import \{ showToast \} from '\.\.\/lib\/toast'/);

  const saveDraft = solutionWorkspace.match(/async function saveDraft\(\)[\s\S]*?\n  \}/)?.[0] || '';
  assert.match(saveDraft, /showToast\('方案已保存。',\s*\{\s*title:\s*'保存成功'\s*\}\)/);
  assert.doesNotMatch(saveDraft, /notice\s*=/);

  const editorModal = solutionWorkspace.match(/<Modal[\s\S]*?<\/Modal>/)?.[0] || '';
  assert.doesNotMatch(editorModal, /\{#if notice\}<div class="solution-message success"/);
  assert.doesNotMatch(solutionWorkspace, /\{#if action === 'save'\}[\s\S]*?<MarkdownWorkbench/);
  assert.match(
    solutionWorkspace,
    /<MarkdownWorkbench[\s\S]*?mode="live"[\s\S]*?saving=\{action === 'save' \|\| action === 'publish'\}[\s\S]*?on:save=\{saveDraft\}/,
  );
});
