import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import test from 'node:test';

function source(relativePath: string) {
  return readFileSync(new URL(`../${relativePath}`, import.meta.url), 'utf8');
}

const dataList = source('src/components/admin-console/AdminDataList.svelte');
const pagination = source('src/components/admin-console/AdminPagination.svelte');
const preview = source('src/components/prototype/AdminDataListPreview.svelte');
const viteConfig = source('vite.config.ts');
const previewHtml = source('admin-data-list-preview.html');

test('data list keeps native table semantics and caller-owned render snippets', () => {
  assert.match(dataList, /<table[\s\S]*?class="admin-table wa-admin-table"/);
  assert.match(dataList, /<caption>\{caption\}<\/caption>/);
  assert.match(dataList, /<th[\s\S]*?scope="col"/);
  assert.match(dataList, /cell\?: Snippet/);
  assert.match(dataList, /actions\?: Snippet/);
  assert.match(dataList, /pagination\?: Snippet/);
  assert.match(dataList, /aria-busy=\{loading \|\| loadingPrevious \|\| loadingMore \|\| Boolean\(requestInFlight\)\}/);
  assert.doesNotMatch(dataList, /fetch\(/);
  assert.doesNotMatch(dataList, /next_cursor|page\.generation/);
});

test('data list owns optional virtual scrolling and controlled append states', () => {
  assert.match(dataList, /virtual\?:/);
  assert.match(dataList, /type Surface = 'standalone' \| 'embedded'/);
  assert.match(dataList, /surface\?: Surface/);
  assert.match(dataList, /loadMoreKey\?: string/);
  assert.match(dataList, /resetKey\?: string/);
  assert.match(dataList, /onLoadMore\?: \(\) => void \| Promise<void>/);
  assert.match(dataList, /onLoadPrevious\?: \(\) => void \| Promise<void>/);
  assert.match(dataList, /loadPreviousError\?: string/);
  assert.match(dataList, /hasPrevious\?: boolean/);
  assert.match(dataList, /computeVirtualWindow/);
  assert.match(dataList, /shouldRequestMore/);
  assert.match(dataList, /class:is-embedded=\{surface === 'embedded'\}/);
  assert.match(dataList, /class:is-virtual=\{Boolean\(virtual\)\}/);
  assert.match(dataList, /class="load-more-row"/);
  assert.match(dataList, /class="load-previous-row"/);
  assert.match(dataList, /加载更多/);
  assert.match(dataList, /已加载全部/);
  assert.match(dataList, /role="status"/);
  assert.match(dataList, /ResizeObserver/);
  assert.match(dataList, /captureScrollAnchor/);
  assert.match(dataList, /restoreScrollAnchor/);
  assert.match(dataList, /await tick\(\)/);
  assert.match(dataList, /await onLoadPrevious\(\);\s*consumedNextKey = ''/);
  assert.match(dataList, /await onLoadMore\(\);\s*consumedPreviousKey = ''/);
  assert.doesNotMatch(dataList, /IntersectionObserver/);
});

test('refresh status overlays retained rows without moving the scroll viewport', () => {
  const overlayBindings = dataList.match(/class:is-overlay=\{hasRows\}/g) ?? [];
  assert.equal(overlayBindings.length, 3, 'success, retained-error and retained-loading states must overlay existing rows');
  assert.match(dataList, /\.admin-data-list \{[\s\S]*?position: relative;/);
  assert.match(dataList, /\.list-status\.is-overlay \{[\s\S]*?position: absolute;/);
  assert.match(dataList, /\.loading-status\.is-overlay \{[\s\S]*?pointer-events: none;/);
});

test('data list hides the exhausted previous boundary and keeps a compact sticky header', () => {
  assert.doesNotMatch(dataList, /已到当前列表起点/);
  assert.match(
    dataList,
    /onLoadPrevious && \(loadPreviousError \|\| loadingPrevious \|\| requestInFlight === 'previous' \|\| hasPrevious\)/
  );
  assert.match(dataList, /height: var\(--wa-touch-h\)/);
  assert.match(dataList, /background-color: var\(--wa-surface-inset\)/);
  assert.match(
    dataList,
    /background-image:\s*linear-gradient\([\s\S]*?color-mix\(in srgb, var\(--wa-surface-flat\)[\s\S]*?color-mix\(in srgb, var\(--wa-surface-inset\)/
  );
  assert.match(dataList, /background-clip: padding-box/);
  assert.match(dataList, /color: var\(--wa-text-strong\)/);
  assert.match(dataList, /text-overflow: ellipsis/);
  assert.match(dataList, /border-bottom-color: var\(--wa-border-strong\)/);
  assert.match(dataList, /inset 0 1px 0 var\(--wa-glass-highlight\)/);
  const headerRule = dataList.match(/\.admin-table th \{([\s\S]*?)\n  \}/)?.[1] ?? '';
  assert.doesNotMatch(headerRule, /backdrop-filter|transition:/);
});

test('responsive column priority replaces caller nth-child coupling', () => {
  assert.match(dataList, /column\.priority === 'secondary'/);
  assert.match(dataList, /\.column-secondary/);
  assert.match(dataList, /--admin-compact-table-min-width/);
});

test('component states, glass boundary and responsive accessibility are explicit', () => {
  for (const state of ['default', 'hover', 'focus-visible', 'active', 'disabled', 'loading', 'error', 'success']) {
    assert.match(dataList, new RegExp(state));
  }
  assert.match(dataList, /prefers-reduced-transparency: reduce/);
  assert.match(dataList, /prefers-reduced-motion: reduce/);
  assert.match(dataList, /@media \(hover: hover\)/);
  assert.match(dataList, /tabindex="0"/);
  assert.match(dataList, /onkeydown=\{handleTableScrollKey\}/);
  assert.match(dataList, /region\.scrollLeft \+= direction \* step/);
  assert.match(dataList, /min-height: var\(--wa-touch-h\)/);
  // Exclude Svelte control blocks such as {#each}; only a literal hex token counts here.
  assert.doesNotMatch(dataList, /(?<!\{)#[0-9a-f]{3,8}\b/i);
  assert.doesNotMatch(pagination, /(?<!\{)#[0-9a-f]{3,8}\b/i);
});

test('pagination is controlled and exposes native navigation semantics', () => {
  assert.match(pagination, /<nav aria-label=\{ariaLabel\}>/);
  assert.match(pagination, /class="page-numbers" role="group"/);
  assert.match(pagination, /aria-current=\{item === summary\.page \? 'page' : undefined\}/);
  assert.match(pagination, /onPageChange/);
  assert.match(pagination, /onPageSizeChange/);
  assert.match(pagination, /disabled=\{controlsDisabled \|\| summary\.page <= 1\}/);
  assert.match(pagination, /disabled=\{controlsDisabled \|\| summary\.page >= summary\.pageCount\}/);
});

test('preview renders real columns, content, operations, pagination and every state', () => {
  assert.match(preview, /columns=\{columns\}/);
  assert.match(preview, /cell=\{renderCell\}/);
  assert.match(preview, /actions=\{renderActions\}/);
  assert.match(preview, /pagination=\{renderPagination\}/);
  for (const state of ['default', 'hover', 'focus', 'active', 'disabled', 'loading', 'error', 'success', 'empty']) {
    assert.match(preview, new RegExp(`key: '${state}'`));
  }
  assert.match(viteConfig, /adminDataListPreview/);
  assert.match(viteConfig, /admin-data-list-preview\.html/);
  assert.match(preview, /Array\.from\(\{ length: 1000 \}/);
  assert.match(preview, /virtual=\{\{ rowHeight: 48, overscan: 8, loadAheadRows: 10 \}\}/);
  assert.match(preview, /onLoadMore=\{handleLazyLoad\}/);
  assert.match(preview, /onLoadPrevious=\{handleLazyPrevious\}/);
  assert.match(preview, /lazyWindowPageLimit\s*=\s*3/);
  assert.match(preview, /页面窗口最多保留 300 行/);
  assert.match(preview, /上一批模拟失败/);
  assert.match(preview, /下一批模拟失败/);
});

test('checked-in preview opens directly without a Vite server', () => {
  assert.doesNotMatch(previewHtml, /<script[^>]+\bsrc=/i);
  assert.doesNotMatch(previewHtml, /<script[^>]+\btype=["']module["']/i);
  assert.match(previewHtml, /<style data-admin-data-list-preview>/);
  assert.match(previewHtml, /<script data-admin-data-list-preview>/);
  assert.match(previewHtml, /id="admin-data-list-preview"/);
});
