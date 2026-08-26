<script lang="ts">
  import type { AdminCellValue, AdminTableColumn, AdminTableRow } from '../../lib/admin-console/contract';
  import { ADMIN_TONE_CLASS } from '../../lib/admin-console/contract';
  import {
    flattenBoundedPages,
    updateBoundedPageWindow,
    type BoundedPage
  } from '../../lib/admin-data-window';
  import AdminDataList from '../admin-console/AdminDataList.svelte';
  import AdminListFilterBar from '../admin-console/AdminListFilterBar.svelte';
  import AdminPagination from '../admin-console/AdminPagination.svelte';

  interface PreviewRow extends AdminTableRow {
    project: string;
  }

  interface PreviewWindowMeta {
    start: number;
    end: number;
  }

  type PreviewWindowPage = BoundedPage<PreviewRow, PreviewWindowMeta>;

  const columns: AdminTableColumn[] = [
    { key: 'title', label: '事项', width: '34%' },
    { key: 'project', label: '项目', width: '18%', priority: 'secondary' },
    { key: 'status', label: '状态', width: '14%' },
    { key: 'owner', label: '负责人', width: '14%' },
    { key: 'updatedAt', label: '更新时间', width: '20%', align: 'right', priority: 'secondary' }
  ];

  const allRows: PreviewRow[] = [
    row('WA-248', '统一调度证据口径', '智能驾驶平台', '进行中', 'info', '陈默', '08-23 16:42'),
    row('WA-251', '补齐发布前检查项', '远程运营中心', '待评审', 'warning', '林岚', '08-23 15:18'),
    row('WA-257', '修复告警确认回执', '设备诊断服务', '已完成', 'success', '周朗', '08-23 14:07'),
    row('WA-263', '收敛历史数据字段', '数据管理服务', '阻塞', 'danger', '许臻', '08-23 11:36'),
    row('WA-268', '验证窄屏操作路径', '智能驾驶平台', '进行中', 'info', '程砚', '08-22 18:55'),
    row('WA-272', '整理分页契约', '远程运营中心', '待评审', 'warning', '沈知', '08-22 17:20'),
    row('WA-279', '补充空状态恢复入口', '设备诊断服务', '已完成', 'success', '叶川', '08-22 14:12'),
    row('WA-283', '校验操作权限边界', '数据管理服务', '进行中', 'info', '顾澄', '08-22 10:48'),
    row('WA-288', '核对列表列宽规则', '智能驾驶平台', '待评审', 'warning', '陆遥', '08-21 19:05'),
    row('WA-294', '压缩刷新期间布局位移', '远程运营中心', '已完成', 'success', '韩序', '08-21 16:33'),
    row('WA-301', '处理不可达操作状态', '设备诊断服务', '阻塞', 'danger', '苏澈', '08-21 13:47'),
    row('WA-307', '复核键盘焦点顺序', '数据管理服务', '进行中', 'info', '方原', '08-21 09:28')
  ];
  const lazyRows: PreviewRow[] = Array.from({ length: 1000 }, (_, index) => {
    const number = index + 1;
    const statuses = [
      ['进行中', 'info'],
      ['待评审', 'warning'],
      ['已完成', 'success'],
      ['阻塞', 'danger']
    ] as const;
    const [status, tone] = statuses[index % statuses.length];
    return row(
      `WA-${String(number).padStart(4, '0')}`,
      `连续滚动验证事项 ${number}`,
      ['智能驾驶平台', '远程运营中心', '设备诊断服务'][index % 3],
      status,
      tone,
      ['陈默', '林岚', '周朗', '许臻'][index % 4],
      `08-${String(23 - (index % 8)).padStart(2, '0')} ${String(9 + (index % 10)).padStart(2, '0')}:20`
    );
  });
  const lazyPageSize = 100;
  const lazyWindowPageLimit = 3;

  const stateRows = allRows.slice(0, 1);
  const states = [
    { key: 'default', label: 'Default', description: '稳定数据与操作。' },
    { key: 'hover', label: 'Hover', description: '指针反馈只在精细指针设备出现。' },
    { key: 'focus', label: 'Focus visible', description: '键盘焦点即时出现，不改变几何。' },
    { key: 'active', label: 'Active', description: '按下反馈与行状态分离。' },
    { key: 'disabled', label: 'Disabled', description: '内容可读，操作与分页冻结。' },
    { key: 'loading', label: 'Loading', description: '骨架保持最终列宽与行高。' },
    { key: 'error', label: 'Error', description: '说明原因并提供恢复入口。' },
    { key: 'success', label: 'Success', description: '结果可见，反馈保持克制。' },
    { key: 'empty', label: 'Empty', description: '解释当前条件并给出下一步。' }
  ] as const;

  let page = $state(1);
  let pageSize = $state(5);
  let previewSearch = $state('');
  let previewProject = $state('all');
  let statusMessage = $state('');
  let retryCount = $state(0);
  let lazyPages = $state<PreviewWindowPage[]>([previewWindowPage(0)]);
  let lazyResetVersion = $state(0);
  let lazyLoadingPrevious = $state(false);
  let lazyPreviousError = $state('');
  let lazyLoading = $state(false);
  let lazyError = $state('');
  let failPreviousLoad = $state(false);
  let failNextLoad = $state(false);
  let filteredPreviewRows = $derived(allRows.filter((item) => {
    const query = previewSearch.trim().toLocaleLowerCase('zh-CN');
    const matchesQuery = !query || [item.id, item.title, item.project, item.owner]
      .some((value) => String(value ?? '').toLocaleLowerCase('zh-CN').includes(query));
    const matchesProject = previewProject === 'all' || item.project === previewProject;
    return matchesQuery && matchesProject;
  }));
  let visibleRows = $derived(filteredPreviewRows.slice((page - 1) * pageSize, page * pageSize));
  let lazyVisibleRows = $derived(flattenBoundedPages(lazyPages));
  let lazyWindowStart = $derived(lazyPages[0]?.meta.start ?? 0);
  let lazyWindowEnd = $derived(lazyPages.at(-1)?.meta.end ?? 0);
  let lazyHasPrevious = $derived(lazyWindowStart > 0);
  let lazyHasMore = $derived(lazyWindowEnd < lazyRows.length);

  function row(
    id: string,
    title: string,
    project: string,
    status: string,
    tone: PreviewRow['tone'],
    owner: string,
    updatedAt: string
  ): PreviewRow {
    return {
      id,
      title,
      project,
      status,
      tone,
      owner,
      cells: { title, project, status, owner, updatedAt }
    };
  }

  function previewWindowPage(start: number): PreviewWindowPage {
    const safeStart = Math.max(0, Math.min(start, Math.max(0, lazyRows.length - lazyPageSize)));
    const end = Math.min(lazyRows.length, safeStart + lazyPageSize);
    return {
      items: lazyRows.slice(safeStart, end),
      meta: { start: safeStart, end }
    };
  }

  function handlePageSizeChange(nextPageSize: number) {
    pageSize = nextPageSize;
    page = 1;
  }

  function resetPreviewFilters() {
    previewSearch = '';
    previewProject = 'all';
    page = 1;
  }

  function handleAction(action: string, item: PreviewRow) {
    statusMessage = `${item.id} 已执行${action}操作。`;
  }

  function handleRetry() {
    retryCount += 1;
    statusMessage = `已重新加载 ${retryCount} 次。`;
  }

  async function handleLazyLoad() {
    if (lazyLoading || lazyLoadingPrevious || !lazyHasMore) return;
    lazyLoading = true;
    lazyError = '';
    await new Promise((resolve) => window.setTimeout(resolve, 220));
    if (failNextLoad) {
      failNextLoad = false;
      lazyError = '下一批内容暂时无法加载。';
      lazyLoading = false;
      return;
    }
    lazyPages = updateBoundedPageWindow(
      lazyPages,
      previewWindowPage(lazyWindowEnd),
      'next',
      lazyWindowPageLimit,
      (item) => item.id
    );
    lazyLoading = false;
  }

  async function handleLazyPrevious() {
    if (lazyLoadingPrevious || lazyLoading || !lazyHasPrevious) return;
    lazyLoadingPrevious = true;
    lazyPreviousError = '';
    await new Promise((resolve) => window.setTimeout(resolve, 220));
    if (failPreviousLoad) {
      failPreviousLoad = false;
      lazyPreviousError = '上一批内容暂时无法回补。';
      lazyLoadingPrevious = false;
      return;
    }
    lazyPages = updateBoundedPageWindow(
      lazyPages,
      previewWindowPage(Math.max(0, lazyWindowStart - lazyPageSize)),
      'previous',
      lazyWindowPageLimit,
      (item) => item.id
    );
    lazyLoadingPrevious = false;
  }

  function resetLazyPreview() {
    lazyPages = [previewWindowPage(0)];
    lazyResetVersion += 1;
    lazyPreviousError = '';
    lazyError = '';
    failPreviousLoad = false;
    failNextLoad = false;
  }

  function simulatePreviousError() {
    lazyPreviousError = '';
    failPreviousLoad = true;
  }

  function simulateNextError() {
    lazyError = '';
    failNextLoad = true;
  }
