import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import test from 'node:test';

function component(relativePath: string) {
  return readFileSync(new URL(`../src/components/${relativePath}`, import.meta.url), 'utf8');
}

const demandKanban = component('DemandKanban.svelte');
const deliveryControl = component('DemandDeliveryControl.svelte');
const solutionCenter = component('SolutionCenter.svelte');
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
  assert.match(solutionWorkspace, /slot="footer"[\s\S]*?on:click=\{requestPublish\}[\s\S]*?>发布<\/button>/);
  assert.doesNotMatch(solutionWorkspace, />保存方案<\/button>|>发布方案<\/button>|>编辑新版本<\/button>/);
  assert.doesNotMatch(solutionWorkspace, /方案 v\{|历史版本 \{|远端新版本|加载远端版本/);
});

test('solution publish requires an in-place confirmation and reports through the global toast', () => {
  assert.match(solutionWorkspace, /let publishPrompt = false/);
  assert.match(solutionWorkspace, /function requestPublish\(\)[\s\S]*?publishPrompt = true/);
  assert.match(solutionWorkspace, /function requestPublish\(\)[\s\S]*?notice = ''/);
  assert.match(solutionWorkspace, /function cancelPublish\(\)[\s\S]*?publishPrompt = false/);
  assert.match(
    solutionWorkspace,
    /slot="footer"[\s\S]*?确认发布当前方案[\s\S]*?不会自动回写 Jira 评论[\s\S]*?on:click=\{cancelPublish\}[\s\S]*?on:click=\{confirmPublish\}/,
  );

  const publishSolution = solutionWorkspace.match(/async function publishSolution\(\)[\s\S]*?\n  \}/)?.[0] || '';
  assert.match(publishSolution, /showToast\('方案已发布。',\s*\{\s*title:\s*'发布成功'\s*\}\)/);
  assert.match(publishSolution, /showToast\(publishError,\s*\{\s*type:\s*'error'/);
  assert.doesNotMatch(publishSolution, /notice\s*=|回写队列/);
  assert.doesNotMatch(solutionWorkspace, /方案已发布，并已进入 Jira 链接回写队列/);
});

test('an existing solution takes precedence over a historical failed generation job', () => {
  assert.match(
    solutionWorkspace,
    /visiblePolishState = workspace\?\.working && polishState\?\.danger \? null : polishState/,
  );
  assert.match(solutionWorkspace, /\{#if visiblePolishState\}[\s\S]*?visiblePolishState\.label/);
  assert.doesNotMatch(solutionWorkspace, /\{#if polishState\}[\s\S]*?polishState\.label/);
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

test('solution center keeps every catalog row to a compact two-line register', () => {
  assert.match(solutionCenter, /class="row-title"/);
  assert.match(solutionCenter, /class="row-identity"/);
  assert.doesNotMatch(solutionCenter, /class="row-summary"/);
  assert.doesNotMatch(solutionCenter, /% 存储/);
  assert.match(solutionCenter, /\.register-row\s*\{[^}]*min-height:\s*72px[^}]*max-height:\s*72px/s);
  assert.match(solutionCenter, /\.skeleton-row\s*\{[^}]*height:\s*72px/s);
  assert.match(solutionCenter, /\.mode-switch button\s*\{[^}]*white-space:\s*nowrap/s);
  assert.match(solutionCenter, /\.document-actions button, \.document-actions a\s*\{[^}]*white-space:\s*nowrap/s);
  assert.match(solutionCenter, /@media \(max-width: 820px\)[\s\S]*?\.center-grid\s*\{\s*display:\s*block/);
});

test('solution center makes markdown the published and standard document surface', () => {
  const catalogDocument = solutionCenter.match(/\{:else if mode === 'catalog' && detail\}[\s\S]*?\{:else if mode === 'comparisons'/)?.[0] || '';
  const standardDocument = solutionCenter.match(/\{:else if mode === 'standards' && standardDetail\}[\s\S]*?\{:else\}/)?.[0] || '';

  assert.match(catalogDocument, /class="document-surface"/);
  assert.match(catalogDocument, /<MarkdownWorkbench[\s\S]*?mode="preview"[\s\S]*?readonly/);
  assert.doesNotMatch(catalogDocument, /当前已发布版本|fact-strip|comparison-section|需求背景/);
  assert.match(standardDocument, /class="document-surface"/);
  assert.match(standardDocument, /<MarkdownWorkbench[\s\S]*?mode="preview"[\s\S]*?readonly/);
  assert.doesNotMatch(standardDocument, /当前版本|fact-strip/);
});

test('similar solutions live in a dedicated center view and preserve proposal review', () => {
  assert.match(solutionCenter, /type CenterMode = 'catalog' \| 'comparisons' \| 'standards'/);
  assert.match(solutionCenter, /mode === 'comparisons'[\s\S]*?>相似方案<\/button>/);
  const comparisonView = solutionCenter.match(/\{:else if mode === 'comparisons' && selectedComparison\}[\s\S]*?\{:else if mode === 'standards'/)?.[0] || '';
  assert.match(comparisonView, /两轮对比与标准化建议/);
  assert.match(comparisonView, /标准化提案/);
  assert.match(comparisonView, /beginReview\(selectedComparison!\.proposal/);
  assert.match(comparisonView, /submitReview/);
});

test('solution center exports markdown and copies stable authenticated deep links', () => {
  assert.match(solutionCenter, /new Blob\(\[markdown\], \{ type: 'text\/markdown;charset=utf-8' \}\)/);
  assert.match(solutionCenter, /anchor\.download = markdownFilename/);
  assert.match(solutionCenter, /URL\.revokeObjectURL/);
  assert.match(solutionCenter, /navigator\.clipboard\.writeText/);
  assert.match(solutionCenter, /params\.set\('solution_entry'/);
  assert.match(solutionCenter, /params\.set\('solution_standard'/);
  assert.match(solutionCenter, /复制链接/);
  assert.match(solutionCenter, /导出 Markdown/);
});

test('solution center opens a stable entry deep link even outside the first catalog page', () => {
  assert.match(solutionCenter, /readLocationIntent\(\)/);
  assert.match(solutionCenter, /solution_entry/);
  assert.match(solutionCenter, /solution_standard/);
  assert.match(solutionCenter, /if \(!entries\.some\(\(item\) => item\.id === response\.entry\.id\)\)/);
  assert.match(solutionCenter, /entries = \[response\.entry, \.\.\.entries\]/);
});
