import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import test from 'node:test';

function source(relativePath: string) {
  return readFileSync(new URL(`../${relativePath}`, import.meta.url), 'utf8');
}

const center = source('src/components/CodeReviewCenter.svelte');
const modal = source('src/components/shared/Modal.svelte');
const markdown = source('src/components/shared/MarkdownWorkbench.svelte');
const switchSource = source('src/components/shared/Switch.svelte');
const styles = source('src/styles/code-review.css');

test('code review uses the production list and keeps the detail entry in the primary column', () => {
  assert.match(center, /import AdminDataList from "\.\/admin-console\/AdminDataList\.svelte"/);
  assert.match(center, /import AdminListFilterBar from "\.\/admin-console\/AdminListFilterBar\.svelte"/);
  assert.match(center, /<AdminDataList[\s\S]*?columns=\{reviewColumns\}[\s\S]*?rows=\{reviewRows\}/);
  assert.match(center, /cell=\{renderReviewCell\}/);
  assert.match(center, /empty=\{renderReviewEmpty\}/);
  assert.match(center, /compactTableMinWidth="0px"/);
  assert.match(center, /\{ key: "sync", label: "评论同步", width: "142px", priority: "secondary" \}/);
  assert.match(center, /class="cr-mobile-sync"/);
  assert.match(center, /class="cr-review-source"/);
  assert.match(center, /\$\: normalizedFilter = filter\.trim\(\)\.toLowerCase\(\)/);
  assert.match(center, /\$\{r\.head_sha\} \$\{r\.base_sha\}/);
  assert.match(center, /function detailNeedsRefresh\(previous: Run, current: Run\)/);
  assert.match(center, /type RefreshMode = "initial" \| "manual" \| "poll"/);
  assert.match(center, /const visibleRefresh = mode !== "poll"/);
  assert.match(center, /const version = \+\+listRequestVersion/);
  assert.match(center, /function captureListAnchor\(\): ListAnchor \| null/);
  assert.match(center, /scrollTop: owner\.scrollTop/);
  assert.match(center, /Math\.abs\(owner\.scrollTop - anchor\.scrollTop\) > 1/);
  assert.match(center, /await restoreListAnchor\(anchor\)/);
  assert.match(center, /queuedRefresh\.resolvers\.push\(resolve\)/);
  assert.match(center, /void refresh\(queued\.initial, queued\.mode\)/);
  assert.match(center, /if \(runListChanged\(runs, nextRuns\)\) \{[\s\S]*?runs = nextRuns/);
  assert.match(center, /document\.visibilityState === "visible"[\s\S]*?refresh\(false, "poll"\)/);
  assert.match(center, /class="cr-review-trigger"[\s\S]*?aria-haspopup="dialog"[\s\S]*?selectRun\(run\.id\)/);
  assert.doesNotMatch(center, /PrototypeTable/);
  assert.doesNotMatch(center, /"操作"/);
  assert.doesNotMatch(center, />查看详情</);
  assert.doesNotMatch(styles, /min-width:\s*1000px/);
});

test('review detail reuses the decision-style modal and renders one markdown document surface', () => {
  assert.match(center, /<Modal[\s\S]*?size="wide"[\s\S]*?shadowless/);
  assert.match(center, /stableHeight/);
  assert.match(center, /footerVisible=\{Boolean\(/);
  assert.match(center, /<MarkdownWorkbench[\s\S]*?mode="preview"[\s\S]*?embedded[\s\S]*?fullWidthPreview[\s\S]*?autoHeight/);
  assert.match(center, /const reviewPanes: ReviewPane\[\] = \[[\s\S]*?"rules",[\s\S]*?\]/);
  assert.match(center, /role="tablist"[\s\S]*?role="tab"[\s\S]*?aria-selected=\{pane === item\}/);
  assert.match(center, /tabindex=\{pane === item \? 0 : -1\}/);
  assert.match(center, /handlePaneKeydown\(event, item\)/);
  assert.match(center, /\["ArrowLeft", "ArrowRight", "Home", "End"\]/);
  assert.match(center, /role="tabpanel"[\s\S]*?aria-labelledby=\{canReadEvidence \? `code-review-tab-\$\{pane\}` : undefined\}/);
  assert.match(center, /paneLabels:\s*Record<ReviewPane, string>/);
  assert.match(center, /## 事实依据/);
  assert.doesNotMatch(center, /detailScroller/);
  assert.doesNotMatch(center, /cr-reading-footer/);
  assert.doesNotMatch(styles, /max-width:\s*880px/);
});

test('detail loading, action feedback and scroll ownership stay inside the modal', () => {
  assert.match(center, /selected = summary \|\| selected;[\s\S]*?detailModal\?\.scrollToTop\(\)/);
  assert.match(center, /if \(showLoading\) \{[\s\S]*?detailError = "";/);
  assert.match(center, /version === requestVersion && showLoading/);
  assert.match(center, /selected = data;\s+detailError = "";/);
  assert.match(center, /async function changePane[\s\S]*?detailModal\?\.scrollToTop\(\)/);
  assert.match(center, /class="cr-modal-feedback" aria-live="polite"/);
  assert.match(center, /actionError[\s\S]*?role="alert"/);
  assert.match(center, /detailError[\s\S]*?重新加载/);
  assert.match(center, /type ReviewAction = "cancel" \| "sync" \| "retry"/);
  assert.match(center, /const generation = \+\+actionGeneration/);
  assert.match(center, /listRequestVersion\+\+/);
  assert.match(center, /selected\?\.id === origin\.id/);
  assert.match(center, /detailModal\?\.focusDialog\(\)/);
  assert.doesNotMatch(center, /await refresh\(false, "poll"\)/);
  assert.match(center, /: "同步评论"}/);
  assert.match(center, /actionKind === "cancel" \? "取消中…" : "取消评审"/);
  assert.match(center, /selected\.status === "failed"[\s\S]*?runAction\("retry"\)/);
  assert.match(center, /action === "retry"[\s\S]*?已重新加入评审队列/);
  assert.match(center, /\$\: canManualSync = Boolean\(/);
  assert.match(center, /selectedSyncEnabled/);
  assert.doesNotMatch(center, /\["completed", "partial"\]\.includes\(selected\.status\)/);
  assert.match(center, /\{#if canManualSync\}/);
  assert.match(center, /target="_blank"[\s\S]*?rel="noopener noreferrer"[\s\S]*?GitLab ↗/);
  assert.match(modal, /export function scrollToTop\(behavior: ScrollBehavior = 'auto'\)/);
  assert.match(modal, /export function focusDialog\(\)/);
  assert.match(modal, /!modalContainer\.contains\(document\.activeElement\)/);
  assert.match(modal, /bind:this=\{modalBody\}/);
  assert.match(modal, /footerVisible && \$\$slots\.footer/);
});

test('responsive interaction targets and transparency fallback are explicit', () => {
  assert.match(styles, /\.cr-review-trigger \{[\s\S]*?min-height: 44px/);
  assert.match(styles, /\.cr-evidence-links button,\s*\.cr-evidence-links a \{[\s\S]*?min-height: 44px/);
  assert.doesNotMatch(styles, /\.cr-reading \.markdown-body :is\(p, ul, ol, blockquote\)/);
  assert.match(markdown, /export let fullWidthPreview = false/);
  assert.match(markdown, /class:full-width-preview=\{fullWidthPreview\}/);
  assert.match(markdown, /\.full-width-preview \.markdown-body :global\(p\)[\s\S]*?max-width: none/);
  assert.match(styles, /\.cr-mobile-sync \{[\s\S]*?flex: 0 0 auto/);
  assert.match(styles, /\.cr-review-source \{[\s\S]*?text-overflow: ellipsis/);
  assert.match(styles, /@media \(max-width: 760px\)/);
  assert.match(styles, /\.cr-reading \.markdown-workbench\.mode-preview \.preview-pane \{[\s\S]*?padding: 18px 16px/);
  assert.match(modal, /@media \(prefers-reduced-transparency: reduce\)/);
  assert.match(modal, /backdrop-filter: none/);
});

test('policy switches are parent-owned, reversible and fully labelled', () => {
  assert.match(center, /const previousPolicy = \{ \.\.\.activeRepo\.policy \}/);
  assert.match(center, /const nextPolicy = \{ \.\.\.previousPolicy, \.\.\.change \}/);
  assert.match(center, /body: JSON\.stringify\(\{\s*project_id: activeRepo\.project_id,\s*\.\.\.change,/);
  assert.match(center, /policy: nextPolicy/);
  assert.match(center, /policy: previousPolicy/);
  assert.match(center, /Commit 自动同步已\$\{p\.sync_commits \? "开启" : "关闭"\}/);
  assert.match(center, /MR 自动同步已\$\{p\.sync_mrs \? "开启" : "关闭"\}/);
  assert.match(center, /class="cr-policy-feedback"/);
  assert.match(center, /saving=\{policySaving === "sync_commits"\}/);
  assert.match(center, /saving=\{policySaving === "sync_mrs"\}/);
  assert.match(center, /expandedHitArea/);
  assert.match(center, /自动同步评论按仓库独立设置/);
  assert.match(switchSource, /export let saving = false/);
  assert.match(switchSource, /export let expandedHitArea = false/);
  assert.match(switchSource, /class:expanded-hit-area=\{expandedHitArea\}/);
  assert.match(switchSource, /\.expanded-hit-area \.switch-control \{[\s\S]*?height: 44px/);
  assert.match(switchSource, /<label class="switch-label" for=\{controlId\}>/);
  assert.match(switchSource, /disabled=\{disabled \|\| saving\}/);
});