</script>

{#snippet renderCell(row: PreviewRow, column: AdminTableColumn, value: AdminCellValue)}
  {#if column.key === 'title'}
    <div class="primary-cell">
      <strong>{row.title}</strong>
      <small>{row.id}</small>
    </div>
  {:else if column.key === 'status'}
    <span class="wa-admin-pill {ADMIN_TONE_CLASS[row.tone || 'neutral']}">{row.status}</span>
  {:else}
    <span class:tabular={column.align === 'right'}>{String(value ?? '-')}</span>
  {/if}
{/snippet}

{#snippet renderActions(row: PreviewRow, listDisabled: boolean)}
  <button
    type="button"
    class="wa-admin-action secondary"
    disabled={listDisabled}
    aria-label={`查看 ${row.id}`}
    onclick={() => handleAction('查看', row)}
  >查看</button>
  <button
    type="button"
    class="wa-admin-action secondary"
    disabled={listDisabled}
    aria-label={`编辑 ${row.id}`}
    onclick={() => handleAction('编辑', row)}
  >编辑</button>
{/snippet}

{#snippet renderPagination(paginationDisabled: boolean)}
  <AdminPagination
    {page}
    {pageSize}
    total={filteredPreviewRows.length}
    pageSizeOptions={[5, 10, 20]}
    disabled={paginationDisabled}
    onPageChange={(nextPage) => page = nextPage}
    onPageSizeChange={handlePageSizeChange}
  />
{/snippet}

{#snippet compactActions(row: PreviewRow, listDisabled: boolean)}
  <button
    type="button"
    class="wa-admin-action secondary"
    disabled={listDisabled}
    aria-label={`查看 ${row.id}`}
  >查看</button>
{/snippet}

<main class="preview-page">
  <header class="preview-header">
    <div>
      <p>Phase 41 组件预览</p>
      <h1>通用列表</h1>
      <span>列、内容、行操作与分页均由调用方传入。毛玻璃只用于外层结构边界。</span>
    </div>
    <dl>
      <div><dt>密度</dt><dd>正常</dd></div>
      <div><dt>行高</dt><dd>48 px</dd></div>
      <div><dt>模式</dt><dd>受控</dd></div>
    </dl>
  </header>

  <section class="preview-section" aria-labelledby="interactive-title">
    <div class="section-heading">
      <div>
        <h2 id="interactive-title">交互预览</h2>
        <p>示例数据仅存在于预览入口。生产组件不请求接口，也不切片业务数据。</p>
      </div>
      <span aria-live="polite">{statusMessage || '等待操作'}</span>
    </div>

    <div class="surface-contract-preview" data-preview="admin-list-surface">
      <AdminListFilterBar label="交付事项筛选">
        {#snippet leading()}
          <div class="preview-filter-copy">
            <span>Selection</span>
            <strong>交付事项列表</strong>
          </div>
        {/snippet}

        {#snippet controls()}
          <div class="preview-filter-controls">
            <input
              class="wa-control"
              type="search"
              bind:value={previewSearch}
              placeholder="搜索编号、标题、项目、负责人"
              aria-label="搜索交付事项"
              oninput={() => page = 1}
            />
            <select class="wa-control" bind:value={previewProject} aria-label="筛选交付项目" onchange={() => page = 1}>
              <option value="all">全部项目</option>
              <option value="智能驾驶平台">智能驾驶平台</option>
              <option value="远程运营中心">远程运营中心</option>
              <option value="设备诊断服务">设备诊断服务</option>
              <option value="数据管理服务">数据管理服务</option>
            </select>
            <button type="button" class="wa-admin-action secondary" onclick={resetPreviewFilters}>重置</button>
          </div>
        {/snippet}

        {#snippet meta()}
          <div class="preview-filter-meta">
            <span>显示 <strong>{filteredPreviewRows.length}</strong> / {allRows.length} 项</span>
            <small>筛选条与列表共享同一表面合同</small>
          </div>
        {/snippet}
      </AdminListFilterBar>

      <AdminDataList
        columns={columns}
        rows={visibleRows}
        caption="交付事项列表"
        cell={renderCell}
        actions={renderActions}
        pagination={renderPagination}
        success={statusMessage}
        tableMinWidth="920px"
        onRetry={handleRetry}
      />
    </div>
  </section>

  <section class="preview-section state-section" aria-labelledby="states-title">
    <div class="section-heading">
      <div>
        <h2 id="states-title">状态总览</h2>
        <p>八个交互状态与额外空状态同时可见，便于逐项会审。</p>
      </div>
    </div>

    <div class="state-grid">
      {#each states as state (state.key)}
        <article class="state-item">
          <header>
            <h3>{state.label}</h3>
            <p>{state.description}</p>
          </header>
          <AdminDataList
            columns={columns.slice(0, 3)}
            rows={state.key === 'loading' || state.key === 'error' || state.key === 'empty' ? [] : stateRows}
            caption={`${state.label} 状态列表`}
            cell={renderCell}
            actions={compactActions}
            disabled={state.key === 'disabled'}
            loading={state.key === 'loading'}
            error={state.key === 'error' ? '请求超时，请检查连接后重新加载。' : ''}
            success={state.key === 'success' ? '数据已更新。' : ''}
            previewState={state.key === 'hover' || state.key === 'focus' || state.key === 'active' ? state.key : 'default'}
            skeletonRows={1}
            tableMinWidth="560px"
            actionsWidth="92px"
            onRetry={handleRetry}
          />
        </article>
      {/each}
    </div>
  </section>

  <section class="preview-section" aria-labelledby="lazy-title">
    <div class="section-heading">
      <div>
        <h2 id="lazy-title">连续滚动预览</h2>
        <p>1000 行本地数据模拟服务端；页面窗口最多保留 300 行，DOM 只保留当前窗口与 overscan。</p>
      </div>
      <div class="preview-actions">
        <span>第 {lazyWindowStart + 1}–{lazyWindowEnd} 行 · 窗口保留 {lazyVisibleRows.length} / {lazyRows.length}</span>
        <button type="button" class="wa-admin-action secondary" onclick={simulatePreviousError}>上一批模拟失败</button>
        <button type="button" class="wa-admin-action secondary" onclick={simulateNextError}>下一批模拟失败</button>
        <button type="button" class="wa-admin-action secondary" onclick={resetLazyPreview}>复位</button>
      </div>
    </div>

    <AdminDataList
      columns={columns}
      rows={lazyVisibleRows}
      caption="连续滚动事项列表"
      cell={renderCell}
      actions={compactActions}
      tableMinWidth="920px"
      viewportHeight="420px"
      virtual={{ rowHeight: 48, overscan: 8, loadAheadRows: 10 }}
      totalRowCount={lazyRows.length}
      resetKey={`preview-lazy-list:${lazyResetVersion}`}
      hasPrevious={lazyHasPrevious}
      loadingPrevious={lazyLoadingPrevious}
      loadPreviousError={lazyPreviousError}
      loadPreviousKey={String(lazyWindowStart)}
      onLoadPrevious={handleLazyPrevious}
      hasMore={lazyHasMore}
      loadingMore={lazyLoading}
      loadMoreError={lazyError}
      loadMoreKey={String(lazyWindowEnd)}
      onLoadMore={handleLazyLoad}
    />
  </section>
</main>

<style>
  .preview-page {
    width: min(1480px, calc(100% - 48px));
    margin: 0 auto;
    padding: var(--wa-space-8) 0 var(--wa-space-12);
    color: var(--wa-text-main);
  }

  .preview-header {
    display: flex;
    align-items: end;
    justify-content: space-between;
    gap: var(--wa-space-8);
    padding: var(--wa-space-6) 0 var(--wa-space-8);
  }

  .preview-header p,
  .preview-header span,
  .section-heading p,
  .state-item header p {
    margin: 0;
    color: var(--wa-text-muted);
  }

  .preview-header > div > p {
    margin-bottom: var(--wa-space-2);
    font-size: 13px;
    font-weight: 720;
  }

  h1,
  h2,
  h3 {
    color: var(--wa-text-strong);
    text-wrap: balance;
  }

  h1 {
    margin: 0 0 var(--wa-space-2);
    font-size: 32px;
    line-height: 1.15;
  }

  .preview-header dl {
    display: flex;
    gap: var(--wa-space-6);
    margin: 0;
  }

  .preview-header dl div {
    min-width: 72px;
  }

  .preview-header dt {
    color: var(--wa-text-muted);
    font-size: 12px;
  }

  .preview-header dd {
    margin: var(--wa-space-1) 0 0;
    color: var(--wa-text-strong);
    font-size: 14px;
    font-weight: 760;
  }

  .preview-section + .preview-section {
    margin-top: var(--wa-space-10);
  }

  .section-heading {
    min-width: 0;
    display: flex;
    align-items: end;
    justify-content: space-between;
    gap: var(--wa-space-4);
    margin-bottom: var(--wa-space-4);
  }

  .section-heading h2 {
    margin: 0 0 var(--wa-space-1);
    font-size: 20px;
  }

  .section-heading p {
    max-width: 68ch;
    font-size: 13px;
    line-height: 1.55;
  }

  .section-heading > span {
    color: var(--wa-accent-strong);
    font-size: 13px;
    font-weight: 720;
    white-space: nowrap;
  }

  .surface-contract-preview {
    min-width: 0;
    display: grid;
    gap: var(--wa-space-3);
  }

  .preview-filter-copy {
    min-width: 148px;
    display: grid;
    gap: 2px;
  }

  .preview-filter-copy span {
    color: var(--wa-text-muted);
    font-size: 10px;
    font-weight: 760;
    letter-spacing: 0.12em;
    text-transform: uppercase;
  }

  .preview-filter-copy strong {
    color: var(--wa-text-strong);
    font-size: 14px;
  }

  .preview-filter-controls {
    display: grid;
    grid-template-columns: minmax(220px, 1fr) minmax(160px, 220px) auto;
    gap: var(--wa-space-2);
  }

  .preview-filter-controls .wa-control {
    min-width: 0;
    padding: 0 var(--wa-space-3);
  }

  .preview-filter-meta {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: var(--wa-space-3);
    color: var(--wa-text-muted);
    font-size: 12px;
  }

  .preview-filter-meta strong {
    color: var(--wa-text-strong);
  }

  .preview-filter-meta small {
    color: var(--wa-text-subtle);
  }

  .preview-actions {
    display: flex;
    align-items: center;
    justify-content: flex-end;
    gap: var(--wa-space-2);
  }

  .preview-actions span {
    color: var(--wa-accent-strong);
    font-size: 13px;
    font-weight: 720;
    white-space: nowrap;
  }

  .preview-actions .wa-admin-action {
    min-height: var(--wa-touch-h);
    white-space: nowrap;
  }

  .primary-cell {
    min-width: 0;
    display: grid;
    gap: 2px;
  }

  .primary-cell strong {
    overflow: hidden;
    color: var(--wa-text-strong);
    font-size: 13px;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .primary-cell small {
    color: var(--wa-text-muted);
    font-family: var(--wa-font-mono);
    font-size: 11px;
  }

  .tabular {
    font-variant-numeric: tabular-nums;
  }

  .state-grid {
    display: grid;
    grid-template-columns: repeat(2, minmax(0, 1fr));
    gap: var(--wa-space-6);
  }

  .state-item {
    min-width: 0;
  }

  .state-item > header {
    min-height: 58px;
    padding: 0 var(--wa-space-1) var(--wa-space-3);
  }

  .state-item h3 {
    margin: 0 0 var(--wa-space-1);
    font-size: 15px;
  }

  .state-item header p {
    font-size: 12px;
    line-height: 1.45;
  }

  @media (max-width: 800px) {
    .preview-filter-controls .wa-control,
    .preview-filter-controls .wa-admin-action {
      min-height: var(--wa-touch-h);
    }
  }

  @media (max-width: 760px) {
    .preview-page {
      width: min(100% - 24px, 1480px);
      padding-top: var(--wa-space-4);
    }

    .preview-header,
    .section-heading {
      align-items: stretch;
      flex-direction: column;
    }

    .preview-actions {
      width: 100%;
      justify-content: flex-start;
      flex-wrap: wrap;
    }

    .preview-filter-controls {
      grid-template-columns: 1fr;
    }

    .preview-filter-controls .wa-control,
    .preview-filter-controls .wa-admin-action {
      width: 100%;
      min-height: var(--wa-touch-h);
    }

    .preview-filter-meta {
      align-items: flex-start;
      flex-direction: column;
    }

    .preview-header dl {
      justify-content: space-between;
      gap: var(--wa-space-3);
    }

    .section-heading > span {
      white-space: normal;
    }

    .state-grid {
      grid-template-columns: 1fr;
    }
  }
</style>
